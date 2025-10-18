# Go Load Balancer

A production-ready HTTP load balancer written in Go with advanced features including health checking, autoscaling, weighted routing, sticky sessions, comprehensive monitoring, and full Kubernetes deployment support.

## Project Structure

```
/go-loadbalancer
│
├── main.go                  # Entry point & load balancer server with dashboard
├── serverpool.go            # Backend server pool management with weighted routing
├── loadbalancer/
│   ├── server.go            # Mock server management & Docker support
│   ├── autoscaler.go        # Auto-scaling logic & background processing
│   ├── healthcheck.go       # Health monitoring & periodic checks
│   ├── balancer.go          # Load balancing algorithms (round-robin & weighted)
│   ├── metrics.go           # Metrics collection and Prometheus integration
│   └── ServerPoolInterface  # Interface for cross-package communication
├── k8s/
│   └── deployment.yaml       # Kubernetes deployment manifests
├── .github/workflows/
│   └── ci-cd.yml            # GitHub Actions CI/CD pipeline
├── Dockerfile               # Multi-stage Docker build
├── docker-compose.yml       # Local development environment
├── README-DEPLOYMENT.md     # Production deployment guide
├── go.mod                   # Go module definition
└── README.md                # This file - project documentation
```

## Features

### ✅ Core Features
- **Round-Robin Load Balancing**: Distributes requests evenly across backend servers
- **Weighted Load Balancing**: Backends can have different weights for uneven load distribution
- **Sticky Sessions**: Users are consistently routed to the same backend server
- **Health Checking**: Automatically detects and removes unhealthy backends
- **Auto-scaling**: Dynamically adds new backend servers based on load
- **Request Metrics**: Tracks request counts and server health status
- **Web Dashboard**: Real-time monitoring interface with server status and metrics
- **Comprehensive Logging**: Detailed logging with different log levels
- **REST API**: Multiple endpoints for different functionalities

### 🚀 Production Features
- **Docker Containerization**: Multi-stage build for optimal image size
- **Kubernetes Ready**: Complete K8s deployment with health checks and scaling
- **Prometheus Metrics**: Rich metrics for monitoring and alerting
- **Health Check Endpoints**: K8s liveness and readiness probes
- **CI/CD Pipeline**: Automated testing and deployment
- **Production Logging**: Structured logging for observability
- **Security Hardening**: Non-root containers and security best practices

### 🔧 Advanced Features
- **Thread-Safe Operations**: Uses mutexes and atomic operations for concurrency
- **Session Management**: Cookie-based sticky session implementation
- **Weighted Routing**: Configurable weights for backend server prioritization
- **Graceful Error Handling**: Proper error handling throughout the application
- **Configurable Timeouts**: Health check timeouts and autoscaling intervals
- **Production Ready**: Structured logging and monitoring capabilities

## Quick Start

### Local Development

```bash
# Run with docker-compose (includes all dependencies)
docker-compose up -d

# Or run directly
go run main.go serverpool.go
```

### Docker Deployment

```bash
# Build and run
docker build -t go-loadbalancer .
docker run -p 8080:8080 go-loadbalancer
```

### Kubernetes Deployment

```bash
# Deploy to Kubernetes
kubectl apply -f k8s/deployment.yaml

# Scale as needed
kubectl scale deployment go-loadbalancer --replicas=5
```

## Configuration

### Backend Servers
The load balancer starts with 3 backend servers with different weights:
- http://localhost:8081 (weight: 3) - Handles 50% of traffic
- http://localhost:8082 (weight: 2) - Handles 33% of traffic
- http://localhost:8083 (weight: 1) - Handles 17% of traffic

### Tuning Parameters
You can modify these values in `main.go`:
- **Health Check Interval**: 10 seconds
- **Autoscaling Interval**: 15 seconds
- **Autoscaling Threshold**: 20 requests per interval
- **Health Check Timeout**: 2 seconds
- **Sticky Session Duration**: 1 hour

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Web dashboard with real-time monitoring |
| `/lb` | GET | Load balanced requests with sticky sessions |
| `/metrics` | GET | JSON metrics showing backend health and weights |
| `/health` | GET | Health check for Kubernetes probes |
| `/prometheus` | GET | Prometheus metrics for monitoring |

