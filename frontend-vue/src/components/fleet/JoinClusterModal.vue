<script setup lang="ts">
import { computed } from 'vue'
import type { ImportClusterForm } from '../../composables/useFleetManagement'
import ModalDrawer from '../ui/ModalDrawer.vue'

const props = defineProps<{
  show: boolean
  importMode: 'file' | 'text' | 'token'
  importForm: ImportClusterForm
  joinToken: string
  actionLoading?: string | null
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'update:importMode', mode: 'file' | 'text' | 'token'): void
  (e: 'fileChange', event: Event): void
  (e: 'submit'): void
  (e: 'generateToken'): void
}>()

const joinCommand = computed(() => {
  const tk = props.joinToken || '<TOKEN>'
  const name = props.importForm.name || 'my-cluster'
  const group = props.importForm.group || 'production'
  const region = props.importForm.region || 'us-east-1'
  return `curl -sSL https://agent.fleet.k8s-aura.io/join.sh | sudo bash -s -- --token ${tk} --name "${name}" --group "${group}" --region "${region}"`
})
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="Register & Join Kubernetes Cluster"
    subtitle="Import kubeconfig or generate agent join token for multi-cluster federation"
    max-width="620px"
    @update:show="emit('update:show', $event)"
  >
    <div class="modal-form">
      <div class="form-group">
        <label class="form-label">Cluster Identifier / Name *</label>
        <input
          v-model="importForm.name"
          type="text"
          placeholder="e.g. prod-us-east-cluster"
          class="input-glass"
        />
      </div>

      <div class="form-row">
        <div class="form-group flex-1">
          <label class="form-label">Fleet Tier</label>
          <select v-model="importForm.group" class="input-glass">
            <option value="production">Production</option>
            <option value="staging">Staging</option>
            <option value="development">Development</option>
            <option value="edge">Edge Computing</option>
          </select>
        </div>

        <div class="form-group flex-1">
          <label class="form-label">Cloud Provider</label>
          <select v-model="importForm.provider" class="input-glass">
            <option value="aws">AWS (EKS)</option>
            <option value="gcp">GCP (GKE)</option>
            <option value="azure">Azure (AKS)</option>
            <option value="onprem">Bare-Metal / On-Premise</option>
            <option value="edge">Edge Device / K3s</option>
            <option value="generic">Generic Kubernetes</option>
          </select>
        </div>
      </div>

      <div class="form-group">
        <label class="form-label">Region / Datacenter</label>
        <input
          v-model="importForm.region"
          type="text"
          placeholder="e.g. us-east-1, eu-west-1, dc-onprem-1"
          class="input-glass font-mono"
        />
      </div>

      <div class="form-group">
        <div class="mode-tabs">
          <button
            type="button"
            class="mode-tab"
            :class="{ active: importMode === 'file' }"
            @click="emit('update:importMode', 'file')"
          >
            📁 File Upload
          </button>
          <button
            type="button"
            class="mode-tab"
            :class="{ active: importMode === 'text' }"
            @click="emit('update:importMode', 'text')"
          >
            📝 Paste YAML
          </button>
          <button
            type="button"
            class="mode-tab"
            :class="{ active: importMode === 'token' }"
            @click="emit('update:importMode', 'token')"
          >
            ⚡ Join Token
          </button>
        </div>

        <!-- Mode 1: File Upload -->
        <div v-if="importMode === 'file'" class="file-input-wrap">
          <label class="form-label">Kubeconfig File (.yaml / .config / .json) *</label>
          <input
            type="file"
            accept=".yaml,.yml,.config,.json,text/*"
            class="input-glass"
            @change="emit('fileChange', $event)"
          />
        </div>

        <!-- Mode 2: Paste YAML -->
        <div v-else-if="importMode === 'text'" class="text-input-wrap">
          <label class="form-label">Paste Kubeconfig YAML Content *</label>
          <textarea
            v-model="importForm.kubeconfigRaw"
            rows="6"
            placeholder="apiVersion: v1&#10;clusters:&#10;  - cluster:&#10;      server: https://..."
            class="input-glass font-mono text-area-input"
          ></textarea>
        </div>

        <!-- Mode 3: Join Token Generator -->
        <div v-else class="token-box">
          <div style="display: flex; justify-content: space-between; align-items: center;">
            <label class="form-label" style="margin: 0;">Self-Hosted Agent Join Command</label>
            <button class="btn btn-secondary btn-xs" type="button" @click="emit('generateToken')">
              <span>⚡ Generate Token</span>
            </button>
          </div>
          <p class="form-hint" style="margin: 0;">
            Run this one-line command on your remote cluster master node to register and link telemetry:
          </p>
          <code class="token-code font-mono">{{ joinCommand }}</code>
        </div>

        <span class="form-hint">
          🔒 Manifests are encrypted with AES-256 GCM in local vault before persistence. Sensitive tokens are never returned in cleartext.
        </span>
      </div>
    </div>

    <template #footer="{ close }">
      <button class="btn btn-secondary" @click="close">Cancel</button>
      <button
        class="btn btn-primary"
        :disabled="actionLoading === 'import'"
        @click="emit('submit')"
      >
        <span>{{ actionLoading === 'import' ? '⏳ Registering Cluster...' : (importMode === 'token' ? 'Confirm Join ➔' : 'Import Cluster ➔') }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>
