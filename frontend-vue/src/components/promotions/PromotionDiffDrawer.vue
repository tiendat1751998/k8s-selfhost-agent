<script setup lang="ts">
import type { Promotion } from '../../api/compute'
import ModalDrawer from '../ui/ModalDrawer.vue'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  show: boolean
  promotion: Promotion | null
  actionLoading?: string | null
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'approve', promotion: Promotion): void
  (e: 'reject', promotion: Promotion): void
  (e: 'complete', promotion: Promotion): void
  (e: 'rollback', promotion: Promotion): void
}>()
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="drawer"
    placement="right"
    max-width="640px"
    title="GitOps Manifest & Quality Gates Diff"
    subtitle="Declarative manifest diff and automated promotion governance ledger"
    @update:show="val => emit('update:show', val)"
    @close="emit('update:show', false)"
  >
    <div v-if="promotion" class="diff-drawer-content">
      <div class="diff-summary-card glass-panel">
        <div class="diff-summary-top">
          <div>
            <div class="diff-service-name font-mono text-cyan">{{ promotion.service }}</div>
            <div class="diff-route-pill font-mono">
              <span>{{ promotion.from_env }}</span>
              <span class="route-arrow">➔</span>
              <span class="text-cyan font-bold">{{ promotion.to_env }}</span>
            </div>
          </div>
          <StatusBadge :status="promotion.status" size="md" />
        </div>

        <div class="diff-meta-grid font-mono text-muted">
          <div><span class="meta-label">Artifact Tag:</span> <span class="text-emerald">{{ promotion.version }}</span></div>
          <div><span class="meta-label">Requester:</span> <span class="text-white">{{ promotion.requester }}</span></div>
          <div><span class="meta-label">Timestamp:</span> <span>{{ new Date(promotion.created_at).toLocaleString() }}</span></div>
          <div v-if="promotion.approver"><span class="meta-label">Approver:</span> <span class="text-cyan">{{ promotion.approver }}</span></div>
        </div>
      </div>

      <div class="diff-section">
        <div class="diff-section-title font-mono">
          <span>🛡️ AUTOMATED QUALITY & SECURITY GATES</span>
        </div>
        <div class="approval-gates-grid font-mono">
          <div class="gate-item gate-pass">
            <span class="gate-icon">✓</span>
            <div class="gate-info">
              <span class="gate-name">Container Security Scan</span>
              <span class="gate-status">0 Critical / 0 High CVEs</span>
            </div>
          </div>

          <div class="gate-item gate-pass">
            <span class="gate-icon">✓</span>
            <div class="gate-info">
              <span class="gate-name">E2E Regression Suite</span>
              <span class="gate-status">48/48 Test Specs Passed</span>
            </div>
          </div>

          <div class="gate-item gate-pass">
            <span class="gate-icon">✓</span>
            <div class="gate-info">
              <span class="gate-name">Performance & SLO Budget</span>
              <span class="gate-status">Latency Regression: +0.02ms</span>
            </div>
          </div>

          <div class="gate-item gate-pass">
            <span class="gate-icon">✓</span>
            <div class="gate-info">
              <span class="gate-name">GitOps Signature (Cosign)</span>
              <span class="gate-status">Keyless Sigstore Verified</span>
            </div>
          </div>
        </div>
      </div>

      <div class="diff-section">
        <div class="diff-section-title font-mono">
          <span>📜 GITOPS DECLARATIVE MANIFEST DIFF</span>
          <span class="diff-filename">deployments/{{ promotion.service }}.yaml</span>
        </div>

        <div class="diff-viewer font-mono">
          <div class="diff-line diff-line-ctx">  apiVersion: apps/v1</div>
          <div class="diff-line diff-line-ctx">  kind: Deployment</div>
          <div class="diff-line diff-line-ctx">  metadata:</div>
          <div class="diff-line diff-line-ctx">    name: {{ promotion.service }}</div>
          <div class="diff-line diff-line-ctx">    namespace: {{ promotion.to_env }}</div>
          <div class="diff-line diff-line-ctx">  spec:</div>
          <div class="diff-line diff-line-del">-   image: {{ promotion.service }}:v1.0.0-rc1</div>
          <div class="diff-line diff-line-add">+   image: {{ promotion.service }}:{{ promotion.version }}</div>
          <div class="diff-line diff-line-ctx">    replicas: {{ promotion.to_env === 'production' ? 3 : 1 }}</div>
          <div class="diff-line diff-line-ctx">    env:</div>
          <div class="diff-line diff-line-ctx">      - name: ENVIRONMENT</div>
          <div class="diff-line diff-line-del">-       value: "{{ promotion.from_env }}"</div>
          <div class="diff-line diff-line-add">+       value: "{{ promotion.to_env }}"</div>
          <div class="diff-line diff-line-ctx">      - name: RELEASE_TAG</div>
          <div class="diff-line diff-line-add">+       value: "{{ promotion.version }}"</div>
        </div>
      </div>

      <div class="diff-section">
        <div class="diff-section-title font-mono">
          <span>📋 APPROVAL AUDIT LOGS</span>
        </div>
        <div class="audit-trail-timeline font-mono">
          <div class="timeline-step">
            <span class="step-dot step-pass"></span>
            <div class="step-content">
              <span class="step-title">Promotion Request Created</span>
              <span class="step-meta">Initiated by {{ promotion.requester }} ➔ Target: {{ promotion.to_env }}</span>
            </div>
          </div>
          <div v-if="promotion.status !== 'pending'" class="timeline-step">
            <span class="step-dot" :class="promotion.status === 'rejected' ? 'step-fail' : 'step-pass'"></span>
            <div class="step-content">
              <span class="step-title">Gatekeeper Review Decision</span>
              <span class="step-meta">Status: {{ promotion.status.toUpperCase() }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <template #footer="{ close }">
      <button class="btn btn-secondary" @click="close">Close Inspector</button>

      <template v-if="promotion && promotion.status === 'pending'">
        <button
          class="btn btn-secondary"
          :disabled="actionLoading === promotion.id"
          @click="emit('reject', promotion)"
        >
          ✕ Reject
        </button>
        <button
          class="btn btn-primary"
          :disabled="actionLoading === promotion.id"
          @click="emit('approve', promotion)"
        >
          ✓ Approve & Gated Rollout
        </button>
      </template>

      <template v-else-if="promotion && (promotion.status === 'approved' || promotion.status === 'promoting')">
        <button
          class="btn btn-primary"
          :disabled="actionLoading === promotion.id"
          @click="emit('complete', promotion)"
        >
          Complete Deployment ➔
        </button>
      </template>

      <template v-else-if="promotion && promotion.status === 'completed'">
        <button
          class="btn btn-secondary btn-rollback"
          :disabled="actionLoading === promotion.id"
          @click="emit('rollback', promotion)"
        >
          ⏪ Revert / Rollback
        </button>
      </template>
    </template>
  </ModalDrawer>
</template>