## Monitoring

### Dashboard Features
- **Real-time Metrics**: Live request counts and backend status
- **Server Health**: Visual indicators for backend server status
- **Weight Display**: Shows the weight assigned to each backend
- **Feature Status**: Lists all enabled load balancing features
- **Auto-refresh**: Updates every 5 seconds

### Prometheus Metrics
- `loadbalancer_requests_total{backend, status}` - Request counters
- `loadbalancer_backend_connections{backend}` - Active connections
- `loadbalancer_request_duration_seconds{backend}` - Response times

### Grafana Integration
Ready-to-use dashboard queries for comprehensive monitoring and alerting.

## Deployment

### Docker
- **Multi-stage build** for optimal image size
- **Security hardened** with non-root user
- **Health checks** built-in for orchestration

### Kubernetes
- **Production manifests** with proper resource limits
- **Horizontal Pod Autoscaling** support
- **Service discovery** and load balancing
- **Rolling updates** and zero-downtime deployments

### CI/CD
- **GitHub Actions** workflow for automated testing and deployment
- **Multi-stage Docker builds** with security scanning
- **Automated deployment** to Kubernetes clusters

## Development

### Adding New Features

1. **New Backend Type**: Add to `serverpool.go` or `loadbalancer/`
2. **New Load Balancing Algorithm**: Implement in `balancer.go`
3. **New Health Check Method**: Extend `healthcheck.go`
4. **New Metrics**: Add to `metrics.go`
5. **Dashboard Enhancements**: Modify dashboard HTML in `main.go`

