package scan

import (
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
)

const (
	Namespace = "port_scan"
)

type Collector struct {
	scanner       *Scanner
	pods          *prometheus.Desc
	ports         *prometheus.Desc
	openPorts     *prometheus.Desc
	took          *prometheus.Desc
	tookHistogram *prometheus.Desc
	age           *prometheus.Desc
}

// NewCollector creates a collector.
func NewCollector(scanner *Scanner) *Collector {
	// Create pods description.
	pods := prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "pods"),
		"Number of scanned pods by namespace and node.",
		[]string{"pod_namespace", "pod_node"},
		nil,
	)

	// Create ports description.
	ports := prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "ports"),
		"Number of scanned ports by pod, namespace, IP, node, protocol and state.",
		[]string{"pod_name", "pod_namespace", "pod_ip", "pod_node", "port_protocol", "port_state"},
		nil,
	)

	// Create open ports description.
	openPorts := prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "open_ports"),
		"HACK: Number of open ports by pod, namespace, IP, node, protocol and port.",
		[]string{"pod_name", "pod_namespace", "pod_ip", "pod_node", "port_protocol", "port_number"},
		nil,
	)

	// Create took description.
	took := prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "took_seconds"),
		"Scan duration in seconds by pod, namespace, IP and node.",
		[]string{"pod_name", "pod_namespace", "pod_ip", "pod_node"},
		nil,
	)

	// Create took histogram description.
	tookHistogram := prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "took_seconds"),
		"Scan duration in seconds by namespace and node.",
		[]string{"pod_namespace", "pod_node"},
		nil,
	)

	// Create age description.
	age := prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "age_seconds"),
		"Age of last scan in seconds.",
		nil,
		nil,
	)

	// Create collector.
	collector := &Collector{
		scanner:       scanner,
		pods:          pods,
		ports:         ports,
		openPorts:     openPorts,
		took:          took,
		tookHistogram: tookHistogram,
		age:           age,
	}

	// Return collector.
	return collector
}

// Describe implements the Describe method of prometheus.Collector.
func (collector *Collector) Describe(channel chan<- *prometheus.Desc) {
	// Send descriptions.
	channel <- collector.pods
	channel <- collector.ports
	channel <- collector.openPorts
	channel <- collector.took
	channel <- collector.tookHistogram
	channel <- collector.age
}

// Collect implements the Collect method of prometheus.Collector.
func (collector *Collector) Collect(channel chan<- prometheus.Metric) {
	// Get scans.
	scans := collector.scanner.scans

	// Initialize pods and took histogram.
	var pods = make(map[string]map[string]uint64)
	var tookHistogram = make(map[string]map[string][]float64)

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
			pods[namespace] = make(map[string]uint64)
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
					collector.openPorts,
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

		// Send took metric.
		channel <- prometheus.MustNewConstMetric(
			collector.took,
			prometheus.GaugeValue,
			took,
			pod, namespace, ip, node,
		)

		// Check if namespace exists.
		if tookHistogram[namespace] == nil {
			// Define namespace.
			tookHistogram[namespace] = make(map[string][]float64)
		}

		// Add took.
		tookHistogram[namespace][node] = append(tookHistogram[namespace][node], took)
	}

	// Iterate pods.
	for namespace, nodes := range pods {
		// Iterate nodes.
		for node, counter := range nodes {
			// Send pods metric.
			channel <- prometheus.MustNewConstMetric(
				collector.pods,
				prometheus.GaugeValue,
				float64(counter),
				namespace, node,
			)
		}
	}

	// Iterate took histogram.
	for namespace, nodes := range tookHistogram {
		// Iterate nodes.
		for node, tooks := range nodes {
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

			// Iterate tooks.
			for _, took := range tooks {
				// Iterate buckets.
				for bucket := range buckets {
					// Check took.
					if took <= bucket {
						// Increase bucket.
						buckets[bucket]++
					}
				}
				// Add sum.
				sum += took
			}

			// Send took histogram metric.
			channel <- prometheus.MustNewConstHistogram(
				collector.tookHistogram,
				uint64(len(tooks)),
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
