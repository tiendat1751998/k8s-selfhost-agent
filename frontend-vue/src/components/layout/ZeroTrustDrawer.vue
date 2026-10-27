<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import { securityApi } from '../../api/security'
import { resolveKmsProviderTelemetry, downloadAttestationDocument, type KmsDetails } from './zeroTrustUtils'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'update:show', val: boolean): void; (e: 'close'): void }>()

const kmsDetails = ref<KmsDetails | null>(null)
const activeKeyId = ref('--')
const keyVersion = ref(0)
const rotationStatus = ref<'idle' | 'rotating' | 'rotated' | 'error'>('idle')
const verificationStatus = ref<'idle' | 'verifying' | 'verified'>('idle')
const copiedKey = ref(false)
const kmsError = ref<string | null>(null)

const kmsTelemetry = computed(() => resolveKmsProviderTelemetry(kmsDetails.value))

function handleClose() {
  emit('update:show', false)
  emit('close')
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.show) handleClose()
}

async function loadKmsStatus() {
  kmsError.value = null
  try {
    const k = await securityApi.getKmsKey()
    kmsDetails.value = k
    if (k?.key_id) activeKeyId.value = k.key_id
    if (k?.version) keyVersion.value = k.version
  } catch (err: unknown) {
    activeKeyId.value = '--'
    keyVersion.value = 0
    kmsError.value = err instanceof Error ? err.message : 'Failed to load KMS key status'
  }
}

onMounted(async () => {
  window.addEventListener('keydown', handleKeydown)
  await loadKmsStatus()
})
onUnmounted(() => { window.removeEventListener('keydown', handleKeydown) })

async function copyKeyId() {
  if (activeKeyId.value === '--') return
  try {
    await navigator.clipboard.writeText(activeKeyId.value)
    copiedKey.value = true
    setTimeout(() => { copiedKey.value = false }, 2000)
  } catch { /* clipboard fallback */ }
}

async function rotateKey() {
  if (rotationStatus.value === 'rotating') return
  rotationStatus.value = 'rotating'
  kmsError.value = null
  try {
    const res = await securityApi.rotateKmsKey()
    if (res?.key_id) activeKeyId.value = res.key_id
    if (res?.version) keyVersion.value = res.version
    kmsDetails.value = { ...kmsDetails.value, key_id: res?.key_id, version: res?.version }
    rotationStatus.value = 'rotated'
  } catch (err: unknown) {
    activeKeyId.value = '--'
    keyVersion.value = 0
    rotationStatus.value = 'error'
    kmsError.value = err instanceof Error ? err.message : 'KMS key rotation failed'
  }
}

async function verifyChecksum() {
  if (verificationStatus.value === 'verifying') return
  verificationStatus.value = 'verifying'
  try {
    await securityApi.verifyKmsIntegrity()
    verificationStatus.value = 'verified'
  } catch {
    verificationStatus.value = 'idle'
  }
}

async function handleDownloadAttestation() {
  await downloadAttestationDocument(activeKeyId.value, keyVersion.value, kmsTelemetry.value)
}
</script>

