# Architecture Decision Records — K8s Self-Host Platform

> **Version:** 1.0
> **Date:** 2026-08-13

---

## ADR-001: Vue.js 3 Frontend Migration
- **Status:** Accepted
- **Context:** The current frontend application relies on Vanilla JS. As the platform has grown, the UI codebase has expanded to over 33 modules and 55+ JS files, resulting in a monolithic `index.html` (149KB). Managing state, reactivity, and complex UI components with pure Vanilla JS has become increasingly difficult, leading to maintainability issues, higher bug rates, and slower feature development.
- **Decision:** Migrate the frontend stack to Vue.js 3 + Vite + Pinia (for state management) + TypeScript. This migration will be executed using an incremental "island" strategy, replacing parts of the application piece-by-piece rather than attempting a high-risk big-bang rewrite.
- **Consequences:** 
  - *Pros:* Component-based architecture improves reusability; TypeScript provides type safety and better developer experience; Vite dramatically speeds up the build process; incremental migration reduces delivery risk and allows concurrent feature development.
  - *Cons:* Introduces a new build process and tooling overhead; requires developers to learn the Vue 3 Composition API; managing state between legacy Vanilla JS and new Vue components during the transition will require careful bridging.
- **Alternatives Considered:** 
  - *React:* Powerful, but has a steeper learning curve and a heavier footprint.
  - *Svelte:* Excellent performance, but slightly less enterprise ecosystem maturity compared to Vue.
  - *Big-bang rewrite:* Deemed too risky as it would halt new feature development for months.

---

## ADR-002: Hybrid Monolith Architecture
- **Status:** Accepted
- **Context:** The platform targets a wide spectrum of users, from solo developers running single-node clusters to large enterprises managing massive multi-tenant environments. A pure microservices architecture is too complex to operate for solo developers, while a traditional monolith struggles to scale heavy background operations independently.
- **Decision:** Adopt a Hybrid Monolith Architecture. The core management API will be compiled as a single Go 1.26 binary, adhering to Clean Architecture/DDD principles. Heavy, compute-intensive workloads (such as the backup engine, log collector, and pipeline runner) will optionally be broken out and deployed as independent microservices.
- **Consequences:** 
  - *Pros:* Extremely easy to deploy for simple use cases (single binary); scales efficiently by allowing enterprises to run heavy workloads on dedicated nodes; maintains strong domain boundaries within the codebase.
  - *Cons:* Requires a flexible configuration system to toggle between embedded and standalone execution modes; increases testing complexity as integration tests must cover both deployment topologies.
- **Alternatives Considered:** 
  - *Pure Microservices:* Rejected due to excessive operational overhead for small deployments.
  - *Pure Monolith:* Rejected because workloads like pipeline execution would heavily impact the performance of the core API.

---

## ADR-003: Multi-DB Backup Operator Pattern
- **Status:** Accepted
- **Context:** Multi-tenant clusters host a variety of stateful workloads (PostgreSQL, MySQL, MongoDB, Redis) that require reliable, scheduled, and ad-hoc backups. Managing distinct backup scripts for each is error-prone and scales poorly.
- **Decision:** Implement a CRD-inspired operator pattern inside the backend, featuring pluggable DB adapters for different database engines. Storage backends will support embedded MinIO, external S3, and the local filesystem. Specifically for Postgres, Point-In-Time-Recovery (PITR) will be supported via WAL (Write-Ahead Logging) streaming.
- **Consequences:** 
  - *Pros:* Unified interface for managing all database backups; easily extensible for new databases via the adapter pattern; enterprise-grade capabilities like PITR for critical Postgres workloads.
  - *Cons:* High initial engineering effort to design the generic adapter interfaces; managing long-running WAL streams requires robust network handling and storage capacity monitoring.
- **Alternatives Considered:** 
  - *Velero:* Excellent for cluster-level backups but lacks deep, application-aware database backup orchestration (like PITR for Postgres).
  - *Cron + Bash Scripts:* Fragile, difficult to monitor, and hard to maintain across multiple environments.

---

## ADR-004: Built-in CI/CD + External Integration
- **Status:** Accepted
- **Context:** Developers need a fast, integrated way to deploy applications. While smaller teams want out-of-the-box CI/CD, enterprise users typically have mature pipelines (ArgoCD, Jenkins, GitHub Actions) that they wish to connect to the platform.
- **Decision:** Provide a simple, built-in pipeline engine covering the standard lifecycle (build → test → scan → deploy). Container builds will use Kaniko to ensure secure, daemonless image creation. Simultaneously, build native API integrations and webhooks for external tools like ArgoCD, Tekton, Jenkins, and GitHub Actions.
- **Consequences:** 
  - *Pros:* Immediate value for new users with zero external dependencies; strict security via Kaniko (no Docker socket exposed); avoids vendor lock-in for enterprise clients.
  - *Cons:* Maintaining a built-in CI/CD engine is a significant ongoing engineering commitment; edge cases in build pipelines can be difficult to troubleshoot.
