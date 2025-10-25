package loadbalancer

import (
	"context"
	"log"
	"net/http"
	"time"
)

// SimpleTelemetryProvider provides basic telemetry without heavy dependencies
type SimpleTelemetryProvider struct {
	serviceName string
}

// NewSimpleTelemetryProvider creates a simple telemetry provider
func NewSimpleTelemetryProvider(serviceName string) *SimpleTelemetryProvider {
	return &SimpleTelemetryProvider{
		serviceName: serviceName,
	}
}

// LogInfo logs informational messages
func (tp *SimpleTelemetryProvider) LogInfo(message string, fields ...interface{}) {
	log.Printf("[INFO] %s: %s", tp.serviceName, message)
}

// LogError logs error messages
func (tp *SimpleTelemetryProvider) LogError(message string, err error, fields ...interface{}) {
	log.Printf("[ERROR] %s: %s - %v", tp.serviceName, message, err)
}

// LogWarn logs warning messages
func (tp *SimpleTelemetryProvider) LogWarn(message string, fields ...interface{}) {
	log.Printf("[WARN] %s: %s", tp.serviceName, message)
}

// LogDebug logs debug messages
func (tp *SimpleTelemetryProvider) LogDebug(message string, fields ...interface{}) {
	log.Printf("[DEBUG] %s: %s", tp.serviceName, message)
}

// TraceRequest creates a trace context (simplified implementation)
func (tp *SimpleTelemetryProvider) TraceRequest(r *http.Request) (context.Context, interface{}) {
	span := struct {
		StartTime time.Time
	}{StartTime: time.Now()}
	return r.Context(), span
}

// RecordRequestMetrics records request metrics (placeholder)
func (tp *SimpleTelemetryProvider) RecordRequestMetrics(ctx context.Context, backend, method, status string, duration time.Duration) {
	tp.LogDebug("Request metrics recorded",
		"backend", backend,
		"method", method,
		"status", status,
		"duration_ms", duration.Milliseconds(),
	)
}

// RecordCircuitBreakerStateChange records circuit breaker state changes
func (tp *SimpleTelemetryProvider) RecordCircuitBreakerStateChange(ctx context.Context, backend, fromState, toState string) {
	tp.LogInfo("Circuit breaker state changed",
		"backend", backend,
		"from_state", fromState,
		"to_state", toState,
	)
}

// RecordBackendConnection records backend connection changes
func (tp *SimpleTelemetryProvider) RecordBackendConnection(ctx context.Context, backend string, delta int64) {
	tp.LogDebug("Backend connection changed",
		"backend", backend,
		"delta", delta,
	)
}

// Shutdown gracefully shuts down the telemetry provider
func (tp *SimpleTelemetryProvider) Shutdown(ctx context.Context) error {
	tp.LogInfo("Graceful shutdown of telemetry provider")
	return nil
}

// GetLogger returns a simplified logger interface
func (tp *SimpleTelemetryProvider) GetLogger() interface{} {
	return tp
}
