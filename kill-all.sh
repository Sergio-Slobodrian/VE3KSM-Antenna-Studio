#!/bin/bash
# Kill all running Antenna Studio processes (backend, frontend, launcher)

echo "Stopping Antenna Studio processes..."

# Kill by known command patterns
pkill -f "cmd/server" 2>/dev/null
pkill -f "cmd/launcher" 2>/dev/null
pkill -f "npx vite" 2>/dev/null
pkill -f "node.*vite" 2>/dev/null

# Kill anything on the backend and frontend ports
for port in 8080 5173; do
    pids=$(lsof -ti :$port 2>/dev/null)
    if [ -n "$pids" ]; then
        echo "  Killing PIDs on port $port: $pids"
        echo "$pids" | xargs kill -9 2>/dev/null
    fi
done

sleep 1

# Verify
remaining=$(lsof -ti :8080,:5173 2>/dev/null)
if [ -n "$remaining" ]; then
    echo "WARNING: Processes still running on ports 8080/5173: $remaining"
    exit 1
else
    echo "All processes stopped. Ports 8080 and 5173 are free."
fi
