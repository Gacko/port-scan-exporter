package health

import (
	"log"
	"net/http"
)

// Health implements http.Handler for serving health status requests.
type Health struct{}

// ServeHTTP serves health status requests.
func (health Health) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	// Write headers.
	writer.WriteHeader(http.StatusOK)
	writer.Header().Set("Content-Type", "application/json")

	// Write body.
	if _, err := writer.Write([]byte(`{ "status": "ok" }`)); err != nil {
		log.Println(err)
	}
}

// Handler returns a health status handler.
func Handler() http.Handler {
	return Health{}
}
