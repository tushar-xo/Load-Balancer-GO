package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"net/http/httputil"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tushar-xo/Load-Balancer-GO/loadbalancer" // Import our loadbalancer package

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Global variables for load balancer state management
var requestCount int64          // Counter for total requests processed
var serverPool = ServerPool{     // Initialize ServerPool with sticky sessions map
	stickySessions: make(map[string]*Backend),
}

// Prometheus metrics
var (
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "loadbalancer_requests_total",
			Help: "Total number of requests processed by the load balancer",
		},
		[]string{"backend", "status"},
	)

	backendConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "loadbalancer_backend_connections",
			Help: "Number of active connections to each backend",
		},
		[]string{"backend"},
	)

	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "loadbalancer_request_duration_seconds",
			Help: "Request duration in seconds",
		},
		[]string{"backend"},
	)
)

func init() {
	prometheus.MustRegister(requestsTotal)
	prometheus.MustRegister(backendConnections)
	prometheus.MustRegister(requestDuration)
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

	// Get backend based on sticky session or load balancing algorithm
	var peer *Backend
	if sessionID != "" {
		peer = serverPool.GetBackendForStickySession(sessionID)
	} else {
		// Use weighted routing for new sessions
		peer = serverPool.GetNextPeerWeighted()
	}

	if peer == nil {
		log.Printf("[ERROR] No healthy backends available")
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Log the routing decision
	log.Printf("[INFO] Routing request to backend: %s (session: %s)", peer.URL.String(), sessionID)

	peer.ReverseProxy.ServeHTTP(w, r)
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
	activeBackends := len(serverPool.backends)
	healthyBackends := 0

	for _, backend := range serverPool.backends {
		if backend.IsAlive() {
			healthyBackends++
		}
	}

	// Generate backend cards
	for _, backend := range serverPool.backends {
		status := "DOWN"
		statusClass := "status-bad"
		if backend.IsAlive() {
			status = "UP"
			statusClass = "status-good"
		}

		dashboardHTML += fmt.Sprintf(`
            <div class="backend-card">
                <div class="backend-url">%s</div>
                <div class="%s">Status: %s</div>
                <div class="weight">Weight: %d</div>
            </div>`, backend.URL.String(), statusClass, status, backend.GetWeight())
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


// healthCheckHandler provides health status for Kubernetes probes
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// Check if we have healthy backends
	healthyBackends := 0
	for _, backend := range serverPool.backends {
		if backend.IsAlive() {
			healthyBackends++
		}
	}

	if healthyBackends > 0 {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("No healthy backends"))
	}
}

// metricsHandler provides JSON metrics for external monitoring systems
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	var result []map[string]interface{}
	for _, b := range serverPool.backends {
		result = append(result, map[string]interface{}{
			"url":   b.URL.String(),
			"alive": b.IsAlive(),
			"weight": b.GetWeight(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("[ERROR] Failed to encode metrics JSON: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}



// main is the entry point of the load balancer application
func main() {
	log.Printf("[INFO] Starting Go Load Balancer Application")
	log.Printf("[INFO] Initializing backend servers...")

	// Initialize backend servers with different weights for demonstration
	backendConfigs := []struct {
		url    string
		weight int
	}{
		{"http://localhost:8081", 3}, // Higher weight - handles more traffic
		{"http://localhost:8082", 2}, // Medium weight
		{"http://localhost:8083", 1}, // Lower weight - handles less traffic
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
		backend := &Backend{
			URL:          u,
			Alive:        true,
			ReverseProxy: proxy,
			Weight:       config.weight,
		}
		serverPool.AddBackend(backend)
		log.Printf("[INFO] Added backend: %s (weight: %d)", config.url, config.weight)
	}

	log.Printf("[INFO] Registered %d backends", len(backendConfigs))

	// Setup HTTP routes
	http.HandleFunc("/", dashboardHandler)        // Dashboard interface
	http.HandleFunc("/lb", lbHandler)             // Load balancing endpoint
	http.HandleFunc("/metrics", metricsHandler)   // JSON metrics endpoint
	http.HandleFunc("/health", healthCheckHandler) // Health check for K8s probes
	http.Handle("/prometheus", promhttp.Handler()) // Prometheus metrics

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
