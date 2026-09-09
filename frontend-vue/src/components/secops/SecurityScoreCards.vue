<script setup lang="ts">
interface Props {
  postureScore: string
  passingRules: number
  totalRules: number
  criticalCves: number
  gatePassed: boolean
  exposedSecrets: number
  totalViolations: number
  highViolations: number
  medViolations: number
  lowViolations: number
  frameworksCount?: number
  frameworkNames?: string
}

defineProps<Props>()
</script>

<template>
  <div class="metrics-grid">
    <!-- Card 1: Production Security Gate -->
    <div class="metric-card glass-panel glass-panel-glow">
      <div class="metric-header">
        <span class="metric-title">PRODUCTION SECURITY GATE</span>
        <span class="badge" :class="gatePassed ? 'badge-emerald' : 'badge-rose'">
          {{ gatePassed ? 'ACTIVE & ENFORCED' : 'GATE BLOCKED' }}
        </span>
      </div>
      <div class="metric-val" :class="gatePassed ? 'text-emerald' : 'text-rose'">
        {{ gatePassed ? 'PASSED ✅' : 'BLOCKED ⚠️' }}
      </div>
      <div class="metric-footer">
        <span :class="criticalCves === 0 ? 'text-emerald' : 'text-rose'">
          ● {{ criticalCves }} Critical CVEs
        </span>
        <span class="text-muted">{{ gatePassed ? 'Deployments Unblocked' : 'Action Required' }}</span>
      </div>
    </div>

    <!-- Card 2: Security Posture Score -->
    <div class="metric-card glass-panel glass-panel-glow">
      <div class="metric-header">
        <span class="metric-title">SECURITY POSTURE SCORE</span>
        <span class="badge badge-cyan">CIS & NIST BENCHMARKS</span>
      </div>
      <div class="metric-val font-mono text-cyan">
        {{ postureScore }} <span class="metric-unit">Compliance</span>
      </div>
      <div class="metric-footer">
        <span class="text-emerald">{{ passingRules }} Passing Rules</span>
        <span class="text-muted">of {{ totalRules > 0 ? totalRules : 42 }} Rules</span>
      </div>
    </div>

    <!-- Card 3: Exposed Secrets & Certificates -->
    <div class="metric-card glass-panel glass-panel-glow">
      <div class="metric-header">
        <span class="metric-title">EXPOSED SECRETS & CERTS</span>
        <span class="badge" :class="exposedSecrets === 0 ? 'badge-emerald' : 'badge-rose'">
          {{ exposedSecrets === 0 ? 'VAULT SECURED' : 'SECRETS AT RISK' }}
        </span>
      </div>
      <div class="metric-val font-mono" :class="exposedSecrets === 0 ? 'text-emerald' : 'text-rose'">
        {{ exposedSecrets }} <span class="metric-unit">Flagged</span>
      </div>
      <div class="metric-footer">
        <span :class="exposedSecrets === 0 ? 'text-emerald' : 'text-amber'">
          {{ exposedSecrets === 0 ? 'HashiCorp Vault + ESO Synced' : 'Action Required on Tokens' }}
        </span>
        <span class="text-muted">Auto-Lease</span>
      </div>
    </div>

    <!-- Card 4: Policy & RBAC Violations -->
    <div class="metric-card glass-panel glass-panel-glow">
      <div class="metric-header">
        <span class="metric-title">POLICY & PRIVILEGE AUDIT</span>
        <span class="badge" :class="totalViolations === 0 ? 'badge-emerald' : 'badge-amber'">
          {{ totalViolations === 0 ? 'CLEAN' : 'AUDIT' }}
        </span>
      </div>
      <div class="metric-val font-mono">
        {{ totalViolations }} <span class="metric-unit">Violations</span>
      </div>
      <div class="metric-footer">
        <span class="text-amber">{{ highViolations }} High Severity</span>
        <span class="text-muted">{{ medViolations }} Med / {{ lowViolations }} Low</span>
      </div>
    </div>
  </div>
</template>