### Testing

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific package tests
go test ./loadbalancer
```

## Production Deployment

For production use, consider:

1. **Configuration Management**: Externalize configuration for different environments
2. **Service Discovery**: Replace hardcoded backend URLs with dynamic discovery
3. **SSL/TLS**: Add HTTPS support for secure communication
4. **Monitoring**: Integrate with monitoring systems (Prometheus, Grafana)
5. **Logging**: Use structured logging (JSON format) for better parsing
6. **Metrics**: Export detailed metrics for observability
7. **High Availability**: Run multiple load balancer instances behind a load balancer
8. **Containerization**: Package in Docker containers for easy deployment
9. **Orchestration**: Use Kubernetes or similar for scaling and management

## Learning Outcomes

This project demonstrates:

- **Go Concurrency**: Goroutines, channels, mutexes, and atomic operations
- **HTTP Handling**: Custom servers, reverse proxies, middleware, and cookie management
- **System Design**: Multiple load balancing algorithms and session management
- **Web Development**: HTML/CSS dashboard with real-time updates
- **Error Handling**: Proper error propagation and graceful degradation
- **API Design**: RESTful endpoints with JSON responses
- **Production Patterns**: Logging, metrics, monitoring, and health checking
- **Algorithm Implementation**: Weighted routing and sticky session algorithms
- **DevOps Practices**: Docker, Kubernetes, CI/CD, and monitoring

## Next Steps

- [ ] Add SSL/TLS support for secure connections
- [ ] Implement service discovery for dynamic backend registration
- [ ] Add configuration file support for easier deployment
- [ ] Integrate with container orchestration (Docker, Kubernetes)
- [ ] Add circuit breaker pattern for fault tolerance
- [ ] Implement rate limiting for DDoS protection
- [ ] Add authentication/authorization for admin endpoints
- [ ] Implement advanced health check methods (HTTP status, response time)
- [ ] Add support for multiple load balancing strategies per endpoint
- [ ] Create comprehensive unit and integration tests

## 🚀 Production-Ready Features Added

- **✅ Docker Containerization**: Multi-stage build with security hardening
- **✅ Kubernetes Deployment**: Complete K8s manifests with health checks
- **✅ Prometheus Monitoring**: Rich metrics for observability
- **✅ CI/CD Pipeline**: GitHub Actions for automated deployment
- **✅ Health Check Endpoints**: K8s liveness and readiness probes
- **✅ Production Logging**: Structured logging for monitoring
- **✅ Security Best Practices**: Non-root containers, resource limits
- **✅ Scalability**: Horizontal Pod Autoscaling support

**Your load balancer is now enterprise-ready with modern DevOps practices!** 🎉

## Project Structure

```
/go-loadbalancer
│
├── main.go                  # Entry point & load balancer server with dashboard
├── serverpool.go            # Backend server pool management with weighted routing
├── loadbalancer/
│   ├── balancer.go          # Core load balancing logic (round-robin & weighted)
│   ├── server.go            # Mock backend server implementations
│   ├── autoscaler.go        # Auto-scaling functionality (placeholder)
│   ├── healthcheck.go       # Health checking system
│   └── metrics.go           # Metrics collection and reporting
├── go.mod                   # Go module definition
└── README.md                # This file - project documentation
```

## Features

### ✅ Core Features
- **Round-Robin Load Balancing**: Distributes requests evenly across backend servers
- **Weighted Load Balancing**: Backends can have different weights for uneven load distribution
- **Sticky Sessions**: Users are consistently routed to the same backend server
- **Health Checking**: Automatically detects and removes unhealthy backends
- **Auto-scaling**: Dynamically adds new backend servers based on load
- **Request Metrics**: Tracks request counts and server health status
- **Web Dashboard**: Real-time monitoring interface with server status and metrics
- **Comprehensive Logging**: Detailed logging with different log levels
- **REST API**: Multiple endpoints for different functionalities

### 🔧 Advanced Features
- **Thread-Safe Operations**: Uses mutexes and atomic operations for concurrency
- **Session Management**: Cookie-based sticky session implementation
- **Weighted Routing**: Configurable weights for backend server prioritization
- **Graceful Error Handling**: Proper error handling throughout the application
- **Configurable Timeouts**: Health check timeouts and autoscaling intervals
- **Production Ready**: Structured logging and monitoring capabilities

## Quick Start

### Prerequisites
- Go 1.21 or higher
- Terminal/Command prompt

### Installation & Running

1. **Clone/Download the project**
   ```bash
   cd /path/to/go-loadbalancer
   ```

2. **Verify Go installation**
   ```bash
   go version
   ```

3. **Run the load balancer**
   ```bash
   go run main.go serverpool.go
   ```

4. **Verify it's running**
   You should see output like:
   ```
   [INFO] Starting Go Load Balancer Application
   [INFO] Initializing backend servers...
   [INFO] Added backend: http://localhost:8081 (weight: 3)
   [INFO] Added backend: http://localhost:8082 (weight: 2)
   [INFO] Added backend: http://localhost:8083 (weight: 1)
   [INFO] Load balancer starting on :8080
   [INFO] Load balancer is ready to accept connections
   ```

### Testing the Load Balancer

1. **View the dashboard**
   Open your browser and go to: `http://localhost:8080/`
   - Real-time server status and metrics
   - Backend health monitoring
   - Request statistics

2. **Test load balancing**
   ```bash
   curl http://localhost:8080/lb
   ```
   You should see responses from different backend servers with sticky session support.

3. **Check metrics endpoint**
   ```bash
   curl http://localhost:8080/metrics
   ```
   Returns JSON with backend health status and weights.

4. **Generate load to trigger autoscaling**
   ```bash
   for i in {1..25}; do
     curl -s http://localhost:8080/lb &
   done
   ```
   Watch the logs for autoscaling messages when request count exceeds threshold.

## Configuration

### Backend Servers
The load balancer starts with 3 backend servers with different weights:
- http://localhost:8081 (weight: 3) - Handles 50% of traffic
- http://localhost:8082 (weight: 2) - Handles 33% of traffic
- http://localhost:8083 (weight: 1) - Handles 17% of traffic

