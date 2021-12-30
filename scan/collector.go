package scan

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	Namespace = "port_scan"
)

type Collector struct {
	scanner *Scanner
	pods    *prometheus.Desc
	ports   *prometheus.Desc
	took    *prometheus.Desc
	age     *prometheus.Desc
}

// Describe implements the Describe method of prometheus.Collector.
func (collector *Collector) Describe(channel chan<- *prometheus.Desc) {
	// Send descriptions.
	channel <- collector.pods
	channel <- collector.ports
	channel <- collector.took
	channel <- collector.age
}

// Collect implements the Collect method of prometheus.Collector.
func (collector *Collector) Collect(channel chan<- prometheus.Metric) {
	// Get last scan.
	scan := collector.scanner.Last()

	// Send pod metric.
	channel <- prometheus.MustNewConstMetric(collector.pods, prometheus.GaugeValue, float64(len(scan.Pods)))

	// Send port metric.
	channel <- prometheus.MustNewConstMetric(collector.ports, prometheus.GaugeValue, float64(scan.Open), PortOpen)
	channel <- prometheus.MustNewConstMetric(collector.ports, prometheus.GaugeValue, float64(scan.Closed), PortClosed)
	channel <- prometheus.MustNewConstMetric(collector.ports, prometheus.GaugeValue, float64(scan.Errors), PortError)

	// Send took metric.
	channel <- prometheus.MustNewConstMetric(collector.took, prometheus.GaugeValue, scan.Took.Seconds())

	// Send age metric.
	channel <- prometheus.MustNewConstMetric(collector.age, prometheus.GaugeValue, scan.Age().Seconds())
}

// NewCollector creates a collector.
func NewCollector(scanner *Scanner) *Collector {
	// Create collector.
	collector := &Collector{
		scanner: scanner,
		pods: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "pods"),
			"Number of scanned pods.",
			nil,
			nil,
		),
		ports: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "ports"),
			"Number of scanned ports by state.",
			[]string{"state"},
			nil,
		),
		took: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "took"),
			"Duration of the last scan in seconds.",
			nil,
			nil,
		),
		age: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "age"),
			"Age of the last scan in seconds.",
			nil,
			nil,
		),
	}

	// Return collector.
	return collector
}
