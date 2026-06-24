#!/bin/bash
# Start Multica Development Environment
# Site: https://testmultica.binguosoft.net
# Usage: ./dev-start.sh

set -e

# Use Go 1.26.1
export PATH=/usr/local/go/bin:$PATH
export GOTOOLCHAIN=auto

cd /home/admin/multica

# Load dev environment
set -a
source <(grep -v '^#' .env.dev | grep -v '^$')
set +a

echo "=== Starting Multica Dev Environment ==="
echo "Frontend: https://testmultica.binguosoft.net (port $FRONTEND_PORT)"
echo "Backend:  http://localhost:$PORT"
echo "Database: $POSTGRES_DB"
echo ""

# Ensure DB is up
docker start multica-postgres-1 2>/dev/null || docker compose up -d postgres

# Run migrations for dev database
echo "Running migrations on $POSTGRES_DB..."
(cd server && go run ./cmd/migrate up)

# Build and start backend in background
echo "Starting backend on :$PORT..."
(cd server && go build -o bin/server ./cmd/server)
./server/bin/server &
BACKEND_PID=$!

# Start frontend dev server
echo "Starting frontend dev server on :$FRONTEND_PORT..."
FRONTEND_PORT=$FRONTEND_PORT pnpm dev:web &
FRONTEND_PID=$!

echo ""
echo "=== Dev environment running ==="
echo "Backend PID:  $BACKEND_PID"
echo "Frontend PID: $FRONTEND_PID"
echo "Access:       https://testmultica.binguosoft.net"
echo ""

# Wait for either to exit
trap "kill $BACKEND_PID $FRONTEND_PID 2>/dev/null; exit" SIGINT SIGTERM
wait
