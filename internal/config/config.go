package config

import (
	"encoding/json"
	"os"
	"time"
)

type Config struct {
	SendInterval   time.Duration `json:"send_interval"`
	NodeTimeout    time.Duration `json:"node_timeout"`
	ExportInterval time.Duration `json:"export_interval"`
	MulticastAddr  string        `json:"multicast_address"`
	MulticastPort  int           `json:"multicast_port"`
	OutputFile     string        `json:"output_file"`
	HTTPAddress    string        `json:"http_address"`
	LogLevel       string        `json:"log_level"`
	Telemetry      TelemetryConfig `json:"telemetry"`
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
		SendInterval:   30 * time.Second,
		NodeTimeout:    120 * time.Second,
		ExportInterval: 60 * time.Second,
		MulticastAddr:  "ff02::4c4c:6469",
		MulticastPort:  9999,
		OutputFile:     "/var/lib/lldiscovery/topology.dot",
		HTTPAddress:    ":8080",
		LogLevel:       "info",
		Telemetry: TelemetryConfig{
			Enabled:       false,
			Endpoint:      "localhost:4317",
			Protocol:      "grpc",
			Insecure:      true,
			EnableTraces:  true,
			EnableMetrics: true,
			EnableLogs:    false,
		},
	}
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
		SendInterval   string `json:"send_interval"`
		NodeTimeout    string `json:"node_timeout"`
		ExportInterval string `json:"export_interval"`
		MulticastAddr  string `json:"multicast_address"`
		MulticastPort  int    `json:"multicast_port"`
		OutputFile     string `json:"output_file"`
		HTTPAddress    string `json:"http_address"`
		LogLevel       string `json:"log_level"`
		Telemetry      TelemetryConfig `json:"telemetry"`
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
	if rawConfig.LogLevel != "" {
		cfg.LogLevel = rawConfig.LogLevel
	}
	
	// Merge telemetry config
	if rawConfig.Telemetry.Endpoint != "" || rawConfig.Telemetry.Enabled {
		cfg.Telemetry = rawConfig.Telemetry
	}

	return cfg, nil
}
