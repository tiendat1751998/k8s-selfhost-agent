# K8sControl Deployment Guide

## Quick Start

### Main Server (K8sControl)
```bash
git clone https://github.com/tiendat1751998/k8s-selfhost-agent.git
cd k8s-selfhost-agent

# Build
go build -o k8scontrol ./cmd/standalone
cd frontend-vue && npm install && npm run build && cd ..

# Run
export JWT_SECRET='your-secret'
export ENCRYPTION_KEY='your-32-char-key'
export K8S_POSTGRES_HOST='db-ip'
export K8S_POSTGRES_USER='user'
export K8S_POSTGRES_PASSWORD='pass'
export K8S_POSTGRES_DBNAME='dbname'
export K8S_DOCKER_HOST='tcp://docker-ip:2375'
./k8scontrol
```

### Deploy Agent to Remote Servers
```bash
./deploy-agent.sh user@db-server-ip
./deploy-agent.sh user@app-server-ip
```

Then in UI: Docker Swarm → Node Management → Add Docker Host → enter server IP:9100
