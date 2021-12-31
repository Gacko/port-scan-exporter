package scan

import (
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

const (
	Namespace = "port_scan"
)

var (
	States = map[byte]string{
		StateOpen:   "open",
		StateClosed: "closed",
		StateError:  "error",
	}
)

type Collector struct {
	scanner *Scanner
	ports   *prometheus.Desc
	age     *prometheus.Desc
}

// NewCollector creates a collector.
func NewCollector(scanner *Scanner) *Collector {
	// Create collector.
	collector := &Collector{
		scanner: scanner,
		ports: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "ports"),
			"Number of ports by pod, namespace, IP, node, protocol and state.",
			[]string{"pod", "namespace", "ip", "node", "protocol", "state"},
			nil,
		),
		age: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "age"),
			"Age of last scan in seconds.",
			nil,
			nil,
		),
	}

	// Return collector.
	return collector
}

// Describe implements the Describe method of prometheus.Collector.
func (collector *Collector) Describe(channel chan<- *prometheus.Desc) {
	// Send descriptions.
	channel <- collector.ports
	channel <- collector.age
}

// Collect implements the Collect method of prometheus.Collector.
func (collector *Collector) Collect(channel chan<- prometheus.Metric) {
	// Get scans.
	scans := collector.scanner.scans

	// Iterate scans.
	for _, scan := range scans {
		// Get pod.
		pod := scan.Pod
		name := pod.Name
		namespace := pod.Namespace
		ip := pod.Status.PodIP
		node := pod.Spec.NodeName

		// Get ports and initialize protocols.
		ports := scan.Ports
		var protocols = make(map[string]map[byte]uint16)

		// Iterate ports.
		for _, port := range ports {
			// Get protocol and state.
			protocol := port.Protocol
			state := port.State

			// Check if protocol exists.
			if protocols[protocol] == nil {
				// Define protocol.
				protocols[protocol] = make(map[byte]uint16)
			}

			// Increase counter.
			protocols[protocol][state]++
		}

		// Iterate states by protocol.
		for protocol, states := range protocols {
			// Iterate counters by state.
			for state, counter := range states {
				// Send port metric.
				channel <- prometheus.MustNewConstMetric(
					collector.ports,
					prometheus.GaugeValue,
					float64(counter),
					name, namespace, ip, node, protocol, States[state],
				)
			}
		}
	}

	// Calculate age of last scan.
	age := time.Since(collector.scanner.last).Seconds()

	// Send age metric.
	channel <- prometheus.MustNewConstMetric(collector.age, prometheus.GaugeValue, age)
}