- **Alternatives Considered:** 
  - *External Only:* Forcing users to bring their own CI/CD hurts the "batteries-included" developer experience.
  - *Full-featured Internal CI/CD:* Attempting to build a GitLab CI competitor is out of scope and would dilute focus from cluster management.

---

## ADR-005: Centralized Logging with Dual Mode
- **Status:** Accepted
- **Context:** Visibility into application and cluster logs is critical. However, full-scale logging stacks (like ELK) consume massive resources, making them inappropriate for edge deployments or solo developers.
- **Decision:** Implement a centralized logging system with a dual-mode architecture. Deploy a DaemonSet log agent to collect logs. For small deployments, logs will route to a built-in lightweight log engine. For enterprise environments, the platform will seamlessly integrate with external stacks like Loki or Elasticsearch.
- **Consequences:** 
  - *Pros:* Zero-config setup for small users; scales to handle millions of log lines per minute for enterprises; provides a consistent UI experience regardless of the backend.
  - *Cons:* The backend and frontend must support abstractions for two entirely different query languages and storage paradigms.
- **Alternatives Considered:** 
  - *Loki as Default:* While lighter than ELK, Loki still requires significant memory and storage, violating the lightweight constraints of small deployments.
  - *Cloud-hosted Logging:* Violates the core self-hosted, air-gap friendly nature of the platform.

---

## ADR-006: Secret Management with Vault Integration
- **Status:** Accepted
- **Context:** Managing credentials securely is paramount in a multi-tenant environment. While the platform currently encrypts secrets using AES-256-GCM, enterprise customers require compliance-driven external key management, rotation, and auditing.
- **Decision:** Retain the existing built-in AES-256-GCM encrypted secret storage as the default. Introduce optional HashiCorp Vault integration for advanced setups. Additionally, implement automated secret rotation scheduling and background scanning for leaked secrets within cluster workloads.
- **Consequences:** 
  - *Pros:* Maintains a frictionless developer experience; meets strict enterprise compliance requirements; proactive security via leak scanning prevents credential exposure.
  - *Cons:* Synchronizing state and handling failovers when the external Vault cluster is unreachable introduces complex distributed systems problems.
- **Alternatives Considered:** 
  - *Vault Only:* Forces all users to operate Vault, which is notoriously complex to deploy and maintain.
  - *Native K8s Secrets Only:* Lacks native robust rotation scheduling, leak scanning, and external KMS synchronization.

---

## ADR-007: Security Scanner Integration
- **Status:** Accepted
- **Context:** Ensuring the security posture of the cluster involves validating container images for vulnerabilities, checking cluster configurations against best practices, and monitoring runtime behavior.
- **Decision:** Integrate the Trivy library directly into the Go backend for embedded container image scanning without needing external services. Incorporate the CIS Kubernetes Benchmark for automated cluster configuration assessments. Add hooks for runtime security monitoring.
- **Consequences:** 
  - *Pros:* Embedding Trivy as a library avoids managing separate binaries; provides a holistic "single pane of glass" for security; immediate feedback loop in the CI/CD pipeline.
  - *Cons:* Trivy's vulnerability database can consume considerable memory and requires periodic updates, potentially causing spikes in resource usage on isolated networks.
- **Alternatives Considered:** 
  - *External SaaS Scanners:* Unacceptable for air-gapped or strict privacy deployments.
  - *Falco Only:* Excellent for runtime, but provides no static analysis for container images or misconfigurations.

---

## ADR-008: Helm Catalog / App Store
- **Status:** Accepted
- **Context:** Users need an easy, standardized way to deploy common platform infrastructure (e.g., Redis, RabbitMQ) and third-party applications without manually writing YAML or learning Helm CLI commands.
- **Decision:** Build a curated Helm Catalog / App Store into the platform. This feature will support 1-click deployments, the addition of custom chart repositories, and full application lifecycle management (upgrades, rollbacks, uninstalls) via a user-friendly UI.
- **Consequences:** 
  - *Pros:* Drastically reduces time-to-value for new clusters; abstracts away Helm CLI complexity; standardizes deployments across the platform.
  - *Cons:* Managing Helm release state programmatically can be prone to deadlocks (e.g., stuck releases); requires ongoing effort to curate and test the default chart repository.
- **Alternatives Considered:** 
  - *GitOps Only:* Requires users to understand Git and ArgoCD/Flux before deploying their first app, steepening the learning curve.
  - *Custom Operators for Everything:* Unscalable engineering effort to maintain operators for dozens of third-party applications.
