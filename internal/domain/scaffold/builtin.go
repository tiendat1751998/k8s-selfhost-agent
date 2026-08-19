package scaffold

import "time"

// Built-in template IDs
const (
	BuiltinIDGoAPI      = "tpl-builtin-go-api"
	BuiltinIDNodeWeb    = "tpl-builtin-node-web"
	BuiltinIDPostgresDB = "tpl-builtin-postgres-db"
)

// GetBuiltinTemplates returns the system-provided starter templates.
func GetBuiltinTemplates() []Template {
	seedTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	return []Template{
		{
			ID:          BuiltinIDGoAPI,
			Name:        "Go API Service",
			Description: "Production-ready Go microservice with chi router, health probes, configmaps, and multi-stage container build.",
			Category:    CategoryAPI,
			Framework:   FrameworkGoChi,
			BuiltIn:     true,
			TenantID:    "default-tenant",
			Tags:        []string{"golang", "chi", "microservice", "api", "kubernetes"},
			Variables: []TemplateVariable{
				{
					Name:     "app_name",
					Label:    "Application Name",
					Type:     "string",
					Default:  "go-api-service",
					Required: true,
				},
				{
					Name:     "namespace",
					Label:    "Kubernetes Namespace",
					Type:     "string",
					Default:  "default",
					Required: true,
				},
				{
					Name:     "image",
					Label:    "Container Image",
					Type:     "string",
					Default:  "golang:1.22-alpine",
					Required: true,
				},
				{
					Name:     "port",
					Label:    "Service Port",
					Type:     "number",
					Default:  "8080",
					Required: true,
				},
				{
					Name:     "replicas",
					Label:    "Replicas",
					Type:     "number",
					Default:  "2",
					Required: true,
				},
				{
					Name:     "environment",
					Label:    "Environment",
					Type:     "select",
					Default:  "production",
					Required: false,
					Options:  []string{"development", "staging", "production"},
				},
			},
			ManifestYAML: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.app_name}}
  namespace: {{.namespace}}
  labels:
    app.kubernetes.io/name: {{.app_name}}
    app.kubernetes.io/component: api
    app.kubernetes.io/framework: go-chi
spec:
  replicas: {{.replicas}}
  selector:
    matchLabels:
      app: {{.app_name}}
  template:
    metadata:
      labels:
        app: {{.app_name}}
    spec:
      containers:
      - name: {{.app_name}}
        image: {{.image}}
        ports:
        - containerPort: {{.port}}
          name: http
        envFrom:
        - configMapRef:
            name: {{.app_name}}-config
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        readinessProbe:
          httpGet:
            path: /healthz
            port: {{.port}}
          initialDelaySeconds: 5
          periodSeconds: 10
        livenessProbe:
          httpGet:
            path: /livez
            port: {{.port}}
          initialDelaySeconds: 15
          periodSeconds: 20
---
apiVersion: v1
kind: Service
metadata:
  name: {{.app_name}}
  namespace: {{.namespace}}
  labels:
    app.kubernetes.io/name: {{.app_name}}
spec:
  type: ClusterIP
  ports:
  - port: {{.port}}
    targetPort: {{.port}}
    protocol: TCP
    name: http
  selector:
    app: {{.app_name}}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{.app_name}}-config
  namespace: {{.namespace}}
data:
  APP_ENV: "{{.environment}}"
  PORT: "{{.port}}"
`,
			HelmValues: `nameOverride: "{{.app_name}}"
replicaCount: {{.replicas}}

image:
  repository: {{.image}}
  pullPolicy: IfNotPresent
  tag: "latest"

service:
  type: ClusterIP
  port: {{.port}}

env:
  APP_ENV: "{{.environment}}"
  PORT: "{{.port}}"

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi
`,
			DockerCompose: `version: '3.8'
services:
  {{.app_name}}:
    image: {{.image}}
    ports:
      - "{{.port}}:{{.port}}"
    environment:
      - APP_ENV={{.environment}}
      - PORT={{.port}}
    restart: unless-stopped
