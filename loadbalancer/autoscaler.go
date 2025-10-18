package loadbalancer

import (
	"log"
	"sync/atomic"
	"time"
)

// AutoScaler monitors request load and automatically scales backend servers
type AutoScaler struct {
	RequestCount *int64
	ServerPool   ServerPoolInterface // Use the interface
	Threshold    int
}

// NewAutoScaler creates a new autoscaler instance
func NewAutoScaler(requestCount *int64, threshold int) *AutoScaler {
	return &AutoScaler{
		RequestCount: requestCount,
		Threshold:    threshold,
	}
}

// Start begins the autoscaling monitoring loop
func (as *AutoScaler) Start() {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			as.checkAndScale()
		}
	}
}

// checkAndScale monitors load and adds new backends if needed
func (as *AutoScaler) checkAndScale() {
	count := atomic.SwapInt64(as.RequestCount, 0)

	// If request count is high, add a new backend server
	if count > int64(as.Threshold) {
		log.Printf("[INFO] High load detected: %d requests, triggering autoscaling", count)
		as.addNewBackend()
	}
}

// addNewBackend adds a new backend server to the pool
func (as *AutoScaler) addNewBackend() {
	// This would need to be implemented with proper ServerPool interface
	// For now, this is a placeholder showing the structure
	log.Printf("[INFO] AutoScaler: Would add new backend server")
	// TODO: Implement actual backend addition logic with StartMockServer from server package
}

// AutoScalerLoop monitors request load and automatically scales backend servers
// This runs as a background goroutine in main.go
func AutoScalerLoop(requestCount *int64, serverPool ServerPoolInterface) {
	as := NewAutoScaler(requestCount, 20) // threshold of 20
	as.ServerPool = serverPool
	as.Start()
}