<script setup lang="ts">
import type { SettingsFormState, CategoryKey, TestState } from '../../composables/useSettings'
import { backupProviderOptions } from '../../composables/useSettings'

const props = defineProps<{
  form: SettingsFormState
  saving: boolean
  isDirty: boolean
  integrationTests: Record<string, TestState>
}>()

const emit = defineEmits<{
  (e: 'save', category: CategoryKey): void
  (e: 'reset', category: CategoryKey): void
  (e: 'testService', key: string, customUrl?: string): void
}>()

const schedulePresets = [
  { label: '⚡ Hourly Snapshot', cron: '0 * * * *' },
  { label: '🌙 Daily @ 02:00 UTC', cron: '0 2 * * *' },
  { label: '📅 Weekly (Sun 03:00)', cron: '0 3 * * 0' },
  { label: '📊 Monthly (1st @ 04:00)', cron: '0 4 1 * *' },
]

const retentionPresets = [7, 14, 30, 60, 90, 180, 365]
</script>

<template>
  <div class="settings-card glass-panel animate-fade-in">
    <div class="card-header">
      <div class="card-title-group">
        <div>
          <h2 class="card-title">Disaster Recovery & Automated Backup Schedules</h2>
          <p class="card-subtitle">
            Configure S3 / MinIO storage targets, cron schedules, snapshot retention, and encryption policies.
          </p>
        </div>
        <span v-if="isDirty" class="dirty-indicator-pill">● Unsaved Changes</span>
      </div>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        @click="emit('reset', 'backup')"
      >
        <span>↺ Reset Defaults</span>
      </button>
    </div>

    <form class="settings-form" @submit.prevent="emit('save', 'backup')">
      <!-- Storage Provider Selection -->
      <div class="form-group">
        <label class="form-label">
          <span>Backup Storage Engine</span>
        </label>
        <p class="field-desc">Select the persistent object storage destination for etcd, volume, and manifest archives.</p>
        <div class="provider-selector-grid">
          <div
            v-for="prov in backupProviderOptions"
            :key="prov.id"
            class="provider-card"
            :class="{ active: form.backup_provider === prov.id }"
            @click="form.backup_provider = prov.id as any"
          >
            <div class="provider-card-head">
              <span class="provider-icon">{{ prov.icon }}</span>
              <span class="provider-name">{{ prov.name }}</span>
            </div>
            <p class="provider-desc">{{ prov.desc }}</p>
          </div>
        </div>
      </div>

      <!-- Endpoint & Bucket Details -->
      <div class="integration-item glass-panel">
        <div class="integration-header">
          <div class="integration-title-group">
            <div class="integration-icon">🗄️</div>
            <div>
              <h3 class="integration-name">Object Storage Bucket Credentials</h3>
              <p class="integration-desc">Target endpoint and bucket parameters for Velero / K8s snapshot sync.</p>
            </div>
          </div>
          <div class="integration-status">
            <span v-if="integrationTests.backup_s3_endpoint?.testing" class="badge badge-amber">⏳ Testing...</span>
            <span v-else-if="integrationTests.backup_s3_endpoint?.result?.reachable" class="badge badge-emerald">
              ✓ HTTP {{ integrationTests.backup_s3_endpoint.result.status_code }} ({{ integrationTests.backup_s3_endpoint.result.latency_ms }}ms)
            </span>
            <span v-else-if="integrationTests.backup_s3_endpoint?.error" class="badge badge-rose">
              ✗ {{ integrationTests.backup_s3_endpoint.error }}
            </span>
            <span v-else-if="form.backup_s3_endpoint" class="badge badge-cyan">CONFIGURED</span>
            <span v-else class="badge badge-muted">NOT CONFIGURED</span>
          </div>
        </div>

        <div class="form-row">
          <div class="form-group flex-2">
            <label class="form-label" for="backup-endpoint">Storage Endpoint URL</label>
            <div class="integration-input-row">
              <input
                id="backup-endpoint"
                v-model="form.backup_s3_endpoint"
                type="url"
                class="input-glass form-input flex-1"
                placeholder="http://minio.backup.svc:9000 or https://s3.us-east-1.amazonaws.com"
              />
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="integrationTests.backup_s3_endpoint?.testing || !form.backup_s3_endpoint"
                @click="emit('testService', 'backup_s3_endpoint')"
              >
                <span>{{ integrationTests.backup_s3_endpoint?.testing ? '⏳ Testing...' : '⚡ Test Connection' }}</span>
              </button>
            </div>
          </div>

          <div class="form-group flex-1">
            <label class="form-label" for="backup-bucket">Target Bucket Name</label>
            <input
              id="backup-bucket"
              v-model="form.backup_s3_bucket"
              type="text"
              class="input-glass form-input font-mono"
              placeholder="k8s-platform-snapshots"
            />
          </div>

          <div class="form-group flex-1">
            <label class="form-label" for="backup-region">Region / Cluster Zone</label>
            <input
              id="backup-region"
              v-model="form.backup_s3_region"
              type="text"
              class="input-glass form-input font-mono"
              placeholder="us-east-1"
            />
          </div>
        </div>
      </div>

      <!-- Schedule Cron & Retention Rules -->
      <div class="form-row">
        <div class="form-group flex-1">
          <label class="form-label" for="backup-cron">
            <span>Automated Schedule (Cron Expression)</span>
            <span class="required">*</span>
          </label>
          <p class="field-desc">5-field standard cron syntax defining automated snapshot execution.</p>
          <input
            id="backup-cron"
            v-model="form.backup_schedule_cron"
            type="text"
            class="input-glass form-input font-mono"
            placeholder="0 2 * * *"
            required
          />
          <div class="schedule-presets">
            <button
              v-for="preset in schedulePresets"
              :key="preset.cron"
              type="button"
              class="preset-chip"
              :class="{ active: form.backup_schedule_cron === preset.cron }"
              @click="form.backup_schedule_cron = preset.cron"
            >
              {{ preset.label }}
            </button>
          </div>
        </div>

        <div class="form-group flex-1">
          <label class="form-label" for="backup-retention">
            <span>Snapshot Retention Rule (Days)</span>
            <span class="required">*</span>
          </label>
          <p class="field-desc">Historical recovery point objective (RPO) retention window.</p>
          <input
            id="backup-retention"
            v-model.number="form.backup_retention_days"
            type="number"
            min="1"
            max="3650"
            class="input-glass form-input"
            required
          />
          <div class="schedule-presets">
            <button
              v-for="days in retentionPresets"
              :key="days"
              type="button"
              class="preset-chip"
              :class="{ active: form.backup_retention_days === days }"
              @click="form.backup_retention_days = days"
            >
              {{ days }} Days
            </button>
          </div>
        </div>
      </div>

      <!-- Security & Compression Toggles -->
      <div class="form-row">
        <div class="form-group flex-1 toggle-group">
          <div class="toggle-info">
            <span class="toggle-label">AES-256 GCM Snapshot Encryption</span>
            <p class="field-desc">Encrypt backup archives at rest with KMS envelope keys before object upload.</p>
          </div>
          <label class="toggle-switch">
            <input v-model="form.backup_encryption_enabled" type="checkbox" />
            <span class="toggle-slider"></span>
          </label>
        </div>

        <div class="form-group flex-1 toggle-group">
          <div class="toggle-info">
            <span class="toggle-label">Auto-Verify Snapshot Integrity</span>
            <p class="field-desc">Executes dry-run manifest decompression and checksum checks post-upload.</p>
          </div>
          <label class="toggle-switch">
            <input v-model="form.backup_auto_verify" type="checkbox" />
            <span class="toggle-slider"></span>
          </label>
        </div>
      </div>

      <div class="form-group">
        <label class="form-label" for="backup-compression">Compression Algorithm</label>
        <p class="field-desc">Select trade-off between backup throughput and storage volume size.</p>
        <select id="backup-compression" v-model="form.backup_compression_level" class="input-glass form-select">
          <option value="none">None (Raw stream uncompressed)</option>
          <option value="fast">LZ4 Fast (Optimized for minimal CPU overhead - Recommended)</option>
          <option value="high">ZSTD High Compression (Optimized for storage savings)</option>
        </select>
      </div>

      <!-- Actions -->
      <div class="form-actions">
        <span class="field-desc">Automated backups synchronize with platform disaster recovery daemon.</span>
        <button type="submit" class="btn btn-primary" :disabled="saving">
          <span v-if="saving" class="spinner spinner-sm"></span>
          <span>{{ saving ? '💾 Saving Changes...' : '💾 Save Backup Settings' }}</span>
        </button>
      </div>
    </form>
  </div>
</template>
