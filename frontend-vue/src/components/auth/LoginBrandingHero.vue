<template>
  <aside class="login-hero-card glass-panel">
    <div class="hero-top">
      <div class="brand-section">
        <div class="brand-icon-wrapper">
          <div class="brand-icon"><BaseIcon name="anchor" size="xl" /></div>
          <div class="brand-glow"></div>
        </div>
        <h1 class="brand-title">K8S<span>CONTROL</span></h1>
        <p class="brand-subtitle">Enterprise Hybrid Control Plane</p>
      </div>

      <div class="hero-feature-list">
        <div class="hero-feature-item">
          <div class="feature-icon-badge"><BaseIcon name="layers" size="md" /></div>
          <div class="feature-content">
            <span class="feature-title">Multi-Cluster Mesh</span>
            <span class="feature-desc">Centralized orchestration & distributed edge node telemetry.</span>
          </div>
        </div>

        <div class="hero-feature-item">
          <div class="feature-icon-badge"><BaseIcon name="activity" size="md" /></div>
          <div class="feature-content">
            <span class="feature-title">Unified Observability</span>
            <span class="feature-desc">High-throughput distributed logs & automated RCA diagnostics.</span>
          </div>
        </div>

        <div class="hero-feature-item">
          <div class="feature-icon-badge"><BaseIcon name="shield" size="md" /></div>
          <div class="feature-content">
            <span class="feature-title">Enterprise Governance</span>
            <span class="feature-desc">Strict multi-tenancy isolation & RBAC security controls.</span>
          </div>
        </div>
      </div>
    </div>

    <div class="hero-bottom">
      <div class="hero-status-mesh">
        <div class="mesh-node">
          <span class="mesh-node-label">Control Plane</span>
          <span class="mesh-node-val">
            <span class="pulse-dot pulse-dot-emerald"></span>
            SYNCHRONIZED
          </span>
        </div>
        <div class="mesh-node">
          <span class="mesh-node-label">Cluster Mesh</span>
          <span class="mesh-node-val">
            <span class="pulse-dot pulse-dot-cyan"></span>
            ONLINE
          </span>
        </div>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
// Pure presentation hero component for brand identity & cluster status mesh
</script>
