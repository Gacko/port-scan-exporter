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

	// Create took description.
	took := prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "took"),
		"Duration by namespace and node.",
		[]string{"namespace", "node"},
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
	channel <- collector.took
	channel <- collector.age
}

// Collect implements the Collect method of prometheus.Collector.
func (collector *Collector) Collect(channel chan<- prometheus.Metric) {
	// Get scans.
	scans := collector.scanner.scans

	// Initialize pods and took.
	var pods = make(map[string]map[string]uint)
	var took = make(map[string]map[string][]float64)

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

		// Check if namespace exists.
		if took[namespace] == nil {
			// Define namespace.
			took[namespace] = make(map[string][]float64)
		}

		// Append duration.
		took[namespace][node] = append(took[namespace][node], scan.Took.Seconds())
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

	// Iterate took.
	for namespace, nodes := range took {
		// Iterate nodes.
		for node, durations := range nodes {
			// Initialize sum and buckets.
			var sum float64
			buckets := map[float64]uint64{
				0.5:  0,
				1.0:  0,
				2.0:  0,
				4.0:  0,
				8.0:  0,
				16.0: 0,
			}

			// Iterate durations.
			for _, duration := range durations {
				// Add duration.
				sum += duration

				// Iterate buckets.
				for bucket := range buckets {
					// Check duration.
					if duration <= bucket {
						// Increase bucket.
						buckets[bucket]++
					}
				}
			}

			// Send took metric.
			channel <- prometheus.MustNewConstHistogram(
				collector.took,
				uint64(len(durations)),
				sum,
				buckets,
				namespace, node,
			)
		}
	}

	// Get age of last scan in seconds.
	age := collector.scanner.Age().Seconds()

	// Send age metric.
	channel <- prometheus.MustNewConstMetric(collector.age, prometheus.GaugeValue, age)
}
