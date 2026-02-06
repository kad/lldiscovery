# Deployment Documentation

This directory contains guides for deploying, configuring, and operating lldiscovery in production environments.

## Documents

### Installation & Setup

**[DEPLOYMENT_GUIDE.md](DEPLOYMENT_GUIDE.md)**
- Complete deployment guide
- Package installation (deb/rpm)
- Systemd service configuration
- Manual installation steps

### Observability

**[OPENTELEMETRY.md](OPENTELEMETRY.md)**
- OpenTelemetry integration overview
- Traces, metrics, and logs
- Configuration examples
- Collector setup

**[OTEL_ENDPOINT_IMPROVEMENT.md](OTEL_ENDPOINT_IMPROVEMENT.md)**
- Enhanced endpoint configuration
- URL-based format with auto-protocol detection
- Migration from legacy format

**[OTEL_SUMMARY.md](OTEL_SUMMARY.md)**
- Summary of OpenTelemetry implementation
- Key features and benefits
- Usage examples

## Quick Start

### Package Installation

**Debian/Ubuntu:**
```bash
wget https://github.com/kad/lldiscovery/releases/download/v0.0.1/lldiscovery_0.0.1_linux_amd64.deb
sudo dpkg -i lldiscovery_0.0.1_linux_amd64.deb
sudo systemctl enable --now lldiscovery
```

**RHEL/CentOS/Fedora:**
```bash
wget https://github.com/kad/lldiscovery/releases/download/v0.0.1/lldiscovery_0.0.1_linux_amd64.rpm
sudo rpm -i lldiscovery_0.0.1_linux_amd64.rpm
sudo systemctl enable --now lldiscovery
```

### Configuration

Edit `/etc/lldiscovery/config.json`:
```json
{
  "send_interval": "30s",
  "node_timeout": "120s",
  "show_segments": true,
  "include_neighbors": true
}
```

### Monitoring

Access the topology:
```bash
# JSON format
curl http://localhost:6469/graph

# DOT/Graphviz
curl http://localhost:6469/graph.dot | neato -Tpng -o topology.png

# PlantUML nwdiag
curl http://localhost:6469/graph.nwdiag > topology.puml
```

## Related Documentation

- **Main Guide**: [../README.md](../../README.md)
- **Quick Start**: [../QUICKSTART.md](../../QUICKSTART.md)
- **Features**: [../features/](../features/)
