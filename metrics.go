package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	podScaleEvent = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "puppeteer",
		Name:      "pod_scale_event",
		Help:      "Pod Scale Up/Down Event Gauge",
	},
		[]string{"service", "status"},
	)

	serviceRestart = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "puppeteer",
			Name:      "service_restart",
			Help:      "Service Restart Process Count",
		},
		[]string{"service", "status"},
	)

	promASGcount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "puppeteer",
			Name:      "asg_count",
			Help:      "AutoScaler Group Current Desired Count",
		},
		[]string{"name"},
	)

	promASGscale = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "puppeteer",
			Name:      "asg_scale_event",
			Help:      "AutoScaler Group Scale Event",
		},
		[]string{"name"},
	)
)
