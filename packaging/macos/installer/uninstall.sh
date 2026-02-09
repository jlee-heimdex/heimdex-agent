#!/bin/bash
set -e

AGENT_NAME="Heimdex Agent"
PLIST_NAME="com.heimdex.agent.plist"
INSTALL_DIR="/Applications/${AGENT_NAME}.app"
PLIST_PATH="$HOME/Library/LaunchAgents/${PLIST_NAME}"
DATA_DIR="$HOME/.heimdex"

echo "Uninstalling ${AGENT_NAME}..."

launchctl unload "${PLIST_PATH}" 2>/dev/null || true

if [ -f "${PLIST_PATH}" ]; then
    rm "${PLIST_PATH}"
    echo "Removed LaunchAgent plist"
fi

if [ -d "${INSTALL_DIR}" ]; then
    rm -rf "${INSTALL_DIR}"
    echo "Removed application"
fi

echo ""
read -p "Remove data directory (${DATA_DIR})? [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    rm -rf "${DATA_DIR}"
    echo "Removed data directory"
else
    echo "Data directory preserved"
fi

echo ""
echo "Uninstallation complete!"
