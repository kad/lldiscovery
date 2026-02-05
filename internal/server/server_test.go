package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"kad.name/lldiscovery/internal/graph"
)

func createTestGraph() *graph.Graph {
	g := graph.New()

	// Create local node
	g.SetLocalNode("local-123", "local-host", map[string]graph.InterfaceDetails{
		"eth0": {
			IPAddress:  "fe80::1",
			RDMADevice: "",
			Speed:      1000,
		},
		"eth1": {
			IPAddress:  "fe80::2",
			RDMADevice: "mlx5_0",
			Speed:      10000,
		},
	})

	// Add remote nodes with direct edges
	g.AddOrUpdate("remote-456", "remote-1", "eth0", "fe80::3", "eth0", "", "", "", 1000, nil, true, "")
	g.AddOrUpdate("remote-789", "remote-2", "eth0", "fe80::4", "eth0", "", "", "", 1000, nil, true, "")

	// Add an indirect edge (remote-1 knows about remote-2)
	// neighborMachineID, neighborHostname, neighborIface, neighborAddress,
	// neighborRDMA, neighborNodeGUID, neighborSysImageGUID, neighborSpeed, neighborPrefixes,
	// intermediateIface, intermediateAddress, intermediateRDMA, intermediateNodeGUID, intermediateSysImageGUID, intermediateSpeed, intermediatePrefixes,
	// learnedFrom
	g.AddOrUpdateIndirectEdge("remote-789", "remote-2", "eth0", "fe80::4", "", "", "", 1000, nil, "eth0", "fe80::3", "", "", "", 1000, nil, "remote-456")

	return g
}

func TestHandleGraph(t *testing.T) {
	g := createTestGraph()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	s := New(":0", g, logger, false)

	req := httptest.NewRequest(http.MethodGet, "/graph", nil)
	w := httptest.NewRecorder()

	s.handleGraph(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify nodes are present
	if _, ok := response["nodes"]; !ok {
		t.Error("expected 'nodes' in response")
	}

	// Verify edges are present
	if _, ok := response["edges"]; !ok {
		t.Error("expected 'edges' in response")
	}

	// Verify segments are NOT present (showSegments is false)
	if _, ok := response["segments"]; ok {
		t.Error("expected 'segments' to be absent when showSegments=false")
	}

	// Verify we can parse nodes
	nodesData, err := json.Marshal(response["nodes"])
	if err != nil {
		t.Fatalf("failed to marshal nodes: %v", err)
	}
	var nodes map[string]*graph.Node
	if err := json.Unmarshal(nodesData, &nodes); err != nil {
		t.Fatalf("failed to unmarshal nodes: %v", err)
	}

	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(nodes))
	}

	// Verify we can parse edges
	edgesData, err := json.Marshal(response["edges"])
	if err != nil {
		t.Fatalf("failed to marshal edges: %v", err)
	}
	var edges map[string]map[string][]*graph.Edge
	if err := json.Unmarshal(edgesData, &edges); err != nil {
		t.Fatalf("failed to unmarshal edges: %v", err)
	}

	if len(edges) == 0 {
		t.Error("expected at least one edge")
	}
}

func TestHandleGraphWithSegments(t *testing.T) {
	g := createTestGraph()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	s := New(":0", g, logger, true) // Enable segments

	req := httptest.NewRequest(http.MethodGet, "/graph", nil)
	w := httptest.NewRecorder()

	s.handleGraph(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify segments are present (showSegments is true)
	if _, ok := response["segments"]; !ok {
		t.Error("expected 'segments' in response when showSegments=true")
	}

	// Verify we can parse segments
	segmentsData, err := json.Marshal(response["segments"])
	if err != nil {
		t.Fatalf("failed to marshal segments: %v", err)
	}
	var segments []graph.NetworkSegment
	if err := json.Unmarshal(segmentsData, &segments); err != nil {
		t.Fatalf("failed to unmarshal segments: %v", err)
	}
}

func TestHandleGraphDOT(t *testing.T) {
	g := createTestGraph()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	s := New(":0", g, logger, false)

	req := httptest.NewRequest(http.MethodGet, "/graph.dot", nil)
	w := httptest.NewRecorder()

	s.handleGraphDOT(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/vnd.graphviz" {
		t.Errorf("expected Content-Type text/vnd.graphviz, got %s", contentType)
	}

	body := w.Body.String()
	if len(body) == 0 {
		t.Error("expected non-empty DOT output")
	}

	if body[:6] != "graph " {
		t.Error("expected DOT output to start with 'graph '")
	}
}

func TestHandleHealth(t *testing.T) {
	g := graph.New()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	s := New(":0", g, logger, false)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	s.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%s'", response["status"])
	}
}

func TestHandleGraphMethodNotAllowed(t *testing.T) {
	g := graph.New()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	s := New(":0", g, logger, false)

	req := httptest.NewRequest(http.MethodPost, "/graph", nil)
	w := httptest.NewRecorder()

	s.handleGraph(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleGraphNwdiag(t *testing.T) {
	g := createTestGraph()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	s := New(":0", g, logger, true) // Enable segments

	req := httptest.NewRequest(http.MethodGet, "/graph.nwdiag", nil)
	w := httptest.NewRecorder()

	s.handleGraphNwdiag(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/plain; charset=utf-8" {
		t.Errorf("expected Content-Type text/plain; charset=utf-8, got %s", contentType)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty response body")
	}

	// Verify nwdiag structure
	if !contains(body, "@startuml") {
		t.Error("expected @startuml in nwdiag output")
	}
	if !contains(body, "nwdiag {") {
		t.Error("expected nwdiag opening")
	}
	if !contains(body, "@enduml") {
		t.Error("expected @enduml in nwdiag output")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

