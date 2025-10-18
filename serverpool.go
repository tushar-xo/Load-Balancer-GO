package main

import (
	"log"
	"math"
	"net"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type Backend struct {
	URL          *url.URL
	Alive        bool
	mux          sync.RWMutex
	ReverseProxy *httputil.ReverseProxy
	Weight       int // Weight for weighted routing (higher = more traffic)
	Region       string
	latencyEWMA  float64
	successEWMA  float64
	active       int64
}

func (b *Backend) SetAlive(alive bool) {
	b.mux.Lock()
	b.Alive = alive
	b.mux.Unlock()
}

func (b *Backend) IsAlive() bool {
	b.mux.RLock()
	defer b.mux.RUnlock()
	return b.Alive
}

// GetWeight returns the weight of the backend for load balancing decisions
func (b *Backend) GetWeight() int {
	b.mux.RLock()
	defer b.mux.RUnlock()
	return b.Weight
}

func (b *Backend) IncrementActive() {
	atomic.AddInt64(&b.active, 1)
}

func (b *Backend) DecrementActive() {
	atomic.AddInt64(&b.active, -1)
}

func (b *Backend) ActiveConnections() int64 {
	return atomic.LoadInt64(&b.active)
}

func (b *Backend) RecordMetrics(duration time.Duration, success bool) {
	b.mux.Lock()
	const latencyAlpha = 0.2
	const successAlpha = 0.1
	sample := duration.Seconds()
	if b.latencyEWMA == 0 {
		b.latencyEWMA = sample
	} else {
		b.latencyEWMA = latencyAlpha*sample + (1-latencyAlpha)*b.latencyEWMA
	}
	if b.successEWMA == 0 {
		b.successEWMA = 1
	}
	value := 0.0
	if success {
		value = 1
	}
	b.successEWMA = successAlpha*value + (1-successAlpha)*b.successEWMA
	b.mux.Unlock()
}

func (b *Backend) Score() float64 {
	b.mux.RLock()
	latency := b.latencyEWMA
	success := b.successEWMA
	weight := b.Weight
	b.mux.RUnlock()
	if latency == 0 {
		latency = 0.1
	}
	if success < 0.1 {
		success = 0.1
	}
	base := latency / math.Max(float64(weight), 1)
	penalty := 1 / success
	load := 1 + float64(b.ActiveConnections())
	return base * penalty * load
}

type ServerPool struct {
	backends       []*Backend
	weighted       []*Backend
	current        uint64
	stickySessions map[string]*Backend // Map to store sticky session assignments
	stickyMux      sync.RWMutex        // Mutex for sticky session operations
	regions        map[string][]*Backend
	mux            sync.RWMutex
}

func (s *ServerPool) AddBackend(backend *Backend) {
	s.mux.Lock()
	s.backends = append(s.backends, backend)
	for i := 0; i < backend.Weight; i++ {
		s.weighted = append(s.weighted, backend)
	}
	if s.regions == nil {
		s.regions = make(map[string][]*Backend)
	}
	if backend.Region != "" {
		s.regions[backend.Region] = append(s.regions[backend.Region], backend)
	}
	s.mux.Unlock()
	log.Printf("[INFO] Added backend: %s (weight: %d)", backend.URL.String(), backend.Weight)
}

func (s *ServerPool) NextIndex() int {
	s.mux.RLock()
	length := len(s.backends)
	s.mux.RUnlock()
	if length == 0 {
		return 0
	}
	return int(atomic.AddUint64(&s.current, uint64(1)) % uint64(length))
}

func (s *ServerPool) GetNextPeer() *Backend {
	next := s.NextIndex()
	s.mux.RLock()
	backends := append([]*Backend(nil), s.backends...)
	s.mux.RUnlock()
	l := len(backends) + next
	for i := next; i < l; i++ {
		if len(backends) == 0 {
			break
		}
		idx := i % len(backends)
		if backends[idx].IsAlive() {
			if i != next {
				atomic.StoreUint64(&s.current, uint64(idx))
			}
			return backends[idx]
		}
	}
	return nil
}

func (s *ServerPool) GetNextPeerWeighted() *Backend {
	s.mux.RLock()
	weighted := append([]*Backend(nil), s.weighted...)
	s.mux.RUnlock()
	if len(weighted) == 0 {
		return nil
	}
	length := len(weighted)
	start := int(atomic.AddUint64(&s.current, 1)-1) % length
	for i := 0; i < length; i++ {
		idx := (start + i) % length
		backend := weighted[idx]
		if backend.IsAlive() {
			return backend
		}
	}
	return nil
}

func (s *ServerPool) GetStickySession(sessionID string) *Backend {
	s.stickyMux.RLock()
	defer s.stickyMux.RUnlock()
	return s.stickySessions[sessionID]
}

func (s *ServerPool) SetStickySession(sessionID string, backend *Backend) {
	s.stickyMux.Lock()
	defer s.stickyMux.Unlock()
	s.stickySessions[sessionID] = backend
}

// GetBackendForStickySession returns a backend for sticky session based on session ID
// If no sticky session exists, it creates one and returns the assigned backend
func (s *ServerPool) GetBackendForStickySession(sessionID string, region string) *Backend {
	s.stickyMux.RLock()
	backend, exists := s.stickySessions[sessionID]
	s.stickyMux.RUnlock()

	if exists && backend.IsAlive() {
		return backend
	}

	// Assign a new backend for this session
	backend = s.SelectBackend(region)
	if backend != nil {
		s.stickyMux.Lock()
		s.stickySessions[sessionID] = backend
		s.stickyMux.Unlock()
	}

	return backend
}

// GetTotalWeight calculates the total weight of all healthy backends
func (s *ServerPool) GetTotalWeight() int {
	total := 0
	s.mux.RLock()
	for _, b := range s.backends {
		if b.IsAlive() {
			total += b.GetWeight()
		}
	}
	s.mux.RUnlock()
	return total
}

func (s *ServerPool) HealthCheck() {
	s.mux.RLock()
	backends := append([]*Backend(nil), s.backends...)
	s.mux.RUnlock()
	for _, b := range backends {
		conn, err := net.DialTimeout("tcp", b.URL.Host, 2*time.Second)
		if err != nil {
			b.SetAlive(false)
			log.Printf("[WARN] Backend %s is DOWN: %v", b.URL.String(), err)
		} else {
			b.SetAlive(true)
			conn.Close()
		}
	}
}

func (s *ServerPool) SelectBackend(region string) *Backend {
	if region == "" || region == "default" {
		return s.GetNextPeerWeighted()
	}
	candidates := s.getHealthyByRegion(region)
	if len(candidates) == 0 {
		return s.GetNextPeerWeighted()
	}
	var best *Backend
	bestScore := math.MaxFloat64
	for _, backend := range candidates {
		score := backend.Score()
		if score < bestScore {
			best = backend
			bestScore = score
		}
	}
	return best
}

func (s *ServerPool) getHealthyByRegion(region string) []*Backend {
	s.mux.RLock()
	defer s.mux.RUnlock()
	var list []*Backend
	if region != "" {
		for _, backend := range s.regions[region] {
			if backend.IsAlive() {
				list = append(list, backend)
			}
		}
		if len(list) > 0 {
			return list
		}
	}
	for _, backend := range s.backends {
		if backend.IsAlive() {
			list = append(list, backend)
		}
	}
	return list
}

func (s *ServerPool) Backends() []*Backend {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return append([]*Backend(nil), s.backends...)
}
