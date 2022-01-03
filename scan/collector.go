package scan

import (
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
)

const (
	Namespace = "port_scan"
)

type Collector struct {
	scanner   *Scanner
	pods      *prometheus.Desc
	ports     *prometheus.Desc
	openPorts *prometheus.Desc
	took      *prometheus.Desc
	tooks     *prometheus.Desc
	age       *prometheus.Desc
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

	// Create tooks description.
	tooks := prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "tooks"),
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
		scanner:   scanner,
		pods:      pods,
		ports:     ports,
		openPorts: openPorts,
		took:      took,
		tooks:     tooks,
		age:       age,
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
	channel <- collector.tooks
	channel <- collector.age
}

// Collect implements the Collect method of prometheus.Collector.
func (collector *Collector) Collect(channel chan<- prometheus.Metric) {
	// Get scans.
	scans := collector.scanner.scans

	// Initialize pods and tooks.
	var pods = make(map[string]map[string]uint64)
	var tooks = make(map[string]map[string][]float64)

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
		if tooks[namespace] == nil {
			// Define namespace.
			tooks[namespace] = make(map[string][]float64)
		}

		// Add took.
		tooks[namespace][node] = append(tooks[namespace][node], took)
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

	// Iterate tooks.
	for namespace, nodes := range tooks {
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
				// Iterate buckets.
				for bucket := range buckets {
					// Check duration.
					if duration <= bucket {
						// Increase bucket.
						buckets[bucket]++
					}
				}
				// Sum duration.
				sum += duration
			}

			// Send tooks metric.
			channel <- prometheus.MustNewConstHistogram(
				collector.tooks,
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
