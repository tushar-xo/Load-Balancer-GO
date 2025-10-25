# Go Enterprise Load Balancer

🏗️ **Production-ready HTTP load balancer** with circuit breakers, Redis distributed sessions, and OpenTelemetry observability. Perfect for demonstrating enterprise-grade backend capabilities in technical interviews.

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue.svg)](https://docker.com)
[![Kubernetes](https://img.shields.io/badge/K8s-Ready-blue.svg)](https://kubernetes.io)

## ✨ **What It Does**

This load balancer distributes traffic across multiple backend servers while providing enterprise-grade features like:

- **🔄 Intelligent Routing**: Weighted load balancing, sticky sessions, and geo-based routing
- **🛡️ Fault Tolerance**: Circuit breakers with automatic recovery, health monitoring, and graceful degradation
- **📊 Observability**: Real-time metrics, structured logging, and Prometheus integration
- **🔐 Security**: mTLS support, rate limiting, and enterprise authentication
- **☁️ Cloud-Native**: Docker, Kubernetes, and auto-scaling ready

## 🚀 **Key Features**

### Core Load Balancing
- **Weighted Distribution**: Traffic allocation based on backend weights (3:2:1 ratio)
- **Sticky Sessions**: Redis-backed session persistence across requests
- **Health Monitoring**: Automatic detection and isolation of unhealthy backends
- **Geo-based Routing**: Region-aware request routing for global deployments

### Enterprise Resilience
- **Circuit Breakers**: Hystrix-style pattern with Closed→Open→Half-Open states
- **Rate Limiting**: DDoS protection with configurable thresholds
- **Auto-scaling**: Dynamic backend scaling based on request load
- **Graceful Degradation**: Service continuity during partial failures

### Production Observability
- **OpenTelemetry Integration**: Distributed tracing and structured logging
- **Prometheus Metrics**: Rich metrics for monitoring and alerting
- **Real-time Dashboard**: Live circuit breaker states and health monitoring
- **Structured JSON Logs**: ELK/Splunk-ready logging format

### Cloud-Native Ready
- **Docker Containerization**: Multi-stage builds with security hardening
- **Kubernetes Deployment**: HPA, ServiceMonitor, and health probes
- **mTLS Support**: Mutual TLS for secure backend communication
- **Service Discovery**: Consul integration for dynamic backend registration

## 🏃 **Quick Start**

### Prerequisites
- Go 1.21+
- Docker (optional)
- Redis (optional, for distributed sessions)

### Run Locally
```bash
# Clone and run
go run main.go serverpool.go

# Test load balancing
curl http://localhost:8080/lb

# View dashboard
open http://localhost:8080/

# Run tests
./tester.sh
```

### Docker Deployment
```bash
# Build and run
docker build -t go-loadbalancer .
docker run -p 8080:8080 go-loadbalancer
```

### Kubernetes Deployment
```bash
kubectl apply -f k8s/deployment.yaml
kubectl scale deployment go-loadbalancer --replicas=3
```

## 🏗️ **Architecture**

```
Client Request → Circuit Breaker → Traffic Policies → Load Balancer → Backend Server
                      ↓                      ↓                      ↓
              Rate Limiter          Geo/Header Routing     Health Monitor
                      ↓                      ↓                      ↓
              mTLS Transport    Redis Session Store ←───────┘
                      ↓                      ↓
              Auth/JWT       OpenTelemetry Metrics & Logging
                                   ↓
                       Real-time Dashboard
```

## 📋 **API Endpoints**

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Real-time dashboard with circuit breaker states |
| `/lb` | GET | Load balanced requests with sticky sessions |
| `/metrics` | GET | JSON metrics and backend health status |
| `/health` | GET | Kubernetes health probe endpoint |
| `/prometheus` | GET | Prometheus metrics export |

## ⚙️ **Configuration**

### Backend Servers
- `http://localhost:8081` (weight: 3) - Primary backend
- `http://localhost:8082` (weight: 2) - Secondary backend
- `http://localhost:8083` (weight: 1) - Tertiary backend

### Environment Variables
```bash
# Redis Configuration
REDIS_ENABLED=true
REDIS_URL=redis://localhost:6379

# Traffic Policies
TRAFFIC_POLICIES_ENABLED=true

# mTLS Configuration
MTLS_ENABLED=true
MTLS_CERT_FILE=/path/to/cert.pem
MTLS_KEY_FILE=/path/to/key.pem
MTLS_CA_FILE=/path/to/ca.pem

# Consul Service Discovery
CONSUL_ENABLED=true
CONSUL_ADDR=consul:8500
```

## 🧪 **Testing**

```bash
# Comprehensive test suite
./tester.sh

# Manual testing
curl http://localhost:8080/health         # Health check
curl http://localhost:8080/metrics        # Metrics API
curl http://localhost:8080/lb             # Load balanced request
```

**Test Coverage:**
- ✅ Circuit breaker functionality
- ✅ Redis session persistence
- ✅ Load distribution accuracy
- ✅ Health monitoring
- ✅ Traffic policy routing
- ✅ Rate limiting
- ✅ Prometheus metrics

## 📦 **Project Structure**

```
go-loadbalancer/
├── main.go                    # HTTP server & request routing
├── serverpool.go             # Backend management & load balancing
├── loadbalancer/             # Enterprise components
│   ├── circuitbreaker.go     # Fault tolerance patterns
│   ├── redis.go              # Distributed session storage
│   ├── telemetry_simple.go   # Observability framework
│   ├── trafficpolicies.go    # Advanced routing policies
│   ├── healthcheck.go        # Backend monitoring
│   ├── ratelimiter.go        # DDoS protection
│   ├── autoscaler.go         # Dynamic scaling
│   ├── mtls.go               # Mutual TLS support
│   └── consul.go             # Service discovery
├── k8s/                      # Kubernetes manifests
├── Dockerfile                # Multi-stage build
├── tester.sh                 # Comprehensive test suite
└── DEPLOYMENT.md             # Production deployment guide
```

## 🎯 **What Makes This Enterprise-Ready**

### 🏆 **Production Capabilities**
- **High Availability**: Redis-backed sessions across multiple instances
- **Fault Tolerance**: Circuit breakers prevent cascading failures
- **Observability**: Complete telemetry stack with monitoring
- **Security**: mTLS, rate limiting, and enterprise authentication
- **Scalability**: Auto-scaling and cloud-native deployment

### 💼 **Interview Highlights**
- **"Built an enterprise load balancer demonstrating production backend skills"**
- **"Implemented circuit breakers for fault tolerance with automatic recovery"**
- **"Created distributed session management using Redis for high availability"**
- **"Integrated comprehensive observability with OpenTelemetry and Prometheus"**
- **"Designed cloud-native architecture ready for Kubernetes deployment"**

## 🆕 **Recent Updates & Fixes**

### 🚀 **Latest Improvements**
- **✅ Sticky Session Thread-Safety**: Fixed concurrency issues in Redis-backed sessions with proper mutex locking
- **✅ Enhanced Circuit Breaker Integration**: Improved fault tolerance with automatic recovery states
- **✅ Traffic Policy Engine**: Advanced routing policies including geo-based, header-based, and canary deployments
- **✅ Production Monitoring**: Comprehensive telemetry with OpenTelemetry and Prometheus metrics
- **✅ Enterprise Security**: mTLS support and JWT authentication capabilities

### 🐛 **Bug Fixes**
- **Fixed**: Sticky session inconsistency caused by thread-unsafe MockRedisClient operations
- **Fixed**: Concurrent access issues in distributed session management
- **Fixed**: Race conditions in session state updates

**All enterprise features now working reliably with comprehensive test coverage.** 🎉

---

**Built with Go • Production-Ready • Enterprise-Grade • Interview-Ready** 🚀