### Tuning Parameters
You can modify these values in `main.go`:
- **Health Check Interval**: 10 seconds
- **Autoscaling Interval**: 15 seconds
- **Autoscaling Threshold**: 20 requests per interval
- **Health Check Timeout**: 2 seconds
- **Sticky Session Duration**: 1 hour

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Web dashboard with real-time monitoring |
| `/lb` | GET | Load balanced requests with sticky sessions |
| `/metrics` | GET | JSON metrics showing backend health and weights |

## Monitoring

### Dashboard Features
- **Real-time Metrics**: Live request counts and backend status
- **Server Health**: Visual indicators for backend server status
- **Weight Display**: Shows the weight assigned to each backend
- **Feature Status**: Lists all enabled load balancing features
- **Auto-refresh**: Updates every 5 seconds

### Log Levels
- **[INFO]**: General information about application state
- **[DEBUG]**: Detailed debugging information
- **[WARN]**: Warning messages for potentially harmful situations
- **[ERROR]**: Error messages for failed operations

### Key Metrics to Monitor
- Backend server health status (up/down)
- Request routing decisions with session information
- Weighted load distribution statistics
- Autoscaling events
- Sticky session assignments
- Error rates and types

## Quick Start

### Prerequisites
- Go 1.21 or higher
- Terminal/Command prompt

### Installation & Running

1. **Clone/Download the project**
   ```bash
   cd /path/to/go-loadbalancer
   ```

2. **Verify Go installation**
   ```bash
   go version
   ```

3. **Run the load balancer**
   ```bash
   go run main.go serverpool.go
   ```

4. **Verify it's running**
   You should see output like:
   ```
   [INFO] Starting Go Load Balancer Application
   [INFO] Initializing backend servers...
   [INFO] Added backend: http://localhost:8081
   [INFO] Added backend: http://localhost:8082
   [INFO] Added backend: http://localhost:8083
   [INFO] Load balancer starting on :8080
   [INFO] Load balancer is ready to accept connections
   ```

### Testing the Load Balancer

1. **Test basic load balancing**
   ```bash
   curl http://localhost:8080/
   ```

   You should see responses from different backend servers (8081, 8082, 8083).

2. **Check metrics endpoint**
   ```bash
   curl http://localhost:8080/metrics
   ```

   This returns JSON with backend health status.

3. **Generate load to trigger autoscaling**
   ```bash
   for i in {1..25}; do
     curl -s http://localhost:8080/ &
   done
   ```

   Watch the logs for autoscaling messages when request count exceeds threshold.

## Configuration

### Backend Servers
The load balancer starts with 3 backend servers by default:
- http://localhost:8081
- http://localhost:8082
- http://localhost:8083

### Tuning Parameters
You can modify these values in `main.go`:
- **Health Check Interval**: 10 seconds
- **Autoscaling Interval**: 15 seconds
- **Autoscaling Threshold**: 20 requests per interval
- **Health Check Timeout**: 2 seconds

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Load balanced requests to backend servers |
| `/metrics` | GET | JSON metrics showing backend health status |

## Monitoring

### Log Levels
- **[INFO]**: General information about application state
- **[DEBUG]**: Detailed debugging information
- **[WARN]**: Warning messages for potentially harmful situations
- **[ERROR]**: Error messages for failed operations

### Key Metrics to Monitor
- Backend server health status (up/down)
- Request routing decisions
- Autoscaling events
- Error rates and types

## Architecture

### Components

1. **Load Balancer Core** (`main.go`)
   - HTTP server setup and request routing with dashboard
   - Backend server initialization with weighted configuration
   - Metrics collection, autoscaling, and sticky session management
   - Web dashboard serving real-time monitoring interface

2. **Server Pool** (`serverpool.go`)
   - Backend server management with weighted routing support
   - Round-robin and weighted load balancing algorithms
   - Health checking logic with thread-safe operations
   - Sticky session management with cookie-based tracking

