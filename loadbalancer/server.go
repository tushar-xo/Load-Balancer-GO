package loadbalancer

import (
	"fmt"
	"log"
	"net/http"
)

// startMockServer creates and starts a mock backend server on the specified port
// This simulates real backend services for testing the load balancer
func StartMockServer(port string) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Create a unique response based on the port to identify which backend served the request
		message := fmt.Sprintf("Response from backend server on port %s\n", port)
		w.Write([]byte(message))
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: http.HandlerFunc(handler),
	}

	log.Printf("[INFO] Mock backend server started on port %s", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("[ERROR] Mock backend server on port %s failed: %v", port, err)
	}
}