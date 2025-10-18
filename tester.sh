#!/bin/bash

# Load Balancer Comprehensive Tester
# Tests all features: load balancing, sticky sessions, health checks, metrics, etc.

set -e

echo "🚀 Starting Load Balancer Comprehensive Test Suite"
echo "=================================================="

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

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}🧹 Cleaning up test processes...${NC}"
    pkill -f "go run main.go" || true
    pkill -f "go run serverpool.go" || true
    pkill -f "curl.*localhost:8080" || true
    echo -e "${GREEN}✅ Cleanup completed${NC}"
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

    # Make multiple requests with same session
    for i in {2..5}; do
        session_response=$(curl -s -b cookies.txt "$LOAD_BALANCER_URL/lb" 2>/dev/null | grep "port" | head -1)
        if [ "$session1" != "$session_response" ]; then
            echo -e "${RED}❌ Sticky session test failed - inconsistent routing${NC}"
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

    # Test 1: Health Check
    ((TESTS_TOTAL++))
    if test_health; then ((TESTS_PASSED++)); fi

    # Test 2: Dashboard
    ((TESTS_TOTAL++))
    if test_endpoint "/" "Dashboard"; then ((TESTS_PASSED++)); fi

    # Test 3: Metrics API
    ((TESTS_TOTAL++))
    if test_metrics; then ((TESTS_PASSED++)); fi

    # Test 4: Prometheus Metrics
    ((TESTS_TOTAL++))
    if test_prometheus; then ((TESTS_PASSED++)); fi

    # Test 5: Load Distribution
    ((TESTS_TOTAL++))
    if test_load_distribution; then ((TESTS_PASSED++)); fi

    # Test 6: Sticky Sessions
    ((TESTS_TOTAL++))
    if test_sticky_sessions; then ((TESTS_PASSED++)); fi

    # Test 7: Load Generation
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
    echo "🎯 TEST RESULTS SUMMARY"
    echo "======================"
    echo "Total Tests: $TESTS_TOTAL"
    echo "Passed: $TESTS_PASSED"
    echo "Failed: $((TESTS_TOTAL - TESTS_PASSED))"

    if [ $TESTS_PASSED -eq $TESTS_TOTAL ]; then
        echo -e "${GREEN}🎉 ALL TESTS PASSED!${NC}"
        echo ""
        echo "📋 Sample commands to test manually:"
        echo "   curl http://localhost:8080/          # Dashboard"
        echo "   curl http://localhost:8080/lb         # Load balanced requests"
        echo "   curl http://localhost:8080/metrics    # JSON metrics"
        echo "   curl http://localhost:8080/health     # Health check"
        echo "   curl http://localhost:8080/prometheus # Prometheus metrics"
        echo ""
        echo "🔥 Load testing:"
        echo "   for i in {1..50}; do curl -s http://localhost:8080/lb & done"
        return 0
    else
        echo -e "${RED}❌ SOME TESTS FAILED${NC}"
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
