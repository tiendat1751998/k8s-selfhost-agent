<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import type { Incident, RCAReport, PullRequest } from '../../api/compute'
import type { BlastRadiusInfo } from '../../composables/useIncidents'

defineProps<{
  incident: Incident | null
  report: RCAReport | null
  activePr: PullRequest | null
  loadingReport?: boolean
  reportError?: string | null
  actionLoading?: string | null
  blastRadius?: BlastRadiusInfo | null
}>()

const emit = defineEmits<{
  (e: 'analyze', incident: Incident): void
  (e: 'openPrModal'): void
  (e: 'mergePr'): void
  (e: 'openRcaModal', incident: Incident): void
  (e: 'mitigate', incident: Incident): void
  (e: 'resolve', incident: Incident): void
  (e: 'close'): void
}>()
</script>

<template>
  <div class="inspector-wrapper">
    <div v-if="!incident" class="no-selection">
      <span class="no-sel-icon">🔑</span>
      <h3>Select an Incident to Inspect</h3>
      <p>Choose an incident from the queue on the left to trigger AI Root Cause Analysis and review GitOps remediation pull requests.</p>
    </div>


    <div v-else class="inspector-content">
      <!-- Top Inspector Bar -->
      <div class="inspector-header">
        <div class="inspector-title-wrap">
          <div class="target-headline">
            <span class="target-pod-name">{{ incident.pod_name }}</span>
            <span class="target-scope font-mono">{{ incident.cluster_name }} · ns/{{ incident.namespace }}</span>
          </div>
          <div class="badges-row">
            <StatusBadge :status="incident.severity" size="sm" />
            <StatusBadge :status="incident.status" size="sm" />
            <span class="type-pill font-mono">{{ incident.type }}</span>
          </div>
        </div>


        <div class="inspector-actions">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="actionLoading === 'analyze' || incident.status === 'analyzing'"
            @click="emit('analyze', incident)"
          >
            <span>{{ actionLoading === 'analyze' ? '⏳ Reasoning...' : '🤖 AI Root Cause' }}</span>
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            @click="emit('openRcaModal', incident)"
          >
            <span>⬰ￏ Chronology</span>
          </button>
          <button
            v-if="!activePr"
            type="button"
            class="btn btn-primary btn-sm"
            @click="emit('openPrModal')"
          >
            <span>🚁 Generate Fix PR</span>
          </button>
        </div>
      </div>


      <!-- AI RCA Reasoning Card -->
      <div class="rca-inspector-card glass-panel">
        <div class="rca-card-header">
          <div class="rca-title-wrap">
            <span class="ai-sparkle">✨</span>
            <div>
              <h3 class="rca-title">AI Root Cause Analysis (RCA)</h3>
              <span class="ai-model-tag font-mono">{{ report?.llm_model || 'Claude 3.5 Sonnet / Multi-Agent' }}</span>
            </div>
          </div>


          <!-- Confidence Gauge -->
          <div v-if="report" class="confidence-gauge">
            <div class="gauge-dial font-mono">
              <span class="gauge-pct">{{ Math.round((report?.confidence || 0.94) * 100) }}%</span>
              <span class="gauge-label">Confidence</span>
            </div>
          </div>
        </div>


        <div v-if="loadingReport" class="rca-loading text-muted font-mono" style="padding: 16px;">
          ⏳ Loading Root Cause Analysis...
        </div>
        <div v-else-if="report" class="rca-body">
          <div class="rca-explanation font-mono">
            {{ report.root_cause || incident.message }}
          </div>


          <!-- Evidence List -->
          <div v-if="report.evidence && report.evidence.length > 0" class="evidence-box">
            <h4 class="evidence-title">Telemetry Evidence & Alert Correlation</h4>
            <div class="evidence-grid">
              <div v-for="(ev, idx) in report.evidence" :key="idx" class="evidence-tag font-mono">
                <span class="ev-bullet">▸</span>
                <span>{{ ev }}</span>
              </div>
            </div>
          </div>
        </div>
        <div v-else class="empty-rca-box text-muted font-mono" style="padding: 16px; font-size: 13px;">
          <span>{{ reportError || 'No RCA report generated yet. Click "🤖 AI Root Cause" above to run diagnostics.' }}</span>
        </div>
      </div>


      <!-- Impact Blast Radius Panel -->
      <div v-if="blastRadius" class="blast-radius-box glass-panel">
        <div class="blast-header">
          <div class="blast-title-wrap">
            <span class="blast-icon">🌊</span>
            <div>
              <h4 class="blast-title">Impact Blast Radius Analysis</h4>
              <span class="blast-subtitle font-mono">{{ blastRadius.riskLevel }} · Impact Score {{ blastRadius.impactScore }}/100</span>
            </div>
          </div>
          <span class="blast-badge font-mono" :class="`blast-${blastRadius.riskLevel.toLowerCase().replace(/\s+/g, '-')}`">
            {{ blastRadius.nodeHealth }}
          </span>
        </div>


        <div class="blast-grid">
          <div class="blast-item">
            <span class="blast-item-label">Impacted Namespace</span>
            <span class="blast-item-val font-mono text-cyan">{{ blastRadius.affectedNamespace }}</span>
          </div>
          <div class="blast-item">
            <span class="blast-item-label">Dependent Services</span>
            <div class="blast-tags">
              <span v-for="svc in blastRadius.affectedServices" :key="svc" class="blast-tag font-mono">{{ svc }}</span>
            </div>
          </div>
          <div class="blast-item">
            <span class="blast-item-label">Correlated Workloads</span>
            <div class="blast-tags">
              <span v-for="w in blastRadius.dependentWorkloads" :key="w" class="blast-tag font-mono">{{ w }}</span>
            </div>
          </div>
        </div>
      </div>


      <!-- GitOps Remediation Diff Panel -->
      <div class="diff-panel glass-panel">
        <div class="diff-header">
          <div class="diff-title-wrap">
            <span class="diff-icon">📗</span>
            <div>
              <h3 class="diff-title">GitOps Remediation Manifest Diff</h3>
              <span class="diff-subtitle font-mono">deployments/{{ incident.namespace }}/{{ (incident.pod_name || 'workload').split('-')[0] }}.yaml</span>
            </div>
          </div>


          <div class="diff-actions">
            <div v-if="activePr" class="pr-status-pill">
              <span class="font-mono text-cyan">PR #{{ activePr.pr_number || 104 }} ({{ activePr.status }})</span>
            </div>
          </div>
        </div>


        <!-- Unified Diff Code Viewer -->
        <div class="diff-code-box font-mono">
          <template v-if="activePr?.files_changed && activePr.files_changed.length > 0">
            <div v-for="file in activePr.files_changed" :key="file.path" class="file-diff-block">
              <div class="diff-line diff-meta">--- {{ file.path }} ({{ file.action }})</div>
              <pre class="diff-file-content">{{ file.content }}</pre>
            </div>
          </template>
          <div v-else class="no-diff-box text-muted">
            No diff available
          </div>
        </div>


        <!-- Bottom Action Controls -->
        <div class="diff-footer">
          <div class="remediation-note font-mono text-muted">
            Remediation: {{ report?.remediation || 'Remediation plan will be generated during AI root cause analysis.' }}
          </div>


          <div class="remediation-btn-group">
            <button
              v-if="!activePr"
              type="button"
              class="btn btn-primary"
              :disabled="actionLoading === 'create-pr'"
              @click="emit('openPrModal')"
            >
              <span>🚁 Create Remediation PR ⚙</span>
            </button>


            <button
              v-else-if="activePr.status === 'open' || activePr.status === 'pending'"
              type="button"
              class="btn-title btn btn-primary"
              :disabled="actionLoading === 'merge-pr'"
              @click="emit('mergePr')"
            >
              <span>{{ actionLoading === 'merge-pr' ? '⏳ Merging PR...' : '⁡ Merge PR & Apply Fix' }}</span>
            </button>


            <div v-else-if="activePr.status === 'merged'" class="merged-badge font-mono">
              <span>✅ REMEDIATION DEPLOYED</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
