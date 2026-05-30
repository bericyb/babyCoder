#!/bin/bash

# Script to reset only the 'test1/' directory structure back to its initial state (empty).
# This script wipes all contents of test1/ and recreates required top-level directories.

echo "Starting environment reset for 'test1/'..."

TEST_DIR="test1"
REQUIRED_DIRS=(
    "documents"
    "email"
    "server"
    "services"
    "tests"
    "tickets"
)

echo "Wiping existing contents in ${TEST_DIR}/..."
# 1. Wipe the entire contents of test1/. We use -f to force removal, ignoring non-existent files.
rm -rf "${TEST_DIR}"/*

echo ""
echo "Recreating required directory structure inside ${TEST_DIR}/..."

# 2. Recreate all specified top-level subdirectories inside test1/.
for dir in "${REQUIRED_DIRS[@]}"; do
    mkdir -p "${TEST_DIR}/${dir}"
    echo "Created directory: ${TEST_DIR}/${dir}"
done

echo ""
echo "Test 1 reset complete. The '${TEST_DIR}/' directory structure has been successfully restored to the baseline state."