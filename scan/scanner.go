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
	"sync"
	"time"
)

type Config struct {
	Interval    time.Duration
	Concurrency uint
	Timeout     time.Duration
}

type Port struct {
	Pod      *core.Pod
	Protocol string
	Port     uint
}

type Scanner struct {
	config Config
	client *kubernetes.Clientset
	ports  []Port
}

// Ports returns a copy of ports.
func (scanner *Scanner) Ports() []Port {
	// Return ports.
	return scanner.ports
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
func (scanner *Scanner) connect(pod *core.Pod, protocol string, port uint) error {
	// Concatenate IP and port.
	address := fmt.Sprintf("%v:%d", pod.Status.PodIP, port)

	// Connect to address.
	connection, err := net.DialTimeout(protocol, address, scanner.config.Timeout)
	if err != nil {
		// Return error.
		return err
	}

	// Close connection.
	//goland:noinspection GoUnhandledErrorResult
	defer connection.Close()

	// Return success.
	return nil
}

// scan runs a scan.
func (scanner *Scanner) scan() {
	log.Printf("before scan: %d ports", len(scanner.ports))

	// Get filtered pods.
	pods, err := scanner.pods()
	if err != nil {
		log.Print(err)
		return
	}

	// Initialize ports, port wait and port channel.
	var ports []Port
	portWait := sync.WaitGroup{}
	portChannel := make(chan Port, scanner.config.Concurrency)

	// Add port wait.
	portWait.Add(1)
	// Concurrently receive ports.
	go func() {
		// Remove port wait.
		defer portWait.Done()

		// Receive ports.
		for port := range portChannel {
			// Add port.
			ports = append(ports, port)
		}
	}()

	// Initialize connection wait and connection pool.
	connectionWait := sync.WaitGroup{}
	connectionPool := make(chan bool, scanner.config.Concurrency)

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
					if err := scanner.connect(pod, protocol, port); err == nil {
						// Log pod, protocol and port.
						log.Printf("%v/%v %v %v/%d", pod.Namespace, pod.Name, pod.Status.PodIP, protocol, port)
						portChannel <- Port{
							Pod:      pod,
							Protocol: protocol,
							Port:     port,
						}
					}
				}(&pod, protocol, port)
			}
		}
	}

	// Wait for connections.
	connectionWait.Wait()

	// Close port channel and wait for ports.
	close(portChannel)
	portWait.Wait()

	// Set ports.
	scanner.ports = ports

	log.Printf("after scan: %d ports", len(scanner.ports))
}

// run runs periodic scans.
func (scanner *Scanner) run() {
	// Create ticker.
	ticker := time.NewTicker(scanner.config.Interval)

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
func New(config Config, client *kubernetes.Clientset) *Scanner {
	// Create scanner.
	scanner := &Scanner{
		config: config,
		client: client,
	}

	// Run periodic scans.
	go scanner.run()

	// Return scanner.
	return scanner
}
