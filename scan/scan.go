package scan

import (
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
	log.Println("scan")
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
