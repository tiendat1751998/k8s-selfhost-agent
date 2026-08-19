/**
 * Lightweight JSON to YAML serializer & YAML helpers for Kubernetes manifests.
 */

function isPrimitive(val: unknown): boolean {
  return (
    val === null ||
    val === undefined ||
    typeof val === 'string' ||
    typeof val === 'number' ||
    typeof val === 'boolean'
  )
}

function escapeYamlString(str: string): string {
  if (str === '') return "''"

  if (str.includes('\n')) {
    const lines = str.split('\n')
    return '|\n' + lines.map(line => '    ' + line).join('\n')
  }

  const needsQuotes =
    /[:#\[\]{},&*?|\-<>=!%@`~]/.test(str) ||
    /^(true|false|null|yes|no|on|off)$/i.test(str) ||
    /^\d/.test(str) ||
    str.startsWith(' ') ||
    str.endsWith(' ')

  if (needsQuotes) {
    return JSON.stringify(str)
  }

  return str
}

export function jsonToYaml(obj: unknown, indent = 0): string {
  const indentStr = ' '.repeat(indent)

  if (obj === null || obj === undefined) {
    return 'null'
  }

  if (typeof obj === 'string') {
    return escapeYamlString(obj)
  }

  if (typeof obj === 'number' || typeof obj === 'boolean') {
    return String(obj)
  }

  if (Array.isArray(obj)) {
    if (obj.length === 0) return '[]'
    const lines: string[] = []
    for (const item of obj) {
      if (isPrimitive(item)) {
        lines.push(`${indentStr}- ${jsonToYaml(item, indent + 2)}`)
      } else if (typeof item === 'object' && item !== null) {
        const itemKeys = Object.keys(item)
        if (itemKeys.length === 0) {
          lines.push(`${indentStr}- {}`)
        } else {
          const firstKey = itemKeys[0]
          const firstVal = (item as Record<string, unknown>)[firstKey]
          if (isPrimitive(firstVal)) {
            lines.push(`${indentStr}- ${firstKey}: ${jsonToYaml(firstVal, 0)}`)
          } else {
            lines.push(`${indentStr}- ${firstKey}:\n${jsonToYaml(firstVal, indent + 4)}`)
          }
          for (let i = 1; i < itemKeys.length; i++) {
            const k = itemKeys[i]
            const v = (item as Record<string, unknown>)[k]
            if (isPrimitive(v)) {
              lines.push(`${indentStr}  ${k}: ${jsonToYaml(v, 0)}`)
            } else {
              lines.push(`${indentStr}  ${k}:\n${jsonToYaml(v, indent + 4)}`)
            }
          }
        }
      }
    }
    return lines.join('\n')
  }

  if (typeof obj === 'object') {
    const keys = Object.keys(obj as Record<string, unknown>)
    if (keys.length === 0) return '{}'
    const lines: string[] = []
    for (const key of keys) {
      const val = (obj as Record<string, unknown>)[key]
      if (val === undefined) continue
      if (isPrimitive(val)) {
        lines.push(`${indentStr}${key}: ${jsonToYaml(val, 0)}`)
      } else if (Array.isArray(val)) {
        if (val.length === 0) {
          lines.push(`${indentStr}${key}: []`)
        } else {
          lines.push(`${indentStr}${key}:\n${jsonToYaml(val, indent + 2)}`)
        }
      } else if (typeof val === 'object') {
        const subKeys = Object.keys(val as Record<string, unknown>)
        if (subKeys.length === 0) {
          lines.push(`${indentStr}${key}: {}`)
        } else {
          lines.push(`${indentStr}${key}:\n${jsonToYaml(val, indent + 2)}`)
        }
      }
    }
    return lines.join('\n')
  }

  return String(obj)
}

export function getDefaultYamlTemplate(
  kind: string,
  name: string = 'sample-resource',
  namespace: string = 'default'
): string {
  switch (kind.toLowerCase()) {
    case 'configmap':
    case 'configmaps':
      return `apiVersion: v1
kind: ConfigMap
metadata:
  name: ${name}
  namespace: ${namespace}
  labels:
    app.kubernetes.io/managed-by: k8sselfhost
data:
  APP_ENV: "production"
  LOG_LEVEL: "info"
  DATABASE_HOST: "postgres.default.svc.cluster.local"
`

    case 'secret':
    case 'secrets':
      return `apiVersion: v1
kind: Secret
metadata:
  name: ${name}
  namespace: ${namespace}
  labels:
    app.kubernetes.io/managed-by: k8sselfhost
type: Opaque
stringData:
  API_KEY: "super-secret-key-12345"
  DB_PASSWORD: "cluster-secret-pass"
`

    case 'deployment':
    case 'deployments':
      return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${name}
  namespace: ${namespace}
  labels:
    app: ${name}
spec:
  replicas: 2
  selector:
    matchLabels:
      app: ${name}
  template:
    metadata:
      labels:
        app: ${name}
    spec:
      containers:
      - name: ${name}
        image: nginx:alpine
        ports:
        - containerPort: 80
        resources:
          limits:
            cpu: "500m"
            memory: "256Mi"
          requests:
            cpu: "100m"
            memory: "128Mi"
`

    case 'service':
    case 'services':
      return `apiVersion: v1
kind: Service
metadata:
  name: ${name}
  namespace: ${namespace}
spec:
  type: ClusterIP
  selector:
    app: ${name}
  ports:
  - name: http
    port: 80
    targetPort: 80
    protocol: TCP
`

    case 'ingress':
    case 'ingresses':
      return `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ${name}
  namespace: ${namespace}
  annotations:
    kubernetes.io/ingress.class: nginx
spec:
  rules:
  - host: ${name}.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: ${name}
            port:
              number: 80
`

    case 'statefulset':
    case 'statefulsets':
      return `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: ${name}
  namespace: ${namespace}
spec:
  serviceName: ${name}
  replicas: 1
  selector:
    matchLabels:
      app: ${name}
  template:
    metadata:
      labels:
        app: ${name}
    spec:
      containers:
      - name: ${name}
        image: redis:7-alpine
        ports:
        - containerPort: 6379
`

    case 'daemonset':
    case 'daemonsets':
      return `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: ${name}
  namespace: ${namespace}
spec:
  selector:
    matchLabels:
      app: ${name}
  template:
    metadata:
      labels:
        app: ${name}
    spec:
      containers:
      - name: ${name}
        image: fluent/fluent-bit:latest
`

    case 'job':
    case 'jobs':
      return `apiVersion: batch/v1
kind: Job
metadata:
  name: ${name}
  namespace: ${namespace}
spec:
  template:
    spec:
      containers:
      - name: ${name}
        image: busybox:latest
        command: ["sh", "-c", "echo Job Completed; sleep 2"]
      restartPolicy: Never
  backoffLimit: 4
`

    case 'cronjob':
    case 'cronjobs':
      return `apiVersion: batch/v1
kind: CronJob
metadata:
  name: ${name}
  namespace: ${namespace}
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: ${name}
            image: busybox:latest
            command: ["sh", "-c", "date; echo CronJob tick"]
          restartPolicy: OnFailure
`

    case 'persistentvolumeclaim':
    case 'persistentvolumeclaims':
      return `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ${name}
  namespace: ${namespace}
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
`

    case 'storageclass':
    case 'storageclasses':
      return `apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ${name}
provisioner: kubernetes.io/no-provisioner
volumeBindingMode: WaitForFirstConsumer
`

    default:
      return `apiVersion: v1
kind: ${kind}
metadata:
  name: ${name}
  namespace: ${namespace}
spec: {}
`
  }
}
