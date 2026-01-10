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
    local test_data_dir="./test/test-data"
    
    # Get deployment replica count
    local expected_replicas=$(kubectl get deployment lucene-shard-analyzer -o jsonpath='{.spec.replicas}')
    log "Deployment configured with $expected_replicas replicas"
    
    # Get actual running pods
    local actual_pods=$(kubectl get pods -l app=lucene-shard-analyzer -o jsonpath='{.items[*].metadata.name}' | wc -w)
    log "Found $actual_pods running pods"
    
    # Clean up any existing test pod
    log "Cleaning up existing test pod if any..."
    kubectl delete pod "$test_pod" --grace-period=0 --force > /dev/null 2>&1 || true
    
    # Create test pod
    log "Creating test pod..."
    if ! kubectl run "$test_pod" \
        --image=ubuntu:22.04 \
        --command -- sleep 3600; then
        log "Failed to create test pod"
        return 1
    fi
    
    # Wait for pod to be ready
    log "Waiting for pod to be ready..."
    kubectl wait pod "$test_pod" --for=condition=ready --timeout=120s > /dev/null 2>&1
    
    # Install dependencies in test pod
    log "Installing test dependencies..."
    kubectl exec "$test_pod" -- bash -c "apt-get update > /dev/null && apt-get install -y curl jq > /dev/null" > /dev/null 2>&1
    
    # Create analyze test script using proper quoting
    log "Creating test scripts..."
    cat > /tmp/test_analyze.sh << 'EOF'
#!/bin/bash

if [ $# -eq 0 ]; then
    echo "Usage: $0 <file_path>"
    exit 1
fi

file_path="$1"
echo "=== Analyzing file: $file_path ==="

# Determine content type based on file extension
file_ext="${file_path##*.}"

case "$file_ext" in
    zip) content_type="application/zip" ;;
    tar) content_type="application/x-tar" ;;
    gz) content_type="application/x-gzip" ;;
    *) content_type="application/octet-stream" ;;
esac

echo "Content-Type: $content_type"

