#!/bin/bash
# TradingAgents Launcher
# Starts both the Go backend and React frontend

set -e
ROOT="/Applications/workspace/ai管力/账本/TradingAgents"
BACKEND_DIR="$ROOT/app/backend"
FRONTEND_DIR="$ROOT/app/frontend"

echo "=== TradingAgents Launcher ==="

# 1. Start Go backend
echo "[1/2] Starting Go backend on :8765..."
cd "$BACKEND_DIR"

# Build if binary doesn't exist
if [ ! -f /tmp/tradingagents-backend ]; then
    echo "  Building backend..."
    go build -buildvcs=false -o /tmp/tradingagents-backend .
fi

# Ensure .env is present
cp "$ROOT/.env" "$BACKEND_DIR/.env" 2>/dev/null || true

# Serve local React dist (not StockGod replay mirror)
export STOCKGOD_LOCAL_FRONTEND=true
unset STOCKGOD_REPLAY

/tmp/tradingagents-backend &
BACKEND_PID=$!
echo "  Backend PID: $BACKEND_PID"

# 2. Start React frontend
echo "[2/2] Starting React frontend on :5173..."
cd "$FRONTEND_DIR"

# Install deps if needed
if [ ! -d node_modules ]; then
    echo "  Installing frontend dependencies..."
    npm install
fi

node ./node_modules/vite/bin/vite.js --host 0.0.0.0 &
FRONTEND_PID=$!
echo "  Frontend PID: $FRONTEND_PID"

sleep 3

# Verify
echo ""
echo "=== Verifying services ==="
curl -s http://localhost:8765/api/health && echo " ✅ Backend OK" || echo " ❌ Backend FAILED"
curl -s -o /dev/null -w "Frontend: HTTP %{http_code}\n" http://localhost:5173/

echo ""
echo "=== Ready ==="
echo "  Frontend: http://localhost:5173"
echo "  Backend:  http://localhost:8765"
echo "  WebSocket: ws://localhost:8765/ws"
echo ""
echo "Press Ctrl+C to stop all services"

# Wait for either process to exit
wait
