#!/bin/bash

# Configuration
TARGET_DIR="test1"
TEST_PROMPT="in each of the different directories in this folder create folders named: claude-code, opencode, pi, cline, cursor, antigravity, babyCoder"

# --- Pre-run checks and setup ---

if [ ! -d "$TARGET_DIR" ]; then
    echo "Error: The '$TARGET_DIR' directory was not found. Please ensure it exists."
    exit 1
fi

# Save the current working directory before changing context
START_PWD=$(pwd)

# Change into the test directory
if ! cd "$TARGET_DIR"; then
    echo "Error: Could not change to $TARGET_DIR. Exiting."
    exit 1
fi

# --- Test Execution ---

echo -e "\n--- Starting Test 1 Execution inside ./$TARGET_DIR ---"
# opencode run "$TEST_PROMPT" -m "lmstudio/google/gemma-4-e4b"
opencode run "$TEST_PROMPT" -m "github-copilot/claude-sonnet-4.5"

TEST_EXIT_CODE=$?

if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo -e "\n--- Test Execution Finished Successfully (Exit Code: 0) ---"
else
    echo -e "\n🚨 Test execution failed (Exit Code: $TEST_EXIT_CODE). Review the logs above."
fi

# Capture final state for comparison
tree . > ../final_state.txt
echo "Final structural snapshot saved in final_state.txt."


# --- README Comparison Check using Diff ---

echo -e "\n=========================================="
echo "             🔍 STRUCTURAL DIFF CHECK"
echo "------------------------------------------"

# For the purpose of this test, we compare:
# 1) The actual final state (final_state.txt) against 
# 2) A file containing the expected README structure (expected_POST_test_tree.txt).

if diff -yiw ../final_state.txt ../expected-test1.txt; then
    echo -e "\n[✅ PASS] The directory structure is structurally consistent with the 'Tree output after' specified in README.md."
else
    # If diff outputs differences, it means the structure *doesn't* match what was expected in the README.
    echo -e "\n[❌ FAIL] Significant structural discrepancies found between final state and required README state!"
fi

# Capture final state for comparison
tree . > ../final_state.txt
echo "Final structural snapshot saved in final_state.txt."

TEST_STATUS=0 # Assume success initially

# --- README Comparison Check using Diff ---

echo -e "\n=========================================="
echo "             🔍 STRUCTURAL DIFF CHECK"
echo "------------------------------------------"

if diff -yiw ../final_state.txt ../expected-test1.txt; then
    echo -e "\n[✅ PASS] The directory structure is structurally consistent with the 'Tree output after' specified in README.md."
else
    # If diff outputs differences, it means the structure *doesn't* match what was expected in the README.
    echo -e "\n[❌ FAIL] Significant structural discrepancies found between final state and required README state!"
    TEST_STATUS=1 # Set failure status if diff fails
fi

# Clean up temporary files and restore environment
rm ../final_state.txt 
cd "$START_PWD"

exit $TEST_STATUS # Exit with the determined test status (0 for success, 1 for failure)

echo -e "\n\n--- Test script finished and working directory restored to: $START_PWD ---"
