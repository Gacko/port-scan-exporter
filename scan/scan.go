package scan

import (
	"log"
	"time"
)

// receive receives ticks and starts scans.
func receive(ticker *time.Ticker) {
	// Receive ticks.
	for {
		select {
		case tick := <-ticker.C:
			// Start scan.
			log.Println(tick)
		}
	}
}

// Schedule schedules scans.
func Schedule() {
	// Schedule scan.
	ticker := time.NewTicker(10 * time.Second)

	// Receive ticks.
	go receive(ticker)
}
