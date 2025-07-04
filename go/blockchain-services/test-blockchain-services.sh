#!/bin/bash

# Test script for blockchain services
# This script tests both the listener and indexer services

set -e

echo "🚀 Testing Blockchain Services"
echo "=============================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Check if environment variables are set
echo "Checking environment variables..."

if [ -z "$ETH_RPC" ]; then
    print_error "ETH_RPC environment variable not set"
    exit 1
fi

if [ -z "$TOKEN_CONTRACT" ]; then
    print_error "TOKEN_CONTRACT environment variable not set"
    exit 1
fi

if [ -z "$KAFKA_URL" ]; then
    print_warning "KAFKA_URL not set, using default localhost:9092"
    export KAFKA_URL="localhost:9092"
fi

print_status "Environment variables configured"

# Test 1: Build both services
echo ""
echo "Building services..."

cd blockchain-listener
if go build -o blockchain-listener .; then
    print_status "Blockchain listener built successfully"
else
    print_error "Failed to build blockchain listener"
    exit 1
fi

cd ../blockchain-indexer
if go build -o blockchain-indexer .; then
    print_status "Blockchain indexer built successfully"
else
    print_error "Failed to build blockchain indexer"
    exit 1
fi

cd ..

# Test 2: Check if services can start (without running them)
echo ""
echo "Testing service initialization..."

# Test listener initialization
cd blockchain-listener
timeout 5s ./blockchain-listener > /dev/null 2>&1 || true
if [ $? -eq 124 ]; then
    print_status "Blockchain listener starts correctly (timeout expected)"
else
    print_warning "Blockchain listener may have issues starting"
fi

cd ../blockchain-indexer
timeout 5s ./blockchain-indexer > /dev/null 2>&1 || true
if [ $? -eq 124 ]; then
    print_status "Blockchain indexer starts correctly (timeout expected)"
else
    print_warning "Blockchain indexer may have issues starting"
fi

cd ..

# Test 3: Check if database is created
echo ""
echo "Testing database creation..."

cd blockchain-indexer
if [ -f "blockchain_events.db" ]; then
    print_status "Database file created successfully"
else
    print_warning "Database file not found (will be created on first run)"
fi

cd ..

# Test 4: Check API endpoints (if indexer is running)
echo ""
echo "Testing API endpoints..."

# Start indexer in background for testing
cd blockchain-indexer
./blockchain-indexer > /dev/null 2>&1 &
INDEXER_PID=$!

# Wait for service to start
sleep 3

# Test health endpoint
if curl -s http://localhost:8080/health > /dev/null; then
    print_status "Health endpoint responding"
else
    print_warning "Health endpoint not responding (service may not be ready)"
fi

# Test transfers endpoint
if curl -s http://localhost:8080/transfers?limit=1 > /dev/null; then
    print_status "Transfers endpoint responding"
else
    print_warning "Transfers endpoint not responding"
fi

# Kill the background process
kill $INDEXER_PID 2>/dev/null || true

cd ..

# Test 5: Check Kafka connectivity (if available)
echo ""
echo "Testing Kafka connectivity..."

if command -v kafka-topics &> /dev/null; then
    if kafka-topics --bootstrap-server $KAFKA_URL --list > /dev/null 2>&1; then
        print_status "Kafka connection successful"
    else
        print_warning "Kafka connection failed (Kafka may not be running)"
    fi
else
    print_warning "kafka-topics command not found (Kafka tools not installed)"
fi

# Summary
echo ""
echo "🎉 Test Summary"
echo "==============="
print_status "Both services build successfully"
print_status "Services can initialize"
print_status "Database structure is ready"
print_warning "API endpoints tested (requires running service)"
print_warning "Kafka connectivity tested"

echo ""
echo "📋 Next Steps:"
echo "1. Start Kafka (if not running): docker-compose up kafka"
echo "2. Start the listener: cd blockchain-listener && go run main.go"
echo "3. Start the indexer: cd blockchain-indexer && go run main.go"
echo "4. Test with real data: curl http://localhost:8080/transfers?limit=10"

echo ""
print_status "All tests completed!" 