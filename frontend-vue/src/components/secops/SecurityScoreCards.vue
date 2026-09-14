<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'
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
        <span class="badge" :class="gatePassed ? 'badge-emerald' : (criticalCves > 0 ? 'badge-rose' : 'badge-amber')">
          {{ gatePassed ? 'ACTIVE & ENFORCED' : (criticalCves > 0 ? 'GATE BLOCKED' : 'GATE RESTRICTED') }}
        </span>
      </div>
      <div class="metric-val gate-metric-val">
        <div
          class="gate-status-badge"
          :class="gatePassed ? 'gate-badge-passed' : (criticalCves > 0 ? 'gate-badge-blocked' : 'gate-badge-restricted')"
        >
          <BaseIcon :name="gatePassed ? 'check-circle' : (criticalCves > 0 ? 'alert-triangle' : 'shield')" size="xs" />
          <span>{{ gatePassed ? 'GATE PASSED' : (criticalCves > 0 ? 'GATE BLOCKED' : 'GATE RESTRICTED') }}</span>
        </div>
      </div>
      <div class="metric-footer gate-footer">
        <div class="gate-reasons">
          <span :class="criticalCves === 0 ? 'text-emerald' : 'text-rose'">
            {{ criticalCves }} Critical CVEs
          </span>
          <span class="text-muted">·</span>
          <span :class="exposedSecrets === 0 ? 'text-emerald' : 'text-amber'">
            {{ exposedSecrets }} Exposed Secrets
          </span>
        </div>
        <span class="gate-status-text" :class="gatePassed ? 'text-muted' : (criticalCves > 0 ? 'text-rose' : 'text-amber')">
          {{ criticalCves > 0 && exposedSecrets > 0 ? 'Blocked by CVEs & Secrets' : (criticalCves > 0 ? 'Blocked by Critical CVEs' : (exposedSecrets > 0 ? 'Blocked by Exposed Secrets' : 'Deployments Unblocked')) }}
        </span>
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
        <span class="text-muted">{{ totalRules > 0 ? `of ${totalRules} Rules` : 'No Active Rules' }}</span>
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
