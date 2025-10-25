package loadbalancer

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// ConsulService represents a service discovered via Consul
type ConsulService struct {
	ID       string            `json:"ID"`
	Name     string            `json:"Service"`
	Address  string            `json:"Address"`
	Port     int               `json:"Port"`
	Weight   int               `json:"Weight"`
	Region   string            `json:"Region"`
	Tags     []string          `json:"Tags"`
	Metadata map[string]string `json:"Meta"`
}

// ConsulCatalog represents the Consul catalog API response
type ConsulCatalog struct {
	Services map[string][]string `json:"Services"`
}

// ConsulHealthService represents health check response
type ConsulHealthService struct {
	Node    string        `json:"Node"`
	Service string        `json:"Service"`
	Checks  []HealthCheck `json:"Checks"`
	ID      string        `json:"ID"`
	Name    string        `json:"Service"`
}

type HealthCheck struct {
	Node        string    `json:"Node"`
	CheckID     string    `json:"CheckID"`
	Name        string    `json:"Name"`
	Status      string    `json:"Status"`
	Output      string    `json:"Output"`
	ServiceID   string    `json:"ServiceID"`
	ServiceName string    `json:"ServiceName"`
	ServiceTags []string  `json:"ServiceTags"`
}

// ConsulClient interface for Consul operations
type ConsulClient interface {
	GetServices(ctx context.Context) (map[string][]string, error)
	GetHealthyServices(ctx context.Context, serviceName string) ([]ConsulHealthService, error)
	GetServicesByService(ctx context.Context, serviceName string) ([]ConsulService, error)
	WatchServices(ctx context.Context, serviceName string) (<-chan []ConsulService, <-chan error)
}

// ConsulServiceManager handles dynamic service discovery via Consul
type ConsulServiceManager struct {
	client      ConsulClient
	serviceName string
	services    []ConsulService
	mutex       sync.RWMutex
	watchers    map[string]chan struct{}
}

// NewConsulServiceManager creates a new Consul service manager
func NewConsulServiceManager(client ConsulClient, serviceName string) *ConsulServiceManager {
	return &ConsulServiceManager{
		client:      client,
		serviceName: serviceName,
		services:    make([]ConsulService, 0),
		watchers:    make(map[string]chan struct{}),
	}
}

// GetAllServices returns all discovered backends
func (csm *ConsulServiceManager) GetAllServices() []ConsulService {
	csm.mutex.RLock()
	defer csm.mutex.RUnlock()
	services := make([]ConsulService, len(csm.services))
	copy(services, csm.services)
	return services
}

// GetServicesByRegion returns services filtered by region
func (csm *ConsulServiceManager) GetServicesByRegion(region string) []ConsulService {
	csm.mutex.RLock()
	defer csm.mutex.RUnlock()
	
	var filtered []ConsulService
	for _, service := range csm.services {
		if service.Region == region || region == "" {
			filtered = append(filtered, service)
		}
	}
	return filtered
}

// StartWatch begins watching for service changes
func (csm *ConsulServiceManager) StartWatch(ctx context.Context) error {
	serviceChan, errChan := csm.client.WatchServices(ctx, csm.serviceName)
	
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case services := <-serviceChan:
				csm.updateServices(services)
			case err := <-errChan:
				log.Printf("[ERROR] Consul watch error: %v", err)
			}
		}
	}()
	
	return nil
}

