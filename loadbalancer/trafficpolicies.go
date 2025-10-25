package loadbalancer

import (
    "fmt"
    "log"
    "net"
    "net/http"
    "regexp"
    "strings"
    "sync"
    "time"
)

// TrafficPolicy represents a traffic routing policy
type TrafficPolicy struct {
	Name        string                 `json:"name"`
	Type        PolicyType            `json:"type"`
	Rules       []PolicyRule         `json:"rules"`
	Enabled     bool                  `json:"enabled"`
	Priority    int                   `json:"priority"`
	Weight      int                   `json:"weight"` // for canary deployments
	Conditions  PolicyConditions      `json:"conditions"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

type PolicyType string

const (
	PolicyTypeHeader  PolicyType = "header"
	PolicyTypeGeo     PolicyType = "geo"
	PolicyTypePath    PolicyType = "path"
	PolicyTypeCanary  PolicyType = "canary"
	PolicyTypeDefault PolicyType = "default"
)

// PolicyRule defines a single matching rule
type PolicyRule struct {
	Field       string      `json:"field"`       // header, query, path, geo, etc.
	Operator    string      `json:"operator"`    // equals, contains, regex, etc.
	Value       string      `json:"value"`      // the value to match against
	Action      string      `json:"action"`     // allow, deny, redirect
	Backend     string      `json:"backend"`    // specific backend when action is redirect
	Weight      int         `json:"weight"`     // weight for load balancing
}

// PolicyConditions define when a policy applies
type PolicyConditions struct {
    TimeRange         string   `json:"time_range"`
    PercentageTraffic int      `json:"percentageTraffic"`
    RequestRate       int64    `json:"request_rate"`
    MinVersion        string   `json:"min_version"`
    Maintainers       []string `json:"maintainers"`
}

// TrafficPolicyEngine manages traffic routing policies
type TrafficPolicyEngine struct {
    policies   []TrafficPolicy
    backendMap map[string]interface{} // URL to backend mapping
    mutex      sync.RWMutex
}

// NewTrafficPolicyEngine creates a new traffic policy engine
func NewTrafficPolicyEngine(backendMap map[string]interface{}) *TrafficPolicyEngine {
	return &TrafficPolicyEngine{
		backendMap: backendMap,
		policies:   make([]TrafficPolicy, 0),
	}
}

// AddPolicy adds a new traffic policy
func (tpe *TrafficPolicyEngine) AddPolicy(policy TrafficPolicy) {
	tpe.mutex.Lock()
	defer tpe.mutex.Unlock()
	
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	
	tpe.policies = append(tpe.policies, policy)
	
	// Sort policies by priority (higher priority = higher precedence)
	for i := 0; i < len(tpe.policies)-1; i++ {
		for j := i + 1; j < len(tpe.policies); j++ {
			if tpe.policies[i].Priority > tpe.policies[j].Priority {
				tpe.policies[i], tpe.policies[j] = tpe.policies[j], tpe.policies[i]
			}
		}
	}
	
	log.Printf("[INFO] Added traffic policy: %s (type: %s, priority: %d)", policy.Name, policy.Type, policy.Priority)
}

// EnablePolicy enables/disables a policy by name
func (tpe *TrafficPolicyEngine) EnablePolicy(name string, enabled bool) bool {
	tpe.mutex.Lock()
	defer tpe.mutex.Unlock()
	
	for i := range tpe.policies {
		if tpe.policies[i].Name == name {
			tpe.policies[i].Enabled = enabled
			tpe.policies[i].UpdatedAt = time.Now()
			log.Printf("[INFO] Traffic policy '%s' %s", name, map[bool]string{true: "enabled", false: "disabled"}[enabled])
			return true
		}
	}
	return false
}

// EvaluateRequest evaluates a request against all policies
func (tpe *TrafficPolicyEngine) EvaluateRequest(r *http.Request) (interface{}, error) {
	tpe.mutex.RLock()
	defer tpe.mutex.RUnlock()
	
	// Check enabled policies in priority order
	for _, policy := range tpe.policies {
		if !policy.Enabled {
			continue
		}
		
		match, action, backendURL := tpe.evaluatePolicy(r, policy)
		if match {
			if action == "deny" {
				return nil, fmt.Errorf("request denied by policy: %s", policy.Name)
			}
			
			if action == "redirect" && backendURL != "" {
				if backend, exists := tpe.backendMap[backendURL]; exists {
					log.Printf("[INFO] Request redirected by policy '%s' to backend: %s", policy.Name, backendURL)
					return backend, nil
				}
			}
			
            if action == "allow" {
                if b := tpe.selectBackendByPolicy(r, policy); b != nil {
                    return b, nil
                }
                return nil, nil
            }
		}
	}
	
    // No policies matched or explicit backend chosen; let caller decide fallback
    return nil, nil
}

// evaluatePolicy checks if a request matches a single policy
func (tpe *TrafficPolicyEngine) evaluatePolicy(r *http.Request, policy TrafficPolicy) (bool, string, string) {
	switch policy.Type {
	case PolicyTypeHeader:
		return tpe.evaluateHeaderPolicy(r, policy)
	case PolicyTypeGeo:
		return tpe.evaluateGeoPolicy(r, policy)
	case PolicyTypePath:
		return tpe.evaluatePathPolicy(r, policy)
	case PolicyTypeCanary:
		return tpe.evaluateCanaryPolicy(r, policy)
	case PolicyTypeDefault:
		return true, "allow", ""
	default:
		return false, "deny", ""
	}
}

// evaluateHeaderPolicy evaluates header-based policies
func (tpe *TrafficPolicyEngine) evaluateHeaderPolicy(r *http.Request, policy TrafficPolicy) (bool, string, string) {
	for _, rule := range policy.Rules {
		headerValue := r.Header.Get(rule.Field)
		
		switch rule.Operator {
		case "equals":
			if headerValue == rule.Value {
				return true, rule.Action, rule.Backend
			}
		case "contains":
			if strings.Contains(headerValue, rule.Value) {
				return true, rule.Action, rule.Backend
			}
		case "regex":
			if matched, _ := regexp.MatchString(rule.Value, headerValue); matched {
				return true, rule.Action, rule.Backend
			}
		}
	}
	return false, "deny", ""
}

// evaluateGeoPolicy evaluates geolocation-based policies
func (tpe *TrafficPolicyEngine) evaluateGeoPolicy(r *http.Request, policy TrafficPolicy) (bool, string, string) {
	// Get region from various sources
	region := r.Header.Get("X-Client-Region")
	if region == "" {
		region = r.Header.Get("X-Geo-Region")
	}
	if region == "" {
		region = r.URL.Query().Get("region")
	}
	if region == "" {
		// Extract region from IP (simplified)
		if ip := tpe.getClientIP(r); ip != "" {
			if strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "192.168.") {
				region = "us-east"
			} else if strings.HasPrefix(ip, "172.") {
				region = "us-west"
			}
		}
	}
	
	for _, rule := range policy.Rules {
		if rule.Field == "region" {
			switch rule.Operator {
			case "equals":
				if region == rule.Value {
					return true, rule.Action, rule.Backend
				}
			case "contains":
				if strings.Contains(region, rule.Value) {
					return true, rule.Action, rule.Backend
				}
			}
		}
	}
	
	return false, "deny", ""
}

// evaluatePathPolicy evaluates path-based policies
func (tpe *TrafficPolicyEngine) evaluatePathPolicy(r *http.Request, policy TrafficPolicy) (bool, string, string) {
	path := r.URL.Path
	
	for _, rule := range policy.Rules {
		switch rule.Operator {
		case "equals":
			if path == rule.Value {
				return true, rule.Action, rule.Backend
			}
		case "contains":
			if strings.Contains(path, rule.Value) {
				return true, rule.Action, rule.Backend
			}
		case "regex":
			if matched, _ := regexp.MatchString(rule.Value, path); matched {
				return true, rule.Action, rule.Backend
			}
		case "prefix":
			if strings.HasPrefix(path, rule.Value) {
				return true, rule.Action, rule.Backend
			}
		}
	}
	
	return false, "deny", ""
}

// evaluateCanaryPolicy evaluates canary deployment policies
func (tpe *TrafficPolicyEngine) evaluateCanaryPolicy(r *http.Request, policy TrafficPolicy) (bool, string, string) {
	// Simple canary implementation: percentage-based traffic splitting
    if policy.Conditions.PercentageTraffic > 0 {
		// Use request hash for consistent canary routing
		hash := tpe.hashRequest(r)
		modulo := hash % 100
		
        if int(modulo) < policy.Conditions.PercentageTraffic {
            // Allow canary cohort; actual backend selection handled by caller
            return true, "allow", ""
        }
	}
    
    return false, "deny", ""
}

// selectBackendByPolicy selects a backend based on policy rules
func (tpe *TrafficPolicyEngine) selectBackendByPolicy(r *http.Request, policy TrafficPolicy) interface{} {
    // If a rule specifies a concrete backend URL, return it
    for _, rule := range policy.Rules {
        if rule.Backend != "" {
            if backend, exists := tpe.backendMap[rule.Backend]; exists {
                return backend
            }
        }
    }
    // Otherwise, let the caller perform its own selection (weighted/healthy)
    return nil
}

// hashRequest creates a consistent hash for the request
func (tpe *TrafficPolicyEngine) hashRequest(r *http.Request) uint32 {
	// Simple hash implementation for consistent routing
	path := r.URL.Path
	userAgent := r.Header.Get("User-Agent")
	ip := tpe.getClientIP(r)
	
	// Combine multiple factors for better distribution
	input := fmt.Sprintf("%s|%s|%s", path, userAgent, ip)
	hash := uint32(0)
	for _, c := range input {
		hash = hash*31 + uint32(c)
	}
	return hash
}

// getClientIP extracts client IP from request
func (tpe *TrafficPolicyEngine) getClientIP(r *http.Request) string {
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


// GetPolicies returns all configured policies
func (tpe *TrafficPolicyEngine) GetPolicies() []TrafficPolicy {
	tpe.mutex.RLock()
	defer tpe.mutex.RUnlock()
	
	policies := make([]TrafficPolicy, len(tpe.policies))
	copy(policies, tpe.policies)
	return policies
}

// GetPolicyByName returns a specific policy by name
func (tpe *TrafficPolicyEngine) GetPolicyByName(name string) (*TrafficPolicy, error) {
	tpe.mutex.RLock()
	defer tpe.mutex.RUnlock()
	
	for _, policy := range tpe.policies {
		if policy.Name == name {
			return &policy, nil
		}
	}
	return nil, fmt.Errorf("policy not found: %s", name)
}
