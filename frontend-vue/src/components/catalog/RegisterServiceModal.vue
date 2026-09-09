<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { ServiceType, ServiceLifecycle } from '../../api/catalog'
import type { ServiceFormState } from '../../composables/useServiceCatalog'

defineProps<{
  show: boolean
  mode: 'create' | 'edit'
  form: ServiceFormState
  formErrors: {
    name?: string
    repo_url?: string
    docs_url?: string
  }
  saving: boolean
  serviceTypes: { value: ServiceType; label: string; icon: string }[]
  lifecycles: { value: ServiceLifecycle; label: string; icon: string }[]
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'save'): void
  (e: 'add-annotation'): void
  (e: 'remove-annotation', idx: number): void
  (e: 'preset-annotation', key: string, defaultValue: string): void
}>()
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    :title="mode === 'create' ? 'Register New Service' : `Edit Service: ${form.name}`"
    :subtitle="mode === 'create' ? 'Register a microservice, API, or library in the platform catalog' : `Update catalog metadata for ${form.id || form.name}`"
    max-width="660px"
    @update:show="emit('update:show', $event)"
  >
    <form class="service-form" @submit.prevent="emit('save')">
      <!-- Service Name -->
      <div class="form-group">
        <label class="form-label" for="form-svc-name">
          <span>Service Name</span>
          <span class="required">*</span>
        </label>
        <input
          id="form-svc-name"
          v-model="form.name"
          type="text"
          class="input-glass font-mono"
          :class="{ 'input-error': formErrors.name }"
          placeholder="e.g. auth-service, payment-gateway, ui-dashboard"
          required
        />
        <p v-if="formErrors.name" class="field-error">{{ formErrors.name }}</p>
      </div>

      <!-- Description -->
      <div class="form-group">
        <label class="form-label" for="form-svc-desc">Description</label>
        <textarea
          id="form-svc-desc"
          v-model="form.description"
          rows="3"
          class="input-glass form-textarea"
          placeholder="Brief overview of functionality, responsibilities, and system tier..."
        ></textarea>
      </div>

      <!-- Type & Lifecycle Row -->
      <div class="form-row">
        <div class="form-group flex-1">
          <label class="form-label" for="form-svc-type">Service Type</label>
          <select id="form-svc-type" v-model="form.type" class="input-glass">
            <option v-for="t in serviceTypes" :key="t.value" :value="t.value">
              {{ t.icon }} {{ t.label }}
            </option>
          </select>
        </div>

        <div class="form-group flex-1">
          <label class="form-label" for="form-svc-lifecycle">Lifecycle Stage</label>
          <select id="form-svc-lifecycle" v-model="form.lifecycle" class="input-glass">
            <option v-for="l in lifecycles" :key="l.value" :value="l.value">
              {{ l.icon }} {{ l.label }}
            </option>
          </select>
        </div>
      </div>

      <!-- Owner Team & Email Row -->
      <div class="form-row">
        <div class="form-group flex-1">
          <label class="form-label" for="form-svc-team">Owner Team</label>
          <input
            id="form-svc-team"
            v-model="form.owner_team"
            type="text"
            class="input-glass font-mono"
            placeholder="e.g. platform-eng, core-billing"
          />
        </div>

        <div class="form-group flex-1">
          <label class="form-label" for="form-svc-email">Owner Contact / Email</label>
          <input
            id="form-svc-email"
            v-model="form.owner_email"
            type="email"
            class="input-glass font-mono"
            placeholder="team-lead@company.com"
          />
        </div>
      </div>

      <!-- Repo & Docs URLs Row -->
      <div class="form-row">
        <div class="form-group flex-1">
          <label class="form-label" for="form-svc-repo">Repository URL</label>
          <input
            id="form-svc-repo"
            v-model="form.repo_url"
            type="url"
            class="input-glass font-mono"
            placeholder="https://github.com/org/repo"
          />
        </div>

        <div class="form-group flex-1">
          <label class="form-label" for="form-svc-docs">Docs / API URL</label>
          <input
            id="form-svc-docs"
            v-model="form.docs_url"
            type="url"
            class="input-glass font-mono"
            placeholder="https://docs.company.com/api"
          />
        </div>
      </div>

      <!-- Tags Input -->
      <div class="form-group">
        <label class="form-label" for="form-svc-tags">
          <span>Tags (comma-separated)</span>
        </label>
        <input
          id="form-svc-tags"
          v-model="form.tagsInput"
          type="text"
          class="input-glass font-mono"
          placeholder="golang, grpc, auth, payments, critical"
        />
        <p class="form-hint">Separate multiple tags with commas (e.g. "auth, jwt, microservice")</p>
      </div>

      <!-- Annotations Dynamic Key-Value Pairs -->
      <div class="form-group">
        <div class="annotation-header-row">
          <label class="form-label">
            <span>Metadata Annotations & Kubernetes References</span>
          </label>
          <button
            type="button"
            class="btn btn-secondary btn-xs"
            @click="emit('add-annotation')"
          >
            + Add Pair
          </button>
        </div>

        <!-- Preset Helpers -->
        <div class="preset-buttons-row">
          <span class="preset-hint text-muted">Quick add:</span>
          <button
            type="button"
            class="preset-btn"
            @click="emit('preset-annotation', 'k8s.io/namespace', 'default')"
          >
            + namespace
          </button>
          <button
            type="button"
            class="preset-btn"
            @click="emit('preset-annotation', 'k8s.io/deployment', form.name || 'app')"
          >
            + deployment
          </button>
          <button
            type="button"
            class="preset-btn"
            @click="emit('preset-annotation', 'k8s.io/cluster', 'primary-cluster')"
          >
            + cluster
          </button>
          <button
            type="button"
            class="preset-btn"
            @click="emit('preset-annotation', 'backstage.io/managed-by-location', 'url:https://github.com/org/repo')"
          >
            + backstage-location
          </button>
        </div>

        <!-- Key-Value Rows List -->
        <div class="kv-form-list">
          <div
            v-for="(row, idx) in form.annotationRows"
            :key="idx"
            class="kv-form-row animate-fade-in"
          >
            <input
              v-model="row.key"
              type="text"
              class="input-glass font-mono flex-1"
              placeholder="Key (e.g. k8s.io/namespace)"
            />
            <input
              v-model="row.value"
              type="text"
              class="input-glass font-mono flex-1"
              placeholder="Value (e.g. prod-workloads)"
            />
            <button
              type="button"
              class="btn-remove-row"
              title="Remove annotation"
              @click="emit('remove-annotation', idx)"
            >
              ✕
            </button>
          </div>
        </div>
      </div>
    </form>

    <template #footer="{ close }">
      <button type="button" class="btn btn-secondary" :disabled="saving" @click="close">
        Cancel
      </button>
      <button
        type="button"
        class="btn btn-primary"
        :disabled="saving || !form.name.trim()"
        @click="emit('save')"
      >
        <span>{{ saving ? '💾 Saving...' : (mode === 'create' ? 'Register Service' : 'Save Changes') }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>
