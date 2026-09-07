<template>
  <aside class="login-hero-card glass-panel">
    <div class="hero-top">
      <div class="brand-section">
        <div class="brand-icon-wrapper">
          <div class="brand-icon">⎈</div>
          <div class="brand-glow"></div>
        </div>
        <h1 class="brand-title">K8S<span>CONTROL</span></h1>
        <p class="brand-subtitle">Enterprise Hybrid Control Plane</p>
      </div>

      <div class="hero-feature-list">
        <div class="hero-feature-item">
          <div class="feature-icon-badge">🛡️</div>
          <div class="feature-content">
            <span class="feature-title">Air-Gapped ZeroTrust</span>
            <span class="feature-desc">FIPS 140-3 cryptographic boundaries & TLS v1.3 mandatory channels.</span>
          </div>
        </div>

        <div class="hero-feature-item">
          <div class="feature-icon-badge">⚡</div>
          <div class="feature-content">
            <span class="feature-title">Dual-Sync DR Gateway</span>
            <span class="feature-desc">Active-active multi-region synchronization with sub-second RPO.</span>
          </div>
        </div>

        <div class="hero-feature-item">
          <div class="feature-icon-badge">🔍</div>
          <div class="feature-content">
            <span class="feature-title">Trivy Supply Chain Gate</span>
            <span class="feature-desc">Pre-admission image attestation & runtime CVE quarantine enforcement.</span>
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
            ONLINE (DR-READY)
          </span>
        </div>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
// Pure presentation hero component for brand identity & cluster status mesh
</script>
