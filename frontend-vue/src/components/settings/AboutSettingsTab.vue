<script setup lang="ts">
</script>

<template>
  <div class="about-container animate-fade-in">
    <div class="settings-card glass-panel">
      <div class="about-hero">
        <div class="about-logo"><BaseIcon name="anchor" size="xl" /></div>
        <div class="about-hero-text">
          <div class="about-version-tag">
            <span class="badge badge-emerald">v0.1.0 STABLE</span>
            <span class="badge badge-cyan">ENTERPRISE EDITION</span>
          </div>
          <h2 class="about-title">Kubernetes Self-Host Management Platform</h2>
          <p class="about-desc">
            High-performance unified Kubernetes control plane, AI-driven root cause diagnosis, GitOps orchestration,
            and zero-trust multi-tenancy suite.
          </p>
        </div>
      </div>

      <div class="spec-grid">
        <div class="spec-card glass-panel">
          <div class="spec-icon"><BaseIcon name="zap" size="md" /></div>
          <div class="spec-meta">
            <span class="spec-label">Backend Control Plane</span>
            <span class="spec-value">Go 1.22+ (Chi Router / Hexagonal Arch)</span>
          </div>
        </div>

        <div class="spec-card glass-panel">
          <div class="spec-icon"><BaseIcon name="grid" size="md" /></div>
          <div class="spec-meta">
            <span class="spec-label">Frontend Framework</span>
            <span class="spec-value">Vue 3.4+ / Vite 5 / TypeScript</span>
          </div>
        </div>

        <div class="spec-card glass-panel">
          <div class="spec-icon"><BaseIcon name="database" size="md" /></div>
          <div class="spec-meta">
            <span class="spec-label">Primary Database</span>
            <span class="spec-value">PostgreSQL 16 Multi-Tenant</span>
          </div>
        </div>

        <div class="spec-card glass-panel">
          <div class="spec-icon"><BaseIcon name="play" size="md" /></div>
          <div class="spec-meta">
            <span class="spec-label">In-Memory Cache & PubSub</span>
            <span class="spec-value">Redis 7 Alpine</span>
          </div>
        </div>

        <div class="spec-card glass-panel">
          <div class="spec-icon"><BaseIcon name="file-text" size="md" /></div>
          <div class="spec-meta">
            <span class="spec-label">Software License</span>
            <span class="spec-value">MIT Open Source License</span>
          </div>
        </div>

        <div class="spec-card glass-panel">
          <div class="spec-icon"><BaseIcon name="lock" size="md" /></div>
          <div class="spec-meta">
            <span class="spec-label">Authentication Scheme</span>
            <span class="spec-value">JWT Bearer Auth & Tenant Isolation</span>
          </div>
        </div>
      </div>

      <div class="about-footer">
        <div class="about-copyright">
          <span>© 2026 K8s Self-Host Platform. Built for autonomous cloud-native operations.</span>
        </div>
        <div class="about-links">
          <a class="about-link" href="https://kubernetes.io/docs/" target="_blank" rel="noopener noreferrer">Documentation</a>
          <span class="dot-sep">•</span>
          <a class="about-link" href="/api/v1/healthz" target="_blank" rel="noopener noreferrer">REST API Docs</a>
          <span class="dot-sep">•</span>
          <a class="about-link" href="https://github.com/tiendat1751998/k8s-selfhost-agent" target="_blank" rel="noopener noreferrer">GitHub Repository</a>
        </div>
      </div>
    </div>
  </div>
</template>
