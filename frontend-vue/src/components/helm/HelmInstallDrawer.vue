<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { HelmChart } from '../../api/helm'
import type { K8sNamespace } from '../../api/k8s'

defineProps<{
  show: boolean
  chart: HelmChart | null
  namespaces: K8sNamespace[]
  selectedCluster: string
  installStep: 1 | 2 | 3
  installForm: {
    releaseName: string
    namespace: string
    createNamespace: boolean
    version: string
    values: string
    loadingDefaultValues: boolean
  }
  installing: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'close'): void
  (e: 'install'): void
  (e: 'stepChange', step: 1 | 2 | 3): void
  (e: 'resetValues'): void
}>()
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    max-width="740px"
    :title="chart ? `Install Chart: ${chart.name}` : 'Install Chart'"
    :subtitle="chart ? `Repository: ${chart.repo} • Version: ${chart.version}` : ''"
    @close="emit('close'); emit('update:show', false)"
  >
    <div v-if="chart" class="install-wizard-container">
      <!-- Wizard Step Indicators -->
      <div class="wizard-steps-header">
        <div class="step-badge" :class="{ active: installStep >= 1 }">
          <span class="step-num">1</span>
          <span class="step-name">Release & Scope</span>
        </div>
        <div class="step-connector" :class="{ active: installStep >= 2 }"></div>
        <div class="step-badge" :class="{ active: installStep >= 2 }">
          <span class="step-num">2</span>
          <span class="step-name">Custom Values</span>
        </div>
        <div class="step-connector" :class="{ active: installStep === 3 }"></div>
        <div class="step-badge" :class="{ active: installStep === 3 }">
          <span class="step-num">3</span>
          <span class="step-name">Review & Deploy</span>
        </div>
      </div>

      <!-- Step 1: Release Configuration -->
      <div v-if="installStep === 1" class="wizard-step-pane">
        <div class="form-grid">
          <div class="form-group">
            <label class="form-label">Release Name <span class="text-rose">*</span></label>
            <input
              v-model="installForm.releaseName"
              type="text"
              class="input-glass"
              placeholder="e.g. my-production-nginx"
            />
            <span class="form-hint">Lowercase alphanumeric characters and dashes only</span>
          </div>

          <div class="form-group">
            <label class="form-label">Chart Version</label>
            <input
              v-model="installForm.version"
              type="text"
              class="input-glass"
              placeholder="e.g. 1.2.0"
            />
          </div>

          <div class="form-group">
            <label class="form-label">Target Namespace <span class="text-rose">*</span></label>
            <select v-model="installForm.namespace" class="input-glass">
              <option v-for="ns in namespaces" :key="ns.name" :value="ns.name">
                🏷️ {{ ns.name }}
              </option>
              <option v-if="!namespaces.some(n => n.name === 'default')" value="default">
                🏷️ default
              </option>
            </select>
          </div>

          <div class="form-group checkbox-group">
            <label class="cyber-checkbox-label">
              <input
                v-model="installForm.createNamespace"
                type="checkbox"
                class="cyber-checkbox"
              />
              <span>Create namespace if it does not exist</span>
            </label>
          </div>
        </div>

        <div class="modal-footer-actions">
          <button type="button" class="btn-cyber btn-secondary" @click="emit('close'); emit('update:show', false)">
            Cancel
          </button>
          <button
            type="button"
            class="btn-cyber btn-primary"
            :disabled="!installForm.releaseName.trim() || !installForm.namespace.trim()"
            @click="emit('stepChange', 2)"
          >
            Next: Configure Values →
          </button>
        </div>
      </div>

      <!-- Step 2: Values Editor -->
      <div v-if="installStep === 2" class="wizard-step-pane">
        <div class="values-editor-container">
          <div class="editor-header-bar">
            <span class="font-mono font-xs text-muted">values.yaml Configuration</span>
            <button
              type="button"
              class="btn-cyber btn-secondary btn-xs"
              :disabled="installForm.loadingDefaultValues"
              @click="emit('resetValues')"
            >
              <span>Reset to Default Values</span>
            </button>
          </div>

          <textarea
            v-model="installForm.values"
            class="cyber-textarea yaml-editor"
            rows="12"
            placeholder="# Enter custom Helm values in YAML syntax..."
          ></textarea>
        </div>

        <div class="modal-footer-actions">
          <button type="button" class="btn-cyber btn-secondary" @click="emit('stepChange', 1)">
            ← Back
          </button>
          <button type="button" class="btn-cyber btn-primary" @click="emit('stepChange', 3)">
            Next: Review & Install →
          </button>
        </div>
      </div>

      <!-- Step 3: Review & Install -->
      <div v-if="installStep === 3" class="wizard-step-pane">
        <div class="summary-card glass-panel">
          <h4 class="summary-title font-mono text-cyan">INSTALLATION SUMMARY</h4>
          <div class="summary-grid">
            <div class="summary-item">
              <span class="summary-label">Target Cluster:</span>
              <span class="summary-val text-gold font-mono">{{ selectedCluster }}</span>
            </div>
            <div class="summary-item">
              <span class="summary-label">Release Name:</span>
              <span class="summary-val text-primary font-mono">{{ installForm.releaseName }}</span>
            </div>
            <div class="summary-item">
              <span class="summary-label">Namespace:</span>
              <span class="summary-val text-cyan font-mono">{{ installForm.namespace }}</span>
            </div>
            <div class="summary-item">
              <span class="summary-label">Chart:</span>
              <span class="summary-val font-mono">{{ chart.repo }}/{{ chart.name }}</span>
            </div>
            <div class="summary-item">
              <span class="summary-label">Version:</span>
              <span class="summary-val font-mono text-gold">v{{ installForm.version }}</span>
            </div>
          </div>
        </div>

        <div class="modal-footer-actions">
          <button type="button" class="btn-cyber btn-secondary" @click="emit('stepChange', 2)">
            ← Back
          </button>
          <button
            type="button"
            class="btn-cyber btn-primary"
            :disabled="installing"
            @click="emit('install')"
          >
            <span :class="{ 'spin-anim': installing }">🚀</span>
            <span>{{ installing ? 'Deploying Chart...' : 'Confirm & Deploy Release' }}</span>
          </button>
        </div>
      </div>
    </div>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/views/helm.css';
</style>
