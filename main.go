package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	yaml "gopkg.in/yaml.v1"

	deis "github.com/deis/controller-sdk-go"
	deisauth "github.com/deis/controller-sdk-go/auth"
)

var (
	// Info Logger
	Info *log.Logger
	// Warning Logger
	Warning *log.Logger
)

// Config File Settings
type Config struct {
	Enabled bool
	Queues  []Queue
	Alerts  []Alert
	ASG     []ASG
}

// Queue Monitoring Definitions
type Queue struct {
	Queue     string
	AmqHost   string
	Threshold int
	Watermark int
	ScaleBy   int
	ScaleMax  int
	ScaleMin  int
	Method    string
	DeisApp   string
	Worker    string
}

// Alert Prometheus Alert Definitions
type Alert struct {
	Name      string
	AlertHost string
	Method    string
	DeisApp   string
	Worker    string
}

// ASG (autoscale Group) Group Definitions
type ASG struct {
	AsGroupName     string
	AWSRegion       string
	Enabled         bool
	Queue           string
	AmqHost         string
	Threshold       int
	Watermark       int
	Method          string
	DisableCoolDown bool
}

// Deis Client and Token Definitions
type Deis struct {
	Client *deis.Client
	Token  string
}

// DeisCreds - Credentials for Login to Deis
type DeisCreds struct {
	Username string
	Password string
	URL      string
}

// LoadConfig - Read in Config file.
func LoadConfig() Config {
	puppetConfig := "./puppeteer.yml"
	if pconfenv := os.Getenv("PUPPET_CONFIG"); pconfenv != "" {
		puppetConfig = pconfenv
	}
	filename, _ := filepath.Abs(puppetConfig)
	yamlFile, err := ioutil.ReadFile(filename)
	if err != nil {
		Warning.Panic("Unable to Read Config file", err)
	}

	var config Config
	config.Enabled = false

	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		Warning.Panic("Unable to Unmarshal Config", err)
	}
	if os.Getenv("STATE") == "enabled" {
		config.Enabled = true
	}
	return config
}

var cfg = LoadConfig()
var deiscfg Deis

func deisAuth() {
	// TODO: This function needs refactoring...
	var creds DeisCreds
	err := envconfig.Process("DEIS", &creds)
	if err != nil {
		Warning.Panic(err.Error())
	}
	client, err := deis.New(true, creds.URL, "")
	if err != nil {
		Warning.Panic("Deis New Creds URL", err)
	}
	token, err := deisauth.Login(client, creds.Username, creds.Password)
	if err != nil {
		Warning.Panic("Deis Auth Login Failed", err)
	}
	// Set the client to use the retrieved token
	client.Token = token
	deiscfg.Client = client
	deiscfg.Token = token
}

// Init Logging
func Init(
	infoHandle io.Writer,
	warningHandle io.Writer) {

	Info = log.New(infoHandle,
		"INFO: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Warning = log.New(warningHandle,
		"WARNING: ",
		log.Ldate|log.Ltime|log.Lshortfile)
}

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(podScaleEvent)
	prometheus.MustRegister(serviceRestart)
	prometheus.MustRegister(promASGcount)
	prometheus.MustRegister(promASGscale)
}

func main() {
	// Initialize logging
	Init(os.Stdout, os.Stdout)
	// Auth to Deis, Env Vars
	deisAuth()
	// Start Queue Monitors
	go queueRunner()

	// Start Alert Runner
	go alertRunner()

	// Start ASG scaleRunner
	go scaleRunner()

	// TODO: Rework for more flexibility
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", Index)
	router.Handle("/metrics", promhttp.Handler())
	Warning.Fatal(http.ListenAndServe(":8080", router))
}

// Index Print Status of Service
func Index(w http.ResponseWriter, r *http.Request) {
	queues := cfg.Queues
	for _, queue := range queues {
		fmt.Fprintf(w, "%s-%s\n", queue.DeisApp, queue.Worker)
		fmt.Fprintln(w, "\tqueue: ", queue.Queue)
		fmt.Fprintln(w, "\tEnv: ", queue.AmqHost)
		fmt.Fprintln(w, "\tMethod: ", queue.Method)
		fmt.Fprintln(w, "\tThreshold: ", queue.Threshold)
		fmt.Fprintln(w, "\tWatermark: ", queue.Watermark)
		fmt.Fprintln(w, "\tScale By: ", queue.ScaleBy)
	}
}
