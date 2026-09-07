<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import type { Incident, RCAReport, PullRequest } from '../../api/compute'

const props = defineProps<{
  selectedIncident: Incident | null
  selectedReport: RCAReport | null
  activePR: PullRequest | null
  loadingReport: boolean
  reportError: string | null
  actionLoading: string | null
}>()

const emit = defineEmits<{
  (e: 'analyze', incident: Incident): void
  (e: 'open-pr-modal'): void
  (e: 'merge-pr'): void
}>()

function isNotFoundError(err: string | null): boolean {
  if (!err) return true
  const lower = err.toLowerCase()
  return lower.includes('not found') || lower.includes('404') || lower.includes('no rca report')
}
</script>

<template>
  <div class="right-pane glass-panel">
    <!-- Unselected Placeholder -->
    <div v-if="!selectedIncident" class="no-selection">
      <span class="no-sel-icon">🔍</span>
      <h3>Select an Incident to Inspect</h3>
      <p>Choose an anomaly from the queue on the left to review telemetry evidence, trigger autonomous AI Root Cause Analysis (RCA), and inspect GitOps remediation diffs.</p>
    </div>

    <!-- Active Incident Inspector -->
    <div v-else class="inspector-content">
      <!-- Top Inspector Bar -->
      <div class="inspector-header">
        <div class="inspector-title-wrap">
          <div class="target-headline">
            <span class="target-pod-name">{{ selectedIncident.pod_name }}</span>
            <span class="target-scope font-mono">{{ selectedIncident.cluster_name }} · ns/{{ selectedIncident.namespace }}</span>
          </div>
          <div class="badges-row">
            <StatusBadge :status="selectedIncident.severity" size="sm" />
            <StatusBadge :status="selectedIncident.status" size="sm" />
            <span class="type-pill font-mono">{{ selectedIncident.type }}</span>
          </div>
        </div>

        <div class="inspector-actions">
          <button 
            v-if="selectedReport"
            class="btn-slate"
            :disabled="actionLoading === 'analyze' || selectedIncident.status === 'analyzing'"
            @click="emit('analyze', selectedIncident)"
          >
            <span>{{ actionLoading === 'analyze' ? '⏳ Reasoning...' : '🤖 Re-run AI RCA' }}</span>
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
              <span class="ai-model-tag font-mono">{{ selectedReport?.llm_model || 'Claude 3.5 Sonnet / Multi-Agent' }}</span>
            </div>
          </div>

          <!-- Confidence Gauge -->
          <div v-if="selectedReport" class="confidence-gauge">
            <div class="gauge-dial font-mono">
              <span class="gauge-pct">{{ Math.round((selectedReport?.confidence || 0.94) * 100) }}%</span>
              <span class="gauge-label">Confidence</span>
            </div>
          </div>
        </div>

        <!-- Loading State -->
        <div v-if="loadingReport" class="rca-loading-state font-mono">
          <span>⏳ Retrieving AI Root Cause Analysis...</span>
        </div>

        <!-- Analyzed Report Content -->
        <div v-else-if="selectedReport" class="rca-body">
          <div class="rca-explanation font-mono">
            {{ selectedReport.root_cause || selectedIncident.message }}
          </div>

          <!-- Evidence List -->
          <div v-if="selectedReport.evidence && selectedReport.evidence.length > 0" class="evidence-box">
            <h4 class="evidence-title">Telemetry Evidence & Alert Correlation</h4>
            <div class="evidence-grid">
              <div v-for="(ev, idx) in selectedReport.evidence" :key="idx" class="evidence-tag font-mono">
                <span class="ev-bullet">▸</span>
                <span>{{ ev }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Diagnostics Error Fallback (non-404 unexpected failure) -->
        <div v-else-if="reportError && !isNotFoundError(reportError)" class="rca-empty-state">
          <span class="rca-empty-icon">⚠️</span>
          <h4 class="rca-empty-title">Diagnostics Query Failed</h4>
          <p class="rca-empty-desc">{{ reportError }}</p>
          <button 
            class="btn-slate-primary"
            :disabled="actionLoading === 'analyze' || selectedIncident.status === 'analyzing'"
            @click="emit('analyze', selectedIncident)"
          >
            <span>{{ actionLoading === 'analyze' ? '⏳ Reasoning...' : '🔄 Retry Autonomous AI RCA' }}</span>
          </button>
        </div>

        <!-- Enterprise AI RCA Empty State: Anomaly not analyzed yet -->
        <div v-else class="rca-empty-state">
          <span class="rca-empty-icon">✨</span>
          <h4 class="rca-empty-title">AI Root Cause Analysis Ready</h4>
          <span class="rca-empty-subtitle font-mono">Claude 3.5 Sonnet / Multi-Agent correlation ready for this incident.</span>
          <p class="rca-empty-desc">
            This anomaly has not been analyzed yet. Run automated diagnostics to inspect container logs, k8s events, and metric anomalies.
          </p>
          <button 
            class="btn-slate-primary"
            :disabled="actionLoading === 'analyze' || selectedIncident.status === 'analyzing'"
            @click="emit('analyze', selectedIncident)"
          >
            <span>{{ actionLoading === 'analyze' ? '⏳ Reasoning...' : '⚡ Run Autonomous AI RCA' }}</span>
          </button>
        </div>
      </div>

      <!-- GitOps Remediation Diff Panel -->
      <div class="diff-panel glass-panel">
        <div class="diff-header">
          <div class="diff-title-wrap">
            <span class="diff-icon">📝</span>
            <div>
              <h3 class="diff-title">GitOps Remediation Manifest Diff</h3>
              <span class="diff-subtitle font-mono">deployments/{{ selectedIncident.namespace }}/{{ (selectedIncident.pod_name || 'workload').split('-')[0] }}.yaml</span>
            </div>
          </div>

          <div class="diff-actions">
            <div v-if="activePR" class="pr-status-pill">
              <span class="font-mono text-cyan">PR #{{ activePR.pr_number || 104 }} ({{ activePR.status }})</span>
            </div>
          </div>
        </div>

        <!-- Unified Diff Code Viewer -->
        <div class="diff-code-box font-mono">
          <template v-if="activePR?.files_changed && activePR.files_changed.length > 0">
            <div v-for="file in activePR.files_changed" :key="file.path" class="file-diff-block">
              <div class="diff-line diff-meta">--- {{ file.path }} ({{ file.action }})</div>
              <pre class="diff-file-content">{{ file.content }}</pre>
            </div>
          </template>
          <div v-else class="diff-placeholder">
            <span class="diff-placeholder-icon">📄</span>
            <p class="diff-placeholder-text">
              Remediation manifest diff will be synthesized once AI RCA identifies the corrective action.
            </p>
          </div>
        </div>

        <!-- Bottom Action Controls: Consolidated single PR action button -->
        <div class="diff-footer">
          <div class="remediation-note font-mono">
            Remediation: {{ selectedReport?.remediation || 'Remediation plan will be generated during AI root cause analysis.' }}
          </div>

          <div class="remediation-btn-group">
            <button 
              v-if="!activePR" 
              class="btn-slate-primary"
              :disabled="actionLoading === 'create-pr'"
              @click="emit('open-pr-modal')"
            >
              <span>{{ actionLoading === 'create-pr' ? '⏳ Synthesizing...' : '🚀 Generate Fix PR' }}</span>
            </button>

            <button 
              v-else-if="activePR.status === 'open' || activePR.status === 'pending'" 
              class="btn-slate-primary"
              :disabled="actionLoading === 'merge-pr'"
              @click="emit('merge-pr')"
            >
              <span>{{ actionLoading === 'merge-pr' ? '⏳ Merging PR...' : '⚡ Merge PR & Apply Fix' }}</span>
            </button>

            <div v-else-if="activePR.status === 'merged'" class="merged-badge font-mono">
              <span>✅ Remediation Deployed to Production</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/incidents.css';
</style>
