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

# Copy binary
echo "📦 Copying to $REMOTE..."
scp k8s-agent $REMOTE:~/k8s-agent

# Install and run (no sudo needed!)
echo "🚀 Installing on $REMOTE..."
ssh -t $REMOTE 'chmod +x ~/k8s-agent && \
  mkdir -p ~/.config/systemd/user && \
  cat > ~/.config/systemd/user/k8s-agent.service << UNIT
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
  loginctl enable-linger $(whoami) && \
  echo "" && \
  echo "✅ Agent running on port 9100" && \
  echo "📊 Test: curl http://localhost:9100/health"'

echo ""
echo "🎉 Done! Add this server in K8sControl UI:"
echo "   Docker Swarm → Node Management → Add Docker Host"
echo "   Endpoint: http://$(echo $REMOTE | cut -d@ -f2):9100"
