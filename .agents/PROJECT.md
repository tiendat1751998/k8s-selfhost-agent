# K8S SELF-HEALING AGENT & AGENT RUNNER SYSTEM

## 1. SYSTEM OVERVIEW

This is a production-grade AI-powered system that combines:

* Kubernetes Self-Healing Platform
* GitOps Controller
* AI Agent Execution Runner

The system continuously detects Kubernetes incidents, performs root cause analysis using LLM, generates fixes, and applies changes via GitOps.

In parallel, an Agent Runner executes software engineering tasks from TASKS.md in an autonomous loop.

---

## 2. CORE OBJECTIVE

Build a system that can:

### Kubernetes Self-Healing

* Detect cluster incidents
* Collect logs, events, metrics, YAML
* Perform Root Cause Analysis (RCA)
* Generate remediation plan
* Create GitOps Pull Request
* Track incident lifecycle

### Agent Runner

* Read TASKS.md
* Execute tasks one by one
* Generate code automatically
* Run tests in sandbox
* Fix errors automatically
* Commit and create PR
* Update TASKS.md state

---

## 3. INCIDENT TYPES

Handle:

* CrashLoopBackOff
* OOMKilled
* ImagePullBackOff
* FailedScheduling
* NodeNotReady
* Probe failures
* Network failures
* Storage failures
* HPA failures
* Resource exhaustion
* Ingress failures

---

## 4. SYSTEM COMPONENTS

### 4.1 Kubernetes Self-Healing System

* Event Watcher
* Log Collector
* Metrics Collector
* Incident Aggregator
* RCA Engine (LLM-based)
* GitOps Controller
* PR Generator
* Incident Store (PostgreSQL)
* Cache (Redis)
* Queue (NATS JetStream)
* Dashboard (React)

---

### 4.2 Agent Runner System

* Task Scheduler (reads TASKS.md)
* LLM Engine (Ollama / OpenAI-compatible API)
* Code Generator (Go/YAML/Tests)
* Execution Sandbox (Docker/K8s Job)
* Test Runner (go test / lint)
* Git Manager (commit / PR)
* State Manager (TASKS.md updater)

---

## 5. TECHNOLOGY STACK

### Backend

* Golang

### Data

* PostgreSQL
* Redis

### Messaging

* NATS JetStream

### AI Layer

* Ollama
* vLLM
* OpenAI-compatible APIs

### Observability

* Prometheus
* Grafana
* Loki
* OpenTelemetry

### GitOps

* ArgoCD
* FluxCD
* GitHub / GitLab / Gitea

### Frontend

* React + TypeScript + Tailwind

### Infrastructure

* Kubernetes
* Docker

---

## 6. RCA ENGINE

For each incident:

Collect:

* kubectl describe
* pod logs (last 50–200 lines)
* events
* deployment/statefulset YAML
* service/ingress YAML
* node metrics
* pod metrics

Output:

* Root Cause
* Evidence
* Confidence Score
* Risk Level
* Remediation Plan
* Rollback Plan

---

## 7. GITOPS FLOW

If RCA confidence ≥ threshold:

1. Generate patch (YAML / Helm / Kustomize)
2. Create Git branch
3. Commit changes
4. Open Pull Request
5. Attach RCA report

Supports:

* GitHub
* GitLab
* Gitea

---

## 8. AGENT RUNNER EXECUTION MODEL

The Agent Runner executes tasks in TASKS.md using a deterministic loop:

### Execution Flow

1. Read TASKS.md
2. Select next uncompleted task
3. Send task to LLM
4. Generate code / config / tests
5. Write files to workspace
6. Run tests in sandbox
7. If failure:

   * send error back to LLM
   * fix automatically
   * retry
8. If success:

   * commit code
   * create PR
   * mark task done
9. Repeat

---

## 9. EXECUTION RULES

### Rules for Agent Runner

* Execute ONE task per cycle
* Never skip tasks
* Never stop mid-task
* Always fix errors automatically
* Always update TASKS.md state

---

## 10. SECURITY REQUIREMENTS

* RBAC (least privilege)
* TLS everywhere
* Secret encryption
* Audit logging
* Input validation
* Protection against prompt injection
* SBOM generation

---

## 11. OBSERVABILITY REQUIREMENTS

Each service must expose:

* Prometheus metrics
* Structured logs (JSON)
* OpenTelemetry traces
* Health endpoints

---

## 12. ARCHITECTURE PRINCIPLES

* Clean Architecture (Go backend)
* Domain-Driven Design (Incidents, Reports)
* Stateless services
* Horizontal scaling
* Fault tolerance
* Retry + backoff mechanisms

---

## 13. DEPLOYMENT MODEL

Supports:

* Single cluster deployment
* Multi-cluster deployment

Includes:

* Helm charts
* Kustomize manifests
* ArgoCD applications
* Terraform (optional)

---

## 14. DEFINITION OF DONE

A task is complete only when:

* Code compiles successfully
* Tests pass
* No critical errors
* Service is runnable
* Security requirements satisfied
* Git commit + PR created
* TASKS.md updated

---

## 15. FINAL SYSTEM GOAL

This system is considered complete when:

* Kubernetes Self-Healing works end-to-end
* Agent Runner executes tasks autonomously
* GitOps PR workflow is fully automated
* RCA engine produces actionable fixes
* System is production deployable
