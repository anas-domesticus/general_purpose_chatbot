#!/bin/bash

# Health check test script
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Testing Health Check Endpoints${NC}"
echo ""

# Default values
HOST="${HOST:-localhost}"
PORT="${PORT:-8080}"
BASE_URL="http://${HOST}:${PORT}"

# Function to test endpoint
test_endpoint() {
    local endpoint="$1"
    local description="$2"
    local url="${BASE_URL}${endpoint}"
    
    echo -n "Testing ${description} (${endpoint})... "
    
    if response=$(curl -s -w "%{http_code}" -o /tmp/health_response.json "$url" 2>/dev/null); then
        status_code="${response}"
        
        if [ "$status_code" = "200" ]; then
            echo -e "${GREEN}✓ PASS${NC}"
            
            # Pretty print JSON response if jq is available
            if command -v jq >/dev/null 2>&1; then
                echo "Response:"
                cat /tmp/health_response.json | jq '.' | sed 's/^/  /'
            else
                echo "Response: $(cat /tmp/health_response.json)"
            fi
            echo ""
            return 0
        else
            echo -e "${RED}✗ FAIL (HTTP $status_code)${NC}"
            echo "Response: $(cat /tmp/health_response.json)"
            echo ""
            return 1
        fi
    else
        echo -e "${RED}✗ FAIL (Connection error)${NC}"
        echo ""
        return 1
    fi
}

# Check if server is running
echo -n "Checking if server is running at ${BASE_URL}... "
if curl -s "$BASE_URL" >/dev/null 2>&1; then
    echo -e "${GREEN}✓ Server is responding${NC}"
else
    echo -e "${RED}✗ Server is not responding${NC}"
    echo ""
    echo "Make sure the server is running:"
    echo "  go run cmd/chatbot/main.go web"
    echo "  or"
    echo "  docker-compose up -d"
    exit 1
fi
echo ""

# Test all health endpoints
failed_tests=0

test_endpoint "/health" "Combined Health Status" || ((failed_tests++))
test_endpoint "/health/live" "Liveness Probe" || ((failed_tests++))
test_endpoint "/health/ready" "Readiness Probe" || ((failed_tests++))

# Summary
echo "========================================"
if [ $failed_tests -eq 0 ]; then
    echo -e "${GREEN}All health check tests passed! ✓${NC}"
    echo ""
    echo "Your application is ready for production deployment."
    echo "Health endpoints are working correctly."
else
    echo -e "${RED}$failed_tests health check test(s) failed! ✗${NC}"
    echo ""
    echo "Please check the server logs and configuration."
    exit 1
fi

# Cleanup
rm -f /tmp/health_response.json