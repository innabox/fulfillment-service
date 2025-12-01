#!/bin/bash
set -e

# Test script for node set removal fix
# Tests https://github.com/innabox/issues/issues/251

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLI_DIR="${CLI_DIR:-$SCRIPT_DIR/../fulfillment-cli}"
DB_NAME="${DB_NAME:-testdb}"
DB_USER="${DB_USER:-user}"
DB_PASS="${DB_PASS:-pass}"
DB_PORT="${DB_PORT:-5432}"
SERVER_PORT="${SERVER_PORT:-8000}"

echo "========================================="
echo "Node Set Removal Test"
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
    error "podman not found - required to run PostgreSQL"
    exit 1
fi

if ! command -v jq &> /dev/null; then
    error "jq not found - required to parse JSON"
    exit 1
fi

if ! command -v grpcurl &> /dev/null; then
    error "grpcurl not found - required to call gRPC APIs"
    exit 1
fi

if [ ! -d "$CLI_DIR" ]; then
    error "fulfillment-cli not found at: $CLI_DIR"
    echo "  Set CLI_DIR environment variable to the correct path"
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
    --log-level=warn \
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

# Wait a bit more for service to fully initialize
sleep 2

# Run the test
info "Running test scenario..."
echo ""

cd "$CLI_DIR"

# Build CLI if needed
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
echo "Step 1: Creating cluster..."
echo ""

CLUSTER_NAME="test-nodeset-removal-$(date +%s)"
TEMPLATE_ID="ocp_4_17_small"

info "Creating cluster with template: $TEMPLATE_ID"
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
echo "Step 2: Adding a second node set to the cluster..."

# Get cluster and extract node sets info - retry a few times if needed
for i in {1..5}; do
    sleep 2
    info "Retrieving cluster (attempt $i)..."
    ./fulfillment-cli get cluster "$CLUSTER_ID" --output json > /tmp/cluster-current.json 2>&1

    # Check if we got valid data
    if jq -e 'if type == "array" then .[0].spec else .spec end' /tmp/cluster-current.json > /dev/null 2>&1; then
        success "Retrieved cluster"
        break
    fi

    if [ $i -eq 5 ]; then
        error "Failed to retrieve cluster after 5 attempts"
        cat /tmp/cluster-current.json
        exit 1
    fi
done

NODESETS=$(jq -r 'if type == "array" then .[0].spec.node_sets else .spec.node_sets end | keys | length' /tmp/cluster-current.json 2>/dev/null)
NODESET_NAMES=$(jq -r 'if type == "array" then .[0].spec.node_sets else .spec.node_sets end | keys | join(", ")' /tmp/cluster-current.json 2>/dev/null)

echo "  Initial node sets count: $NODESETS"
echo "  Initial node sets: $NODESET_NAMES"

# Add a storage node set
info "Adding 'storage' node set to cluster..."
jq 'if type == "array" then .[0] else . end | {"@type": "type.googleapis.com/fulfillment.v1.Cluster", id, metadata, spec: (.spec | .node_sets.storage = {"host_class": "worker", "size": 2})}' /tmp/cluster-current.json > /tmp/cluster-with-storage.json

./fulfillment-cli update --filename /tmp/cluster-with-storage.json > /tmp/update-add-result.txt 2>&1

if [ $? -ne 0 ]; then
    error "Failed to add storage node set"
    cat /tmp/update-add-result.txt
    exit 1
fi

success "Added 'storage' node set"
sleep 1

# Verify we now have 2 node sets
./fulfillment-cli get cluster "$CLUSTER_ID" --output json > /tmp/cluster-current.json 2>&1
NODESETS=$(jq -r 'if type == "array" then .[0].spec.node_sets else .spec.node_sets end | keys | length' /tmp/cluster-current.json 2>/dev/null)
NODESET_NAMES=$(jq -r 'if type == "array" then .[0].spec.node_sets else .spec.node_sets end | keys | join(", ")' /tmp/cluster-current.json 2>/dev/null)

echo "  Updated node sets count: $NODESETS"
echo "  Updated node sets: $NODESET_NAMES"

if [ "$NODESETS" != "2" ]; then
    error "Expected 2 node sets after adding storage, got $NODESETS"
    exit 1
fi

success "Cluster now has 2 node sets"

echo ""
echo "Step 3: Editing cluster to remove 'storage' node set..."
info "This is where the bug was - the node set would reappear after removal"
echo ""

# Get current cluster and modify to remove storage node set
jq 'if type == "array" then .[0] else . end | {"@type": "type.googleapis.com/fulfillment.v1.Cluster", id, metadata, spec: (.spec | del(.node_sets.storage))}' /tmp/cluster-current.json > /tmp/cluster-edited.json

# Update the cluster using the CLI
./fulfillment-cli update --filename /tmp/cluster-edited.json > /tmp/update-result.txt 2>&1

if [ $? -ne 0 ]; then
    error "Failed to update cluster"
    cat /tmp/update-result.txt
    echo ""
    info "Cluster edit spec:"
    cat /tmp/cluster-edited.json
    exit 1
fi

success "Edit command completed"
echo ""

echo "Step 4: Monitoring cluster to ensure 'storage' node set stays removed..."
info "Checking every 5 seconds for 30 seconds to ensure it doesn't reappear"
echo ""

FAILED=false
for i in {1..7}; do
    if [ $i -gt 1 ]; then
        sleep 5
    fi

    ELAPSED=$((($i - 1) * 5))
    echo -n "  [${ELAPSED}s] Checking... "

    ./fulfillment-cli get cluster "$CLUSTER_ID" --output json > /tmp/cluster-check.json 2>&1

    NODESETS_NOW=$(jq -r 'if type == "array" then .[0].spec.node_sets else .spec.node_sets end | keys | length' /tmp/cluster-check.json 2>/dev/null)
    HAS_STORAGE=$(jq -r 'if type == "array" then .[0].spec.node_sets else .spec.node_sets end | has("storage")' /tmp/cluster-check.json 2>/dev/null)
    NODESET_NAMES=$(jq -r 'if type == "array" then .[0].spec.node_sets else .spec.node_sets end | keys | join(", ")' /tmp/cluster-check.json 2>/dev/null)

    if [ "$HAS_STORAGE" = "true" ]; then
        echo -e "${RED}storage reappeared!${NC}"
        error "FAILED: 'storage' node set reappeared after ${ELAPSED} seconds!"
        echo ""
        echo "Current node sets:"
        jq -r 'if type == "array" then .[0].spec.node_sets else .spec.node_sets end' /tmp/cluster-check.json
        FAILED=true
        break
    fi

    if [ "$NODESETS_NOW" != "1" ]; then
        echo -e "${RED}wrong count: $NODESETS_NOW${NC}"
        error "FAILED: Expected 1 node set, got $NODESETS_NOW"
        echo ""
        echo "Current node sets:"
        jq -r 'if type == "array" then .[0].spec.node_sets else .spec.node_sets end' /tmp/cluster-check.json
        FAILED=true
        break
    fi

    echo -e "${GREEN}OK${NC} (node sets: $NODESET_NAMES)"
done

echo ""

if [ "$FAILED" = "true" ]; then
    echo "========================================="
    error "FAILED: Node set removal bug reproduced!"
    echo "========================================="
    echo ""
    info "The fix is NOT working correctly."
    exit 1
fi

echo "========================================="
success "SUCCESS: Node set stayed removed for 30+ seconds!"
echo "========================================="
echo ""
info "The fix is working correctly - 'storage' node set was removed and did not reappear."
echo ""
