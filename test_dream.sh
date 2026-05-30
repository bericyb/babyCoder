#!/bin/bash

# Test script for dream memory system

echo "=== Testing Dream Memory System ==="
echo ""

# Check if dream.txt exists
if [ -f ".babycoder/dream.txt" ]; then
    echo "✓ Dream file exists"
    echo "Content:"
    echo "---"
    cat .babycoder/dream.txt
    echo "---"
else
    echo "✗ Dream file does not exist yet"
fi

echo ""
echo "To test the dream system:"
echo "1. Run: ./babyCoder"
echo "2. Have a conversation about making code changes"
echo "3. Wait 10+ seconds after agent responds"
echo "4. Check .babycoder/dream.txt for updates"
echo ""
echo "Or to test dream loading:"
echo "1. Create a dream manually: echo 'This is a test project' > .babycoder/dream.txt"
echo "2. Run: ./babyCoder"
echo "3. Look for '💭 Project memory loaded' message"
