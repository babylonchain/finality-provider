#!/bin/bash

# Check if anvil binary is available
if ! command -v anvil &> /dev/null; then
    exit 1
fi

# Check if the anvil process is running
if pgrep -x "anvil" > /dev/null; then
  echo "Stopping Anvil..."
  pkill -x "anvil"
fi