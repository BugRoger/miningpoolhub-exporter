package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	NAMESPACE = "miningpoolhub"
	VERSION   = "latest"
)

var (
	info = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   NAMESPACE,
			Name:        "info",
			Help:        "Info about this exporter",
			ConstLabels: prometheus.Labels{"version": VERSION},
		},
	)

	balance = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: NAMESPACE,
			Name:      "balance",
			Help:      "Balances by coin, wallet and confirmation status",
		},
		[]string{"coin", "symbol", "wallet", "status", "currency"},
	)
)

type Exporter struct {
	APIKey       string
	FiatCurrency string
}

func main() {
	var (
		listenAddress = flag.String("web.listen-address", ":9401", "Address to listen on for web interface and telemetry.")
		metricsPath   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	)
	flag.Parse()

	prometheus.MustRegister(&Exporter{"12345678904a87a4f24a2c5da62bedc6d133dfe3967869279669645bae379985", "EUR"})

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>MiningPoolHub Exporter</title></head>
             <body>
             <h1>MiningPoolHub Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	fmt.Println("Starting HTTP server on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}

func (e *Exporter) Collect(metrics chan<- prometheus.Metric) {
}

func (e *Exporter) Describe(descs chan<- *prometheus.Desc) {
}
