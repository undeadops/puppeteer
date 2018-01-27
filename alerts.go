package main

import (
	"io/ioutil"
	"net/http"
	"time"

	"github.com/buger/jsonparser"
)

func alertRunner() {
	alerts := cfg.Alerts
	for _, alert := range alerts {
		switch method := alert.Method; method {
		case "restartworker":
			go alertLookup(alert)
		default:
			Info.Println("Nothing to do")
		}
	}
}

func checkforRabbitStats(body []byte) (stats string) {
	//looks for an alert called RabbitMQStatZero if this is true mark stats as down so puppeteer doesn't restart pods
	stats = "up"
	jsonparser.ArrayEach(body, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		active, err := jsonparser.GetString(value, "labels", "alertname")
		if active == "RabbitMQStatsZero" {
			stats = "down"
		}
	}, "data")
	return stats
}

func alertLookup(a Alert) {
	for {
		resp, err := http.Get(a.AlertHost + "/api/v1/alerts/")
		if err != nil {
			Warning.Println(err)
		}
		body, _ := ioutil.ReadAll(resp.Body)
		stats := checkforRabbitStats(body)

		if stats == "down" {
			Warning.Println("Stat is down, not restarting")
			time.Sleep(150 * time.Second)
		} else {
			//loop over json returned from alertmanager API, drill down into data, labels, alertname
			jsonparser.ArrayEach(body, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
				active, err := jsonparser.GetString(value, "labels", "alertname")
				if err != nil {
					Warning.Println(err)
				}
				if active == a.Name {
					if cfg.Enabled {
						Info.Printf("Restarting %s %s", a.DeisApp, a.Worker)
						deisRestart(a.DeisApp, a.Worker)
						time.Sleep(120 * time.Second)
					} else {
						Warning.Println("Status disabled, not restarting")
					}
				}
			}, "data") //top level json that contains the list of alerts
			time.Sleep(60 * time.Second)
		}
	}
}
