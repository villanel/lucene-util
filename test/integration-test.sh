#!/bin/bash

set -e

# Integration test script for Lucene Shard Analyzer Service

# Function to log messages with timestamp
log() {
    echo "$(date +"%Y-%m-%d %H:%M:%S") - $1"
}

# Function to run health check test
run_health_check() {
    log "=== Running Health Check Test ==="
    
    local health_status=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/healthz)
    
    if [ "$health_status" -eq 200 ]; then
        log "✓ Health check passed: /healthz returned 200 OK"
        return 0
    else
        log "✗ Health check failed: /healthz returned $health_status"
        return 1
    fi
}

# Function to run load balancing test
run_load_balancing_test() {
    log "=== Running Load Balancing Test ==="
    
    local hostnames=()
    local unique_hostnames
    
    # Call /info 5 times to check load balancing
    for i in {1..5}; do
        local hostname=$(curl -s http://localhost:8080/info | jq -r '.hostname')
        hostnames+=($hostname)
        log "Request $i: Hostname = $hostname"
        sleep 1
    done
    
    # Count unique hostnames
    unique_hostnames=$(echo "${hostnames[@]}" | tr ' ' '\n' | sort -u | wc -l)
    log "Unique hostnames: $unique_hostnames"
    
    if [ "$unique_hostnames" -ge 2 ]; then
        log "✓ Load balancing confirmed: requests served by $unique_hostnames different pods"
        return 0
    else
        log "✗ Load balancing not observed: all requests served by same pod"
        return 1
    fi
}

# Function to run analyze endpoint test
run_analyze_test() {
    log "=== Running Analyze Endpoint Test ==="
    
    local sample_archive
    local analyze_status
    
    # Check if sample data exists, if not, create a simple test archive
    if [ ! -d "test" ]; then
        log "Creating sample test directory structure..."
        mkdir -p test/index
        touch test/index/segments_1
        tar -czf test-shard.tar.gz test
        sample_archive=test-shard.tar.gz
    else
        # Find a sample archive in the test directory
        sample_archive=$(find test -name "*.tar.gz" -o -name "*.zip" | head -1)
        if [ -z "$sample_archive" ]; then
            log "Creating sample test archive..."
            mkdir -p test/index
            touch test/index/segments_1
            tar -czf test-shard.tar.gz test
            sample_archive=test-shard.tar.gz
        fi
    fi
    
    log "Using sample archive: $sample_archive"
    
    analyze_status=$(curl -s -o analyze-response.json -w "%{http_code}" -X POST -H "Content-Type: application/x-gzip" --data-binary @"$sample_archive" http://localhost:8080/analyze)
    
    if [ "$analyze_status" -eq 200 ]; then
        log "✓ Analyze endpoint passed: returned 200 OK"
        log "Response summary:"
        jq -r '.total_segments, .total_docs, .total_deleted_docs' analyze-response.json
        return 0
    else
        log "✗ Analyze endpoint failed: returned $analyze_status"
        cat analyze-response.json
        return 1
    fi
}

# Main test function
main() {
    log "Starting Integration Tests for Lucene Shard Analyzer Service"
    log "Service URL: http://localhost:8080"
    
    local all_passed=true
    
    # Run all tests
    run_health_check || all_passed=false
    run_load_balancing_test || all_passed=false
    run_analyze_test || all_passed=false
    
    # Clean up
    rm -f test-shard.tar.gz analyze-response.json 2>/dev/null
    
    if $all_passed; then
        log "=== ALL TESTS PASSED! ==="
        exit 0
    else
        log "=== SOME TESTS FAILED! ==="
        exit 1
    fi
}

# Execute main function
main
