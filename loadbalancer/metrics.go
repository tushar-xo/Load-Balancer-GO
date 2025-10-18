package loadbalancer

import (
	"log"
	"sync"
	"time"
)

type Metrics struct {
	mu            sync.Mutex
	RequestCounts map[string]int
	ResponseTimes map[string]time.Duration
}

func NewMetrics() *Metrics {
	return &Metrics{
		RequestCounts: make(map[string]int),
		ResponseTimes: make(map[string]time.Duration),
	}
}

func (m *Metrics) LogRequest(server string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RequestCounts[server]++
	m.ResponseTimes[server] = duration
}

func (m *Metrics) PrintMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()
	log.Println("=== Metrics ===")
	for server, count := range m.RequestCounts {
		log.Printf("%s => %d requests | Last response time: %v\n", server, count, m.ResponseTimes[server])
	}
	log.Println("================")
}
