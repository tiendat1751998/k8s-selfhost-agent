package scaffold

func buildGoMicroserviceTemplate() Template {
	return Template{
		ID:          BuiltinIDGoMicroservice,
		Name:        "Go Microservice",
		Description: "High-performance Go microservice with chi router, health probes, configmaps, and multi-stage container build.",
		Category:    CategoryAPI,
		Framework:   FrameworkGoChi,
		BuiltIn:     true,
		TenantID:    "default-tenant",
		Tags:        []string{"golang", "chi", "microservice", "api", "kubernetes"},
		Variables: []TemplateVariable{
			{Name: "app_name", Label: "Application Name", Type: "string", Default: "go-microservice", Required: true},
			{Name: "namespace", Label: "Kubernetes Namespace", Type: "string", Default: "default", Required: true},
			{Name: "image", Label: "Container Image", Type: "string", Default: "golang:1.22-alpine", Required: true},
			{Name: "port", Label: "Service Port", Type: "number", Default: "8080", Required: true},
			{Name: "replicas", Label: "Replicas", Type: "number", Default: "2", Required: true},
			{Name: "environment", Label: "Environment", Type: "select", Default: "production", Required: false, Options: []string{"development", "staging", "production"}},
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
		CreatedAt: builtinSeedTime,
		UpdatedAt: builtinSeedTime,
	}
}

func buildNodeFastifyTemplate() Template {
	return Template{
		ID:          BuiltinIDNodeFastify,
		Name:        "Node.js Fastify",
		Description: "Ultra-fast low-overhead Node.js REST API with Fastify framework, JSON schema validation, and health checks.",
		Category:    CategoryAPI,
		Framework:   FrameworkNodeFastify,
		BuiltIn:     true,
		TenantID:    "default-tenant",
		Tags:        []string{"nodejs", "fastify", "typescript", "api", "rest"},
		Variables: []TemplateVariable{
			{Name: "app_name", Label: "Application Name", Type: "string", Default: "fastify-api", Required: true},
			{Name: "namespace", Label: "Kubernetes Namespace", Type: "string", Default: "default", Required: true},
			{Name: "image", Label: "Container Image", Type: "string", Default: "node:20-alpine", Required: true},
			{Name: "port", Label: "Service Port", Type: "number", Default: "3000", Required: true},
			{Name: "replicas", Label: "Replicas", Type: "number", Default: "2", Required: true},
		},
		ManifestYAML: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.app_name}}
  namespace: {{.namespace}}
  labels:
    app.kubernetes.io/name: {{.app_name}}
    app.kubernetes.io/component: api
    app.kubernetes.io/framework: node-fastify
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
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
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
		CreatedAt: builtinSeedTime,
		UpdatedAt: builtinSeedTime,
	}
}
