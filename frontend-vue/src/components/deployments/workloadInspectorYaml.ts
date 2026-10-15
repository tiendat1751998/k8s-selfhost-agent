import type { DeploymentApp } from '../../api/compute'

export function generateYamlManifest(app: DeploymentApp): string {
  if (app.type === 'swarm' || app.type === 'docker') {
    return `version: "3.8"
services:
  ${app.name}:
    image: ${app.image}
    deploy:
      replicas: ${app.replicas}
      resources:
        limits:
          cpus: "${app.cpu}"
          memory: ${app.memory}
    ports:
      - "${app.port}:${app.port}"
    environment:
      - APP_ENV=${app.env}`
  }

  const isCanary = app.strategy === 'Canary'
  const isBlueGreen = app.strategy === 'BlueGreen'

  return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${app.name}
  namespace: ${app.namespace || 'default'}
  labels:
    app.kubernetes.io/name: ${app.name}
    app.kubernetes.io/team: ${app.team || 'platform'}
    app.kubernetes.io/env: ${app.env || 'production'}
    app.kubernetes.io/strategy: ${app.strategy || 'RollingUpdate'}
spec:
  replicas: ${app.replicas}
  strategy:
    type: ${app.strategy === 'Recreate' ? 'Recreate' : 'RollingUpdate'}
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 0
  selector:
    matchLabels:
      app: ${app.name}
  template:
    metadata:
      labels:
        app: ${app.name}
        track: ${isCanary ? 'stable' : isBlueGreen ? (app.blueGreenActive || 'blue') : 'main'}
    spec:
      containers:
      - name: ${app.name}
        image: ${app.image}
        ports:
        - containerPort: ${app.port}
          name: http
        resources:
          requests:
            cpu: "${app.cpu || '250m'}"
            memory: "${app.memory || '512Mi'}"
          limits:
            cpu: "${app.cpu || '500m'}"
            memory: "${app.memory || '1Gi'}"
        readinessProbe:
          httpGet:
            path: ${app.readinessProbe || '/healthz'}
            port: ${app.port}
          initialDelaySeconds: 5
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: ${app.name}
  namespace: ${app.namespace || 'default'}
spec:
  type: ${app.netType || 'ClusterIP'}
  selector:
    app: ${app.name}
  ports:
  - port: ${app.port}
    targetPort: ${app.port}
    name: http`
}
