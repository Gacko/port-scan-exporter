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

type Scanner struct {
	config Config
	client *kubernetes.Clientset
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
		// Ignore non-running, host network and self.
		if pod.Status.Phase == core.PodRunning && !pod.Spec.HostNetwork && !(pod.Name == podName && pod.Namespace == podNamespace) {
			// Append pod.
			pods = append(pods, pod)
		}
	}

	// Return pods.
	return pods, nil
}

// connect connects to an address by network, IP and port.
func (scanner *Scanner) connect(network string, ip string, port uint) {
	// Concatenate IP and port.
	address := fmt.Sprintf("%v:%d", ip, port)

	// Connect to address.
	connection, err := net.DialTimeout(network, address, scanner.config.Timeout)
	if err != nil {
		//if err.Error() != fmt.Sprintf("dial %v %v: connect: connection refused", network, address) {
		//	log.Print(err)
		//}
		return
	}

	// Close connection.
	if err := connection.Close(); err != nil {
		log.Print(err)
	}

	// Log port.
	log.Printf("port: %v %v/%d", ip, network, port)
}

// scan runs a scan.
func (scanner *Scanner) scan() {
	// Get filtered pods.
	pods, err := scanner.pods()
	if err != nil {
		log.Print(err)
		return
	}

	// Initialize concurrency pool and wait group.
	pool := make(chan bool, scanner.config.Concurrency)
	wait := sync.WaitGroup{}

	// Define concurrent connect function.
	connect := func(network string, ip string, port uint) {
		// Remove wait and free concurrency slot.
		defer wait.Done()
		defer func() { <-pool }()
		// Connect to address by network, IP and port.
		scanner.connect(network, ip, port)
	}

	// Iterate pods.
	for _, pod := range pods {
		// Get pod IP.
		ip := pod.Status.PodIP

		// Log pod.
		log.Printf("pod: %v/%v (%v)", pod.Namespace, pod.Name, ip)

		// Iterate networks.
		for _, network := range []string{"tcp"} {
			// Iterate ports.
			for port := uint(1); port <= uint(65535); port++ {
				// Obtain concurrency slot and add wait.
				pool <- true
				wait.Add(1)
				// Concurrently connect to address by network, IP and port.
				go connect(network, ip, port)
			}
		}
	}

	// Wait for routines.
	wait.Wait()
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
