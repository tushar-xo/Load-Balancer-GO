package loadbalancer

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type RateLimiter struct {
	capacity   float64
	refillRate float64
	buckets    map[string]*tokenBucket
	warmup     int
	mux        sync.Mutex
}

type tokenBucket struct {
	tokens float64
	last   time.Time
	warmup int
}

func NewRateLimiter(capacity int, refillPerSecond int) *RateLimiter {
	return &RateLimiter{
		capacity:   float64(capacity),
		refillRate: float64(refillPerSecond),
		buckets:    make(map[string]*tokenBucket),
		warmup:     capacity * 3,
	}
}

func (rl *RateLimiter) Allow(key string) bool {
	now := time.Now()
	rl.mux.Lock()
	bucket, ok := rl.buckets[key]
	if !ok {
		bucket = &tokenBucket{tokens: rl.capacity, last: now, warmup: rl.warmup}
		rl.buckets[key] = bucket
	}
	if bucket.warmup > 0 {
		bucket.warmup--
		bucket.last = now
		rl.mux.Unlock()
		return true
	}
	elapsed := now.Sub(bucket.last).Seconds()
	if elapsed > 0 {
		bucket.tokens += elapsed * rl.refillRate
		if bucket.tokens > rl.capacity {
			bucket.tokens = rl.capacity
		}
		bucket.last = now
	}
	allowed := bucket.tokens >= 1
	if allowed {
		bucket.tokens -= 1
	}
	rl.mux.Unlock()
	return allowed
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientKey(r)
		if !rl.Allow(key) {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientKey(r *http.Request) string {
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
