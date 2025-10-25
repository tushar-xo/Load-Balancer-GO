package main

import (
	"context"
	"errors"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tushar-xo/Load-Balancer-GO/loadbalancer" // Import our loadbalancer package
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Global variables for load balancer state management
var (
	requestCount int64       // Counter for total requests processed
	serverPool = ServerPool{ // Initialize ServerPool with sticky sessions map
		stickySessions: make(map[string]*Backend),
	}
	telemetry *loadbalancer.SimpleTelemetryProvider // Simple telemetry provider for observability
)

func getClientRegion(r *http.Request) string {
	if region := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Client-Region"))); region != "" {
		return region
	}
	if region := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Geo-Region"))); region != "" {
		return region
	}
	if region := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("region"))); region != "" {
		return region
	}
	ip := clientIPFromRequest(r)
	if ip == "" {
		return "default"
	}
	if strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "192.168.") {
		return "us-east"
	}
	if strings.HasPrefix(ip, "172.") {
		return "us-west"
	}
	return "default"
}

func clientIPFromRequest(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// Prometheus metrics
var (
	promRegistry = prometheus.NewRegistry()

	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "loadbalancer_requests_total",
			Help: "Total number of requests processed by the load balancer",
		},
		[]string{"backend", "status"},
	)

	backendConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "loadbalancer_z_backend_connections",
			Help: "Number of active connections to each backend",
		},
		[]string{"backend"},
	)

	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "loadbalancer_z_request_duration_seconds",
			Help: "Request duration in seconds",
		},
		[]string{"backend"},
	)
)

