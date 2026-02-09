#!/bin/bash
set -e

AGENT_NAME="Heimdex Agent"
BINARY_NAME="heimdex-agent"
PLIST_NAME="com.heimdex.agent.plist"
INSTALL_DIR="/Applications/${AGENT_NAME}.app/Contents/MacOS"
PLIST_DIR="$HOME/Library/LaunchAgents"

echo "Installing ${AGENT_NAME}..."

if [ ! -f "./${BINARY_NAME}" ]; then
    echo "Error: ${BINARY_NAME} not found in current directory"
    exit 1
fi

mkdir -p "${INSTALL_DIR}"
cp "./${BINARY_NAME}" "${INSTALL_DIR}/"
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

mkdir -p "${PLIST_DIR}"
cp "./${PLIST_NAME}" "${PLIST_DIR}/"

launchctl unload "${PLIST_DIR}/${PLIST_NAME}" 2>/dev/null || true
launchctl load "${PLIST_DIR}/${PLIST_NAME}"

echo "Installation complete!"
echo ""
echo "The agent is now running. Check the menu bar for the Heimdex icon."
echo ""
echo "To view logs:"
echo "  tail -f /tmp/heimdex-agent.stdout.log"
echo ""
echo "To stop the agent:"
echo "  launchctl unload ~/Library/LaunchAgents/${PLIST_NAME}"
echo ""
echo "To start the agent:"
echo "  launchctl load ~/Library/LaunchAgents/${PLIST_NAME}"
