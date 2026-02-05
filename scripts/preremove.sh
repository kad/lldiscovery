#!/bin/sh
# Pre-removal script for lldiscovery

set -e

# Stop and disable service if running
if command -v systemctl >/dev/null 2>&1; then
    if systemctl is-active --quiet lldiscovery; then
        systemctl stop lldiscovery || true
    fi
    
    if systemctl is-enabled --quiet lldiscovery 2>/dev/null; then
        systemctl disable lldiscovery || true
    fi
fi

exit 0
