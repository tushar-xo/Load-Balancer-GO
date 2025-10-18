# Go Load Balancer - Production Deployment Guide

## 🚀 Quick Start with Docker

### Build and Run Locally

```bash
# Build the Docker image
docker build -t go-loadbalancer .

# Run with docker-compose (includes Redis for sessions)
docker-compose up -d

# Or run standalone
docker run -p 8080:8080 go-loadbalancer
```

### Test the Deployment

```bash
# Check health
curl http://localhost:8080/health

# View dashboard
open http://localhost:8080/

# Test load balancing
curl http://localhost:8080/lb

# Check metrics
curl http://localhost:8080/metrics | jq .

# Prometheus metrics
curl http://localhost:8080/prometheus
```

## ☸️ Kubernetes Deployment

### Prerequisites

```bash
# Install kubectl and minikube (for local testing)
# Or use your preferred Kubernetes cluster
```

### Deploy to Kubernetes

```bash
# Apply the deployment
kubectl apply -f k8s/deployment.yaml

# Check deployment status
kubectl get pods -l app=go-loadbalancer
kubectl get services

# View logs
kubectl logs -l app=go-loadbalancer

# Scale the deployment
kubectl scale deployment go-loadbalancer --replicas=5
```

### Access the Application

```bash
# Get the service URL
kubectl get services go-loadbalancer-service

# Port forward for local access
kubectl port-forward service/go-loadbalancer-service 8080:80
```

## 📊 Monitoring with Prometheus & Grafana

### Prometheus Metrics

The load balancer exposes metrics at `/prometheus`:

```bash
# View available metrics
curl http://localhost:8080/prometheus

# Key metrics:
# - loadbalancer_requests_total{backend, status}
# - loadbalancer_backend_connections{backend}
# - loadbalancer_request_duration_seconds{backend}
```

### Grafana Dashboard

Create a dashboard with these queries:

```promql
# Total requests per backend
sum(rate(loadbalancer_requests_total[5m])) by (backend)

# Request duration
histogram_quantile(0.95, rate(loadbalancer_request_duration_seconds_bucket[5m]))

# Active connections
loadbalancer_backend_connections
```

## 🔧 Configuration

### Environment Variables

```bash
PORT=8080                    # Server port
HEALTH_CHECK_INTERVAL=10s    # Health check frequency
AUTOSCALER_THRESHOLD=20      # Request threshold for scaling
SESSION_TIMEOUT=3600s        # Sticky session duration
```

### Docker Multi-Stage Build

The Dockerfile uses multi-stage builds for optimal image size:

1. **Builder stage**: Compiles Go application
2. **Runtime stage**: Minimal Alpine Linux with just the binary

### Kubernetes Best Practices

- **Health Checks**: Liveness and readiness probes configured
- **Resource Limits**: CPU and memory limits set
- **Horizontal Pod Autoscaling**: Can scale based on CPU/memory
- **Service Discovery**: LoadBalancer service type for external access

## 🚢 CI/CD Pipeline

The GitHub Actions workflow (`.github/workflows/ci-cd.yml`) provides:

- ✅ **Automated Testing**: Runs on every push/PR
- ✅ **Docker Build**: Multi-stage build with security scanning
- ✅ **Deployment**: Automated deployment to Kubernetes
- ✅ **Quality Gates**: Linting, security checks, and more

## 🔒 Security Considerations

### Production Checklist

- [ ] Enable HTTPS with cert-manager
- [ ] Configure proper RBAC for Kubernetes
- [ ] Set up network policies
- [ ] Implement rate limiting
- [ ] Add authentication/authorization
- [ ] Configure proper logging and auditing
- [ ] Set up alerts and monitoring
- [ ] Implement backup strategies

## 📈 Scaling Strategies

### Horizontal Scaling
```bash
# Scale manually
kubectl scale deployment go-loadbalancer --replicas=10

# HPA based on CPU
kubectl autoscale deployment go-loadbalancer --cpu-percent=70 --min=3 --max=20
```

### Session Affinity
For sticky sessions in Kubernetes:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: go-loadbalancer-sticky
spec:
  selector:
    app: go-loadbalancer
  ports:
  - port: 80
    targetPort: 8080
  sessionAffinity: ClientIP
```

## 🛠️ Development Workflow

1. **Local Development**: Use `docker-compose up` for full stack
2. **Testing**: Run `go test ./...` for unit tests
3. **Building**: `docker build -t go-loadbalancer .`
4. **Deployment**: `kubectl apply -f k8s/`
5. **Monitoring**: Access Grafana dashboard

## 📚 Learning Path

1. **Docker Fundamentals** ✅
2. **Kubernetes Basics** ✅
3. **Service Mesh** (Istio/Linkerd)
4. **API Gateway** (Kong, Ambassador)
5. **Observability** (Jaeger, ELK stack)
6. **Security** (Vault, SPIFFE/SPIRE)
7. **GitOps** (ArgoCD, Flux)

## 🎯 Next Steps

- **Service Mesh**: Add Istio for traffic management
- **API Gateway**: Implement Kong for authentication
- **Observability**: Set up distributed tracing with Jaeger
- **Security**: Implement mTLS with SPIFFE/SPIRE
- **GitOps**: Use ArgoCD for automated deployments

Your load balancer is now **production-ready** with modern DevOps practices! 🚀