// updateServices updates the internal service list
func (csm *ConsulServiceManager) updateServices(services []ConsulService) {
	csm.mutex.Lock()
	defer csm.mutex.Unlock()
	
	csm.services = services
	log.Printf("[INFO] Updated service list via Consul: %d services discovered", len(services))
	
	// Notify watchers
	for _, ch := range csm.watchers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// GetHealthyServices returns only healthy services
func (csm *ConsulServiceManager) GetHealthyServices(ctx context.Context) ([]ConsulService, error) {
	healthyServices, err := csm.client.GetHealthyServices(ctx, csm.serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get healthy services: %w", err)
	}

	var services []ConsulService
	for _, health := range healthyServices {
		// Only include services with passing health checks
		allHealthy := true
		for _, check := range health.Checks {
			if check.Status != "passing" {
				allHealthy = false
				break
			}
		}
		
		if allHealthy {
			// Parse service details from health data
			// This would need to be implemented based on your Consul setup
			service := ConsulService{
				ID:     health.ID,
				Name:   health.Name,
				Tags:   []string{}, // Would be parsed from actual Consul response
			}
			services = append(services, service)
		}
	}

	return services, nil
}

// NotifyOnChange returns a channel that's notified when services change
func (csm *ConsulServiceManager) NotifyOnChange() <-chan struct{} {
	ch := make(chan struct{}, 1)
	csm.watchers["global"] = ch
	return ch
}

// MockConsulClient implements a mock Consul client for testing
type MockConsulClient struct {
	services map[string][]string
	healthy   []ConsulHealthService
	watchChan chan []ConsulService
	errChan   chan error
}

// NewMockConsulClient creates a mock Consul client
func NewMockConsulClient() *MockConsulClient {
	return &MockConsulClient{
		services: map[string][]string{
			"web-app": {"web-app-1", "web-app-2", "web-app-3"},
		},
		healthy: []ConsulHealthService{
			{
				ID:   "web-app-1",
				Name: "web-app",
				Checks: []HealthCheck{
					{
						CheckID: "service:web-app-1",
						Status:  "passing",
					},
				},
			},
			{
				ID:   "web-app-2", 
				Name: "web-app",
				Checks: []HealthCheck{
					{
						CheckID: "service:web-app-2",
						Status:  "passing",
					},
				},
			},
			{
				ID:   "web-app-3",
				Name: "web-app",
				Checks: []HealthCheck{
					{
						CheckID: "service:web-app-3",
						Status:  "passing",
					},
				},
			},
		},
		watchChan: make(chan []ConsulService, 10),
		errChan:   make(chan error, 1),
	}
}

func (mcc *MockConsulClient) GetServices(ctx context.Context) (map[string][]string, error) {
	return mcc.services, nil
}

func (mcc *MockConsulClient) GetHealthyServices(ctx context.Context, serviceName string) ([]ConsulHealthService, error) {
	return mcc.healthy, nil
}

func (mcc *MockConsulClient) GetServicesByService(ctx context.Context, serviceName string) ([]ConsulService, error) {
	services := []ConsulService{
		{
			ID:      "web-app-1",
			Name:    serviceName,
			Address: "localhost",
			Port:    8081,
			Weight:  3,
			Region:  "us-east",
			Tags:    []string{"web", "frontend"},
		},
		{
			ID:      "web-app-2", 
			Name:    serviceName,
			Address: "localhost",
			Port:    8082,
			Weight:  2,
			Region:  "us-west",
			Tags:    []string{"web", "api"},
		},
		{
			ID:      "web-app-3",
			Name:    serviceName,
			Address: "localhost",
			Port:    8083,
			Weight:  1,
			Region:  "asia",
			Tags:    []string{"web", "cache"},
		},
	}
	return services, nil
}

func (mcc *MockConsulClient) WatchServices(ctx context.Context, serviceName string) (<-chan []ConsulService, <-chan error) {
	// Start a goroutine to periodically send updates
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				services, _ := mcc.GetServicesByService(ctx, serviceName)
				select {
				case mcc.watchChan <- services:
				default:
				}
			}
		}
	}()
	
	return mcc.watchChan, mcc.errChan
}

// RealConsulClient implements actual Consul API communication
type RealConsulClient struct {
	baseURL string
	client  *http.Client
}

// NewRealConsulClient creates a real Consul client
func NewRealConsulClient(baseURL string) *RealConsulClient {
	return &RealConsulClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (rcc *RealConsulClient) GetServices(ctx context.Context) (map[string][]string, error) {
	// This would implement actual Consul API calls
	// For now, returning empty as mock implementation
	return map[string][]string{}, nil
}

func (rcc *RealConsulClient) GetHealthyServices(ctx context.Context, serviceName string) ([]ConsulHealthService, error) {
	// Implement actual Consul health API call
	return nil, fmt.Errorf("real Consul client not implemented yet")
}

func (rcc *RealConsulClient) GetServicesByService(ctx context.Context, serviceName string) ([]ConsulService, error) {
	// Implement actual Consul service discovery
	return nil, fmt.Errorf("real Consul client not implemented yet")
}

func (rcc *RealConsulClient) WatchServices(ctx context.Context, serviceName string) (<-chan []ConsulService, <-chan error) {
	// Implement actual Consul blocking query
	errChan := make(chan error, 1)
	errChan <- fmt.Errorf("real Consul client not implemented yet")
	return nil, errChan
}
