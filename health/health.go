package health

import (
	"github.com/gacko/port-scan-exporter/scan"
	"net/http"
	"time"
)

// Health implements http.Handler for serving health status requests.
type Health struct {
	scanner *scan.Scanner
	age     time.Duration
}

// ServeHTTP serves health status requests.
func (health Health) ServeHTTP(writer http.ResponseWriter, _ *http.Request) {
	// Check age of last scan.
	if health.scanner.Age() < health.age {
		// Write headers.
		writer.WriteHeader(http.StatusOK)
	} else {
		// Write headers.
		writer.WriteHeader(http.StatusServiceUnavailable)
	}
}

// Handler returns a health status handler.
func Handler(scanner *scan.Scanner, age time.Duration) http.Handler {
	// Create health status handler.
	health := Health{
		scanner: scanner,
		age:     age,
	}

	// Return health status handler.
	return health
}
