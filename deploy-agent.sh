#!/bin/bash
# deploy-agent.sh — Deploy k8s-agent to remote server
# Usage: ./deploy-agent.sh user@remote-ip
set -e
REMOTE=$1

if [ -z "$REMOTE" ]; then
    echo "Usage: $0 user@remote-ip"
    exit 1
fi

# Build agent for Linux
echo "🔨 Building agent for linux/amd64..."
GOOS=linux GOARCH=amd64 go build -o k8s-agent ./cmd/agent

# Copy binary to temp file first (prevents ETXTBSY error when agent is already running)
TMP_BIN="/tmp/k8s-agent-new.$$"
echo "📦 Copying to $REMOTE ($TMP_BIN)..."
scp k8s-agent $REMOTE:$TMP_BIN

# Install and run (no sudo needed!)
echo "🚀 Installing and restarting on $REMOTE..."
ssh -t $REMOTE "chmod +x $TMP_BIN && \
  mkdir -p ~/.config/systemd/user && \
  (systemctl --user stop k8s-agent 2>/dev/null || true) && \
  mv -f $TMP_BIN ~/k8s-agent && \
  cat > ~/.config/systemd/user/k8s-agent.service << 'UNIT'
[Unit]
Description=K8sControl Monitoring Agent
After=network.target

[Service]
Type=simple
ExecStart=%h/k8s-agent --port 9100
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
UNIT
  systemctl --user daemon-reload && \
  systemctl --user enable k8s-agent && \
  systemctl --user restart k8s-agent && \
  (loginctl enable-linger \$(whoami) 2>/dev/null || true) && \
  echo '' && \
  echo '✅ Agent successfully updated & running on port 9100' && \
  echo '📊 Test: curl http://localhost:9100/health'"

echo ""
echo "🎉 Done! Server monitored in K8sControl UI:"
echo "   Infrastructure Fleet Registry (/hosts)"
echo "   Endpoint: http://$(echo $REMOTE | cut -d@ -f2):9100"

