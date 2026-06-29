#!/bin/bash
set -e

echo "=== InsightPilot E2E Test Runner ==="

# Kill any existing processes on our ports
kill $(lsof -t -i:3000) 2>/dev/null || true
sleep 1

# Build the backend if not already built
cd "$(dirname "$0")/../.."
if [ ! -f server_bin ]; then
  echo "Building Go backend..."
  go build -o server_bin ./cmd/server
fi

# Ensure the out directory exists for frontend assets
if [ ! -d frontend/out ]; then
  echo "Building frontend..."
  cd frontend && npm run build && cd ..
fi

echo "Starting Go backend on port 3000 (serves frontend + API)..."
./server_bin &
BACKEND_PID=$!

# Wait for backend to be ready
echo "Waiting for backend..."
for i in $(seq 1 60); do
  if curl -s http://127.0.0.1:3000/api/health > /dev/null 2>&1; then
    echo "Backend is ready!"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "Backend failed to start"
    kill $BACKEND_PID 2>/dev/null || true
    exit 1
  fi
  sleep 1
done

# Wait for frontend to be served by Go (it might have a cold-start delay)
echo "Waiting for frontend at http://localhost:3000..."
for i in $(seq 1 30); do
  if curl -s http://localhost:3000 > /dev/null 2>&1; then
    echo "Frontend is ready!"
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "Frontend failed to start"
    kill $BACKEND_PID 2>/dev/null || true
    exit 1
  fi
  sleep 1
done

echo "Running Playwright tests against http://localhost:3000..."
cd frontend
set +e
BASE_URL=http://localhost:3000 npx playwright test --config=playwright.config.ts "$@"
EXIT_CODE=$?
set -e

echo "Cleaning up..."
kill $BACKEND_PID 2>/dev/null || true
wait $BACKEND_PID 2>/dev/null || true

echo "=== E2E tests complete (exit code: $EXIT_CODE) ==="
exit $EXIT_CODE
