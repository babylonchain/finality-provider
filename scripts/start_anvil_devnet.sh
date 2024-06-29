#!/bin/bash

# Detect the project root directory
PROJECT_ROOT=$(cd "$(dirname "$0")/.." && pwd)

# Check if anvil binary is available
if ! command -v anvil &> /dev/null; then
    echo "Anvil not found. Installing Anvil..."
    curl -L https://foundry.paradigm.xyz | bash
    foundryup
    export PATH=$PATH:$HOME/.foundry/bin
    echo "Anvil installed and added to PATH."
fi

# Check if the anvil process is running
if pgrep -x "anvil" > /dev/null; then
  echo "Anvil is already running, stopping it..."
  pkill -x "anvil"
fi

# Start the anvil process with nohup
echo "Starting anvil process..."
nohup anvil -b 3 --optimism > "$PROJECT_ROOT/anvil.log" 2>&1 &
echo "Anvil process started, outputting to anvil.log"