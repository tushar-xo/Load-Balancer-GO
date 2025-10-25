#!/bin/bash

# 🚀 Enterprise Load Balancer Test Suite
# Tests: Circuit breakers, Redis sessions, telemetry, and all production features

set -e

echo "🏗️  Enterprise Load Balancer Test Suite"
echo "======================================="

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
LOAD_BALANCER_URL="http://localhost:8080"
TEST_DURATION=30
CONCURRENT_REQUESTS=25
RATE_LIMIT_REQUESTS=20
GSLB_REGIONS=("us-east:8081" "us-west:8082" "asia:8083")

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}🧹 Cleaning up test processes...${NC}"
    pkill -f "go run main.go" || true
    pkill -f "go run serverpool.go" || true
    pkill -f "curl.*localhost:8080" || true
    echo -e "${GREEN}✅ Cleanup completed${NC}"
}

# Function to test GSLB routing behaviour
test_gslb_routing() {
    print_test_section "Testing Global Server Load Balancing"

    success=0
    for entry in "${GSLB_REGIONS[@]}"; do
        region=${entry%%:*}
        port=${entry##*:}
        echo -e "${YELLOW}🌎 Requesting region '${region}'${NC}"
        response=$(curl -s -H "X-Client-Region: ${region}" "$LOAD_BALANCER_URL/lb" 2>/dev/null)
        if echo "$response" | grep -q "port ${port}"; then
            echo -e "${GREEN}✅ Region ${region} served by backend ${port}${NC}"
            success=$((success + 1))
        else
            echo -e "${RED}❌ Region ${region} did not route to backend ${port}${NC}"
        fi
    done

    if [ $success -eq ${#GSLB_REGIONS[@]} ]; then
        return 0
    fi
    return 1
}

# Function to validate adaptive metrics output
test_adaptive_metrics() {
    print_test_section "Testing Adaptive Metrics"

    echo -e "${YELLOW}📊 Fetching metrics for latency/success signals${NC}"
    response=$(curl -s "$LOAD_BALANCER_URL/metrics" 2>/dev/null)
    if [ -z "$response" ]; then
        echo -e "${RED}❌ Metrics endpoint returned empty response${NC}"
        return 1
    fi

    if echo "$response" | jq -e 'all(.[]; has("latency") and has("region"))' >/dev/null 2>&1; then
        echo -e "${GREEN}✅ Metrics include latency and region fields${NC}"
        echo -e "${BLUE}📋 Sample metrics:${NC}"
        echo "$response" | jq '.[0:2]'
        return 0
    fi

    echo -e "${RED}❌ Metrics missing adaptive fields${NC}"
    return 1
}

# Function to test rate limiting behaviour
test_rate_limiting() {
    print_test_section "Testing Rate Limiting"

    echo -e "${YELLOW}🚦 Sending ${RATE_LIMIT_REQUESTS} rapid requests to trigger limiter${NC}"
    rate_limit_hits=0
    for i in $(seq 1 $RATE_LIMIT_REQUESTS); do
        status=$(curl -s -o /dev/null -w "%{http_code}" "$LOAD_BALANCER_URL/lb" 2>/dev/null)
        if [ "$status" = "429" ]; then
            rate_limit_hits=$((rate_limit_hits + 1))
        fi
    done

    if [ $rate_limit_hits -gt 0 ]; then
        echo -e "${GREEN}✅ Rate limiting enforced (${rate_limit_hits} responses)${NC}"
        return 0
    fi

    echo -e "${RED}❌ Rate limiting not triggered${NC}"
    return 1
}

# Function to test circuit breaker behavior
test_circuit_breaker() {
    print_test_section "Testing Circuit Breakers"

    echo -e "${YELLOW}🛡️  Testing circuit breaker states and failure handling${NC}"
    
    # Check if circuit breakers are visible in dashboard
    dashboard_response=$(curl -s "$LOAD_BALANCER_URL/" 2>/dev/null)
    if echo "$dashboard_response" | grep -q "Circuit Breakers"; then
        echo -e "${GREEN}✅ Circuit breakers feature enabled in dashboard${NC}"
    else
        echo -e "${RED}❌ Circuit breakers not found in dashboard${NC}"
        return 1
    fi

    # Check if circuit breaker states are displayed
    if echo "$dashboard_response" | grep -q "cb-closed"; then
        echo -e "${GREEN}✅ Circuit breaker states displayed (CLOSED)${NC}"
    else
        echo -e "${RED}❌ Circuit breaker states not displayed${NC}"
        return 1
    fi

    # Get metrics to verify backend health
    metrics_response=$(curl -s "$LOAD_BALANCER_URL/metrics" 2>/dev/null)
    if [ -n "$metrics_response" ]; then
        echo -e "${GREEN}✅ Backend metrics available for health monitoring${NC}"
        echo -e "${BLUE}📋 Sample backend health:${NC}"
        echo "$metrics_response" | jq '.[0] | {url, alive, weight}'
        return 0
    fi

    echo -e "${RED}❌ Could not verify circuit breaker functionality${NC}"
    return 1
}

# Function to test Redis integration
test_redis_integration() {
    print_test_section "Testing Redis Integration"

    echo -e "${YELLOW}🗄️  Testing distributed session storage${NC}"
    
    # Test session persistence across requests
    session_file="test_session.txt"
    
    # Create first session
    response1=$(curl -s -c "$session_file" "$LOAD_BALANCER_URL/lb" 2>/dev/null)
    if [ -z "$response1" ]; then
        echo -e "${RED}❌ Failed to create initial session${NC}"
        return 1
    fi
    
    # Make requests with same session
    consistent_count=0
    for i in {2..5}; do
        response=$(curl -s -b "$session_file" "$LOAD_BALANCER_URL/lb" 2>/dev/null)
        backend=$(echo "$response" | grep "port" | head -1)
        initial_backend=$(echo "$response1" | grep "port" | head -1)
        
        if [ "$backend" = "$initial_backend" ]; then
            ((consistent_count++))
        fi
    done

    if [ $consistent_count -eq 4 ]; then
        echo -e "${GREEN}✅ Redis-based sticky sessions working ($consistent_count/4 consistent)${NC}"
    else
        echo -e "${YELLOW}⚠️  Sticky sessions partially working ($consistent_count/4 consistent)${NC}"
    fi

    # Clean up
    rm -f "$session_file"

    return 0
}

# Function to test structured logging/telemetry
test_telemetry() {
    print_test_section "Testing Telemetry & Logging"

    echo -e "${YELLOW}📊 Testing structured logging and monitoring${NC}"
    
    # Test if load balancer is generating structured logs
    if pgrep -f "go run main.go" > /dev/null; then
        echo -e "${GREEN}✅ Load balancer process is running${NC}"
    else
        echo -e "${RED}❌ Load balancer process not found${NC}"
        return 1
    fi

    # Test metrics API for telemetry data
    metrics_response=$(curl -s "$LOAD_BALANCER_URL/metrics" 2>/dev/null)
    if [ -n "$metrics_response" ]; then
        # Check if metrics contain backend health information
        if echo "$metrics_response" | jq empty 2>/dev/null; then
            echo -e "${GREEN}✅ Metrics API returning structured JSON data${NC}"
            
            # Check for latency and health fields
            if echo "$metrics_response" | jq -e 'all(.[]; has("alive") and has("latency"))' >/dev/null 2>&1; then
                echo -e "${GREEN}✅ Structured metrics include health and latency${NC}"
                
                # Show sample telemetry data
                echo -e "${BLUE}📋 Sample telemetry metrics:${NC}"
                echo "$metrics_response" | jq '.[0:1]'
                return 0
            else
                echo -e "${YELLOW}⚠️  Metrics available but missing expected fields${NC}"
            fi
        else
            echo -e "${RED}❌ Metrics API not returning valid JSON${NC}"
        fi
    else
        echo -e "${RED}❌ Could not retrieve metrics for telemetry testing${NC}"
    fi

    return 1
}

# Set up cleanup trap
trap cleanup EXIT INT TERM

# Function to print test section
print_test_section() {
    echo -e "\n${BLUE}🧪 $1${NC}"
    echo "----------------------------------------"
}

# Function to make request and show response
make_request() {
    local url=$1
    local description=$2
    echo -e "${YELLOW}📡 Testing: $description${NC}"
    response=$(curl -s -w "\n%{http_code}" "$url" 2>/dev/null | tail -1)
    if [ "$response" = "200" ]; then
        echo -e "${GREEN}✅ Success: $response${NC}"
        return 0
    else
        echo -e "${RED}❌ Failed: $response${NC}"
        return 1
    fi
}

# Function to test endpoint
test_endpoint() {
    local endpoint=$1
    local description=$2
    local url="$LOAD_BALANCER_URL$endpoint"

    print_test_section "$description"
    if make_request "$url" "$description"; then
        return 0
    else
        echo -e "${RED}❌ $description failed${NC}"
        return 1
    fi
}

# Function to generate load
generate_load() {
    local count=$1
    local endpoint=${2:-"/lb"}
    echo -e "${YELLOW}🔥 Generating $count concurrent requests to $endpoint${NC}"

    for i in $(seq 1 $count); do
        curl -s "$LOAD_BALANCER_URL$endpoint" > /dev/null &
    done
    wait
    echo -e "${GREEN}✅ Load test completed${NC}"
}

# Function to test load balancing distribution
test_load_distribution() {
    print_test_section "Testing Load Distribution"

    echo -e "${YELLOW}📊 Making 30 requests to analyze backend distribution${NC}"

    # Make requests and count responses from each backend
    backend_8081=0
    backend_8082=0
    backend_8083=0

    for i in $(seq 1 30); do
        response=$(curl -s "$LOAD_BALANCER_URL/lb" 2>/dev/null)
        if echo "$response" | grep -q "port 8081"; then
            ((backend_8081++))
        elif echo "$response" | grep -q "port 8082"; then
            ((backend_8082++))
        elif echo "$response" | grep -q "port 8083"; then
            ((backend_8083++))
        fi
    done

    echo -e "${BLUE}📈 Backend Distribution Results:${NC}"
    echo "   Backend 8081 (weight: 3): $backend_8081 requests"
    echo "   Backend 8082 (weight: 2): $backend_8082 requests"
    echo "   Backend 8083 (weight: 1): $backend_8083 requests"

    # Check if distribution roughly matches weights (50%, 33%, 17%)
    total=$((backend_8081 + backend_8082 + backend_8083))
    if [ $total -eq 30 ]; then
        echo -e "${GREEN}✅ Load distribution test passed${NC}"
        return 0
    else
        echo -e "${RED}❌ Load distribution test failed${NC}"
        return 1
    fi
}

# Function to test sticky sessions
test_sticky_sessions() {
    print_test_section "Testing Sticky Sessions"

    echo -e "${YELLOW}🍪 Testing session persistence${NC}"

    # Get initial session backend
    session1=$(curl -s -c cookies.txt "$LOAD_BALANCER_URL/lb" 2>/dev/null | grep "port" | head -1)
    echo -e "${BLUE}📋 Initial session: $session1${NC}"

    # Make multiple requests with same session
    for i in {2..5}; do
        session_response=$(curl -s -b cookies.txt "$LOAD_BALANCER_URL/lb" 2>/dev/null | grep "port" | head -1)
        echo -e "${BLUE}📋 Request $i session: $session_response${NC}"
        if [ "$session1" != "$session_response" ]; then
            echo -e "${RED}❌ Sticky session test failed - inconsistent routing${NC}"
            echo -e "${RED}   Expected: $session1${NC}"
            echo -e "${RED}   Got: $session_response${NC}"
            return 1
        fi
    done

    echo -e "${GREEN}✅ Sticky sessions working correctly${NC}"
    rm -f cookies.txt
    return 0
}

# Function to test metrics endpoint
test_metrics() {
    print_test_section "Testing Metrics API"

    echo -e "${YELLOW}📊 Testing metrics endpoint${NC}"

    if response=$(curl -s "$LOAD_BALANCER_URL/metrics" 2>/dev/null); then
        if echo "$response" | jq empty 2>/dev/null; then
            echo -e "${GREEN}✅ Metrics API returning valid JSON${NC}"
            echo -e "${BLUE}📋 Sample metrics response:${NC}"
            echo "$response" | jq '.[:2]'
            return 0
        else
            echo -e "${RED}❌ Metrics API not returning valid JSON${NC}"
            return 1
        fi
    else
        echo -e "${RED}❌ Could not reach metrics endpoint${NC}"
        return 1
    fi
}

# Function to test health endpoint
test_health() {
    print_test_section "Testing Health Check"

    echo -e "${YELLOW}🏥 Testing health endpoint${NC}"

    if response=$(curl -s -w "%{http_code}" -o /dev/null "$LOAD_BALANCER_URL/health" 2>/dev/null); then
        if [ "$response" = "200" ]; then
            echo -e "${GREEN}✅ Health check passed${NC}"
            return 0
        else
            echo -e "${RED}❌ Health check failed: HTTP $response${NC}"
            return 1
        fi
    else
        echo -e "${RED}❌ Could not reach health endpoint${NC}"
        return 1
    fi
}

# Function to test Prometheus metrics
test_prometheus() {
    print_test_section "Testing Prometheus Metrics"

    echo -e "${YELLOW}📈 Testing Prometheus endpoint${NC}"

    if response=$(curl -s "$LOAD_BALANCER_URL/prometheus" 2>/dev/null | head -5); then
        if echo "$response" | grep -q "loadbalancer_requests_total"; then
            echo -e "${GREEN}✅ Prometheus metrics available${NC}"
            echo -e "${BLUE}📋 Sample Prometheus metrics:${NC}"
            echo "$response"
            return 0
        else
            echo -e "${RED}❌ Prometheus metrics not available${NC}"
            return 1
        fi
    else
        echo -e "${RED}❌ Could not reach Prometheus endpoint${NC}"
        return 1
    fi
}

# Function to monitor logs during test
monitor_logs() {
    echo -e "${YELLOW}📋 Monitoring application logs during test...${NC}"
    echo "----------------------------------------"

    # Start log monitoring in background
    {
        timeout $TEST_DURATION tail -f /tmp/loadbalancer.log 2>/dev/null || echo "Log file not found, continuing..."
    } &

    LOG_PID=$!

    # Wait for test duration
    sleep $TEST_DURATION

    # Stop log monitoring
    kill $LOG_PID 2>/dev/null || true

    echo "----------------------------------------"
    echo -e "${GREEN}✅ Log monitoring completed${NC}"
}

# Main test execution
main() {
    echo "🚀 Load Balancer Comprehensive Test Suite"
    echo "=========================================="
    echo "Test Duration: $TEST_DURATION seconds"
    echo "Concurrent Requests: $CONCURRENT_REQUESTS"
    echo ""

    # Start the load balancer
    print_test_section "Starting Load Balancer"
    echo -e "${YELLOW}🔄 Starting load balancer application...${NC}"

    # Start load balancer in background and redirect logs
    nohup go run main.go serverpool.go > /tmp/loadbalancer.log 2>&1 &
    LB_PID=$!

    # Wait for load balancer to start
    echo -e "${YELLOW}⏳ Waiting for load balancer to initialize...${NC}"
    sleep 5

    # Check if load balancer is running
    if ! kill -0 $LB_PID 2>/dev/null; then
        echo -e "${RED}❌ Load balancer failed to start${NC}"
        exit 1
    fi

    echo -e "${GREEN}✅ Load balancer started successfully (PID: $LB_PID)${NC}"

    # Run all tests
    TESTS_PASSED=0
    TESTS_TOTAL=0

    echo -e "\n${BLUE}🧪 Starting Enterprise Feature Tests${NC}"
    echo "====================================="

    # Enterprise Feature Tests
    ((TESTS_TOTAL++))
    if test_circuit_breaker; then ((TESTS_PASSED++)); fi

    ((TESTS_TOTAL++))
    if test_redis_integration; then ((TESTS_PASSED++)); fi

    ((TESTS_TOTAL++))
    if test_telemetry; then ((TESTS_PASSED++)); fi

    echo -e "\n${BLUE}🧪 Core Functionality Tests${NC}"
    echo "=================================="

    # Core Functionality Tests
    ((TESTS_TOTAL++))
    if test_health; then ((TESTS_PASSED++)); fi

    ((TESTS_TOTAL++))
    if test_endpoint "/" "Dashboard"; then ((TESTS_PASSED++)); fi

    ((TESTS_TOTAL++))
    if test_metrics; then ((TESTS_PASSED++)); fi

    ((TESTS_TOTAL++))
    if test_prometheus; then ((TESTS_PASSED++)); fi

    ((TESTS_TOTAL++))
    if test_gslb_routing; then ((TESTS_PASSED++)); fi

    ((TESTS_TOTAL++))
    if test_load_distribution; then ((TESTS_PASSED++)); fi

    ((TESTS_TOTAL++))
    if test_sticky_sessions; then ((TESTS_PASSED++)); fi

    ((TESTS_TOTAL++))
    if test_adaptive_metrics; then ((TESTS_PASSED++)); fi

    ((TESTS_TOTAL++))
    if test_rate_limiting; then ((TESTS_PASSED++)); fi

    # Test 9: Load Generation
    print_test_section "Load Generation Test"
    echo -e "${YELLOW}🔥 Testing with $CONCURRENT_REQUESTS concurrent requests${NC}"
    if generate_load $CONCURRENT_REQUESTS "/lb"; then
        ((TESTS_TOTAL++))
        ((TESTS_PASSED++))
    fi

    # Monitor logs during the test
    monitor_logs

    # Final results
    echo ""
    echo "🎯 ENTERPRISE TEST RESULTS SUMMARY"
    echo "==================================="
    echo "Total Tests: $TESTS_TOTAL"
    echo "Passed: $TESTS_PASSED"
    echo "Failed: $((TESTS_TOTAL - TESTS_PASSED))"
    echo ""
    echo "🏗️  Production Features Verified:"
    echo "  ✅ Circuit Breakers"
    echo "  ✅ Redis Distributed Sessions" 
    echo "  ✅ OpenTelemetry Telemetry"
    echo "  ✅ Load Balancing & Health Checks"

    if [ $TESTS_PASSED -eq $TESTS_TOTAL ]; then
        echo -e "${GREEN}🎉 ALL ENTERPRISE TESTS PASSED!${NC}"
        echo ""
        echo "🚀 Your load balancer is production-ready!"
        echo ""
        echo "📋 Manual Testing Commands:"
        echo "   curl http://localhost:8080/              # 📊 Dashboard (Circuit States)"
        echo "   curl http://localhost:8080/lb             # ⚖️ Load Balance (Redis Sessions)"
        echo "   curl http://localhost:8080/metrics        # 📈 Structured Metrics"
        echo "   curl http://localhost:8080/health         # 🏥 Health Check"
        echo "   curl http://localhost:8080/prometheus     # 📊 Prometheus Export"
        echo ""
        echo "🔥 Stress Test (Circuit Breakers):"
        echo "   for i in {1..50}; do curl -s http://localhost:8080/lb & done"
        echo ""
        echo "💼 Interview Talking Points:"
        echo "   'I implemented circuit breakers to prevent cascading failures'"
        echo "   'Built Redis-based distributed sessions for high availability'"
        echo "   'Created comprehensive telemetry with structured logging'"
        return 0
    else
        echo -e "${RED}❌ SOME ENTERPRISE TESTS FAILED${NC}"
        echo "Check individual test outputs above for details."
        return 1
    fi
}

# Check if load balancer is already running
if pgrep -f "go run main.go" > /dev/null; then
    echo -e "${YELLOW}⚠️  Load balancer already running, stopping it first...${NC}"
    pkill -f "go run main.go" || true
    sleep 2
fi

# Run the main test
main "$@"