`,
			CreatedAt: seedTime,
			UpdatedAt: seedTime,
		},
		{
			ID:          BuiltinIDNodeWeb,
			Name:        "Node.js Web App",
			Description: "Express.js web application with Nginx Ingress routing, health endpoints, and Docker compose support.",
			Category:    CategoryWeb,
			Framework:   FrameworkNodeExpress,
			BuiltIn:     true,
			TenantID:    "default-tenant",
			Tags:        []string{"nodejs", "express", "web", "frontend", "ingress"},
			Variables: []TemplateVariable{
				{
					Name:     "app_name",
					Label:    "Application Name",
					Type:     "string",
					Default:  "node-web-app",
					Required: true,
				},
				{
					Name:     "namespace",
					Label:    "Kubernetes Namespace",
					Type:     "string",
					Default:  "default",
					Required: true,
				},
				{
					Name:     "image",
					Label:    "Container Image",
					Type:     "string",
					Default:  "node:20-alpine",
					Required: true,
				},
				{
					Name:     "port",
					Label:    "Service Port",
					Type:     "number",
					Default:  "3000",
					Required: true,
				},
				{
					Name:     "replicas",
					Label:    "Replicas",
					Type:     "number",
					Default:  "2",
					Required: true,
				},
				{
					Name:     "host",
					Label:    "Ingress Hostname",
					Type:     "string",
					Default:  "app.example.com",
					Required: false,
				},
			},
			ManifestYAML: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.app_name}}
  namespace: {{.namespace}}
  labels:
    app.kubernetes.io/name: {{.app_name}}
    app.kubernetes.io/component: web
    app.kubernetes.io/framework: node-express
spec:
  replicas: {{.replicas}}
  selector:
    matchLabels:
      app: {{.app_name}}
  template:
    metadata:
      labels:
        app: {{.app_name}}
    spec:
      containers:
      - name: {{.app_name}}
        image: {{.image}}
        ports:
        - containerPort: {{.port}}
          name: http
        resources:
          requests:
            cpu: 100m
            memory: 256Mi
          limits:
            cpu: 1000m
            memory: 1Gi
        readinessProbe:
          httpGet:
            path: /health
            port: {{.port}}
          initialDelaySeconds: 5
          periodSeconds: 10
        livenessProbe:
          httpGet:
            path: /health
            port: {{.port}}
          initialDelaySeconds: 10
          periodSeconds: 15
---
apiVersion: v1
kind: Service
metadata:
  name: {{.app_name}}
  namespace: {{.namespace}}
  labels:
    app.kubernetes.io/name: {{.app_name}}
spec:
  type: ClusterIP
  ports:
  - port: {{.port}}
    targetPort: {{.port}}
    protocol: TCP
    name: http
  selector:
    app: {{.app_name}}
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{.app_name}}-ingress
  namespace: {{.namespace}}
  annotations:
    kubernetes.io/ingress.class: nginx
spec:
  rules:
  - host: {{.host}}
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: {{.app_name}}
            port:
              number: {{.port}}
`,
			HelmValues: `nameOverride: "{{.app_name}}"
replicaCount: {{.replicas}}

image:
  repository: {{.image}}
  pullPolicy: IfNotPresent
  tag: "latest"

service:
  type: ClusterIP
  port: {{.port}}

ingress:
  enabled: true
  className: "nginx"
  hosts:
    - host: "{{.host}}"
      paths:
        - path: /
          pathType: Prefix

resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 100m
    memory: 256Mi
`,
			DockerCompose: `version: '3.8'
services:
  {{.app_name}}:
    image: {{.image}}
    ports:
      - "{{.port}}:{{.port}}"
    environment:
      - NODE_ENV=production
      - PORT={{.port}}
    restart: unless-stopped
`,
			CreatedAt: seedTime,
			UpdatedAt: seedTime,
		},
		{
			ID:          BuiltinIDPostgresDB,
			Name:        "PostgreSQL Database",
			Description: "StatefulSet-managed PostgreSQL database with persistent volume claims, credentials secret, and headless service.",
			Category:    CategoryDatabase,
			Framework:   FrameworkPostgres,
			BuiltIn:     true,
			TenantID:    "default-tenant",
			Tags:        []string{"postgres", "sql", "database", "statefulset", "storage"},
			Variables: []TemplateVariable{
				{
					Name:     "db_name",
					Label:    "Database Name",
					Type:     "string",
					Default:  "appdb",
					Required: true,
				},
				{
					Name:     "namespace",
					Label:    "Kubernetes Namespace",
					Type:     "string",
					Default:  "default",
					Required: true,
				},
				{
					Name:     "db_user",
					Label:    "Database User",
					Type:     "string",
					Default:  "postgres",
					Required: true,
				},
				{
					Name:     "db_password",
					Label:    "Database Password",
					Type:     "string",
					Default:  "secretpassword",
					Required: true,
				},
				{
					Name:     "storage_size",
					Label:    "Storage Size",
					Type:     "string",
					Default:  "10Gi",
					Required: true,
				},
				{
					Name:     "port",
					Label:    "Database Port",
					Type:     "number",
					Default:  "5432",
					Required: true,
				},
			},
			ManifestYAML: `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{.db_name}}
  namespace: {{.namespace}}
  labels:
    app.kubernetes.io/name: {{.db_name}}
    app.kubernetes.io/component: database
spec:
  serviceName: {{.db_name}}
  replicas: 1
  selector:
    matchLabels:
      app: {{.db_name}}
  template:
    metadata:
      labels:
        app: {{.db_name}}
    spec:
      containers:
      - name: postgres
        image: postgres:16-alpine
        ports:
        - containerPort: {{.port}}
          name: postgres
        env:
        - name: POSTGRES_DB
          value: "{{.db_name}}"
        - name: POSTGRES_USER
          valueFrom:
            secretKeyRef:
              name: {{.db_name}}-secret
              key: username
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: {{.db_name}}-secret
              key: password
        volumeMounts:
        - name: data
          mountPath: /var/lib/postgresql/data
        resources:
          requests:
            cpu: 250m
            memory: 512Mi
          limits:
            cpu: 1000m
            memory: 2Gi
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: {{.storage_size}}
---
apiVersion: v1
kind: Service
metadata:
  name: {{.db_name}}
  namespace: {{.namespace}}
  labels:
    app.kubernetes.io/name: {{.db_name}}
spec:
  type: ClusterIP
  ports:
  - port: {{.port}}
    targetPort: {{.port}}
    name: postgres
  selector:
    app: {{.db_name}}
---
apiVersion: v1
kind: Secret
metadata:
  name: {{.db_name}}-secret
  namespace: {{.namespace}}
type: Opaque
stringData:
  username: "{{.db_user}}"
  password: "{{.db_password}}"
`,
			HelmValues: `nameOverride: "{{.db_name}}"

image:
  repository: postgres
  tag: 16-alpine

auth:
  database: "{{.db_name}}"
  username: "{{.db_user}}"
  password: "{{.db_password}}"

primary:
  persistence:
    enabled: true
    size: "{{.storage_size}}"
  resources:
    limits:
      cpu: 1000m
      memory: 2Gi
    requests:
      cpu: 250m
      memory: 512Mi

service:
  port: {{.port}}
`,
			DockerCompose: `version: '3.8'
services:
  {{.db_name}}:
    image: postgres:16-alpine
    ports:
      - "{{.port}}:5432"
    environment:
      POSTGRES_DB: {{.db_name}}
      POSTGRES_USER: {{.db_user}}
      POSTGRES_PASSWORD: {{.db_password}}
    volumes:
      - pgdata:/var/lib/postgresql/data
    restart: unless-stopped

volumes:
  pgdata:
`,
			CreatedAt: seedTime,
			UpdatedAt: seedTime,
		},
	}
}
