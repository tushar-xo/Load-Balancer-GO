package loadbalancer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// RedisClient interface defines the operations needed for distributed sessions
type RedisClient interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, keys ...string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
}

// StickySessionManager manages sticky sessions using Redis
type StickySessionManager struct {
	redisClient  RedisClient
	keyPrefix    string
	sessionTTL   time.Duration
}

// SessionData represents the data stored for a sticky session
type SessionData struct {
	BackendURL string    `json:"backend_url"`
	Region     string    `json:"region"`
	CreatedAt  time.Time `json:"created_at"`
	LastAccess time.Time `json:"last_access"`
}

// NewStickySessionManager creates a new Redis-based sticky session manager
func NewStickySessionManager(redisClient RedisClient, keyPrefix string, sessionTTL time.Duration) *StickySessionManager {
	return &StickySessionManager{
		redisClient: redisClient,
		keyPrefix:   keyPrefix,
		sessionTTL:  sessionTTL,
	}
}

// GetSession retrieves session data from Redis
func (sm *StickySessionManager) GetSession(ctx context.Context, sessionID string) (*SessionData, error) {
	key := sm.sessionKey(sessionID)
	data, err := sm.redisClient.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get session from Redis: %w", err)
	}
	if data == "" {
		return nil, nil // Session not found
	}

	var session SessionData
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	// Update last access time
	session.LastAccess = time.Now()
	if err := sm.updateSession(ctx, sessionID, &session); err != nil {
		log.Printf("[WARN] Failed to update session last access time: %v", err)
	}

	return &session, nil
}

// SetSession stores session data in Redis
func (sm *StickySessionManager) SetSession(ctx context.Context, sessionID string, backendURL, region string) error {
	session := &SessionData{
		BackendURL: backendURL,
		Region:     region,
		CreatedAt:  time.Now(),
		LastAccess: time.Now(),
	}

	key := sm.sessionKey(sessionID)
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	return sm.redisClient.Set(ctx, key, data, sm.sessionTTL)
}

// DeleteSession removes a session from Redis
func (sm *StickySessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	key := sm.sessionKey(sessionID)
	return sm.redisClient.Del(ctx, key)
}

// SessionExists checks if a session exists
func (sm *StickySessionManager) SessionExists(ctx context.Context, sessionID string) (bool, error) {
	key := sm.sessionKey(sessionID)
	exists, err := sm.redisClient.Exists(ctx, key)
	if err != nil {
		return false, fmt.Errorf("failed to check session existence: %w", err)
	}
	return exists > 0, nil
}

// updateSession updates session data in Redis
func (sm *StickySessionManager) updateSession(ctx context.Context, sessionID string, session *SessionData) error {
	key := sm.sessionKey(sessionID)
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}
	return sm.redisClient.Set(ctx, key, data, sm.sessionTTL)
}

// sessionKey generates the Redis key for a session
func (sm *StickySessionManager) sessionKey(sessionID string) string {
	return fmt.Sprintf("%s:session:%s", sm.keyPrefix, sessionID)
}

// AutoScalingState represents the distributed state for auto-scaling
type AutoScalingState struct {
	CurrentReplicas int     `json:"current_replicas"`
	TotalRequests    int64   `json:"total_requests"`
	AverageLatency   float64 `json:"average_latency"`
	LastScaleUp      time.Time `json:"last_scale_up"`
	LastScaleDown    time.Time `json:"last_scale_down"`
	CooldownPeriod   time.Duration `json:"cooldown_period"`
}

// AutoScalingStateManager manages auto-scaling state using Redis
type AutoScalingStateManager struct {
	redisClient RedisClient
	keyPrefix   string
	stateTTL    time.Duration
}

// NewAutoScalingStateManager creates a new Redis-based auto-scaling state manager
func NewAutoScalingStateManager(redisClient RedisClient, keyPrefix string, stateTTL time.Duration) *AutoScalingStateManager {
	return &AutoScalingStateManager{
		redisClient: redisClient,
		keyPrefix:   keyPrefix,
		stateTTL:    stateTTL,
	}
}

