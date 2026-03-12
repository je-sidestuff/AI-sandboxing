#!/bin/bash

# DevContainer initialization script
# This script runs on the host before the container is created

echo "Initializing devcontainer..."

# Ensure required directories exist on host
mkdir -p ~/.aws ~/.claude ~/.ssh ~/.exreg

echo "DevContainer initialization complete."