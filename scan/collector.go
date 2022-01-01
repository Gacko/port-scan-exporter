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
	age     *prometheus.Desc
}

// NewCollector creates a collector.
func NewCollector(scanner *Scanner) *Collector {
	// Create pods description.
	pods := prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "pods"),
		"Number of pods by namespace and node",
		[]string{"namespace", "node"},
		nil,
	)

	// Create ports description.
	ports := prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "ports"),
		"Number of ports by pod, namespace, IP, node, protocol and state.",
		[]string{"pod", "namespace", "ip", "node", "protocol", "state"},
		nil,
	)

	// Create age description.
	age := prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "age"),
		"Age of last scan in seconds.",
		nil,
		nil,
	)

	// Create collector.
	collector := &Collector{
		scanner: scanner,
		pods:    pods,
		ports:   ports,
		age:     age,
	}

	// Return collector.
	return collector
}

// Describe implements the Describe method of prometheus.Collector.
func (collector *Collector) Describe(channel chan<- *prometheus.Desc) {
	// Send descriptions.
	channel <- collector.pods
	channel <- collector.ports
	channel <- collector.age
}

// Collect implements the Collect method of prometheus.Collector.
func (collector *Collector) Collect(channel chan<- prometheus.Metric) {
	// Get scans.
	scans := collector.scanner.scans

	// Initialize pods.
	var pods = make(map[string]map[string]uint)

	// Iterate scans.
	for _, scan := range scans {
		// Get pod, namespace, IP and node.
		pod := scan.Pod.Name
		namespace := scan.Pod.Namespace
		ip := scan.Pod.Status.PodIP
		node := scan.Pod.Spec.NodeName

		// Check if namespace exists.
		if pods[namespace] == nil {
			// Define namespace.
			pods[namespace] = make(map[string]uint)
		}

		// Increase pods.
		pods[namespace][node]++

		// Initialize ports.
		var ports = make(map[string]map[string]uint16)

		// Iterate ports.
		for _, port := range scan.Ports {
			// Get protocol and state.
			protocol := port.Protocol
			state := port.State

			// Check if protocol exists.
			if ports[protocol] == nil {
				// Define protocol.
				ports[protocol] = make(map[string]uint16)
			}

			// Increase ports.
			ports[protocol][state]++
		}

		// Iterate ports.
		for protocol, states := range ports {
			// Iterate states.
			for state, counter := range states {
				// Send port metric.
				channel <- prometheus.MustNewConstMetric(
					collector.ports,
					prometheus.GaugeValue,
					float64(counter),
					pod, namespace, ip, node, protocol, state,
				)
			}
		}
	}

	// Iterate pods.
	for namespace, nodes := range pods {
		// Iterate nodes.
		for node, counter := range nodes {
			// Send pod metric.
			channel <- prometheus.MustNewConstMetric(
				collector.pods,
				prometheus.GaugeValue,
				float64(counter),
				namespace, node,
			)
		}
	}

	// Get age of last scan in seconds.
	age := collector.scanner.Age().Seconds()

	// Send age metric.
	channel <- prometheus.MustNewConstMetric(collector.age, prometheus.GaugeValue, age)
}
