-- Migration: 038_scaffold_templates.up.sql
-- Description: Creates scaffold_templates table for 1-click application scaffolding.

DROP TABLE IF EXISTS scaffold_templates;

CREATE TABLE IF NOT EXISTS scaffold_templates (
    id             TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name           TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    category       TEXT NOT NULL DEFAULT 'web',
    framework      TEXT NOT NULL DEFAULT 'go-chi',
    manifest_yaml  TEXT NOT NULL DEFAULT '',
    helm_values    TEXT NOT NULL DEFAULT '',
    docker_compose TEXT NOT NULL DEFAULT '',
    variables      JSONB NOT NULL DEFAULT '[]',
    tags           JSONB NOT NULL DEFAULT '[]',
    built_in       BOOLEAN NOT NULL DEFAULT false,
    tenant_id      TEXT NOT NULL DEFAULT 'default-tenant',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(name, tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_scaffold_tenant ON scaffold_templates(tenant_id);
CREATE INDEX IF NOT EXISTS idx_scaffold_category ON scaffold_templates(category);
CREATE INDEX IF NOT EXISTS idx_scaffold_framework ON scaffold_templates(framework);
CREATE INDEX IF NOT EXISTS idx_scaffold_builtin ON scaffold_templates(built_in);

-- Seed real starter templates
INSERT INTO scaffold_templates (id, name, description, category, framework, manifest_yaml, helm_values, docker_compose, variables, tags, built_in, tenant_id)
VALUES
('tpl-builtin-go-microservice', 'Go Microservice', 'High-performance Go microservice with chi router, health probes, configmaps, and multi-stage container build.', 'api', 'go-chi',
'apiVersion: apps/v1
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
  PORT: "{{.port}}"',
'nameOverride: "{{.app_name}}"
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
  PORT: "{{.port}}"',
'version: "3.8"
services:
  {{.app_name}}:
    image: {{.image}}
    ports:
      - "{{.port}}:{{.port}}"
    environment:
      - APP_ENV={{.environment}}
      - PORT={{.port}}
    restart: unless-stopped',
'[{"name":"app_name","label":"Application Name","type":"string","default":"go-microservice","required":true},{"name":"namespace","label":"Kubernetes Namespace","type":"string","default":"default","required":true},{"name":"image","label":"Container Image","type":"string","default":"golang:1.22-alpine","required":true},{"name":"port","label":"Service Port","type":"number","default":"8080","required":true},{"name":"replicas","label":"Replicas","type":"number","default":"2","required":true},{"name":"environment","label":"Environment","type":"select","default":"production","required":false,"options":["development","staging","production"]}]',
'["golang", "chi", "microservice", "api", "kubernetes"]', true, 'default-tenant'),

('tpl-builtin-node-fastify', 'Node.js Fastify', 'Ultra-fast low-overhead Node.js REST API with Fastify framework, JSON schema validation, and health checks.', 'api', 'node-fastify',
'apiVersion: apps/v1
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
    app: {{.app_name}}',
'nameOverride: "{{.app_name}}"
replicaCount: {{.replicas}}
image:
  repository: {{.image}}
  pullPolicy: IfNotPresent
  tag: "latest"
service:
  type: ClusterIP
  port: {{.port}}',
'version: "3.8"
services:
  {{.app_name}}:
    image: {{.image}}
    ports:
      - "{{.port}}:{{.port}}"
    environment:
      - NODE_ENV=production
      - PORT={{.port}}
    restart: unless-stopped',
'[{"name":"app_name","label":"Application Name","type":"string","default":"fastify-api","required":true},{"name":"namespace","label":"Kubernetes Namespace","type":"string","default":"default","required":true},{"name":"image","label":"Container Image","type":"string","default":"node:20-alpine","required":true},{"name":"port","label":"Service Port","type":"number","default":"3000","required":true},{"name":"replicas","label":"Replicas","type":"number","default":"2","required":true}]',
'["nodejs", "fastify", "typescript", "api", "rest"]', true, 'default-tenant'),

('tpl-builtin-python-fastapi', 'Python FastAPI', 'High-performance Python asynchronous REST API service with FastAPI, Pydantic type validation, and OpenAPI documentation.', 'api', 'python-fastapi',
'apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.app_name}}
  namespace: {{.namespace}}
  labels:
    app.kubernetes.io/name: {{.app_name}}
    app.kubernetes.io/component: api
    app.kubernetes.io/framework: python-fastapi
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
    app: {{.app_name}}',
'nameOverride: "{{.app_name}}"
replicaCount: {{.replicas}}
image:
  repository: {{.image}}
  pullPolicy: IfNotPresent
  tag: "latest"
service:
  type: ClusterIP
  port: {{.port}}',
'version: "3.8"
services:
  {{.app_name}}:
    image: {{.image}}
    ports:
      - "{{.port}}:{{.port}}"
    environment:
      - PYTHONUNBUFFERED=1
      - PORT={{.port}}
    restart: unless-stopped',
'[{"name":"app_name","label":"Application Name","type":"string","default":"python-fastapi","required":true},{"name":"namespace","label":"Kubernetes Namespace","type":"string","default":"default","required":true},{"name":"image","label":"Container Image","type":"string","default":"python:3.11-slim","required":true},{"name":"port","label":"Service Port","type":"number","default":"8000","required":true},{"name":"replicas","label":"Replicas","type":"number","default":"2","required":true}]',
'["python", "fastapi", "uvicorn", "async", "api"]', true, 'default-tenant'),

('tpl-builtin-nginx-proxy', 'Nginx Reverse Proxy', 'Production Nginx edge reverse proxy & static asset server with TLS termination, caching headers, and rate-limiting.', 'web', 'nginx',
'apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.app_name}}
  namespace: {{.namespace}}
  labels:
    app.kubernetes.io/name: {{.app_name}}
    app.kubernetes.io/component: proxy
    app.kubernetes.io/framework: nginx
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
            cpu: 50m
            memory: 64Mi
          limits:
            cpu: 250m
            memory: 256Mi
        readinessProbe:
          httpGet:
            path: /
            port: {{.port}}
          initialDelaySeconds: 2
          periodSeconds: 5
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
              number: {{.port}}',
'nameOverride: "{{.app_name}}"
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
          pathType: Prefix',
'version: "3.8"
services:
  {{.app_name}}:
    image: {{.image}}
    ports:
      - "{{.port}}:{{.port}}"
    restart: unless-stopped',
'[{"name":"app_name","label":"Application Name","type":"string","default":"nginx-proxy","required":true},{"name":"namespace","label":"Kubernetes Namespace","type":"string","default":"default","required":true},{"name":"image","label":"Container Image","type":"string","default":"nginx:alpine","required":true},{"name":"port","label":"Service Port","type":"number","default":"80","required":true},{"name":"replicas","label":"Replicas","type":"number","default":"2","required":true},{"name":"host","label":"Ingress Hostname","type":"string","default":"proxy.example.com","required":false}]',
'["nginx", "proxy", "web", "frontend", "ingress"]', true, 'default-tenant'),

('tpl-builtin-postgres-db', 'PostgreSQL StatefulSet', 'StatefulSet-managed PostgreSQL 16 database with persistent volume claims, credentials secret, and headless service.', 'database', 'postgres',
'apiVersion: apps/v1
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
  password: "{{.db_password}}"',
'nameOverride: "{{.db_name}}"
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
  port: {{.port}}',
'version: "3.8"
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
  pgdata:',
'[{"name":"db_name","label":"Database Name","type":"string","default":"appdb","required":true},{"name":"namespace","label":"Kubernetes Namespace","type":"string","default":"default","required":true},{"name":"db_user","label":"Database User","type":"string","default":"postgres","required":true},{"name":"db_password","label":"Database Password","type":"string","default":"secretpassword","required":true},{"name":"storage_size","label":"Storage Size","type":"string","default":"10Gi","required":true},{"name":"port","label":"Database Port","type":"number","default":"5432","required":true}]',
'["postgres", "sql", "database", "statefulset", "storage"]', true, 'default-tenant')
ON CONFLICT (name, tenant_id) DO UPDATE SET
    description = EXCLUDED.description,
    category = EXCLUDED.category,
    framework = EXCLUDED.framework,
    manifest_yaml = EXCLUDED.manifest_yaml,
    helm_values = EXCLUDED.helm_values,
    docker_compose = EXCLUDED.docker_compose,
    variables = EXCLUDED.variables,
    tags = EXCLUDED.tags,
    built_in = EXCLUDED.built_in,
    updated_at = now();

