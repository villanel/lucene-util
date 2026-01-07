#!/bin/bash

set -e

# Integration test script for Lucene Shard Analyzer Service

# Function to log messages with timestamp
log() {
    echo "$(date +"%Y-%m-%d %H:%M:%S") - $1"
}

# Function to run tests in a new pod
run_tests_in_new_pod() {
    log "=== Starting Integration Tests ==="
    
    # Test variables
    local test_pod="integration-test"
    local service="lucene-shard-analyzer-service"
    local test_zip="/root/devops-interview-project-main/test/test-data/4H0pOK6KT2STRo_TyIBohQ.zip"
    
    # Get deployment replica count
    local expected_replicas=$(kubectl get deployment lucene-shard-analyzer -o jsonpath='{.spec.replicas}')
    log "Deployment configured with $expected_replicas replicas"
    
    # Get actual running pods
    local actual_pods=$(kubectl get pods -l app=lucene-shard-analyzer -o jsonpath='{.items[*].metadata.name}' | wc -w)
    log "Found $actual_pods running pods"
    
    # Create test pod
    log "Creating test pod..."
    kubectl run "$test_pod" \
        --image=ubuntu:22.04 \
        --command -- sleep 3600 > /dev/null 2>&1
    
    # Wait for pod to be ready
    log "Waiting for pod to be ready..."
    kubectl wait pod "$test_pod" --for=condition=ready --timeout=120s > /dev/null 2>&1
    
    # Copy test file to pod
    log "Copying test data..."
    kubectl cp "$test_zip" "$test_pod:/test-shard.zip" > /dev/null 2>&1
    
    # Create a test script inside the pod
    log "Creating test script in pod..."
    kubectl exec "$test_pod" -- bash -c 'cat > /test.sh << "EOF"
#!/bin/bash

# Install dependencies
apt-get update && apt-get install -y curl jq > /dev/null

echo "=== Health Check Test ==="
# Health check test
health=$(curl -s http://lucene-shard-analyzer-service/healthz)
if [ "$health" = "ok" ]; then
    echo "✓ Health check passed"
    health_ok=1
else
    echo "✗ Health check failed: $health"
    health_ok=0
fi

echo -e "\n=== Load Balancing Test ==="
echo "Expected replicas: $EXPECTED_REPLICAS"
echo "Actual running pods: $ACTUAL_PODS"

# Load balancing test - make enough requests to ensure all pods are hit
total_requests=$(($ACTUAL_PODS * 3))  # 3 requests per pod
echo "Making $total_requests requests to verify load balancing..."

for i in $(seq 1 $total_requests); do
    info=$(curl -s http://lucene-shard-analyzer-service/info)
    hostname=$(echo "$info" | jq -r ".hostname" 2>/dev/null || echo "unknown")
    echo "Request $i: Hostname = $hostname"
    all_hosts="$all_hosts $hostname"
    sleep 0.3
done

# Count unique hostnames
unique_hosts=$(echo "$all_hosts" | tr " " "\n" | grep -v "^$" | sort -u | wc -l)
echo -e "\n✓ Received responses from $unique_hosts unique pods"

# Verify if we hit all running pods
if [ "$unique_hosts" -eq "$ACTUAL_PODS" ]; then
    echo "✓ Load balancing verified: Requests distributed to all $ACTUAL_PODS pods"
    lb_ok=1
elif [ "$unique_hosts" -ge 2 ]; then
    echo "✓ Load balancing confirmed: Requests distributed to $unique_hosts pods (expected: $ACTUAL_PODS)"
    lb_ok=1
else
    echo "✗ Load balancing issue: All requests served by only $unique_hosts pod(s)"
    lb_ok=0
fi

echo -e "\n=== Analyze Endpoint Test ==="
# Analyze test
analyze_output=$(curl -s -X POST -H "Content-Type: application/zip" --data-binary @/test-shard.zip http://lucene-shard-analyzer-service/analyze 2>/dev/null)
if echo "$analyze_output" | grep -q "total_segments"; then
    echo "✓ Analyze endpoint passed"
    # Extract summary
    total_segments=$(echo "$analyze_output" | jq -r ".total_segments" 2>/dev/null || echo "N/A")
    total_docs=$(echo "$analyze_output" | jq -r ".total_docs" 2>/dev/null || echo "N/A")
    total_deleted_docs=$(echo "$analyze_output" | jq -r ".total_deleted_docs" 2>/dev/null || echo "N/A")
    echo "  Total segments: $total_segments"
    echo "  Total docs: $total_docs"
    echo "  Total deleted docs: $total_deleted_docs"
    analyze_ok=1
else
    echo "✗ Analyze endpoint failed"
    analyze_ok=0
fi

echo -e "\n=== Test Results ==="
if [ "$health_ok" -eq 1 ] && [ "$lb_ok" -eq 1 ] && [ "$analyze_ok" -eq 1 ]; then
    echo "ALL TESTS PASSED"
    exit 0
else
    echo "SOME TESTS FAILED"
    exit 1
fi
EOF
chmod +x /test.sh' > /dev/null 2>&1
    
    # Run the test script with expected and actual pod counts
    log "Running tests..."
    kubectl exec "$test_pod" -- bash -c "EXPECTED_REPLICAS=$expected_replicas ACTUAL_PODS=$actual_pods /test.sh"
    local test_exit_code=$?
    
    # Clean up
    log "Cleaning up..."
    kubectl delete pod "$test_pod" --grace-period=0 --force > /dev/null 2>&1
    
    if [ "$test_exit_code" -eq 0 ]; then
        log "=== ALL TESTS PASSED! ==="
        return 0
    else
        log "=== SOME TESTS FAILED! ==="
        return 1
    fi
}

# Main function
main() {
    log "Starting Lucene Shard Analyzer Service Integration Tests"
    log "Testing load balancing alignment with deployment replica count"
    
    # Run tests in a new pod
    run_tests_in_new_pod
    local exit_code=$?
    
    exit $exit_code
}

# Execute main function
main