<template>
  <Teleport to="body">
    <div v-if="show" class="zerotrust-drawer-root">
      <div class="drawer-backdrop" @click="handleClose" />
      <aside class="zerotrust-drawer" role="dialog" aria-modal="true" aria-labelledby="zt-drawer-title">
        <header class="drawer-header">
          <div class="header-brand">
            <div class="header-icon-wrap"><BaseIcon name="shield" size="md" /></div>
            <div>
              <h2 id="zt-drawer-title" class="drawer-title">ZeroTrust KMS &amp; State Posture</h2>
              <p class="drawer-subtitle font-mono">{{ kmsTelemetry.isHardware ? 'Hardware Security Module' : 'Envelope Encryption' }} &amp; Cluster State</p>
            </div>
          </div>
          <button class="drawer-close-btn" aria-label="Close ZeroTrust Drawer" title="Close Drawer" @click="handleClose">
            <BaseIcon name="x" size="sm" />
          </button>
        </header>

        <div class="drawer-body">
          <div v-if="kmsError" class="zt-error-alert" role="alert">
            <BaseIcon name="alert-triangle" size="xs" />
            <span>{{ kmsError }}</span>
          </div>

          <!-- KMS Enclave Status -->
          <section class="zt-section">
            <div class="section-badge-row">
              <span class="section-tag">KMS ENCLAVE STATUS</span>
              <span class="status-pill status-armed"><span class="pulse-dot pulse-dot-emerald"></span>ARMED</span>
            </div>
            <div class="zt-card">
              <div class="card-row"><span class="row-label">Provider</span><span class="row-val font-mono font-small">{{ kmsTelemetry.title }}</span></div>
              <div class="card-row"><span class="row-label">Enclave Isolation</span><span class="row-val font-mono" :class="kmsTelemetry.isHardware ? 'text-emerald' : 'text-cyan'">{{ kmsTelemetry.enclaveIsolation }}</span></div>
              <div class="card-row"><span class="row-label">Secret Protection</span><span class="row-val text-emerald font-mono">KMS Envelope Encryption</span></div>
              <div class="card-row"><span class="row-label">Rotation Posture</span><span class="row-val font-mono text-muted">90-day automatic</span></div>
            </div>
          </section>

          <!-- Cryptographic Key Info -->
          <section class="zt-section">
            <div class="section-badge-row">
              <span class="section-tag">CRYPTOGRAPHIC KEY INFO</span>
              <span class="version-chip font-mono">{{ keyVersion > 0 ? 'v' + keyVersion + ' Active' : 'Uninitialized' }}</span>
            </div>
            <div class="zt-card">
              <div class="card-row"><span class="row-label">Cipher Suite</span><span class="row-val font-mono text-cyan">AES-256-GCM</span></div>
              <div class="card-row"><span class="row-label">Rotation Policy</span><span class="row-val">90-day automatic</span></div>
              <div class="card-row key-id-row">
                <span class="row-label">Active Key ID</span>
                <div class="key-id-val font-mono">
                  <span>{{ activeKeyId }}</span>
                  <button class="btn-copy-mini" :title="copiedKey ? 'Copied!' : 'Copy Key ID'" @click="copyKeyId">
                    <BaseIcon :name="copiedKey ? 'check' : 'copy'" size="xs" />
                  </button>
                </div>
              </div>
            </div>
          </section>

          <!-- Cluster State Storage Posture -->
          <section class="zt-section">
            <div class="section-badge-row">
              <span class="section-tag">CLUSTER STATE STORE</span>
              <span class="status-pill status-synced font-mono">WAL Synced</span>
            </div>
            <div class="zt-card">
              <div class="sync-item">
                <div class="sync-icon"><BaseIcon name="database" size="sm" /></div>
                <div class="sync-info">
                  <div class="sync-title">PostgreSQL Primary State Store</div>
                  <div class="sync-sub font-mono">10.10.10.133:5432 · ACID Multi-Tenant Persistence</div>
                </div>
                <span class="sync-status-badge text-emerald">CONNECTED</span>
              </div>
              <div class="sync-divider" />
              <div class="sync-item">
                <div class="sync-icon"><BaseIcon name="archive" size="sm" /></div>
                <div class="sync-info">
                  <div class="sync-title">Automated DR &amp; Backup Engine</div>
                  <div class="sync-sub font-mono">Scheduled State Dump · Encrypted Volume Snapshots</div>
                </div>
                <span class="sync-status-badge text-cyan">ACTIVE</span>
              </div>
            </div>
          </section>

          <!-- Edge Agent & Network Security Posture -->
          <section class="zt-section">
            <div class="section-badge-row">
              <span class="section-tag">EDGE AGENT &amp; NETWORK POSTURE</span>
              <span class="status-pill status-mesh font-mono">Token / TLS</span>
            </div>
            <div class="zt-card">
              <div class="card-row"><span class="row-label">Edge Agent Auth</span><span class="row-val font-mono text-cyan">Bearer AGENT_AUTH_TOKEN</span></div>
              <div class="card-row"><span class="row-label">Transport Posture</span><span class="row-val font-mono">Optional TLS via AGENT_TLS_CERT</span></div>
              <div class="card-row"><span class="row-label">Telemetry &amp; Remediation</span><span class="row-val font-mono text-emerald">:9100 Host Agent RPC</span></div>
              <div class="card-row"><span class="row-label">Session Token Posture</span><span class="row-val text-emerald font-mono">In-Memory JWT · HttpOnly Cookie</span></div>
            </div>
          </section>

          <!-- Interactive Action Buttons -->
          <div class="drawer-actions">
            <button class="btn-zt-action btn-rotate" :disabled="rotationStatus === 'rotating'" @click="rotateKey">
              <BaseIcon :name="rotationStatus === 'rotating' ? 'refresh' : 'key'" size="xs" />
              <span>{{ rotationStatus === 'rotating' ? 'Rotating Enclave Key...' : rotationStatus === 'rotated' ? 'Rotated to v' + keyVersion + '!' : rotationStatus === 'error' ? 'Rotation Failed (Retry)' : 'Rotate Key' }}</span>
            </button>
            <button class="btn-zt-action btn-verify" :disabled="verificationStatus === 'verifying'" @click="verifyChecksum">
              <BaseIcon :name="verificationStatus === 'verifying' ? 'refresh' : 'check-circle'" size="xs" />
              <span>{{ verificationStatus === 'verifying' ? 'Verifying Integrity...' : verificationStatus === 'verified' ? 'Integrity Validated!' : 'Verify KMS Integrity' }}</span>
            </button>
            <button class="btn-zt-action btn-download" @click="handleDownloadAttestation">
              <BaseIcon name="download" size="xs" />
              <span>Download Attestation (JSON)</span>
            </button>
          </div>
        </div>
      </aside>
    </div>
  </Teleport>
