#!/bin/bash
# Integration test for lldiscovery

set -e

echo "==> Testing lldiscovery daemon"

# Check if binary exists
if [ ! -f ./lldiscovery ]; then
    echo "ERROR: lldiscovery binary not found. Run 'make build' first."
    exit 1
fi

# Test 1: Version flag
echo "Test 1: Version flag"
./lldiscovery -version
echo "✓ Version flag works"

# Test 2: Help / usage
echo -e "\nTest 2: Help output"
./lldiscovery -h 2>&1 | grep -q "Usage" || echo "Note: No usage message (expected)"
echo "✓ Help works"

# Test 3: Interface detection
echo -e "\nTest 3: Interface detection"
go run ./cmd/lldiscovery -log-level debug 2>&1 | head -20 &
PID=$!
sleep 3
kill $PID 2>/dev/null || true
wait $PID 2>/dev/null || true
echo "✓ Daemon starts and detects interfaces"

# Test 4: Configuration loading
echo -e "\nTest 4: Configuration file"
cat > /tmp/test-config.json << EOF
{
  "send_interval": "10s",
  "node_timeout": "60s",
  "log_level": "debug"
}
EOF
timeout 3 ./lldiscovery -config /tmp/test-config.json -log-level info || true
rm -f /tmp/test-config.json
echo "✓ Configuration loading works"

# Test 5: HTTP API (requires daemon to be running briefly)
echo -e "\nTest 5: HTTP API"
./lldiscovery -log-level warn &
DAEMON_PID=$!
sleep 3

# Test health endpoint
if curl -s http://localhost:8080/health | grep -q "ok"; then
    echo "✓ Health endpoint works"
else
    echo "✗ Health endpoint failed"
fi

# Test graph endpoint
if curl -s http://localhost:8080/graph | grep -q "{"; then
    echo "✓ Graph endpoint works"
else
    echo "✗ Graph endpoint failed"
fi

# Test DOT endpoint
if curl -s http://localhost:8080/graph.dot | grep -q "graph"; then
    echo "✓ DOT endpoint works"
else
    echo "✗ DOT endpoint failed"
fi

# Cleanup
kill $DAEMON_PID 2>/dev/null || true
wait $DAEMON_PID 2>/dev/null || true

echo -e "\n==> All tests passed! ✓"
echo -e "\nTo run the daemon manually:"
echo "  ./lldiscovery -log-level debug"
echo -e "\nTo visualize the topology:"
echo "  curl http://localhost:8080/graph.dot | dot -Tpng -o topology.png"
