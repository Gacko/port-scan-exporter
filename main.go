package main

import (
	"flag"
	"github.com/gacko/port-scan-exporter/health"
	"github.com/gacko/port-scan-exporter/scan"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
)

var (
	listen string
)

func init() {
	// Setup arguments.
	flag.StringVar(&listen, "listen", ":8080", "Listen address of port-scan-exporter")
}

func main() {
	// Parse arguments.
	flag.Parse()

	// Schedule scans.
	scan.Schedule()

	// Register paths.
	http.Handle("/healthz", health.Handler())
	http.Handle("/metrics", promhttp.Handler())

	// Start HTTP server.
	log.Printf("listening on %v", listen)
	if err := http.ListenAndServe(listen, nil); err != nil {
		log.Println(err)
	}
}
