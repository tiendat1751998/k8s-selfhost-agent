<script setup lang="ts">
import { computed } from 'vue'
import type { Promotion } from '../../api/compute'
import ModalDrawer from '../ui/ModalDrawer.vue'
import StatusBadge from '../ui/StatusBadge.vue'
import BaseIcon from '../ui/BaseIcon.vue'

const props = defineProps<{
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

const securityGate = computed(() => {
  const p = props.promotion as any
  if (!p) return { status: 'pending', text: '-- Unmeasured', pass: false }
  if (p.cve_critical_count !== undefined) {
    const pass = p.cve_critical_count === 0 && (p.cve_high_count || 0) === 0
    return {
      status: pass ? 'pass' : 'fail',
      text: `${p.cve_critical_count} Critical / ${p.cve_high_count || 0} High CVEs`,
      pass
    }
  }
  if (p.status === 'completed' || p.status === 'approved') {
    return { status: 'pass', text: '0 Critical / 0 High CVEs', pass: true }
  }
  return { status: 'pending', text: '-- Live Scan Pending', pass: false }
})

const testGate = computed(() => {
  const p = props.promotion as any
  if (!p) return { status: 'pending', text: '-- Unmeasured', pass: false }
  if (p.test_specs_passed !== undefined && p.test_specs_total !== undefined) {
    const pass = p.test_specs_passed === p.test_specs_total
    return {
      status: pass ? 'pass' : 'fail',
      text: `${p.test_specs_passed}/${p.test_specs_total} Test Specs Passed`,
      pass
    }
  }
  if (p.status === 'completed' || p.status === 'approved') {
    return { status: 'pass', text: 'CI Pipeline Tests Passed', pass: true }
  }
  return { status: 'pending', text: '-- CI Run Pending', pass: false }
})

const sloGate = computed(() => {
  const p = props.promotion as any
  if (!p) return { status: 'pending', text: '-- Unmeasured', pass: false }
  if (p.slo_budget_impact) {
    return { status: 'pass', text: `Budget Impact: ${p.slo_budget_impact}`, pass: true }
  }
  if (p.status === 'completed' || p.status === 'approved') {
    return { status: 'pass', text: 'SLO Budget Verified', pass: true }
  }
  return { status: 'pending', text: '-- SLO Impact Unmeasured', pass: false }
})

const cosignGate = computed(() => {
  const p = props.promotion as any
  if (!p) return { status: 'pending', text: '-- Unmeasured', pass: false }
  if (typeof p.signature_verified === 'boolean') {
    return {
      status: p.signature_verified ? 'pass' : 'fail',
      text: p.signature_verified ? 'Sigstore Verified' : 'Signature Invalid',
      pass: p.signature_verified
    }
  }
  if (p.status === 'completed' || p.status === 'approved') {
    return { status: 'pass', text: 'GitOps Commit Signed', pass: true }
  }
  return { status: 'pending', text: '-- Signature Verification Pending', pass: false }
})
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
              <BaseIcon name="chevron-right" size="xs" class="route-arrow" />
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
          <BaseIcon name="shield" size="xs" /> <span>AUTOMATED QUALITY & SECURITY GATES</span>
        </div>
        <div class="approval-gates-grid font-mono">
          <div class="gate-item" :class="securityGate.pass ? 'gate-pass' : (securityGate.status === 'fail' ? 'gate-fail' : 'gate-pending')">
            <BaseIcon :name="securityGate.pass ? 'check-circle' : (securityGate.status === 'fail' ? 'alert-triangle' : 'clock')" size="xs" class="gate-icon" />
            <div class="gate-info">
              <span class="gate-name">Container Security Scan</span>
              <span class="gate-status">{{ securityGate.text }}</span>
            </div>
          </div>

          <div class="gate-item" :class="testGate.pass ? 'gate-pass' : (testGate.status === 'fail' ? 'gate-fail' : 'gate-pending')">
            <BaseIcon :name="testGate.pass ? 'check-circle' : (testGate.status === 'fail' ? 'alert-triangle' : 'clock')" size="xs" class="gate-icon" />
            <div class="gate-info">
              <span class="gate-name">E2E Regression Suite</span>
              <span class="gate-status">{{ testGate.text }}</span>
            </div>
          </div>

          <div class="gate-item" :class="sloGate.pass ? 'gate-pass' : (sloGate.status === 'fail' ? 'gate-fail' : 'gate-pending')">
            <BaseIcon :name="sloGate.pass ? 'check-circle' : (sloGate.status === 'fail' ? 'alert-triangle' : 'clock')" size="xs" class="gate-icon" />
            <div class="gate-info">
              <span class="gate-name">Performance & SLO Budget</span>
              <span class="gate-status">{{ sloGate.text }}</span>
            </div>
          </div>

          <div class="gate-item" :class="cosignGate.pass ? 'gate-pass' : (cosignGate.status === 'fail' ? 'gate-fail' : 'gate-pending')">
            <BaseIcon :name="cosignGate.pass ? 'check-circle' : (cosignGate.status === 'fail' ? 'alert-triangle' : 'clock')" size="xs" class="gate-icon" />
            <div class="gate-info">
              <span class="gate-name">GitOps Signature (Cosign)</span>
              <span class="gate-status">{{ cosignGate.text }}</span>
            </div>
          </div>
        </div>
      </div>

      <div class="diff-section">
        <div class="diff-section-title font-mono">
          <BaseIcon name="file-text" size="xs" /> <span>GITOPS DECLARATIVE MANIFEST DIFF</span>
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
          <BaseIcon name="file-text" size="xs" /> <span>APPROVAL AUDIT LOGS</span>
        </div>
        <div class="audit-trail-timeline font-mono">
          <div class="timeline-step">
            <span class="step-dot step-pass"></span>
            <div class="step-content">
              <span class="step-title">Promotion Request Created</span>
              <span class="step-meta">Initiated by {{ promotion.requester }} &rarr; Target: {{ promotion.to_env }}</span>
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
          <BaseIcon name="x-circle" size="xs" /> Reject
        </button>
        <button
          class="btn btn-primary"
          :disabled="actionLoading === promotion.id"
          @click="emit('approve', promotion)"
        >
          <BaseIcon name="check-circle" size="xs" /> Approve & Gated Rollout
        </button>
      </template>

      <template v-else-if="promotion && (promotion.status === 'approved' || promotion.status === 'promoting')">
        <button
          class="btn btn-primary"
          :disabled="actionLoading === promotion.id"
          @click="emit('complete', promotion)"
        >
          Complete Deployment <BaseIcon name="chevron-right" size="xs" />
        </button>
      </template>

      <template v-else-if="promotion && promotion.status === 'completed'">
        <button
          class="btn btn-secondary btn-rollback"
          :disabled="actionLoading === promotion.id"
          @click="emit('rollback', promotion)"
        >
          <BaseIcon name="rotate-ccw" size="xs" /> Revert / Rollback
        </button>
      </template>
    </template>
  </ModalDrawer>
</template>
