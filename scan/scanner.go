package scan

import (
	"context"
	"fmt"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Client      *kubernetes.Clientset
	Interval    time.Duration
	Concurrency uint
	Timeout     time.Duration
}

type Port struct {
	Pod      *core.Pod
	Protocol string
	Port     uint
	State    string
}

const (
	PortOpen   = "open"
	PortClosed = "closed"
	PortError  = "error"
)

type Scan struct {
	Pods   []core.Pod
	Ports  []Port
	Open   uint
	Closed uint
	Errors uint
	Took   time.Duration
}

// Total calculates the total amount of scanned ports.
func (scan *Scan) Total() uint {
	return scan.Open + scan.Closed + scan.Errors
}

// PodsPerSecond calculates the amount of scanned pods per second.
func (scan *Scan) PodsPerSecond() float64 {
	return float64(len(scan.Pods)) / scan.Took.Seconds()
}

// PortsPerSecond calculates the amount of scanned ports per second.
func (scan *Scan) PortsPerSecond() float64 {
	return float64(scan.Total()) / scan.Took.Seconds()
}

type Scanner struct {
	client      *kubernetes.Clientset
	interval    time.Duration
	concurrency uint
	timeout     time.Duration
	last        Scan
}

// Last returns the last scan.
func (scanner *Scanner) Last() Scan {
	// Return last scan.
	return scanner.last
}

// pods gets filtered pods.
func (scanner *Scanner) pods() ([]core.Pod, error) {
	// Get pod name, pod namespace and node name.
	podName := os.Getenv("PORT_SCAN_EXPORTER_POD_NAME")
	podNamespace := os.Getenv("PORT_SCAN_EXPORTER_POD_NAMESPACE")
	nodeName := os.Getenv("PORT_SCAN_EXPORTER_NODE_NAME")

	// Define context and options.
	ctx := context.Background()
	options := meta.ListOptions{
		// Select pods of the same node.
		FieldSelector: fmt.Sprintf("spec.nodeName=%v", nodeName),
	}

	// Get pods.
	allPods, err := scanner.client.CoreV1().Pods("").List(ctx, options)
	if err != nil {
		return []core.Pod{}, err
	}

	// Initialize pods.
	var pods []core.Pod

	// Filter pods.
	for _, pod := range allPods.Items {
		// Ignore self, host network and non-running.
		if pod.Name == podName && pod.Namespace == podNamespace || pod.Spec.HostNetwork || pod.Status.Phase != core.PodRunning {
			continue
		}

		// Append pod.
		pods = append(pods, pod)
	}

	// Return pods.
	return pods, nil
}

// connect connects to an address by pod, protocol and port.
func (scanner *Scanner) connect(pod *core.Pod, protocol string, port uint) Port {
	// Initialize state.
	var state string

	// Concatenate and connect to address.
	address := fmt.Sprintf("%v:%d", pod.Status.PodIP, port)
	connection, err := net.DialTimeout(protocol, address, scanner.timeout)
	if err == nil {
		// Close connection.
		//goland:noinspection GoUnhandledErrorResult
		defer connection.Close()
		// Set state open.
		state = PortOpen
	} else if strings.Contains(err.Error(), "connection refused") {
		// Set state closed.
		state = PortClosed
	} else {
		// Set state error.
		state = PortError
	}

	// Return port.
	return Port{
		Pod:      pod,
		Protocol: protocol,
		Port:     port,
		State:    state,
	}
}

// scan runs a scan.
func (scanner *Scanner) scan() {
	// Initialize measurements.
	var open uint
	var closed uint
	var errors uint
	start := time.Now()

	// Get pods.
	pods, err := scanner.pods()
	if err != nil {
		log.Print(err)
	}

	// Initialize ports, port wait and port channel.
	var ports []Port
	portWait := sync.WaitGroup{}
	portChannel := make(chan Port, scanner.concurrency)

	// Add port wait.
	portWait.Add(1)

	// Concurrently receive ports.
	go func() {
		// Remove port wait.
		defer portWait.Done()

		// Receive ports.
		for port := range portChannel {
			// Switch port state.
			switch port.State {
			case PortOpen:
				// Add port.
				// Move out of switch if you want to go out of memory.
				ports = append(ports, port)
				// Increase open counter.
				open++
			case PortClosed:
				// Increase closed counter.
				closed++
			case PortError:
				// Increase errors counter.
				errors++
			}
		}
	}()

	// Initialize connection wait and connection pool.
	connectionWait := sync.WaitGroup{}
	connectionPool := make(chan bool, scanner.concurrency)

	// Iterate pods.
	for _, pod := range pods {
		// Iterate protocols.
		for _, protocol := range []string{"tcp"} {
			// Iterate ports.
			for port := uint(1); port <= uint(65535); port++ {
				// Add connection wait and obtain connection slot.
				connectionWait.Add(1)
				connectionPool <- true

				// Concurrently connect to address by pod, protocol and port.
				go func(pod *core.Pod, protocol string, port uint) {
					// Free connection slot and remove connection wait.
					defer func() { <-connectionPool }()
					defer connectionWait.Done()

					// Connect to address by pod, protocol and port.
					portChannel <- scanner.connect(pod, protocol, port)
				}(&pod, protocol, port)
			}
		}
	}

	// Wait for connections.
	connectionWait.Wait()

	// Close port channel and wait for ports.
	close(portChannel)
	portWait.Wait()

	// Get took.
	took := time.Since(start)

	// Store scan.
	scanner.last = Scan{
		Pods:   pods,
		Ports:  ports,
		Open:   open,
		Closed: closed,
		Errors: errors,
		Took:   took,
	}

	// Log scan.
	log.Printf("scan: %d pods, %d ports, %d open, %d closed, %d errors, took %v", len(pods), len(ports), open, closed, errors, took)
	log.Printf("scan: %d total, %f pods/s, %f ports/s", scanner.last.Total(), scanner.last.PodsPerSecond(), scanner.last.PortsPerSecond())
}

// run runs periodic scans.
func (scanner *Scanner) run() {
	// Create ticker.
	ticker := time.NewTicker(scanner.interval)

	// Run initial scan.
	scanner.scan()

	// Receive ticks.
	for {
		select {
		case <-ticker.C:
			// Run periodic scan.
			scanner.scan()
		}
	}
}

// New creates a scanner & runs periodic scans.
func New(config Config) *Scanner {
	// Create scanner.
	scanner := &Scanner{
		client:      config.Client,
		interval:    config.Interval,
		concurrency: config.Concurrency,
		timeout:     config.Timeout,
	}

	// Run periodic scans.
	go scanner.run()

	// Return scanner.
	return scanner
}
