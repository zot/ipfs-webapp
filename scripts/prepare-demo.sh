#!/bin/bash
# Prepare demo site for bundling

set -e

DEMO_STAGING="build/demo-staging"

echo "Preparing demo site for bundling..."

# Clean and create staging directory
rm -rf "$DEMO_STAGING"
mkdir -p "$DEMO_STAGING/html"
mkdir -p "$DEMO_STAGING/ipfs"
mkdir -p "$DEMO_STAGING/storage"

# Copy demo files to html/
cp internal/commands/demo/* "$DEMO_STAGING/html/" 2>/dev/null || true

# Remove Go source files if any
rm -f "$DEMO_STAGING/html"/*.go

echo "Demo site prepared in $DEMO_STAGING"
