#!/bin/bash

# Script to run the test suite multiple times and calculate statistics
MAX_ITERATIONS=10
SUCCESS_COUNT=0
FAILURE_COUNT=0

echo "Starting automated test run for $MAX_ITERATIONS iterations..."

for i in $(seq 1 $MAX_ITERATIONS); do
    echo -e "\n=========================================="
    echo "Running Test Iteration $i of $MAX_ITERATIONS"
    echo "=========================================="

    # Execute the original test command sequence
    sh reset_test1.sh && sh run_test1.sh
    
    TEST_EXIT_CODE=$?

    if [ $TEST_EXIT_CODE -eq 0 ]; then
        echo -e "\n[✅ SUCCESS] Test Iteration $i passed."
        SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
    else
        echo -e "\n[❌ FAILURE] Test Iteration $i failed with exit code: $TEST_EXIT_CODE."
        FAILURE_COUNT=$((FAILURE_COUNT + 1))
    fi
done

# Calculate and display results
TOTAL_RUNS=$((SUCCESS_COUNT + FAILURE_COUNT))
SUCCESS_PERCENT=$(awk "BEGIN {printf \"%.2f\", ($SUCCESS_COUNT / $TOTAL_RUNS) * 100}")

echo -e "\n=========================================="
echo "           TEST SUMMARY REPORT"
echo "=========================================="
echo "Total Runs: $TOTAL_RUNS"
echo "Successful Tests: $SUCCESS_COUNT"
echo "Failed Tests: $FAILURE_COUNT"
echo "Success Rate: ${SUCCESS_PERCENT}%"
echo "=========================================="