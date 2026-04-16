#!/bin/bash
# Kill the Antenna Studio server (single Go process).

echo "Stopping Antenna Studio..."

pkill -f "cmd/server" 2>/dev/null
pkill -f "antenna-studio" 2>/dev/null

# Kill anything listening on the default port.
pids=$(lsof -ti :8080 2>/dev/null)
if [ -n "$pids" ]; then
    echo "  Killing PIDs on port 8080: $pids"
    echo "$pids" | xargs kill -9 2>/dev/null
fi

echo "Done."
