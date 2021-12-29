package scan

import (
	"context"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"log"
	"time"
)

// Scanner contains all information required for scans.
type Scanner struct {
	ticker *time.Ticker
	client *kubernetes.Clientset
}

// scan runs a scan.
func (scanner *Scanner) scan() {
	// Define context.
	ctx := context.Background()

	// Get pods.
	pods, err := scanner.client.CoreV1().Pods("").List(ctx, meta.ListOptions{})
	if err != nil {
		log.Print(err)
		return
	}

	// Iterate pods.
	for _, pod := range pods.Items {
		// Print pod.
		log.Print(pod)
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

	// Receive ticks.
	go scanner.receive()

	// Return scanner.
	return scanner
}
