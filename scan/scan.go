package scan

import (
	"context"
	"fmt"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"log"
	"os"
	"sync"
	"time"
)

// Scanner contains information required for scans.
type Scanner struct {
	ticker *time.Ticker
	client *kubernetes.Clientset
	mutex  *sync.Mutex
}

// scan runs a scan.
func (scanner *Scanner) scan() {
	// Synchronize access.
	scanner.mutex.Lock()
	defer scanner.mutex.Unlock()

	// Get pod name, pod namespace and node name.
	podName := os.Getenv("PORT_SCAN_EXPORTER_POD_NAME")
	podNamespace := os.Getenv("PORT_SCAN_EXPORTER_POD_NAMESPACE")
	nodeName := os.Getenv("PORT_SCAN_EXPORTER_NODE_NAME")
	log.Printf("(Self) Name: %v, Namespace: %v, Node: %v", podName, podNamespace, nodeName)

	// Define context and options.
	ctx := context.Background()
	options := meta.ListOptions{
		// Select pods of the same node.
		FieldSelector: fmt.Sprintf("spec.nodeName=%v", nodeName),
	}

	// Get pods.
	pods, err := scanner.client.CoreV1().Pods("").List(ctx, options)
	if err != nil {
		log.Print(err)
		return
	}

	// Iterate pods.
	for _, pod := range pods.Items {
		// Ignore host network and self.
		if pod.Spec.HostNetwork || pod.Name == podName && pod.Namespace == podNamespace {
			continue
		}

		// Print pod.
		log.Printf("(Pod) Name: %v, Namespace: %v, Node: %v", pod.Name, pod.Namespace, pod.Spec.NodeName)
	}
}

// receive receives ticks and starts scans.
func (scanner *Scanner) receive() {
	// Receive ticks.
	for {
		select {
		case <-scanner.ticker.C:
			// Start scan.
			scanner.scan()
		}
	}
}

// Schedule schedules scans.
func Schedule(interval time.Duration, client *kubernetes.Clientset) *Scanner {
	// Create scanner.
	scanner := &Scanner{
		ticker: time.NewTicker(interval),
		client: client,
	}

	// Start initial scan and receive ticks.
	go scanner.scan()
	go scanner.receive()

	// Return scanner.
	return scanner
}
