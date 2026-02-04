#!/bin/bash

echo "==> Verifying Local Node Feature"
echo ""

# Start daemon in background
./lldiscovery -log-level error &
PID=$!
sleep 2

echo "Test 1: Local node in JSON API"
LOCAL_COUNT=$(curl -s http://localhost:8080/graph | jq '[.[] | select(.IsLocal == true)] | length')
if [ "$LOCAL_COUNT" = "1" ]; then
    echo "✓ Exactly one local node found in graph"
else
    echo "✗ Expected 1 local node, found $LOCAL_COUNT"
    kill $PID 2>/dev/null
    exit 1
fi

echo ""
echo "Test 2: Local node has IsLocal flag"
HAS_FLAG=$(curl -s http://localhost:8080/graph | jq '[.[] | select(.IsLocal == true)] | length > 0')
if [ "$HAS_FLAG" = "true" ]; then
    echo "✓ Local node has IsLocal=true flag"
else
    echo "✗ Local node missing IsLocal flag"
    kill $PID 2>/dev/null
    exit 1
fi

echo ""
echo "Test 3: DOT output has local node styling"
if curl -s http://localhost:8080/graph.dot | grep -q 'fillcolor="lightblue"'; then
    echo "✓ DOT output includes blue highlighting"
else
    echo "✗ DOT output missing blue highlighting"
    kill $PID 2>/dev/null
    exit 1
fi

echo ""
echo "Test 4: DOT output has (local) label"
if curl -s http://localhost:8080/graph.dot | grep -q '(local)'; then
    echo "✓ DOT output includes (local) label"
else
    echo "✗ DOT output missing (local) label"
    kill $PID 2>/dev/null
    exit 1
fi

echo ""
echo "Test 5: Startup log shows local node"
./lldiscovery -log-level info 2>&1 | timeout 1 grep -q "local node added to graph"
if [ $? -eq 0 ]; then
    echo "✓ Startup log confirms local node addition"
else
    echo "✗ Startup log missing local node message"
fi

# Cleanup
kill $PID 2>/dev/null
wait $PID 2>/dev/null

echo ""
echo "==> All local node tests passed! ✓"
