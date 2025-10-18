package main

import (
	"log"
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

type ServerPool struct {
	backends []*Backend
	current  uint64
	stickySessions map[string]*Backend // Map to store sticky session assignments
	stickyMux      sync.RWMutex       // Mutex for sticky session operations
}

func (s *ServerPool) AddBackend(backend *Backend) {
	s.backends = append(s.backends, backend)
	log.Printf("[INFO] Added backend: %s (weight: %d)", backend.URL.String(), backend.Weight)
}

func (s *ServerPool) NextIndex() int {
	return int(atomic.AddUint64(&s.current, uint64(1)) % uint64(len(s.backends)))
}

func (s *ServerPool) GetNextPeer() *Backend {
	next := s.NextIndex()
	l := len(s.backends) + next
	for i := next; i < l; i++ {
		idx := i % len(s.backends)
		if s.backends[idx].IsAlive() {
			if i != next {
				atomic.StoreUint64(&s.current, uint64(idx))
			}
			return s.backends[idx]
		}
	}
	return nil
}

func (s *ServerPool) GetNextPeerWeighted() *Backend {
	totalWeight := s.GetTotalWeight()
	if totalWeight == 0 {
		return nil
	}

	random := int(atomic.AddUint64(&s.current, uint64(time.Now().UnixNano())) % uint64(totalWeight))

	currentWeight := 0
	for _, b := range s.backends {
		if b.IsAlive() {
			currentWeight += b.GetWeight()
			if random < currentWeight {
				return b
			}
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
func (s *ServerPool) GetBackendForStickySession(sessionID string) *Backend {
	s.stickyMux.RLock()
	backend, exists := s.stickySessions[sessionID]
	s.stickyMux.RUnlock()

	if exists && backend.IsAlive() {
		return backend
	}

	// Assign a new backend for this session
	backend = s.GetNextPeer()
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
	for _, b := range s.backends {
		if b.IsAlive() {
			total += b.GetWeight()
		}
	}
	return total
}

func (s *ServerPool) HealthCheck() {
	for _, b := range s.backends {
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
