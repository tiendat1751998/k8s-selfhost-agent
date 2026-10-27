import { sanitizeNamespace } from '../../utils/yaml'

export type TemplateKey = 'deployment' | 'service' | 'configmap' | 'secret' | 'ingress' | 'pvc' | 'job'

export interface TemplateOption {
  key: TemplateKey
  label: string
  icon: string
  yaml: (ns?: string) => string
}

export const manifestTemplates: TemplateOption[] = [
  {
    key: 'deployment',
    label: 'Deployment',
    icon: 'play',
    yaml: (ns) => {
      const safeNs = sanitizeNamespace(ns)
      return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: sample-workload
  namespace: ${safeNs}
spec:
  replicas: 2
  selector:
    matchLabels:
      app: sample-workload
  template:
    metadata:
      labels:
        app: sample-workload
    spec:
      containers:
      - name: web
        image: nginx:alpine
        ports:
        - containerPort: 80`
    },
  },
  {
    key: 'service',
    label: 'Service',
    icon: 'plug',
    yaml: (ns) => {
      const safeNs = sanitizeNamespace(ns)
      return `apiVersion: v1
kind: Service
metadata:
  name: sample-service
  namespace: ${safeNs}
spec:
  type: ClusterIP
  selector:
    app: sample-workload
  ports:
  - protocol: TCP
    port: 80
    targetPort: 80`
    },
  },
  {
    key: 'configmap',
    label: 'ConfigMap',
    icon: 'file-text',
    yaml: (ns) => {
      const safeNs = sanitizeNamespace(ns)
      return `apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: ${safeNs}
data:
  APP_ENV: "production"`
    },
  },
  {
    key: 'secret',
    label: 'Secret',
    icon: 'lock',
    yaml: (ns) => {
      const safeNs = sanitizeNamespace(ns)
      return `apiVersion: v1
kind: Secret
metadata:
  name: app-secret
  namespace: ${safeNs}
type: Opaque
stringData:
  API_KEY: "change-me"`
    },
  },
  {
    key: 'ingress',
    label: 'Ingress',
    icon: 'globe',
    yaml: (ns) => {
      const safeNs = sanitizeNamespace(ns)
      return `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: sample-ingress
  namespace: ${safeNs}
spec:
  rules:
  - http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: sample-service
            port:
              number: 80`
    },
  },
  {
    key: 'pvc',
    label: 'PVC',
    icon: 'hard-drive',
    yaml: (ns) => {
      const safeNs = sanitizeNamespace(ns)
      return `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: sample-pvc
  namespace: ${safeNs}
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi`
    },
  },
  {
    key: 'job',
    label: 'Job',
    icon: 'zap',
    yaml: (ns) => {
      const safeNs = sanitizeNamespace(ns)
      return `apiVersion: batch/v1
kind: Job
metadata:
  name: sample-job
  namespace: ${safeNs}
spec:
  template:
    spec:
      containers:
      - name: runner
        image: busybox:latest
        command: ["echo", "Done"]
      restartPolicy: Never`
    },
  },
]
