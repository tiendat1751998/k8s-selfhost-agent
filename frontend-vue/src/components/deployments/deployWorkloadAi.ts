import type { DeploymentApp } from '../../api/compute'

export interface NewAppForm extends DeploymentApp {
  servicePort: number
  cpuLimit?: string
  memLimit?: string
  envList: { key: string; value: string }[]
}

export function createInitialForm(): NewAppForm {
  return {
    name: '', team: 'platform-engineering', env: 'production', image: '', target: 'default',
    namespace: 'default', type: 'kubernetes', replicas: 3, status: 'healthy', cpu: '250m', memory: '512Mi',
    port: 80, servicePort: 80, netType: 'ClusterIP', strategy: 'RollingUpdate', canaryWeight: 10,
    canaryVersion: '', blueGreenActive: 'blue', blueVersion: '', greenVersion: '', ingressHost: '',
    tlsEnabled: false, storageClass: 'standard', storageSize: '10Gi', mountPath: '/data',
    livenessProbe: '/healthz', readinessProbe: '/ready',
    envList: [{ key: 'NODE_ENV', value: 'production' }, { key: 'LOG_LEVEL', value: 'info' }]
  }
}

export function parseAiPrompt(prompt: string): Partial<NewAppForm> {
  const p = prompt.toLowerCase()

  let parsedName = 'microservice-api'
  let parsedImage = 'nginx:alpine'
  let parsedReplicas = 3
  let parsedPort = 80
  let parsedStrategy: 'RollingUpdate' | 'Recreate' | 'Canary' | 'BlueGreen' = 'RollingUpdate'
  let parsedCanaryWeight = 20

  const imgMatch = prompt.match(/(?:image|deploy|service)\s+([a-zA-Z0-9_\-\.\/]+)(?::([a-zA-Z0-9_\-\.]+))?/i)
  if (imgMatch) {
    parsedImage = imgMatch[1] + (imgMatch[2] ? `:${imgMatch[2]}` : ':latest')
    parsedName = imgMatch[1].split('/').pop()?.split(':')[0] || 'app'
  } else if (p.includes('redis')) {
    parsedName = 'redis-store'
    parsedImage = 'redis:7.2-alpine'
    parsedPort = 6379
  } else if (p.includes('postgres') || p.includes('database')) {
    parsedName = 'postgres-db'
    parsedImage = 'postgres:16-alpine'
    parsedPort = 5432
  } else if (p.includes('node') || p.includes('express')) {
    parsedName = 'node-api'
    parsedImage = 'node:20-alpine'
    parsedPort = 3000
  } else if (p.includes('python') || p.includes('fastapi')) {
    parsedName = 'fastapi-gateway'
    parsedImage = 'python:3.11-slim'
    parsedPort = 8000
  }

  const repMatch = p.match(/(\d+)\s*(?:replicas|pods|instances)/)
  if (repMatch) parsedReplicas = parseInt(repMatch[1], 10)

  const portMatch = p.match(/port\s*(\d+)/)
  if (portMatch) parsedPort = parseInt(portMatch[1], 10)

  if (p.includes('canary')) {
    parsedStrategy = 'Canary'
    const weightMatch = p.match(/(\d+)\s*%/)
    if (weightMatch) parsedCanaryWeight = parseInt(weightMatch[1], 10)
  } else if (p.includes('blue') || p.includes('green') || p.includes('bluegreen')) {
    parsedStrategy = 'BlueGreen'
  }

  const result: Partial<NewAppForm> = {
    name: parsedName,
    image: parsedImage,
    replicas: parsedReplicas,
    port: parsedPort,
    servicePort: parsedPort,
    strategy: parsedStrategy,
    canaryWeight: parsedCanaryWeight,
  }

  if (p.includes('ingress')) {
    result.netType = 'Ingress'
    result.ingressHost = `${parsedName}.corp.internal`
  }

  return result
}
