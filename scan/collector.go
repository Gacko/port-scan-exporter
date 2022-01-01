package scan

import (
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
)

const (
	Namespace = "port_scan"
)

type Collector struct {
	scanner *Scanner
	pods    *prometheus.Desc
	ports   *prometheus.Desc
	open    *prometheus.Desc
	took    *prometheus.Desc
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

	// Create open ports description.
	open := prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "open_ports"),
		"Open ports by pod, namespace, IP, node, protocol and port",
		[]string{"pod", "namespace", "ip", "node", "protocol", "port"},
		nil,
	)

	// Create took description.
	took := prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "took"),
		"Duration by pod, namespace, IP and node.",
		[]string{"pod", "namespace", "ip", "node"},
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
		open:    open,
		took:    took,
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
	channel <- collector.open
	channel <- collector.took
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
		// Get pod, namespace, IP, node and took.
		pod := scan.Pod.Name
		namespace := scan.Pod.Namespace
		ip := scan.Pod.Status.PodIP
		node := scan.Pod.Spec.NodeName
		took := scan.Took.Seconds()

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

			// Check state.
			if state == StateOpen {
				// Send open ports metric.
				channel <- prometheus.MustNewConstMetric(
					collector.open,
					prometheus.GaugeValue,
					1,
					pod, namespace, ip, node, protocol, strconv.Itoa(int(port.Port)),
				)
			}
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

		// Initialize buckets.
		buckets := map[float64]uint64{
			0.5:  0,
			1.0:  0,
			2.0:  0,
			4.0:  0,
			8.0:  0,
			16.0: 0,
		}

		// Iterate buckets.
		for bucket := range buckets {
			// Check took.
			if took <= bucket {
				// Increase bucket.
				buckets[bucket]++
			}
		}

		// Send took metric.
		channel <- prometheus.MustNewConstHistogram(
			collector.took,
			1,
			took,
			buckets,
			pod, namespace, ip, node,
		)
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