func init() {
	promRegistry.MustRegister(requestsTotal)
	promRegistry.MustRegister(backendConnections)
	promRegistry.MustRegister(requestDuration)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// lbHandler handles load balancing requests with support for weighted routing and sticky sessions
// It checks for sticky session cookies and routes accordingly
func lbHandler(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&requestCount, 1)

	// Check for sticky session cookie
	var sessionID string
	if cookie, err := r.Cookie("LOAD-BALANCING_SESSION"); err == nil {
		sessionID = cookie.Value
	} else {
		// Generate a new session ID for sticky sessions
		sessionID = fmt.Sprintf("session_%d", time.Now().UnixNano())
		cookie := &http.Cookie{
			Name:     "LOAD-BALANCING_SESSION",
			Value:    sessionID,
			Path:     "/",
			MaxAge:   3600, // 1 hour
			HttpOnly: true,
		}
		http.SetCookie(w, cookie)
	}

	// Get backend based on traffic policies, sticky session, or load balancing algorithm
	var peer *Backend
	region := getClientRegion(r)
	
	// Check if traffic policies are enabled and use them
	if sessionID != "" {
		peer = serverPool.GetBackendForStickySession(sessionID, region)
	} else {
		// Use traffic policies first, fall back to normal selection
		peer = serverPool.SelectBackendWithPolicy(r)
		if peer == nil {
			peer = serverPool.SelectBackend(region)
		}
	}
	
	// Finally fallback to weighted routing if still no backend
	if peer == nil {
		peer = serverPool.GetNextPeerWeighted()
	}

	if peer == nil {
		telemetry.LogError("No healthy backends available", fmt.Errorf("service unavailable"))
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Log the routing decision with structured logging
	telemetry.LogInfo("Routing request to backend",
		"backend", peer.URL.String(),
		"session", sessionID,
		"circuit_breaker_state", fmt.Sprintf("%v", peer.GetCircuitBreakerState()),
	)

	peer.IncrementActive()
	defer func() {
		peer.DecrementActive()
		backendConnections.WithLabelValues(peer.URL.String()).Set(float64(peer.ActiveConnections()))
	}()

	recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	start := time.Now()

	// Execute request through circuit breaker
	_, err := peer.ExecuteRequest(func() (interface{}, error) {
		// Create a response recorder to capture the response
		peer.ReverseProxy.ServeHTTP(recorder, r)
		return nil, nil
	})

	duration := time.Since(start)
	statusCode := recorder.status
	if statusCode == 0 {
		statusCode = http.StatusBadGateway
	}
	success := statusCode < 500 && err == nil

	// Record metrics
	telemetry.RecordRequestMetrics(context.Background(), peer.URL.String(), r.Method, strconv.Itoa(statusCode), duration)
	peer.RecordMetrics(duration, success)
	
	// Update legacy Prometheus metrics
	requestsTotal.WithLabelValues(peer.URL.String(), strconv.Itoa(statusCode)).Inc()
	requestDuration.WithLabelValues(peer.URL.String()).Observe(duration.Seconds())

	// Handle circuit breaker failures
	if err != nil {
		telemetry.LogWarn("Circuit breaker triggered",
			"backend", peer.URL.String(),
			"error", err,
		)
		
		if errors.Is(err, loadbalancer.ErrTooManyRequests) {
			http.Error(w, "Service temporarily unavailable (circuit breaker open)", http.StatusServiceUnavailable)
		} else {
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		}
		return
	}

	telemetry.LogDebug("Request completed successfully",
		"backend", peer.URL.String(),
		"duration", duration,
	)
}

// dashboardHandler serves a web dashboard showing load balancer status and metrics
func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	dashboardHTML := `
<!DOCTYPE html>
<html>
<head>
    <title>Go Load Balancer Dashboard</title>
    <meta http-equiv="refresh" content="5">
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background: #f0f0f0; padding: 20px; border-radius: 5px; margin-bottom: 20px; }
        .metrics { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin-bottom: 20px; }
        .metric-card { background: white; border: 1px solid #ddd; padding: 15px; border-radius: 5px; }
        .metric-value { font-size: 24px; font-weight: bold; color: #007bff; }
        .metric-label { color: #666; font-size: 14px; }
        .backend-status { margin-bottom: 20px; }
        .backend-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 15px; }
        .backend-card { border: 1px solid #ddd; padding: 15px; border-radius: 5px; }
        .backend-url { font-weight: bold; margin-bottom: 10px; }
        .status-good { color: #28a745; }
        .status-bad { color: #dc3545; }
        .weight { background: #e9ecef; padding: 2px 8px; border-radius: 3px; font-size: 12px; }
        .circuit-breaker { background: #f8f9fa; padding: 2px 8px; border-radius: 3px; font-size: 12px; }
        .cb-closed { color: #28a745; }
        .cb-open { color: #dc3545; }
        .cb-half-open { color: #ffc107; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Go Load Balancer Dashboard</h1>
        <p>Real-time monitoring and management interface</p>
    </div>

    <div class="metrics">
        <div class="metric-card">
            <div class="metric-value">%d</div>
            <div class="metric-label">Total Requests</div>
        </div>
        <div class="metric-card">
            <div class="metric-value">%d</div>
            <div class="metric-label">Active Backends</div>
        </div>
        <div class="metric-card">
            <div class="metric-value">%d</div>
            <div class="metric-label">Healthy Backends</div>
        </div>
    </div>

    <div class="backend-status">
        <h2>Backend Servers Status</h2>
        <div class="backend-grid">`

	totalRequests := atomic.LoadInt64(&requestCount)
	backends := serverPool.Backends()
	activeBackends := len(backends)
	healthyBackends := 0

	for _, backend := range backends {
		if backend.IsAlive() {
			healthyBackends++
		}
	}

	// Generate backend cards
	for _, backend := range backends {
		status := "DOWN"
		statusClass := "status-bad"
		if backend.IsAlive() {
			status = "UP"
			statusClass = "status-good"
		}

		// Get circuit breaker status
		cbState := backend.GetCircuitBreakerState()
		cbStateText := ""
		cbStateClass := ""
		switch cbState {
		case loadbalancer.StateClosed:
			cbStateText = "CLOSED"
			cbStateClass = "cb-closed"
		case loadbalancer.StateOpen:
			cbStateText = "OPEN"
			cbStateClass = "cb-open"
		case loadbalancer.StateHalfOpen:
			cbStateText = "HALF-OPEN"
			cbStateClass = "cb-half-open"
		}

		dashboardHTML += fmt.Sprintf(`
            <div class="backend-card">
                <div class="backend-url">%s</div>
                <div class="%s">Status: %s</div>
                <div class="weight">Weight: %d</div>
                <div class="circuit-breaker %s">CB: %s</div>
            </div>`, backend.URL.String(), statusClass, status, backend.GetWeight(), cbStateClass, cbStateText)
	}

	dashboardHTML += `
        </div>
    </div>

    <div class="features">
        <h2>Features Enabled</h2>
        <ul>
            <li>✅ Weighted Load Balancing</li>
            <li>✅ Sticky Sessions</li>
            <li>✅ Health Checking</li>
            <li>✅ Circuit Breakers</li>
            <li>✅ Auto-scaling</li>
            <li>✅ Real-time Metrics</li>
            <li>✅ Prometheus Monitoring</li>
            <li>✅ Docker Containerization</li>
            <li>✅ Kubernetes Ready</li>
        </ul>
    </div>
</body>
</html>`

	fmt.Fprintf(w, dashboardHTML, totalRequests, activeBackends, healthyBackends)
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	backends := serverPool.Backends()
	result := make([]map[string]interface{}, 0, len(backends))
	for _, b := range backends {
		result = append(result, map[string]interface{}{
			"url":     b.URL.String(),
			"alive":   b.IsAlive(),
			"weight":  b.GetWeight(),
			"region":  b.Region,
			"latency": b.Score(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("[ERROR] Failed to encode metrics JSON: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	healthyBackends := 0
	for _, backend := range serverPool.Backends() {
		if backend.IsAlive() {
			healthyBackends++
		}
	}

	if healthyBackends > 0 {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	}

	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write([]byte("No healthy backends"))
}

// main is the entry point of the load balancer application
func main() {
	log.Printf("[INFO] Starting Go Load Balancer Application")
	log.Printf("[INFO] Initializing backend servers...")

	// Initialize backend servers with different weights for demonstration
	backendConfigs := []struct {
		url    string
		weight int
		region string
	}{
		{"http://localhost:8081", 3, "us-east"}, // Higher weight - handles more traffic
		{"http://localhost:8082", 2, "us-west"}, // Medium weight
		{"http://localhost:8083", 1, "asia"},    // Lower weight - handles less traffic
	}

	// Start backend servers first
	for _, config := range backendConfigs {
		go loadbalancer.StartMockServer(strings.TrimPrefix(config.url, "http://localhost:"))
	}

	// Give backend servers time to start
	time.Sleep(1 * time.Second)

	for _, config := range backendConfigs {
		u, err := url.Parse(config.url)
		if err != nil {
			log.Fatalf("[ERROR] Failed to parse backend URL %s: %v", config.url, err)
		}

        proxy := httputil.NewSingleHostReverseProxy(u)
        if tr, err := loadbalancer.NewMTLSTransportFromEnv(); err != nil {
            log.Printf("[ERROR] mTLS transport setup failed: %v", err)
        } else if tr != nil {
            proxy.Transport = tr
            log.Printf("[INFO] mTLS enabled for backend %s", config.url)
        }
		
		// Initialize circuit breaker for each backend
		circuitBreaker := loadbalancer.NewCircuitBreaker(
			fmt.Sprintf("backend-%s", strings.TrimPrefix(config.url, "http://localhost:")),
			loadbalancer.WithMaxRequests(3),
			loadbalancer.WithInterval(10*time.Second),
			loadbalancer.WithTimeout(30*time.Second),
			loadbalancer.WithReadyToTrip(func(counts loadbalancer.Counts) bool {
				// Trip circuit breaker after 5 consecutive failures
				return counts.ConsecutiveFailures >= 5
			}),
			loadbalancer.WithOnStateChange(func(name string, from, to loadbalancer.CircuitBreakerState) {
				log.Printf("[INFO] Circuit breaker '%s' changed from %v to %v", name, from, to)
			}),
		)
		
        backend := &Backend{
			URL:            u,
			Alive:          true,
			ReverseProxy:   proxy,
			Weight:         config.weight,
			Region:         config.region,
			CircuitBreaker: circuitBreaker,
		}
		serverPool.AddBackend(backend)
		log.Printf("[INFO] Added backend: %s (weight: %d)", config.url, config.weight)
		backendConnections.WithLabelValues(backend.URL.String()).Set(0)
		requestsTotal.WithLabelValues(backend.URL.String(), "200").Add(0)
		requestDuration.WithLabelValues(backend.URL.String()).Observe(0)
	}

	// Enable Redis support with mock client for demonstration
	redisClient := loadbalancer.NewMockRedisClient()
	serverPool.EnableRedisSupport(redisClient, "loadbalancer", time.Hour)
	log.Printf("[INFO] Redis support enabled for distributed sticky sessions")
	
	// Enable Consul service discovery (can be toggled with environment variable)
	consulEnabled := os.Getenv("CONSUL_ENABLED") == "true"
	if consulEnabled {
		consulClient := loadbalancer.NewMockConsulClient()
		consulManager := loadbalancer.NewConsulServiceManager(consulClient, "web-app")
		serverPool.EnableConsulSupport(consulManager)
		
		// Add initial Consul-discovered backends
		serverPool.UpdateBackendsFromConsul()
		log.Printf("[INFO] Consul service discovery enabled")
	} else {
		log.Printf("[INFO] Using static backend configuration (set CONSUL_ENABLED=true for service discovery)")
	}

	// Initialize Simple Telemetry for observability
	telemetry = loadbalancer.NewSimpleTelemetryProvider("go-loadbalancer")
	log.Printf("[INFO] Simple telemetry provider enabled")
	
	// Enable traffic policies (can be toggled with environment variable)
	trafficPoliciesEnabled := os.Getenv("TRAFFIC_POLICIES_ENABLED") == "true"
	if trafficPoliciesEnabled {
		policies := []loadbalancer.TrafficPolicy{
			{
				Name:     "Geo-Based Routing",
				Type:     loadbalancer.PolicyTypeGeo,
				Enabled:  true,
				Priority: 100,
				Rules: []loadbalancer.PolicyRule{
					{
						Field:    "region",
						Operator: "equals",
						Value:   "us-east",
						Action:  "allow",
						Weight:   10,
					},
					{
						Field:    "region", 
						Operator: "equals",
						Value:   "asia",
						Action:  "redirect",
						Backend: "http://localhost:8083",
						Weight:   5,
					},
				},
			},
			{
				Name:     "Header-Based API Routing",
				Type:     loadbalancer.PolicyTypeHeader,
				Enabled:  true,
				Priority: 90,
				Rules: []loadbalancer.PolicyRule{
					{
						Field:    "X-API-Version",
						Operator: "equals",
						Value:   "v2",
						Action:  "allow",
						Weight:   8,
					},
					{
						Field:    "X-Client-Type",
						Operator: "contains",
						Value:   "premium",
						Action:  "redirect",
						Backend: "http://localhost:8081",
						Weight:   15,
					},
				},
			},
			{
				Name:     "Canary Deployment",
				Type:     loadbalancer.PolicyTypeCanary,
				Enabled:  true,
				Priority: 80,
				Weight:   20,
				Conditions: loadbalancer.PolicyConditions{
					PercentageTraffic: 30, // 30% of traffic to canary
				},
				Rules: []loadbalancer.PolicyRule{
					{
						Field:    "path",
						Operator: "prefix",
						Value:   "/api/v2",
						Action:  "allow",
						Weight:   10,
					},
				},
			},
		}
		
		serverPool.EnableTrafficPolicies(policies)
		log.Printf("[INFO] Traffic policies engine enabled with %d policies", len(policies))
	} else {
		log.Printf("[INFO] Using standard load balancing (set TRAFFIC_POLICIES_ENABLED=true for policies)")
	}

	log.Printf("[INFO] Registered %d backends", len(backendConfigs))

	// Setup HTTP routes
	rateLimiter := loadbalancer.NewRateLimiter(10, 5)

	http.HandleFunc("/", dashboardHandler)                                  // Dashboard interface
	http.Handle("/lb", rateLimiter.Middleware(http.HandlerFunc(lbHandler))) // Load balancing endpoint
	http.HandleFunc("/metrics", metricsHandler)                             // JSON metrics endpoint
	http.HandleFunc("/health", healthCheckHandler)                          // Health check for K8s probes
	http.Handle("/prometheus", promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{})) // Prometheus metrics

	log.Printf("[INFO] Load balancer starting on :8080")
	log.Printf("[INFO] Available endpoints:")
	log.Printf("[INFO]   - Dashboard: http://localhost:8080/")
	log.Printf("[INFO]   - Load balancer: http://localhost:8080/lb")
	log.Printf("[INFO]   - Metrics: http://localhost:8080/metrics")
	log.Printf("[INFO]   - Health: http://localhost:8080/health")
	log.Printf("[INFO]   - Prometheus: http://localhost:8080/prometheus")

	// Start background services
	go loadbalancer.HealthCheckLoop(&serverPool)
	go loadbalancer.AutoScalerLoop(&requestCount, &serverPool)

	log.Printf("[INFO] Load balancer is ready to accept connections")

	// Start the HTTP server with default ServeMux
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("[ERROR] Failed to start server: %v", err)
	}
}
