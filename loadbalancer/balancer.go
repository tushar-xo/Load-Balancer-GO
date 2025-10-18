package loadbalancer

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type Server struct {
	URL *url.URL
}

type LoadBalancer struct {
	Servers       []*Server
	Index         int
	HealthChecker *HealthChecker
}

// NewLoadBalancer initializes a load balancer with health checking
func NewLoadBalancer(backendURLs []string) *LoadBalancer {
	var servers []*Server
	for _, backend := range backendURLs {
		parsedURL, err := url.Parse(backend)
		if err != nil {
			log.Fatal(err)
		}
		servers = append(servers, &Server{URL: parsedURL})
	}
	hc := NewHealthChecker()
	hc.Monitor(backendURLs)
	return &LoadBalancer{Servers: servers, Index: 0, HealthChecker: hc}
}

// GetNextServer returns the next healthy backend in round-robin order
func (lb *LoadBalancer) GetNextServer() *Server {
	startIndex := lb.Index
	for {
		server := lb.Servers[lb.Index]
		if lb.HealthChecker.IsHealthy(server.URL.String()) {
			lb.Index = (lb.Index + 1) % len(lb.Servers)
			return server
		}
		lb.Index = (lb.Index + 1) % len(lb.Servers)
		if lb.Index == startIndex {
			// All are unhealthy
			return nil
		}
	}
}

// HandleRequest forwards request to a healthy backend or returns 503
func (lb *LoadBalancer) HandleRequest(w http.ResponseWriter, r *http.Request) {
	server := lb.GetNextServer()
	if server == nil {
		http.Error(w, "No healthy servers available", http.StatusServiceUnavailable)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(server.URL)
	proxy.ServeHTTP(w, r)
}
