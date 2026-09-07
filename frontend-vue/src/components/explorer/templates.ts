export type TemplateKey = 'deployment' | 'service' | 'configmap' | 'secret' | 'ingress' | 'pvc' | 'job'

export interface TemplateOption {
  key: TemplateKey
  label: string
  icon: string
  yaml: (ns: string) => string
}

export const manifestTemplates: TemplateOption[] = [
  {
    key: 'deployment',
    label: 'Deployment',
    icon: '🚀',
    yaml: (ns) => `apiVersion: apps/v1
kind: Deployment
metadata:
  name: sample-workload
  namespace: ${ns}
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
        - containerPort: 80`,
  },
  {
    key: 'service',
    label: 'Service',
    icon: '🔌',
    yaml: (ns) => `apiVersion: v1
kind: Service
metadata:
  name: sample-service
  namespace: ${ns}
spec:
  type: ClusterIP
  selector:
    app: sample-workload
  ports:
  - protocol: TCP
    port: 80
    targetPort: 80`,
  },
  {
    key: 'configmap',
    label: 'ConfigMap',
    icon: '🗺️',
    yaml: (ns) => `apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: ${ns}
data:
  APP_ENV: "production"`,
  },
  {
    key: 'secret',
    label: 'Secret',
    icon: '🔒',
    yaml: (ns) => `apiVersion: v1
kind: Secret
metadata:
  name: app-secret
  namespace: ${ns}
type: Opaque
stringData:
  API_KEY: "change-me"`,
  },
  {
    key: 'ingress',
    label: 'Ingress',
    icon: '🌐',
    yaml: (ns) => `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: sample-ingress
  namespace: ${ns}
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
              number: 80`,
  },
  {
    key: 'pvc',
    label: 'PVC',
    icon: '💾',
    yaml: (ns) => `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: sample-pvc
  namespace: ${ns}
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi`,
  },
  {
    key: 'job',
    label: 'Job',
    icon: '⚡',
    yaml: (ns) => `apiVersion: batch/v1
kind: Job
metadata:
  name: sample-job
  namespace: ${ns}
spec:
  template:
    spec:
      containers:
      - name: runner
        image: busybox:latest
        command: ["echo", "Done"]
      restartPolicy: Never`,
  },
]