// GetAutoScalingState retrieves the current auto-scaling state
func (asm *AutoScalingStateManager) GetAutoScalingState(ctx context.Context) (*AutoScalingState, error) {
	key := asm.stateKey()
	data, err := asm.redisClient.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get auto-scaling state from Redis: %w", err)
	}
	if data == "" {
		// Return default state if not found
		return &AutoScalingState{
			CurrentReplicas: 3,
			TotalRequests:   0,
			AverageLatency:  0.0,
			LastScaleUp:     time.Time{},
			LastScaleDown:   time.Time{},
			CooldownPeriod:  2 * time.Minute,
		}, nil
	}

	var state AutoScalingState
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal auto-scaling state: %w", err)
	}

	return &state, nil
}

// SetAutoScalingState stores the auto-scaling state in Redis
func (asm *AutoScalingStateManager) SetAutoScalingState(ctx context.Context, state *AutoScalingState) error {
	key := asm.stateKey()
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal auto-scaling state: %w", err)
	}
	return asm.redisClient.Set(ctx, key, data, asm.stateTTL)
}

// UpdateRequestCount increments the total request count
func (asm *AutoScalingStateManager) UpdateRequestCount(ctx context.Context, requestCount int64) error {
	state, err := asm.GetAutoScalingState(ctx)
	if err != nil {
		return err
	}
	
	state.TotalRequests = requestCount
	return asm.SetAutoScalingState(ctx, state)
}

// ShouldScale determines if auto-scaling should occur based on request count
func (asm *AutoScalingStateManager) ShouldScale(ctx context.Context, requestCount int64, threshold int64) (bool, bool, error) {
	state, err := asm.GetAutoScalingState(ctx)
	if err != nil {
		return false, false, err
	}

	now := time.Now()
	scaleUp := false
	scaleDown := false

	// Check if we should scale up
	if requestCount > threshold && 
	   (state.LastScaleUp.IsZero() || now.Sub(state.LastScaleUp) > state.CooldownPeriod) {
		scaleUp = true
		state.LastScaleUp = now
		state.CurrentReplicas++
		state.TotalRequests = 0
	}

	// Check if we should scale down
	if requestCount < threshold/2 && requestCount > 0 && state.CurrentReplicas > 1 &&
	   (state.LastScaleDown.IsZero() || now.Sub(state.LastScaleDown) > state.CooldownPeriod) {
		scaleDown = true
		state.LastScaleDown = now
		state.CurrentReplicas--
		state.TotalRequests = 0
	}

	if scaleUp || scaleDown {
		if err := asm.SetAutoScalingState(ctx, state); err != nil {
			return false, false, fmt.Errorf("failed to update auto-scaling state: %w", err)
		}
	}

	return scaleUp, scaleDown, nil
}

// stateKey generates the Redis key for auto-scaling state
func (asm *AutoScalingStateManager) stateKey() string {
	return fmt.Sprintf("%s:autoscaling:state", asm.keyPrefix)
}

// MockRedisClient implements a simple in-memory Redis client for testing
type MockRedisClient struct {
	data map[string]string
	mu   sync.RWMutex
}

// NewMockRedisClient creates a new mock Redis client
func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data: make(map[string]string),
	}
}

// Set stores a key-value pair with expiration
func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch v := value.(type) {
	case []byte:
		m.data[key] = string(v)
	case string:
		m.data[key] = v
	default:
		m.data[key] = fmt.Sprintf("%v", value)
	}
	return nil
}

// Get retrieves a value by key
func (m *MockRedisClient) Get(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data[key], nil
}

// Del deletes keys
func (m *MockRedisClient) Del(ctx context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

// Exists checks if keys exist
func (m *MockRedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := int64(0)
	for _, key := range keys {
		if _, exists := m.data[key]; exists {
			count++
		}
	}
	return count, nil
}


// Expire sets the expiration time for a key
func (m *MockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	// Mock implementation - in a real Redis client, this would set TTL
	return nil
}
