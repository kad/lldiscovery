#!/bin/sh
# Post-installation script for lldiscovery

set -e

# Set proper ownership and permissions
chown root:lldiscovery /etc/lldiscovery/config.json 2>/dev/null || true
chmod 640 /etc/lldiscovery/config.json 2>/dev/null || true

chown lldiscovery:lldiscovery /var/lib/lldiscovery 2>/dev/null || true

# Reload systemd daemon
if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload
    
    # Print instructions for enabling and starting the service
    echo ""
    echo "lldiscovery has been installed."
    echo ""
    echo "To enable and start the service:"
    echo "  sudo systemctl enable lldiscovery"
    echo "  sudo systemctl start lldiscovery"
    echo ""
    echo "To check status:"
    echo "  sudo systemctl status lldiscovery"
    echo ""
    echo "Configuration file: /etc/lldiscovery/config.json"
    echo "Logs: journalctl -u lldiscovery -f"
    echo ""
fi

exit 0
