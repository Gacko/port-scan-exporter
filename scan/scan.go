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
	Concurrency int
	Timeout     time.Duration
}

type Scanner struct {
	config Config
	client *kubernetes.Clientset
}

// pods gets selected pods.
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
		// Ignore host network and self.
		if !pod.Spec.HostNetwork && !(pod.Name == podName && pod.Namespace == podNamespace) {
			// Append pod.
			pods = append(pods, pod)
		}
	}

	// Return pods.
	return pods, nil
}

// connect connects to an address by network, IP and port.
func (scanner *Scanner) connect(network string, ip string, port int) {
	// Concatenate IP and port.
	address := fmt.Sprintf("%v:%d", ip, port)

	// Connect to address.
	connection, err := net.DialTimeout(network, address, scanner.config.Timeout)
	if err != nil {
		//log.Print(err)
		return
	}

	// Close connection.
	if err := connection.Close(); err != nil {
		log.Print(err)
		return
	}

	// Log success.
	log.Printf("connect: %v/%v", network, address)
}

// scan runs a scan.
func (scanner *Scanner) scan() {
	// Get pods.
	pods, err := scanner.pods()
	if err != nil {
		log.Print(err)
		return
	}

	// Initialize concurrency pool and wait group.
	pool := make(chan bool, scanner.config.Concurrency)
	wait := sync.WaitGroup{}

	// Define concurrent connect function.
	connect := func(network string, ip string, port int) {
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
		log.Printf("(pod) name: %v, namespace: %v, IP: %v", pod.Name, pod.Namespace, ip)

		// Iterate networks.
		for _, network := range []string{"tcp"} {
			// Iterate ports.
			for port := 1; port <= 65535; port++ {
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
			// Run scan.
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
