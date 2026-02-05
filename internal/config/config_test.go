package config

import (
	"testing"
)

func TestTelemetryConfig_ParseEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       string
		wantEndpoint   string
		wantProtocol   string
		wantErr        bool
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
