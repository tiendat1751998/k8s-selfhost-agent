<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { HostFormData, HostTypeDefinition, ModalTestResult } from '../../types/hosts'

defineProps<{
  show: boolean
  isEditing: boolean
  submitting: boolean
  modalTesting: boolean
  modalTestResult: ModalTestResult | null
  hostForm: HostFormData
  hostTypeDefinitions: HostTypeDefinition[]
  getHostTypeMeta: (type?: string) => HostTypeDefinition
  formatUptime: (seconds?: number) => string
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'submit'): void
  (e: 'test-connection'): void
  (e: 'add-label'): void
  (e: 'remove-label', index: number): void
}>()
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    :title="isEditing ? `Edit Host: ${hostForm.name}` : 'Register Infrastructure Host'"
    :subtitle="isEditing ? 'Modify endpoint configuration, labels, or credentials' : 'Register a compute node, database, Git repository, or monitoring target'"
    max-width="680px"
    @update:show="emit('update:show', $event)"
  >
    <form class="host-form" @submit.prevent="emit('submit')">
      <!-- Host Name -->
      <div class="form-group">
        <label class="form-label">Host Name <span class="text-rose">*</span></label>
        <input
          v-model="hostForm.name"
          type="text"
          required
          placeholder="e.g. edge-worker-tokyo-01"
          class="input-glass"
        />
      </div>

      <!-- Host Type Selection -->
      <div class="form-group">
        <label class="form-label">Host Type <span class="text-rose">*</span></label>
        <select v-model="hostForm.host_type" class="input-glass font-mono">
          <option v-for="def in hostTypeDefinitions" :key="def.type" :value="def.type">
            {{ def.icon }} {{ def.label }} — {{ def.desc }}
          </option>
        </select>
        <p class="form-hint text-cyan" style="margin-top: 6px; font-size: 12px;">
          {{ getHostTypeMeta(hostForm.host_type).hint }}
        </p>
      </div>

      <!-- Endpoint URL -->
      <div class="form-group">
        <label class="form-label">Endpoint URL / Connection String <span class="text-rose">*</span></label>
        <input
          v-model="hostForm.endpoint"
          type="text"
          required
          :placeholder="getHostTypeMeta(hostForm.host_type).placeholder"
          class="input-glass font-mono"
        />
      </div>

      <!-- Optional Description -->
      <div class="form-group">
        <label class="form-label">Description / Notes</label>
        <input
          v-model="hostForm.description"
          type="text"
          placeholder="Optional human-readable notes or placement description"
          class="input-glass"
        />
      </div>

      <!-- SECURITY & CREDENTIALS SECTION -->
      <div v-if="hostForm.host_type === 'docker' || hostForm.host_type === 'k8s'" class="security-box glass-panel">
        <div class="form-group" style="margin-bottom: 8px;">
          <div class="tls-toggle-row">
            <label class="toggle-switch">
              <input v-model="hostForm.tls_enabled" type="checkbox" />
              <span class="toggle-slider"></span>
            </label>
            <div class="toggle-label-wrap">
              <span class="toggle-title">Enable TLS / mTLS Authentication</span>
              <span class="toggle-sub text-muted">Use client certificates for secured API/daemon socket communication</span>
            </div>
          </div>
        </div>

        <div v-if="hostForm.tls_enabled" class="tls-certs-group animate-fade-in">
          <div class="form-row-2">
            <div class="form-group">
              <label class="form-label">API Version</label>
              <input
                v-model="hostForm.api_version"
                type="text"
                placeholder="e.g. 1.45 or v1.28 (auto)"
                class="input-glass font-mono"
              />
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">CA Certificate (ca.pem)</label>
            <textarea
              v-model="hostForm.tls_ca"
              rows="2"
              placeholder="-----BEGIN CERTIFICATE----- ... -----END CERTIFICATE-----"
              class="input-glass font-mono cert-textarea"
            ></textarea>
          </div>

          <div class="form-group">
            <label class="form-label">Client Certificate (cert.pem)</label>
            <textarea
              v-model="hostForm.tls_cert"
              rows="2"
              placeholder="-----BEGIN CERTIFICATE----- ... -----END CERTIFICATE-----"
              class="input-glass font-mono cert-textarea"
            ></textarea>
          </div>

          <div class="form-group">
            <label class="form-label">Client Private Key (key.pem)</label>
            <textarea
              v-model="hostForm.tls_key"
              rows="2"
              :placeholder="isEditing ? 'Leave blank to preserve existing encrypted key' : '-----BEGIN RSA PRIVATE KEY----- ... -----END RSA PRIVATE KEY-----'"
              class="input-glass font-mono cert-textarea"
            ></textarea>
          </div>
        </div>
      </div>

      <!-- For Agent / Prometheus / Custom: Auth Token -->
      <div v-else-if="hostForm.host_type === 'agent' || hostForm.host_type === 'prometheus' || hostForm.host_type === 'custom'" class="security-box glass-panel">
        <div class="form-group">
          <label class="form-label">Authentication Token (Optional)</label>
          <input
            v-model="hostForm.auth_token"
            type="password"
            placeholder="Bearer token or API Secret for authenticated requests"
            class="input-glass font-mono"
          />
        </div>
      </div>

      <!-- LABELS EDITOR -->
      <div class="form-group">
        <div class="labels-heading">
          <label class="form-label">Host Placement Labels</label>
          <button type="button" class="btn btn-secondary btn-xs" @click="emit('add-label')">
            <span>+ Add Label</span>
          </button>
        </div>

        <div v-if="hostForm.labels.length === 0" class="text-muted font-mono" style="font-size: 11px;">
          No labels defined. Labels assist with workload scheduling, alerting, and filtering.
        </div>

        <div v-else class="labels-list">
          <div v-for="(lbl, idx) in hostForm.labels" :key="idx" class="label-row">
            <input v-model="lbl.key" type="text" placeholder="key (e.g. region)" class="input-glass font-mono" />
            <span class="label-eq">=</span>
            <input v-model="lbl.value" type="text" placeholder="value (e.g. ap-southeast)" class="input-glass font-mono" />
            <button type="button" class="btn-remove-lbl" title="Remove label" @click="emit('remove-label', idx)">✕</button>
          </div>
        </div>
      </div>

      <!-- In-Modal Test Connection Results -->
      <div v-if="modalTestResult" class="modal-test-banner font-mono animate-fade-in" :class="modalTestResult.success ? 'test-pass' : 'test-fail'">
        <div class="modal-test-header">
          <span>{{ modalTestResult.success ? '✅ CONNECTION SUCCESSFUL' : '❌ CONNECTION FAILED' }}</span>
          <span v-if="modalTestResult.latency_ms > 0">⚡ {{ modalTestResult.latency_ms }}ms</span>
        </div>
        <div class="modal-test-msg">{{ modalTestResult.message }}</div>
        <div v-if="modalTestResult.agent_info" class="agent-telemetry-mini" style="margin-top: 6px;">
          <span v-if="modalTestResult.agent_info.hostname">🖥️ {{ modalTestResult.agent_info.hostname }}</span>
          <span v-if="modalTestResult.agent_info.os_distro || modalTestResult.agent_info.os">
            🐧 {{ modalTestResult.agent_info.os_distro || modalTestResult.agent_info.os }} ({{ modalTestResult.agent_info.arch }})
          </span>
          <span v-if="modalTestResult.agent_info.uptime || modalTestResult.agent_info.uptime_seconds">⏱️ {{ formatUptime(modalTestResult.agent_info.uptime || modalTestResult.agent_info.uptime_seconds) }}</span>
        </div>
      </div>

      <!-- Modal Actions -->
      <div class="modal-actions">
        <button type="button" class="btn btn-secondary" :disabled="modalTesting" @click="emit('test-connection')">
          <span>{{ modalTesting ? '⏳ Testing...' : '⚡ Test Connection' }}</span>
        </button>

        <div class="modal-action-right">
          <button type="button" class="btn btn-secondary" @click="emit('update:show', false)">Cancel</button>
          <button type="submit" class="btn btn-primary" :disabled="submitting">
            <span>{{ submitting ? '⏳ Saving...' : (isEditing ? '💾 Update Host' : '💾 Register Host') }}</span>
          </button>
        </div>
      </div>
    </form>
  </ModalDrawer>
</template>
