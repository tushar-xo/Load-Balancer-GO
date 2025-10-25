package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"net"
    "net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tushar-xo/Load-Balancer-GO/loadbalancer"
)

type Backend struct {
	URL            *url.URL
	Alive          bool
	mux            sync.RWMutex
	ReverseProxy   *httputil.ReverseProxy
	Weight         int // Weight for weighted routing (higher = more traffic)
	Region         string
	latencyEWMA    float64
	successEWMA    float64
	active         int64
	CircuitBreaker *loadbalancer.CircuitBreaker // Circuit breaker for fault tolerance
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

// ExecuteRequest runs the request through the circuit breaker
func (b *Backend) ExecuteRequest(req func() (interface{}, error)) (interface{}, error) {
	if b.CircuitBreaker == nil {
		// Fallback if circuit breaker is not initialized
		return req()
	}
	return b.CircuitBreaker.Execute(req)
}

// GetCircuitBreakerState returns the current state of the circuit breaker
func (b *Backend) GetCircuitBreakerState() loadbalancer.CircuitBreakerState {
	b.mux.RLock()
	defer b.mux.RUnlock()
	if b.CircuitBreaker == nil {
		return loadbalancer.StateClosed
	}
	return b.CircuitBreaker.State()
}

// IsCircuitBreakerOpen returns true if the circuit breaker is in OPEN state
func (b *Backend) IsCircuitBreakerOpen() bool {
	return b.GetCircuitBreakerState() == loadbalancer.StateOpen
}

func (b *Backend) Score() float64 {
	b.mux.RLock()
	latency := b.latencyEWMA
	success := b.successEWMA
	weight := b.Weight
	b.mux.RUnlock()
	
	// If circuit breaker is open, return an extremely high score to avoid selecting this backend
	if b.IsCircuitBreakerOpen() {
		return math.MaxFloat64
	}
	
	if latency == 0 {
		latency = 0.1
	}
	if success < 0.1 {
		success = 0.1
	}
	base := latency / math.Max(float64(weight), 1)
	penalty := 1 / success
	load := 1 + float64(b.ActiveConnections())

	// Also factor in circuit breaker states - half-open gets higher score, closed gets normal
	circuitMultiplier := 1.0
	switch b.GetCircuitBreakerState() {
	case loadbalancer.StateHalfOpen:
		circuitMultiplier = 2.0
	case loadbalancer.StateOpen:
		circuitMultiplier = 100.0
	}

	return base * penalty * load * circuitMultiplier
}

type ServerPool struct {
	backends          []*Backend
	weighted          []*Backend
	current           uint64
	stickySessions    map[string]*Backend // Map to store sticky session assignments (fallback)
	stickyMux         sync.RWMutex        // Mutex for sticky session operations
	regions           map[string][]*Backend
	mux               sync.RWMutex
	sessionManager    *loadbalancer.StickySessionManager
	autoScalingManager *loadbalancer.AutoScalingStateManager
	useRedis           bool
	consulManager     *loadbalancer.ConsulServiceManager
	useConsul          bool
	trafficPolicyEngine *loadbalancer.TrafficPolicyEngine
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

// EnableRedisSupport enables Redis-based distributed session management
func (s *ServerPool) EnableRedisSupport(redisClient loadbalancer.RedisClient, keyPrefix string, sessionTTL time.Duration) {
	s.sessionManager = loadbalancer.NewStickySessionManager(redisClient, keyPrefix, sessionTTL)
	s.autoScalingManager = loadbalancer.NewAutoScalingStateManager(redisClient, keyPrefix, time.Hour)
	s.useRedis = true
	log.Printf("[INFO] Redis support enabled for distributed sessions")
}

// IsRedisEnabled returns true if Redis support is enabled
func (s *ServerPool) IsRedisEnabled() bool {
	return s.useRedis && s.sessionManager != nil
}

// EnableConsulSupport enables dynamic service discovery via Consul
func (s *ServerPool) EnableConsulSupport(consulManager *loadbalancer.ConsulServiceManager) {
	s.consulManager = consulManager
	s.useConsul = true
	
	// Start Consul service discovery
	go func() {
		if err := s.consulManager.StartWatch(context.Background()); err != nil {
			log.Printf("[ERROR] Failed to start Consul watch: %v", err)
		}
	}()
	
	log.Printf("[INFO] Consul service discovery enabled")
}

// IsConsulEnabled returns true if Consul support is enabled
func (s *ServerPool) IsConsulEnabled() bool {
	return s.useConsul && s.consulManager != nil
}

// UpdateBackendsFromConsul updates backends based on Consul discovery
func (s *ServerPool) UpdateBackendsFromConsul() {
	if !s.IsConsulEnabled() {
		return
	}
	
	services := s.consulManager.GetAllServices()
	s.mux.Lock()
	defer s.mux.Unlock()
	
	// Clear existing backends and reinitialize
	s.backends = nil
	s.weighted = nil
	s.regions = make(map[string][]*Backend)
	
	for _, service := range services {
		// Create backend URL from Consul service data
		serviceURL, err := url.Parse(fmt.Sprintf("http://%s:%d", service.Address, service.Port))
		if err != nil {
			log.Printf("[ERROR] Failed to parse service URL %s:%d: %v", service.Address, service.Port, err)
			continue
		}
		
		// Create proxy for this backend
        proxy := httputil.NewSingleHostReverseProxy(serviceURL)
        if tr, err := loadbalancer.NewMTLSTransportFromEnv(); err != nil {
            log.Printf("[ERROR] mTLS transport setup failed for Consul service %s: %v", service.ID, err)
        } else if tr != nil {
            proxy.Transport = tr
            log.Printf("[INFO] mTLS enabled for Consul service %s", service.ID)
        }
		
		backend := &Backend{
			URL:          serviceURL,
			Alive:        true,
			ReverseProxy: proxy,
			Weight:       service.Weight,
			Region:       service.Region,
			CircuitBreaker: s.createCircuitBreakerForService(service),
		}
		
		s.backends = append(s.backends, backend)
		
		// Add to weighted routing
		for i := 0; i < service.Weight; i++ {
			s.weighted = append(s.weighted, backend)
		}
		
		// Add to region mapping
		s.regions[service.Region] = append(s.regions[service.Region], backend)
	}
	
	log.Printf("[INFO] Updated %d backends from Consul discovery", len(s.backends))
}

// createCircuitBreakerForService creates a circuit breaker for a Consul service
func (s *ServerPool) createCircuitBreakerForService(service loadbalancer.ConsulService) *loadbalancer.CircuitBreaker {
	return loadbalancer.NewCircuitBreaker(
		fmt.Sprintf("consul-service-%s", service.ID),
		loadbalancer.WithMaxRequests(3),
		loadbalancer.WithInterval(10*time.Second),
		loadbalancer.WithTimeout(30*time.Second),
		loadbalancer.WithReadyToTrip(func(counts loadbalancer.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		}),
		loadbalancer.WithOnStateChange(func(name string, from, to loadbalancer.CircuitBreakerState) {
			log.Printf("[INFO] Consul service circuit breaker '%s' changed from %v to %v", name, from, to)
		}),
	)
}

// EnableTrafficPolicies enables dynamic traffic routing policies
func (s *ServerPool) EnableTrafficPolicies(policies []loadbalancer.TrafficPolicy) {
	s.trafficPolicyEngine = loadbalancer.NewTrafficPolicyEngine(s.createBackendMap())
	
	for _, policy := range policies {
		s.trafficPolicyEngine.AddPolicy(policy)
	}
	
	log.Printf("[INFO] Traffic policies engine enabled with %d policies", len(policies))
}

// createBackendMap creates a backendMap for traffic policy engine
func (s *ServerPool) createBackendMap() map[string]interface{} {
	backendMap := make(map[string]interface{})
	s.mux.RLock()
	defer s.mux.RUnlock()
	
	for _, backend := range s.backends {
		backendMap[backend.URL.String()] = backend
	}
	
	return backendMap
}

// IsTrafficPoliciesEnabled returns true if traffic policies are enabled
func (s *ServerPool) IsTrafficPoliciesEnabled() bool {
	return s.trafficPolicyEngine != nil
}

// SelectBackendWithPolicy routes request using traffic policies
func (s *ServerPool) SelectBackendWithPolicy(r *http.Request) *Backend {
	if !s.IsTrafficPoliciesEnabled() {
		// Fallback to normal selection if no policies
		return s.GetNextPeerWeighted()
	}
	
    selected, err := s.trafficPolicyEngine.EvaluateRequest(r)
	if err != nil {
		log.Printf("[WARN] Traffic policy evaluation failed: %v", err)
		// Fallback to normal selection
		return s.GetNextPeerWeighted()
	}
	
    if selected == nil {
		log.Printf("[WARN] No backend selected by traffic policies, using fallback")
		return s.GetNextPeerWeighted()
	}
	
    if b, ok := selected.(*Backend); ok {
        return b
    }
    // If type assertion fails, fallback
    return s.GetNextPeerWeighted()
}

func (s *ServerPool) SetStickySession(sessionID string, backend *Backend) {
	s.stickyMux.Lock()
	defer s.stickyMux.Unlock()
	s.stickySessions[sessionID] = backend
}

// GetBackendForStickySession returns a backend for sticky session based on session ID
// If no sticky session exists, it creates one and returns the assigned backend
func (s *ServerPool) GetBackendForStickySession(sessionID string, region string) *Backend {
	// Try Redis first if enabled
	if s.IsRedisEnabled() {
		ctx := context.Background()
		sessionData, err := s.sessionManager.GetSession(ctx, sessionID)
		if err != nil {
			log.Printf("[WARN] Failed to get session from Redis: %v", err)
		} else if sessionData != nil {
			// Find backend by URL from Redis session data
			for _, backend := range s.backends {
				if backend.URL.String() == sessionData.BackendURL && backend.IsAlive() {
					return backend
				}
			}
		}
	}

	// Fallback to local sticky sessions
	s.stickyMux.RLock()
	backend, exists := s.stickySessions[sessionID]
	s.stickyMux.RUnlock()

	if exists && backend.IsAlive() {
		// Update Redis if enabled
		if s.IsRedisEnabled() {
			ctx := context.Background()
			if err := s.sessionManager.SetSession(ctx, sessionID, backend.URL.String(), region); err != nil {
				log.Printf("[WARN] Failed to store session in Redis: %v", err)
			}
		}
		return backend
	}

	// Assign a new backend for this session
	backend = s.SelectBackend(region)
	if backend != nil {
		// Store in local fallback
		s.stickyMux.Lock()
		s.stickySessions[sessionID] = backend
		s.stickyMux.Unlock()

		// Store in Redis if enabled
		if s.IsRedisEnabled() {
			ctx := context.Background()
			if err := s.sessionManager.SetSession(ctx, sessionID, backend.URL.String(), region); err != nil {
				log.Printf("[WARN] Failed to store session in Redis: %v", err)
			}
		}
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
