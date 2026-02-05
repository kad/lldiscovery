#!/bin/sh
# Pre-installation script for lldiscovery

set -e

# Create system user if it doesn't exist
if ! getent passwd lldiscovery >/dev/null 2>&1; then
    useradd -r -s /bin/false -d /var/lib/lldiscovery -c "lldiscovery daemon" lldiscovery
fi

# Create configuration directory
mkdir -p /etc/lldiscovery

exit 0
