package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	SendInterval     time.Duration   `json:"send_interval"`
	NodeTimeout      time.Duration   `json:"node_timeout"`
	ExportInterval   time.Duration   `json:"export_interval"`
	MulticastAddr    string          `json:"multicast_address"`
	MulticastPort    int             `json:"multicast_port"`
	OutputFile       string          `json:"output_file"`
	HTTPAddress      string          `json:"http_address"`
	LogLevel         string          `json:"log_level"`
	IncludeNeighbors bool            `json:"include_neighbors"`
	Telemetry        TelemetryConfig `json:"telemetry"`
}

type TelemetryConfig struct {
	Enabled       bool   `json:"enabled"`
	Endpoint      string `json:"endpoint"`
	Protocol      string `json:"protocol"`
	Insecure      bool   `json:"insecure"`
	EnableTraces  bool   `json:"enable_traces"`
	EnableMetrics bool   `json:"enable_metrics"`
	EnableLogs    bool   `json:"enable_logs"`
}

func Default() *Config {
	return &Config{
		SendInterval:     30 * time.Second,
		NodeTimeout:      120 * time.Second,
		ExportInterval:   60 * time.Second,
		MulticastAddr:    "ff02::4c4c:6469",
		MulticastPort:    9999,
		OutputFile:       getDefaultOutputFile(),
		HTTPAddress:      ":8080",
		LogLevel:         "info",
		IncludeNeighbors: false,
		Telemetry: TelemetryConfig{
			Enabled:       false,
			Endpoint:      "grpc://localhost:4317",
			Protocol:      "grpc",
			Insecure:      true,
			EnableTraces:  true,
			EnableMetrics: true,
			EnableLogs:    false,
		},
	}
}

func getDefaultOutputFile() string {
	defaultPath := "/var/lib/lldiscovery/topology.dot"
	dir := filepath.Dir(defaultPath)
	
	// Check if directory exists and is writable
	if err := os.MkdirAll(dir, 0755); err == nil {
		// Try to create a test file to verify write permission
		testFile := filepath.Join(dir, ".write_test")
		if f, err := os.Create(testFile); err == nil {
			f.Close()
			os.Remove(testFile)
			return defaultPath
		}
	}
	
	// Fallback to current directory if default path not writable
	return "./topology.dot"
}

func Load(path string) (*Config, error) {
	cfg := Default()

	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	var rawConfig struct {
		SendInterval     string          `json:"send_interval"`
		NodeTimeout      string          `json:"node_timeout"`
		ExportInterval   string          `json:"export_interval"`
		MulticastAddr    string          `json:"multicast_address"`
		MulticastPort    int             `json:"multicast_port"`
		OutputFile       string          `json:"output_file"`
		HTTPAddress      string          `json:"http_address"`
		LogLevel         string          `json:"log_level"`
		IncludeNeighbors bool            `json:"include_neighbors"`
		Telemetry        TelemetryConfig `json:"telemetry"`
	}

	if err := json.Unmarshal(data, &rawConfig); err != nil {
		return nil, err
	}

	if rawConfig.SendInterval != "" {
		if d, err := time.ParseDuration(rawConfig.SendInterval); err == nil {
			cfg.SendInterval = d
		}
	}
	if rawConfig.NodeTimeout != "" {
		if d, err := time.ParseDuration(rawConfig.NodeTimeout); err == nil {
			cfg.NodeTimeout = d
		}
	}
	if rawConfig.ExportInterval != "" {
		if d, err := time.ParseDuration(rawConfig.ExportInterval); err == nil {
			cfg.ExportInterval = d
		}
	}
	if rawConfig.MulticastAddr != "" {
		cfg.MulticastAddr = rawConfig.MulticastAddr
	}
	if rawConfig.MulticastPort != 0 {
		cfg.MulticastPort = rawConfig.MulticastPort
	}
	if rawConfig.OutputFile != "" {
		cfg.OutputFile = rawConfig.OutputFile
	}
	if rawConfig.HTTPAddress != "" {
		cfg.HTTPAddress = rawConfig.HTTPAddress
	}
	if rawConfig.OutputFile != "" {
		cfg.OutputFile = rawConfig.OutputFile
	}
	if rawConfig.LogLevel != "" {
		cfg.LogLevel = rawConfig.LogLevel
	}
	
	cfg.IncludeNeighbors = rawConfig.IncludeNeighbors
	
	// Merge telemetry config
	if rawConfig.Telemetry.Endpoint != "" || rawConfig.Telemetry.Enabled {
		cfg.Telemetry = rawConfig.Telemetry
	}
	
	// Parse endpoint URL to extract protocol if needed
	if err := cfg.Telemetry.ParseEndpoint(); err != nil {
		return nil, fmt.Errorf("invalid telemetry endpoint: %w", err)
	}

	return cfg, nil
}

// ParseEndpoint parses the endpoint URL and extracts protocol and address.
// Supports formats:
//   - grpc://host:port (default port 4317)
//   - http://host:port (default port 4318)
//   - https://host:port (default port 4318)
//   - host:port (assumes grpc://, port 4317 if not specified)
func (t *TelemetryConfig) ParseEndpoint() error {
	if t.Endpoint == "" {
		return nil
	}
	
	endpoint := t.Endpoint
	
	// If no scheme, check if it looks like a URL pattern
	if !strings.Contains(endpoint, "://") {
		// If it's just host:port or host, assume grpc://
		endpoint = "grpc://" + endpoint
	}
	
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse endpoint URL: %w", err)
	}
	
	// Extract protocol from scheme
	switch u.Scheme {
	case "grpc":
		t.Protocol = "grpc"
		// Add default port if not specified
		if u.Port() == "" {
			u.Host = u.Hostname() + ":4317"
		}
	case "http", "https":
		t.Protocol = "http"
		// Add default port if not specified
		if u.Port() == "" {
			u.Host = u.Hostname() + ":4318"
		}
	default:
		return fmt.Errorf("unsupported protocol scheme: %s (use grpc://, http://, or https://)", u.Scheme)
	}
	
	// Update endpoint to be just host:port
	t.Endpoint = u.Host
	
	return nil
}
