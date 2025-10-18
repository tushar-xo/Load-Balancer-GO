package loadbalancer

import (
	"log"
	"net/http"
	"sync"
	"time"
)

type HealthChecker struct {
	mu      sync.RWMutex     // LOCK -- write operation on map -- UNLOCK
	healthy map[string]bool
}

// NewHealthChecker initializes a health map
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		healthy: make(map[string]bool),
	}
}

// CheckServer pings a backend and updates health map
func (hc *HealthChecker) CheckServer(url string) bool {
	client := http.Client{
		Timeout: 1 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		hc.mu.Lock() 
		hc.healthy[url] = false
		hc.mu.Unlock()
		return false
	}
	hc.mu.Lock()
	hc.healthy[url] = true
	hc.mu.Unlock()
	return true
}

// IsHealthy checks if a server is healthy
func (hc *HealthChecker) IsHealthy(url string) bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.healthy[url]
}

// Monitor starts a background goroutine that checks all servers periodically
func (hc *HealthChecker) Monitor(urls []string) {
	ticker := time.NewTicker(3 * time.Second)
	go func() {
		for range ticker.C { // infinite loop
			for _, url := range urls {
				ok := hc.CheckServer(url)
				if !ok {
					log.Printf("[WARN] %s is unhealthy\n", url)
				}
			}
		}
	}()
}

// ServerPoolInterface defines the interface for server pool operations
type ServerPoolInterface interface {
	HealthCheck()
}

// HealthCheckLoop runs periodic health checks on all backend servers
// This ensures the load balancer only routes traffic to healthy servers
func HealthCheckLoop(serverPool ServerPoolInterface) {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()

	for range t.C {
		// This would need to call the appropriate health check method
		// on the serverPool interface
		serverPool.HealthCheck()
	}
}