# Run analyze request
analyze_output=$(curl -s -X POST -H "Content-Type: $content_type" \
    --data-binary @"$file_path" \
    http://lucene-shard-analyzer-service/analyze 2>/dev/null)

if echo "$analyze_output" | grep -q "total_segments"; then
    echo "✓ Analysis succeeded"
    # Extract summary
    total_segments=$(echo "$analyze_output" | jq -r ".total_segments" 2>/dev/null || echo "N/A")
    total_docs=$(echo "$analyze_output" | jq -r ".total_docs" 2>/dev/null || echo "N/A")
    total_deleted_docs=$(echo "$analyze_output" | jq -r ".total_deleted_docs" 2>/dev/null || echo "N/A")
    total_soft_deleted_docs=$(echo "$analyze_output" | jq -r ".total_soft_deleted_docs" 2>/dev/null || echo "N/A")
    echo "  Total segments: $total_segments"
    echo "  Total docs: $total_docs"
    echo "  Total deleted docs: $total_deleted_docs"
    echo "  Total soft deleted docs: $total_soft_deleted_docs"
    
    # Extract and display segment names
    if [ "$total_segments" -gt 0 ] 2>/dev/null; then
        echo "  Segment names:"
        echo "$analyze_output" | jq -r ".segments[].name" 2>/dev/null | while read -r seg_name; do
            echo "    - $seg_name"
        done
    fi
    analyze_ok=0
else
    echo "✗ Analysis failed"
    echo "  Error: $analyze_output"
    analyze_ok=1
fi

echo -e "\n"

# Use exit instead of return since we're running as a script, not a function
exit $analyze_ok
EOF

    # Create load balancing test script
    cat > /tmp/test_lb.sh << 'EOF'
#!/bin/bash

if [ -z "$EXPECTED_REPLICAS" ] || [ -z "$ACTUAL_PODS" ]; then
    echo "Usage: EXPECTED_REPLICAS=<number> ACTUAL_PODS=<number> $0"
    exit 1
fi

echo "=== Load Balancing Test ==="
echo "Expected replicas: $EXPECTED_REPLICAS"
echo "Actual running pods: $ACTUAL_PODS"

# Load balancing test - make enough requests to ensure all pods are hit
total_requests=$((ACTUAL_PODS * 6))  #  requests per pod
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
    echo "OK: Load Balancing" >> /test_results.txt
else
    echo "✗ Load balancing issue: Received responses from only $unique_hosts pod(s)"
    echo "FAILED: Load Balancing" >> /test_results.txt
fi
EOF

    # Copy scripts to pod
    kubectl cp /tmp/test_analyze.sh "$test_pod:/test_analyze.sh" > /dev/null 2>&1
    kubectl cp /tmp/test_lb.sh "$test_pod:/test_lb.sh" > /dev/null 2>&1
    
    # Make scripts executable
    kubectl exec "$test_pod" -- chmod +x /test_analyze.sh /test_lb.sh
    
    # Create test results file
    kubectl exec "$test_pod" -- touch /test_results.txt
    
    # Process all files in test-data directory
    log "Processing files in $test_data_dir..."
    local test_files=($(ls "$test_data_dir"))
    local total_files=${#test_files[@]}
    local analyze_success_count=0
    
    log "Found $total_files test files to analyze"
    
    for test_file in "${test_files[@]}"; do
        local full_path="$test_data_dir/$test_file"
        local pod_path="/tmp/$test_file"
        
        log "Analyzing file: $test_file"
        
        # Copy test file to pod
        kubectl cp "$full_path" "$test_pod:$pod_path" > /dev/null 2>&1
        
        # Run analyze test on this file, but don't write to results file yet
        analyze_output=$(kubectl exec "$test_pod" -- bash -c "/test_analyze.sh '$pod_path'")
        
        if echo "$analyze_output" | grep -q "✓ Analysis succeeded"; then
            analyze_success_count=$((analyze_success_count+1))
        fi
        
        # Clean up test file
        kubectl exec "$test_pod" -- rm "$pod_path" > /dev/null 2>&1
    done
    
    # Write analyze test results as a single entry
    log "Writing analyze test results as a single entry..."
    if [ $analyze_success_count -eq $total_files ]; then
        kubectl exec "$test_pod" -- bash -c "echo 'OK: All Analyze Tests' >> /test_results.txt"
    else
        kubectl exec "$test_pod" -- bash -c "echo 'FAILED: Analyze Tests - $analyze_success_count/$total_files files passed' >> /test_results.txt"
    fi
    
    # Run load balancing test
    log "Running load balancing test..."
    kubectl exec "$test_pod" -- bash -c "EXPECTED_REPLICAS=$expected_replicas ACTUAL_PODS=$actual_pods /test_lb.sh"
    
    # Display test results summary
    log "=== Test Results Summary ==="
    kubectl exec "$test_pod" -- cat /test_results.txt
    
    # Get final success count
    local results=$(kubectl exec "$test_pod" -- cat /test_results.txt)
    local total_tests=$(echo "$results" | grep -v '^$' | wc -l)
    local passed_tests=$(echo "$results" | grep -c "OK:")
    
    log "Total tests: $total_tests"
    log "Passed tests: $passed_tests"
    log "Failed tests: $((total_tests - passed_tests))"
    
    # Clean up
    log "Cleaning up..."
    kubectl delete pod "$test_pod" --grace-period=0 --force > /dev/null 2>&1
    rm -f /tmp/test_analyze.sh /tmp/test_lb.sh
    
    if [ "$passed_tests" -eq "$total_tests" ]; then
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
    log "Testing analyze endpoint with all files in test-data directory"
    
    # Run tests in a new pod
    run_tests_in_new_pod
    local exit_code=$?
    
    exit $exit_code
}

# Execute main function
main