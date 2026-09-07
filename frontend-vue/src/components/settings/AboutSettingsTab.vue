<script setup lang="ts">
</script>

<template>
  <div class="about-container animate-fade-in">
    <div class="settings-card glass-panel">
      <div class="about-hero">
        <div class="about-logo">⎈</div>
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
          <div class="spec-icon">⚡</div>
          <div class="spec-meta">
            <span class="spec-label">Backend Control Plane</span>
            <span class="spec-value">Go 1.22+ (Chi Router / Hexagonal Arch)</span>
          </div>
        </div>

        <div class="spec-card glass-panel">
          <div class="spec-icon">🎨</div>
          <div class="spec-meta">
            <span class="spec-label">Frontend Framework</span>
            <span class="spec-value">Vue 3.4+ / Vite 5 / TypeScript</span>
          </div>
        </div>

        <div class="spec-card glass-panel">
          <div class="spec-icon">🗄️</div>
          <div class="spec-meta">
            <span class="spec-label">Primary Database</span>
            <span class="spec-value">PostgreSQL 16 Multi-Tenant</span>
          </div>
        </div>

        <div class="spec-card glass-panel">
          <div class="spec-icon">🚀</div>
          <div class="spec-meta">
            <span class="spec-label">In-Memory Cache & PubSub</span>
            <span class="spec-value">Redis 7 Alpine</span>
          </div>
        </div>

        <div class="spec-card glass-panel">
          <div class="spec-icon">📜</div>
          <div class="spec-meta">
            <span class="spec-label">Software License</span>
            <span class="spec-value">MIT Open Source License</span>
          </div>
        </div>

        <div class="spec-card glass-panel">
          <div class="spec-icon">🔒</div>
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
          <span class="about-link">Documentation</span>
          <span class="dot-sep">•</span>
          <span class="about-link">REST API Docs</span>
          <span class="dot-sep">•</span>
          <span class="about-link">GitHub Repository</span>
        </div>
      </div>
    </div>
  </div>
</template>
