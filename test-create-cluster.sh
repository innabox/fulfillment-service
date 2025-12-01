#!/bin/bash
set -e

# Simple test script to create a cluster
# Simplified version to get basic functionality working first

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLI_DIR="${CLI_DIR:-$SCRIPT_DIR/../fulfillment-cli}"
DB_NAME="${DB_NAME:-testdb}"
DB_USER="${DB_USER:-user}"
DB_PASS="${DB_PASS:-pass}"
DB_PORT="${DB_PORT:-5432}"
SERVER_PORT="${SERVER_PORT:-8000}"

echo "========================================="
echo "Simple Cluster Creation Test"
echo "========================================="
echo ""

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

success() {
    echo -e "${GREEN}✓${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1"
}

info() {
    echo -e "${YELLOW}→${NC} $1"
}

# Check prerequisites
info "Checking prerequisites..."

if ! command -v podman &> /dev/null; then
    error "podman not found"
    exit 1
fi

if ! command -v jq &> /dev/null; then
    error "jq not found"
    exit 1
fi

if [ ! -d "$CLI_DIR" ]; then
    error "fulfillment-cli not found at: $CLI_DIR"
    exit 1
fi

success "Prerequisites OK"
echo ""

# Setup PostgreSQL
info "Setting up PostgreSQL..."

# Check if container already exists
if podman ps -a --format "{{.Names}}" | grep -q "^postgresql_test_$DB_NAME$"; then
    info "Stopping existing PostgreSQL container..."
    podman stop "postgresql_test_$DB_NAME" 2>/dev/null || true
    podman rm "postgresql_test_$DB_NAME" 2>/dev/null || true
fi

# Start PostgreSQL
info "Starting PostgreSQL container..."
podman run -d --name "postgresql_test_$DB_NAME" \
    -e POSTGRESQL_USER="$DB_USER" \
    -e POSTGRESQL_PASSWORD="$DB_PASS" \
    -e POSTGRESQL_DATABASE="$DB_NAME" \
    -p 127.0.0.1:$DB_PORT:5432 \
    quay.io/sclorg/postgresql-15-c9s:latest > /dev/null

# Wait for PostgreSQL to be ready
info "Waiting for PostgreSQL to be ready..."
for i in {1..30}; do
    if podman exec "postgresql_test_$DB_NAME" pg_isready -U "$DB_USER" &> /dev/null; then
        success "PostgreSQL is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        error "PostgreSQL failed to start"
        exit 1
    fi
    sleep 1
done

echo ""

# Build fulfillment-service
info "Building fulfillment-service..."
cd "$SCRIPT_DIR"
go build -o fulfillment-service . > /dev/null 2>&1
success "Built fulfillment-service"
echo ""

# Start fulfillment-service
info "Starting fulfillment-service..."
DB_URL="postgres://$DB_USER:$DB_PASS@localhost:$DB_PORT/$DB_NAME"

./fulfillment-service start server \
    --log-level=info \
    --grpc-listener-address=localhost:$SERVER_PORT \
    --db-url="$DB_URL" \
    > /tmp/fulfillment-service.log 2>&1 &

SERVER_PID=$!
sleep 3

# Check if server started
if ! kill -0 $SERVER_PID 2>/dev/null; then
    error "fulfillment-service failed to start"
    echo "Check logs at: /tmp/fulfillment-service.log"
    cat /tmp/fulfillment-service.log
    exit 1
fi

success "fulfillment-service is running (PID: $SERVER_PID)"
echo ""

# Cleanup function
cleanup() {
    echo ""
    info "Cleaning up..."

    if [ -n "$SERVER_PID" ]; then
        kill $SERVER_PID 2>/dev/null || true
        success "Stopped fulfillment-service"
    fi

    if [ -n "$CLUSTER_ID" ]; then
        cd "$CLI_DIR"
        ./fulfillment-cli delete cluster "$CLUSTER_ID" 2>/dev/null || true
        success "Deleted test cluster"
    fi

    podman stop "postgresql_test_$DB_NAME" > /dev/null 2>&1 || true
    podman rm "postgresql_test_$DB_NAME" > /dev/null 2>&1 || true
    success "Removed PostgreSQL container"

    rm -f "$SCRIPT_DIR/fulfillment-service"

    echo ""
}

trap cleanup EXIT

# Wait for service to fully initialize
sleep 2

# Build CLI if needed
cd "$CLI_DIR"
if [ ! -f "./fulfillment-cli" ]; then
    info "Building fulfillment-cli..."
    go build . > /dev/null 2>&1
    success "Built fulfillment-cli"
fi

# Configure CLI to use local service
info "Logging in to local service..."
./fulfillment-cli login --address "http://127.0.0.1:$SERVER_PORT" > /dev/null 2>&1
success "Logged in to fulfillment service"

echo ""
echo "Testing cluster creation..."
echo ""

# Use the existing template
TEMPLATE_ID="ocp_4_17_small"
info "Using template: $TEMPLATE_ID"

# Try creating a simple cluster
info "Creating cluster with default settings..."
CLUSTER_NAME="test-cluster-$(date +%s)"

./fulfillment-cli create cluster --name "$CLUSTER_NAME" --template "$TEMPLATE_ID" > /tmp/cluster-result.txt 2>&1

if [ $? -ne 0 ]; then
    error "Failed to create cluster"
    cat /tmp/cluster-result.txt
    exit 1
fi

CLUSTER_ID=$(grep -oP "Created cluster '\K[^']+" /tmp/cluster-result.txt)

if [ -z "$CLUSTER_ID" ]; then
    error "No cluster ID returned"
    cat /tmp/cluster-result.txt
    exit 1
fi

success "Created cluster: $CLUSTER_ID"
echo ""

# Get the cluster to see what we got
sleep 1
info "Retrieving cluster details..."
./fulfillment-cli get cluster "$CLUSTER_ID" 2>/dev/null

success "Cluster retrieved successfully"

echo ""
echo "========================================="
success "SUCCESS: Cluster creation works!"
echo "========================================="
echo ""
