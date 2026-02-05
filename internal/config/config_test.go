package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.SendInterval != 30*time.Second {
		t.Errorf("Expected default send interval 30s, got %v", cfg.SendInterval)
	}

	if cfg.NodeTimeout != 120*time.Second {
		t.Errorf("Expected default node timeout 120s, got %v", cfg.NodeTimeout)
	}

	if cfg.MulticastAddr != "ff02::4c4c:6469" {
		t.Errorf("Expected default multicast address ff02::4c4c:6469, got %s", cfg.MulticastAddr)
	}

	if cfg.MulticastPort != 9999 {
		t.Errorf("Expected default multicast port 9999, got %d", cfg.MulticastPort)
	}

	if cfg.HTTPAddress != ":6469" {
		t.Errorf("Expected default HTTP address :6469, got %s", cfg.HTTPAddress)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("Expected default log level info, got %s", cfg.LogLevel)
	}

	if cfg.IncludeNeighbors {
		t.Error("Expected include_neighbors to be false by default")
	}

	if cfg.Telemetry.Enabled {
		t.Error("Expected telemetry to be disabled by default")
	}

	if cfg.Telemetry.Endpoint != "grpc://localhost:4317" {
		t.Errorf("Expected default telemetry endpoint grpc://localhost:4317, got %s", cfg.Telemetry.Endpoint)
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.json")
	if err != nil {
		t.Fatalf("Load should not error on missing file, got: %v", err)
	}

	// Should return defaults
	if cfg.SendInterval != 30*time.Second {
		t.Error("Should use default values when file doesn't exist")
	}
}

