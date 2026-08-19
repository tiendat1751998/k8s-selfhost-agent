#!/bin/bash
# deploy-agent.sh — Deploy k8s-agent to remote server
# Usage: ./deploy-agent.sh user@remote-ip
set -e
REMOTE=$1
AGENT_BIN="./k8s-agent"

if [ -z "$REMOTE" ]; then
    echo "Usage: $0 user@remote-ip"
    exit 1
fi

echo "Building agent..."
GOOS=linux GOARCH=amd64 go build -o $AGENT_BIN ./cmd/agent

echo "Copying to $REMOTE..."
scp $AGENT_BIN $REMOTE:~/k8s-agent

echo "Installing on remote..."
ssh $REMOTE 'sudo mkdir -p /opt/k8scontrol && \
  sudo mv ~/k8s-agent /opt/k8scontrol/ && \
  sudo chmod +x /opt/k8scontrol/k8s-agent && \
  cat << UNIT | sudo tee /etc/systemd/system/k8s-agent.service
[Unit]
Description=K8sControl Monitoring Agent
After=network.target
[Service]
Type=simple
ExecStart=/opt/k8scontrol/k8s-agent --port 9100
Restart=always
RestartSec=5
[Install]
WantedBy=multi-user.target
UNIT
  sudo systemctl daemon-reload && \
  sudo systemctl enable k8s-agent && \
  sudo systemctl restart k8s-agent && \
  echo "Agent running on port 9100"'

echo "Done! Add http://$REMOTE:9100 in K8sControl UI"
