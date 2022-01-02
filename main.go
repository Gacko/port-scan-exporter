package main

import (
	"flag"
	"github.com/gacko/port-scan-exporter/health"
	"github.com/gacko/port-scan-exporter/scan"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
	"time"
)

var (
	interval    time.Duration
	concurrency uint
	timeout     time.Duration
	age         time.Duration
	listen      string
)

func init() {
	// Setup arguments.
	flag.DurationVar(&interval, "interval", time.Minute, "Interval at which scans are performed.")
	flag.UintVar(&concurrency, "concurrency", 1024, "Number of parallel connection attempts.")
	flag.DurationVar(&timeout, "timeout", time.Second, "Timeout of connection attempts.")
	flag.DurationVar(&age, "age", 10*time.Minute, "Maximum age of last scan.")
	flag.StringVar(&listen, "listen", ":8000", "Listen address of the exporter.")
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

	// Create scanner.
	scanner := scan.NewScanner(client, interval, concurrency, timeout)

	// Create and register collector.
	collector := scan.NewCollector(scanner)
	prometheus.MustRegister(collector)

	// Register paths.
	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/healthz", health.Handler(scanner, age))

	// Start HTTP server.
	log.Printf("listening on %v", listen)
	if err = http.ListenAndServe(listen, nil); err != nil {
		log.Fatal(err)
	}
}