func TestLoad_EmptyPath(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load should not error on empty path, got: %v", err)
	}

	// Should return defaults
	if cfg.MulticastPort != 9999 {
		t.Error("Should use default values when path is empty")
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"send_interval": "10s",
		"node_timeout": "60s",
		"export_interval": "30s",
		"multicast_address": "ff02::1",
		"multicast_port": 8888,
		"output_file": "/tmp/test.dot",
		"http_address": ":9090",
		"log_level": "debug",
		"include_neighbors": true,
		"telemetry": {
			"enabled": true,
			"endpoint": "http://localhost:4318",
			"insecure": false,
			"enable_traces": false,
			"enable_metrics": true,
			"enable_logs": true
		}
	}`

	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.SendInterval != 10*time.Second {
		t.Errorf("Expected send_interval 10s, got %v", cfg.SendInterval)
	}

	if cfg.NodeTimeout != 60*time.Second {
		t.Errorf("Expected node_timeout 60s, got %v", cfg.NodeTimeout)
	}

	if cfg.ExportInterval != 30*time.Second {
		t.Errorf("Expected export_interval 30s, got %v", cfg.ExportInterval)
	}

	if cfg.MulticastAddr != "ff02::1" {
		t.Errorf("Expected multicast_address ff02::1, got %s", cfg.MulticastAddr)
	}

	if cfg.MulticastPort != 8888 {
		t.Errorf("Expected multicast_port 8888, got %d", cfg.MulticastPort)
	}

	if cfg.OutputFile != "/tmp/test.dot" {
		t.Errorf("Expected output_file /tmp/test.dot, got %s", cfg.OutputFile)
	}

	if cfg.HTTPAddress != ":9090" {
		t.Errorf("Expected http_address :9090, got %s", cfg.HTTPAddress)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("Expected log_level debug, got %s", cfg.LogLevel)
	}

	if !cfg.IncludeNeighbors {
		t.Error("Expected include_neighbors to be true")
	}

	// Telemetry settings
	if !cfg.Telemetry.Enabled {
		t.Error("Expected telemetry enabled")
	}

	if cfg.Telemetry.Endpoint != "localhost:4318" {
		t.Errorf("Expected endpoint localhost:4318 after parsing, got %s", cfg.Telemetry.Endpoint)
	}

	if cfg.Telemetry.Protocol != "http" {
		t.Errorf("Expected protocol http after parsing, got %s", cfg.Telemetry.Protocol)
	}

	if cfg.Telemetry.Insecure {
		t.Error("Expected insecure to be false")
	}

	if cfg.Telemetry.EnableTraces {
		t.Error("Expected enable_traces to be false")
	}

	if !cfg.Telemetry.EnableMetrics {
		t.Error("Expected enable_metrics to be true")
	}

	if !cfg.Telemetry.EnableLogs {
		t.Error("Expected enable_logs to be true")
	}
}

func TestLoad_PartialConfig(t *testing.T) {
	// Test that partial config merges with defaults
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"send_interval": "5s",
		"log_level": "warn"
	}`

	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Check overridden values
	if cfg.SendInterval != 5*time.Second {
		t.Errorf("Expected send_interval 5s, got %v", cfg.SendInterval)
	}

	if cfg.LogLevel != "warn" {
		t.Errorf("Expected log_level warn, got %s", cfg.LogLevel)
	}

	// Check default values still apply
	if cfg.NodeTimeout != 120*time.Second {
		t.Errorf("Expected default node_timeout 120s, got %v", cfg.NodeTimeout)
	}

	if cfg.MulticastPort != 9999 {
		t.Errorf("Expected default multicast_port 9999, got %d", cfg.MulticastPort)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	invalidJSON := `{
		"send_interval": "10s"
		"missing_comma": true
	}`

	if err := os.WriteFile(configPath, []byte(invalidJSON), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestLoad_InvalidDuration(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"send_interval": "invalid"
	}`

	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load should not fail on invalid duration: %v", err)
	}

	// Should keep default value
	if cfg.SendInterval != 30*time.Second {
		t.Errorf("Expected default send_interval when invalid, got %v", cfg.SendInterval)
	}
}

func TestLoad_InvalidTelemetryEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := `{
		"telemetry": {
			"enabled": true,
			"endpoint": "ftp://invalid:21"
		}
	}`

	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Expected error for invalid telemetry endpoint protocol")
	}

	if err != nil && !contains(err.Error(), "unsupported protocol") {
		t.Errorf("Expected 'unsupported protocol' error, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestTelemetryConfig_ParseEndpoint(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		wantEndpoint string
		wantProtocol string
		wantErr      bool
	}{
		{
			name:         "grpc with port",
			endpoint:     "grpc://localhost:4317",
			wantEndpoint: "localhost:4317",
			wantProtocol: "grpc",
			wantErr:      false,
		},
		{
			name:         "grpc without port",
			endpoint:     "grpc://localhost",
			wantEndpoint: "localhost:4317",
			wantProtocol: "grpc",
			wantErr:      false,
		},
		{
			name:         "http with port",
			endpoint:     "http://localhost:4318",
			wantEndpoint: "localhost:4318",
			wantProtocol: "http",
			wantErr:      false,
		},
		{
			name:         "http without port",
			endpoint:     "http://localhost",
			wantEndpoint: "localhost:4318",
			wantProtocol: "http",
			wantErr:      false,
		},
		{
			name:         "https with port",
			endpoint:     "https://otel.example.com:4318",
			wantEndpoint: "otel.example.com:4318",
			wantProtocol: "http",
			wantErr:      false,
		},
		{
			name:         "https without port",
			endpoint:     "https://otel.example.com",
			wantEndpoint: "otel.example.com:4318",
			wantProtocol: "http",
			wantErr:      false,
		},
		{
			name:         "bare host:port (assumes grpc)",
			endpoint:     "localhost:4317",
			wantEndpoint: "localhost:4317",
			wantProtocol: "grpc",
			wantErr:      false,
		},
		{
			name:         "bare host (assumes grpc, adds port)",
			endpoint:     "localhost",
			wantEndpoint: "localhost:4317",
			wantProtocol: "grpc",
			wantErr:      false,
		},
		{
			name:         "custom port with grpc",
			endpoint:     "grpc://otel.example.com:9999",
			wantEndpoint: "otel.example.com:9999",
			wantProtocol: "grpc",
			wantErr:      false,
		},
		{
			name:         "custom port with http",
			endpoint:     "http://otel.example.com:8888",
			wantEndpoint: "otel.example.com:8888",
			wantProtocol: "http",
			wantErr:      false,
		},
		{
			name:     "unsupported protocol",
			endpoint: "ftp://localhost:21",
			wantErr:  true,
		},
		{
			name:         "empty endpoint",
			endpoint:     "",
			wantEndpoint: "",
			wantProtocol: "",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &TelemetryConfig{
				Endpoint: tt.endpoint,
			}

			err := tc.ParseEndpoint()

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseEndpoint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if tc.Endpoint != tt.wantEndpoint {
					t.Errorf("ParseEndpoint() endpoint = %v, want %v", tc.Endpoint, tt.wantEndpoint)
				}
				if tc.Protocol != tt.wantProtocol {
					t.Errorf("ParseEndpoint() protocol = %v, want %v", tc.Protocol, tt.wantProtocol)
				}
			}
		})
	}
}
