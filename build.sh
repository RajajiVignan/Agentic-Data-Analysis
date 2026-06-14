#!/bin/bash
# build.sh — Build the complete InsightPilot application
# Produces: server_bin (Go binary) + frontend/out/ (static Next.js export)
# Usage: ./build.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=== Building InsightPilot ==="

# 1. Build Next.js frontend (static export)
echo "[1/2] Building Next.js frontend..."
cd "$SCRIPT_DIR/frontend"
npm run build
echo "  -> Static export: $SCRIPT_DIR/frontend/out/"

# 2. Build Go backend
echo "[2/2] Building Go backend..."
cd "$SCRIPT_DIR"
go build -o server_bin ./cmd/server
echo "  -> Binary: $SCRIPT_DIR/server_bin"

echo ""
echo "=== Build complete ==="
echo "Run: $SCRIPT_DIR/server_bin"
echo "Then open: http://127.0.0.1:3000"
