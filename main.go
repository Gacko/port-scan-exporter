package main

import (
	"flag"
	"github.com/gacko/port-scan-exporter/health"
	"github.com/gacko/port-scan-exporter/scan"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
	"time"
)

var (
	interval time.Duration
	listen   string
)

func init() {
	// Setup arguments.
	flag.DurationVar(&interval, "interval", 10*time.Minute, "Interval at which scans are performed.")
	flag.StringVar(&listen, "listen", ":9882", "Listen address of the exporter.")
}

func main() {
	// Parse arguments.
	flag.Parse()

	// Create Kubernetes config.
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}

	// Create Kubernetes client.
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	// Schedule scans.
	scan.Schedule(interval, client)

	// Register paths.
	http.Handle("/healthz", health.Handler())
	http.Handle("/metrics", promhttp.Handler())

	// Start HTTP server.
	log.Printf("listening on %v", listen)
	if err = http.ListenAndServe(listen, nil); err != nil {
		log.Fatal(err)
	}
}
