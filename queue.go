package main

import (
	"net/url"
	"os"
	"time"

	rabbithole "github.com/michaelklishin/rabbit-hole"
	"github.com/prometheus/client_golang/prometheus"

	deisps "github.com/deis/controller-sdk-go/ps"
)

func queueRunner() {
	queues := cfg.Queues
	for _, queue := range queues {
		switch method := queue.Method; method {
		case "scale":
			go scalePods(queue)
		case "restart":
			go restartPods(queue)
		default:
			Info.Println("Queue Missing/Invalid Method")
		}
	}
}

func amqConnection(q Queue) *rabbithole.Client {
	amqURL := os.Getenv(q.AmqHost)
	if amqURL == "" {
		Warning.Panic("Missing AMQ Environment Variable")
	}
	u, err := url.Parse(amqURL)
	if err != nil {
		Warning.Panic(err)
	}

	password, _ := u.User.Password()
	rmqc, _ := rabbithole.NewClient(u.Scheme+"://"+u.Host, u.User.Username(), password)

	return rmqc
}

func restartPods(q Queue) {
	// Setting Last Restart to Now
	lastRestart := time.Now().Add(-10 * time.Minute)
	rmqc := amqConnection(q)
	for {
		mq, err := rmqc.GetQueue("/", q.Queue)
		if err != nil {
			Warning.Printf("Error : %s", err)
		}
		Info.Printf("%s = %d\n", q.Queue, mq.Messages)

		if mq.Messages > q.Threshold {
			// Wait 10 Minutes between restarts
			now := time.Now()
			delayInterval := lastRestart.Add(10 * time.Minute)
			if now.After(delayInterval) {
				Info.Printf("Queue is over threshold, restarting app %s-%s\n", q.DeisApp, q.Worker)
				lastRestart = time.Now()
				deisRestart(q.DeisApp, q.Worker)
			} else {
				Info.Printf("Queue is over threshold, but within cool down period for app %s-%s\n", q.DeisApp, q.Worker)
			}

		}
		time.Sleep(101 * time.Second)
	}
}

func scalePods(q Queue) {
	rmqc := amqConnection(q)
	for {
		// Need to handle timeouts...
		mq, err := rmqc.GetQueue("/", q.Queue)
		if err != nil {
			Warning.Printf("Error : %s", err)
		}
		Info.Printf("%s = %d\n", q.Queue, mq.Messages)
		podcount := deisPodCount(q.DeisApp, q.Worker)
		podScaleEvent.With(prometheus.Labels{"service": q.DeisApp + "-" + q.Worker, "status": "success"}).Set(float64(podcount))

		if mq.Messages >= q.Threshold {
			desiredPodCount := podcount + q.ScaleBy
			if podcount < q.ScaleMin || podcount > q.ScaleMax {
				Info.Printf("%s is outside of Min/Max Pod Settings, Change back inside range %d - %d", q.DeisApp+"-"+q.Worker, q.ScaleMin, q.ScaleMax)
			} else if desiredPodCount > q.ScaleMax {
				Info.Printf("%s over threshold, scaling to Maximum %d\n", q.DeisApp+"-"+q.Worker, q.ScaleMax)
				deisScale(q.DeisApp, q.Worker, q.ScaleMax)
			} else {
				Info.Printf("%s over threshold, scaling to %d from %d(+%d)\n", q.DeisApp+"-"+q.Worker, desiredPodCount, podcount, q.ScaleBy)
				deisScale(q.DeisApp, q.Worker, desiredPodCount)
			}
		}
		if mq.Messages < q.Watermark {
			if podcount == q.ScaleMin {
				Info.Printf("%s is at minimum(%d) Pods defined by ScaleMin", q.DeisApp+"-"+q.Worker, podcount)
			} else if podcount < q.ScaleMin || podcount > q.ScaleMax {
				Info.Printf("%s is outside of Min/Max Pod Settings, Change back inside range %d - %d", q.DeisApp+"-"+q.Worker, q.ScaleMin, q.ScaleMax)
			} else if podcount > q.ScaleMin {
				desiredPodCount := podcount - 1
				Info.Printf("%s under Watermark, scale down to %d from %d(-1)\n", q.DeisApp+"-"+q.Worker, desiredPodCount, podcount)
				deisScale(q.DeisApp, q.Worker, desiredPodCount)
			}
		}

		time.Sleep(95 * time.Second)
	}
}

func deisPodCount(app string, worker string) int {
	// Verify SSL, Controller URL, API Token
	podlist, _, err := deisps.List(deiscfg.Client, app, 0)
	if err != nil {
		Warning.Panic(err)
	}
	var workercount int
	podtypes := deisps.ByType(podlist)
	for _, pt := range podtypes {
		if pt.Type == worker {
			workercount = pt.PodsList.Len()
		}
	}
	return workercount
}

func deisScale(app string, worker string, desired int) {
	if cfg.Enabled {
		targets := make(map[string]int)
		targets[worker] = desired
		err := deisps.Scale(deiscfg.Client, app, targets)
		if err != nil {
			podScaleEvent.With(prometheus.Labels{"service": app + "-" + worker, "status": "failed"}).Set(float64(desired))
			Info.Println("Error: Deis unable to process scale event", err)
		} else {
			workerCount := deisPodCount(app, worker)
			podScaleEvent.With(prometheus.Labels{"service": app + "-" + worker, "status": "success"}).Set(float64(desired))
			Info.Printf("%s pod count now %d\n", app+"-"+worker, workerCount)
		}
	}
}

func deisRestart(app string, worker string) {
	if cfg.Enabled {
		_, err := deisps.Restart(deiscfg.Client, app, worker, "")
		if err != nil {
			serviceRestart.With(prometheus.Labels{"service": app + "-" + worker, "status": "failed"}).Inc()
			Info.Println("Deis unable to restart process event", err)
		} else {
			serviceRestart.With(prometheus.Labels{"service": app + "-" + worker, "status": "success"}).Inc()
			Info.Printf("%s Services Restarted\n", app+"-"+worker)
		}
	}
}