3. **Supporting Modules** (`loadbalancer/`)
   - `balancer.go`: Core balancing algorithms (round-robin & weighted)
   - `healthcheck.go`: Health monitoring system
   - `metrics.go`: Metrics collection and JSON API
   - `server.go`: Backend server implementations

### Data Flow

```
Client Request → Load Balancer → Sticky Session Check → Weighted Routing → Backend Server
                                ↓                          ↓
                            Dashboard Display ←─── Health Check ←──┘
                                ↓                          ↓
                            Metrics Collection ←── Auto-scaling Decision
```

### Load Balancing Algorithms

1. **Round-Robin**: Cycles through available backends sequentially
2. **Weighted Routing**: Distributes load based on backend weights
   - Higher weight = more requests
   - Formula: `random % totalWeight` determines selection
3. **Sticky Sessions**: Routes users to the same backend consistently
   - Uses HTTP cookies for session tracking
   - Falls back to weighted routing for new sessions

## Development

### Adding New Features

1. **New Backend Type**: Add to `serverpool.go` or `loadbalancer/`
2. **New Load Balancing Algorithm**: Implement in `balancer.go`
3. **New Health Check Method**: Extend `healthcheck.go`
4. **New Metrics**: Add to `metrics.go`
5. **Dashboard Enhancements**: Modify dashboard HTML in `main.go`

### Testing

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific package tests
go test ./loadbalancer
```

## Troubleshooting

### Common Issues

1. **Port already in use**
   ```bash
   # Find process using port 8080
   lsof -i :8080
   # Kill the process
   kill -9 <PID>
   ```

2. **Backend servers not starting**
   - Check if ports 8081-8083 are available
   - Verify network connectivity
   - Check firewall settings

3. **Import errors**
   ```bash
   go mod tidy
   ```

4. **Dashboard not loading**
   - Ensure no firewall blocking the port
   - Check if the load balancer is running
   - Verify browser cache isn't causing issues

### Debug Mode

The application includes comprehensive logging. For more verbose output, you can modify log levels in the code or add debug flags.

## Production Deployment

For production use, consider:

1. **Configuration Management**: Externalize configuration for different environments
2. **Service Discovery**: Replace hardcoded backend URLs with dynamic discovery
3. **SSL/TLS**: Add HTTPS support for secure communication
4. **Monitoring**: Integrate with monitoring systems (Prometheus, Grafana)
5. **Logging**: Use structured logging (JSON format) for better parsing
6. **Metrics**: Export detailed metrics for observability
7. **High Availability**: Run multiple load balancer instances behind a load balancer
8. **Containerization**: Package in Docker containers for easy deployment
9. **Orchestration**: Use Kubernetes or similar for scaling and management

## Learning Outcomes

This project demonstrates:

- **Go Concurrency**: Goroutines, channels, mutexes, and atomic operations
- **HTTP Handling**: Custom servers, reverse proxies, middleware, and cookie management
- **System Design**: Multiple load balancing algorithms and session management
- **Web Development**: HTML/CSS dashboard with real-time updates
- **Error Handling**: Proper error propagation and graceful degradation
- **API Design**: RESTful endpoints with JSON responses
- **Production Patterns**: Logging, metrics, monitoring, and health checking
- **Algorithm Implementation**: Weighted routing and sticky session algorithms

## Next Steps

- [ ] Add SSL/TLS support for secure connections
- [ ] Implement service discovery for dynamic backend registration
- [ ] Add configuration file support for easier deployment
- [ ] Integrate with container orchestration (Docker, Kubernetes)
- [ ] Add circuit breaker pattern for fault tolerance
- [ ] Implement rate limiting for DDoS protection
- [ ] Add authentication/authorization for admin endpoints
- [ ] Implement advanced health check methods (HTTP status, response time)
- [ ] Add support for multiple load balancing strategies per endpoint
- [ ] Create comprehensive unit and integration tests
