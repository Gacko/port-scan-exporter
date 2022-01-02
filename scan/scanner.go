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

type Scanner struct {
	client      *kubernetes.Clientset
	interval    time.Duration
	concurrency uint
	timeout     time.Duration
	scans       []Scan
	last        time.Time
}

type Scan struct {
	Pod   core.Pod
	Ports []Port
	Took  time.Duration
}

type Port struct {
	Protocol string
	Port     uint16
	State    string
}

const (
	ProtocolTCP = "tcp"
	StateOpen   = "open"
	StateClosed = "closed"
	StateError  = "error"
)

// NewScanner creates a scanner & runs periodic scans.
func NewScanner(client *kubernetes.Clientset, interval time.Duration, concurrency uint, timeout time.Duration) *Scanner {
	// Create scanner.
	scanner := &Scanner{
		client:      client,
		interval:    interval,
		concurrency: concurrency,
		timeout:     timeout,
		last:        time.Now(),
	}

	// Run periodic scans.
	go scanner.run()

	// Return scanner.
	return scanner
}

// Age calculates age of last scan.
func (scanner *Scanner) Age() time.Duration {
	// Calculate and return age of last scan.
	return time.Since(scanner.last)
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

// scan runs a scan.
func (scanner *Scanner) scan() {
	log.Print("scan: starting scan")

	// Get pods.
	pods, err := scanner.pods()
	if err != nil {
		log.Print(err)
		return
	}

	// Initialize scans.
	var scans []Scan

	// Iterate pods.
	for _, pod := range pods {
		// Get time.
		start := time.Now()

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
				// Add port.
				ports = append(ports, port)
			}
		}()

		// Initialize connection wait and connection pool.
		connectionWait := sync.WaitGroup{}
		connectionPool := make(chan bool, scanner.concurrency)

		// Iterate protocols.
		for _, protocol := range []string{ProtocolTCP} {
			// Iterate ports.
			for port := uint16(1); port >= uint16(1) && port <= uint16(65535); port++ {
				// Add connection wait and obtain connection slot.
				connectionWait.Add(1)
				connectionPool <- true

				// Concurrently connect to address by IP, protocol and port.
				go func(ip string, protocol string, port uint16) {
					// Free connection slot and remove connection wait.
					defer func() { <-connectionPool }()
					defer connectionWait.Done()

					// Connect to address by IP, protocol and port.
					portChannel <- scanner.connect(ip, protocol, port)
				}(pod.Status.PodIP, protocol, port)
			}

		}

		// Wait for connections.
		connectionWait.Wait()

		// Close port channel and wait for ports.
		close(portChannel)
		portWait.Wait()

		// Get took.
		took := time.Since(start)

		// Create scan.
		scan := Scan{
			Pod:   pod,
			Ports: ports,
			Took:  took,
		}

		// Add scan.
		scans = append(scans, scan)
	}

	// Set scans and time.
	scanner.scans = scans
	scanner.last = time.Now()

	log.Printf("scan: finished scanning %d pods", len(pods))
}

// pods gets filtered pods.
func (scanner *Scanner) pods() ([]core.Pod, error) {
	// Get node name.
	nodeName := os.Getenv("PORT_SCAN_EXPORTER_NODE_NAME")

	// Define context and options.
	ctx := context.Background()
	options := meta.ListOptions{
		// Select pods of the own node.
		FieldSelector: fmt.Sprintf("spec.nodeName=%v", nodeName),
	}

	// Get pods of the own node.
	nodePods, err := scanner.client.CoreV1().Pods("").List(ctx, options)
	if err != nil {
		return []core.Pod{}, err
	}

	// Initialize pods.
	var pods []core.Pod

	// Filter pods.
	for _, pod := range nodePods.Items {
		// Ignore host network and non-running.
		if pod.Spec.HostNetwork || pod.Status.Phase != core.PodRunning {
			continue
		}

		// Append pod.
		pods = append(pods, pod)
	}

	// Return pods.
	return pods, nil
}

// connect connects to an address by IP, protocol and port.
//
// Packets sent to open ports are usually discarded if they do not comply with the respective protocol.
// Packets sent to closed ports did not receive ICMP Port Unreachable responses in my tests.
// The result is therefore ambiguous and scanning an UDP port is difficult or even impossible.
//
// https://www.rawhex.com/blog/2016/04/12/the-ultimate-portscanning-guide-part1-theory
// https://www.rawhex.com/blog/2016/05/26/the-ultimate-portscanning-guide-part2-practical-tcp-port-scans
// https://www.rawhex.com/blog/2016/07/05/the-ultimate-portscanning-guide-part3-udp-port-scans
//
func (scanner *Scanner) connect(ip string, protocol string, port uint16) Port {
	// Initialize state.
	var state string

	// Concatenate and connect to address.
	address := fmt.Sprintf("%v:%d", ip, port)
	connection, err := net.DialTimeout(protocol, address, scanner.timeout)
	if err == nil {
		// Close connection.
		//goland:noinspection GoUnhandledErrorResult
		defer connection.Close()
		// Set state open.
		state = StateOpen
	} else if strings.Contains(err.Error(), "connection refused") {
		// Set state closed.
		state = StateClosed
	} else {
		// Set state error.
		state = StateError
	}

	// Return port.
	return Port{
		Protocol: protocol,
		Port:     port,
		State:    state,
	}
}