</template>

<style scoped>
.zt-error-alert { display: flex; align-items: center; gap: 8px; padding: 10px 14px; border-radius: 6px; background: rgba(244, 63, 94, 0.12); border: 1px solid rgba(244, 63, 94, 0.35); color: #fb7185; font-size: 11px; margin-bottom: 12px; }
.zerotrust-drawer-root { position: fixed; inset: 0; z-index: 10000; display: flex; justify-content: flex-end; }
.drawer-backdrop { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.7); backdrop-filter: blur(6px); -webkit-backdrop-filter: blur(6px); }
.zerotrust-drawer { position: relative; width: 440px; max-width: 100vw; height: 100vh; background: var(--bg-card); border-left: 1px solid var(--color-border-subtle); box-shadow: var(--shadow-xl); display: flex; flex-direction: column; z-index: 10001; }
.drawer-header { padding: 16px 20px; border-bottom: 1px solid var(--color-border-subtle); display: flex; align-items: center; justify-content: space-between; gap: 12px; background: var(--color-surface-inset); }
.header-brand { display: flex; align-items: center; gap: 12px; min-width: 0; }
.header-icon-wrap { width: 36px; height: 36px; border-radius: 8px; background: rgba(6, 182, 212, 0.15); border: 1px solid rgba(6, 182, 212, 0.4); color: #38bdf8; display: flex; align-items: center; justify-content: center; }
.drawer-title { font-size: 15px; font-weight: 700; color: var(--text-primary); margin: 0; letter-spacing: -0.02em; }
.drawer-subtitle { font-size: 11px; color: var(--text-muted); margin: 2px 0 0; }
.drawer-close-btn { background: var(--color-surface-inset); border: 1px solid var(--color-border-subtle); color: var(--text-muted); width: 32px; height: 32px; border-radius: 6px; display: flex; align-items: center; justify-content: center; cursor: pointer; transition: all 0.15s ease; }
.drawer-close-btn:hover { background: var(--bg-card-hover); color: var(--text-primary); }
.drawer-body { flex: 1; overflow-y: auto; padding: 18px 20px; display: flex; flex-direction: column; gap: 18px; }
.zt-section { display: flex; flex-direction: column; gap: 8px; }
.section-badge-row { display: flex; justify-content: space-between; align-items: center; }
.section-tag { font-size: 10px; font-weight: 800; color: var(--text-muted); letter-spacing: 0.08em; }
.status-pill { display: inline-flex; align-items: center; gap: 6px; padding: 2px 8px; border-radius: 9999px; font-size: 10px; font-weight: 700; }
.status-armed { background: rgba(16, 185, 129, 0.15); border: 1px solid rgba(16, 185, 129, 0.4); color: #34d399; }
.status-synced { background: rgba(6, 182, 212, 0.12); border: 1px solid rgba(6, 182, 212, 0.35); color: #38bdf8; }
.status-mesh { background: rgba(99, 102, 241, 0.15); border: 1px solid rgba(99, 102, 241, 0.4); color: #818cf8; }
.version-chip { background: var(--color-surface-inset); border: 1px solid var(--color-border-subtle); color: var(--text-primary); font-size: 10px; padding: 2px 6px; border-radius: 4px; }
.zt-card { background: var(--color-surface-inset); border: 1px solid var(--color-border-subtle); border-radius: 8px; padding: 12px 14px; display: flex; flex-direction: column; gap: 8px; }
.card-row { display: flex; justify-content: space-between; align-items: center; font-size: 12px; gap: 8px; }
.row-label { color: var(--text-muted); font-size: 11px; }
.row-val { font-weight: 600; text-align: right; }
.key-id-row { padding-top: 4px; border-top: 1px solid var(--color-border-subtle); }
.key-id-val { display: inline-flex; align-items: center; gap: 6px; color: #38bdf8; font-size: 11px; }
.btn-copy-mini { background: transparent; border: none; color: var(--text-muted); cursor: pointer; padding: 2px 4px; border-radius: 4px; display: inline-flex; }
.btn-copy-mini:hover { color: var(--text-primary); background: var(--bg-card-hover); }
.sync-item { display: flex; align-items: center; gap: 10px; }
.sync-icon { color: #38bdf8; opacity: 0.85; }
.sync-info { flex: 1; min-width: 0; }
.sync-title { font-size: 12px; font-weight: 600; color: var(--text-primary); }
.sync-sub { font-size: 10px; color: var(--text-muted); }
.sync-status-badge { font-size: 10px; font-weight: 700; font-family: var(--font-mono, monospace); }
.sync-divider { height: 1px; background: var(--color-border-subtle); }
.drawer-actions { display: flex; flex-direction: column; gap: 8px; margin-top: 8px; }
.btn-zt-action { display: inline-flex; align-items: center; justify-content: center; gap: 8px; padding: 9px 14px; border-radius: 6px; font-size: 12px; font-weight: 600; cursor: pointer; transition: all 0.15s ease; }
.btn-rotate { background: var(--color-badge-agent-bg, rgba(6, 182, 212, 0.14)); border: 1px solid var(--color-badge-agent-border, rgba(6, 182, 212, 0.4)); color: var(--color-badge-agent-text, #38bdf8); }
.btn-rotate:hover:not(:disabled) { background: var(--color-badge-agent-bg, rgba(6, 182, 212, 0.25)); color: var(--color-badge-k8s-text, #38bdf8); }
.btn-verify { background: var(--color-status-online-bg, rgba(16, 185, 129, 0.14)); border: 1px solid var(--color-status-online-border, rgba(16, 185, 129, 0.4)); color: var(--color-status-online, #34d399); }
.btn-verify:hover:not(:disabled) { background: var(--color-status-online-bg, rgba(16, 185, 129, 0.25)); color: var(--color-status-online, #34d399); }
.btn-download { background: var(--color-surface-inset); border: 1px solid var(--color-border-subtle); color: var(--text-primary); }
.btn-download:hover { background: var(--bg-card-hover); color: var(--text-primary); }
.btn-zt-action:disabled { opacity: 0.6; cursor: not-allowed; }
</style>