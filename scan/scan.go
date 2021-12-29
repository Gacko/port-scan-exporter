package scan

import (
	"log"
	"time"
)

// Scanner contains all information required for scans.
type Scanner struct {
	ticker *time.Ticker
}

// receive receives ticks and starts scans.
func (scanner *Scanner) receive() {
	// Receive ticks.
	for {
		select {
		case tick := <-scanner.ticker.C:
			// Start scan.
			log.Println(tick)
		}
	}
}

// Schedule schedules scans.
func Schedule(interval time.Duration) *Scanner {
	// Create scanner.
	scanner := &Scanner{
		ticker: time.NewTicker(interval),
	}

	// Receive ticks.
	go scanner.receive()

	// Return scanner.
	return scanner
}
