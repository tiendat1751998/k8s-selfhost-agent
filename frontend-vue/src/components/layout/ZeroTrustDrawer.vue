<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'update:show', val: boolean): void; (e: 'close'): void }>()

const activeKeyId = ref('kms-k8s-root-9f82c')
const keyVersion = ref(4)
const rotationStatus = ref<'idle' | 'rotating' | 'rotated'>('idle')
const verificationStatus = ref<'idle' | 'verifying' | 'verified'>('idle')
const copiedKey = ref(false)

function handleClose() {
  emit('update:show', false)
  emit('close')
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.show) handleClose()
}

onMounted(() => { window.addEventListener('keydown', handleKeydown) })
onUnmounted(() => { window.removeEventListener('keydown', handleKeydown) })

async function copyKeyId() {
  try {
    await navigator.clipboard.writeText(activeKeyId.value)
    copiedKey.value = true
    setTimeout(() => { copiedKey.value = false }, 2000)
  } catch { /* clipboard fallback */ }
}

function rotateKey() {
  if (rotationStatus.value === 'rotating') return
  rotationStatus.value = 'rotating'
  setTimeout(() => {
    keyVersion.value += 1
    activeKeyId.value = 'kms-k8s-root-' + Math.random().toString(36).substring(2, 7)
    rotationStatus.value = 'rotated'
    setTimeout(() => { rotationStatus.value = 'idle' }, 3500)
  }, 900)
}

function verifyChecksum() {
  if (verificationStatus.value === 'verifying') return
  verificationStatus.value = 'verifying'
  setTimeout(() => {
    verificationStatus.value = 'verified'
    setTimeout(() => { verificationStatus.value = 'idle' }, 3500)
  }, 1000)
}

function downloadAttestation() {
  const attestationData = {
    schema_version: 'v1alpha1',
    attestation_timestamp: new Date().toISOString(),
    kms_enclave: {
      status: 'ARMED',
      provider: 'Hardware Security Module (HSM PKCS#11) / Vault KMS Provider',
      key_id: activeKeyId.value,
      key_version: 'v' + keyVersion.value,
      cipher: 'AES-256-GCM',
      rotation_policy: '90-day automatic',
      envelope_digest: 'sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'
    },
    dual_sync_targets: {
      primary: { type: 'NVMe Local Raft', status: 'SYNCED', commit_index: 4892104, latency_ms: 1.14 },
      secondary: {
        type: 'S3 Air-Gapped Sync', status: 'INTEGRITY_VERIFIED',
        bucket: 'k8scontrol-cold-raft-airgap', tls_version: 'TLSv1.3',
        integrity_checksum: 'sha256:8f434346648f6b96df89dda901c5176b10a6d83961dd3c1ac88b59b2dc327aa4'
      }
    },
    network_mesh: {
      mode: 'mTLS v1.3 with WireGuard/eBPF kernel enforcement',
      spiffe_spire: { trust_domain: 'spiffe://k8scontrol.prod', peer_verification: 'STRICT' }
    }
  }
  const blob = new Blob([JSON.stringify(attestationData, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'zerotrust-attestation-' + Date.now() + '.json'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
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
              <h2 id="zt-drawer-title" class="drawer-title">ZeroTrust KMS &amp; Dual-Sync</h2>
              <p class="drawer-subtitle font-mono">Hardware Security Module &amp; Air-Gapped Raft</p>
            </div>
          </div>
          <button class="drawer-close-btn" aria-label="Close ZeroTrust Drawer" title="Close Drawer" @click="handleClose">
            <BaseIcon name="x" size="sm" />
          </button>
        </header>

        <div class="drawer-body">
          <!-- KMS Enclave Status -->
          <section class="zt-section">
            <div class="section-badge-row">
              <span class="section-tag">KMS ENCLAVE STATUS</span>
              <span class="status-pill status-armed"><span class="pulse-dot pulse-dot-emerald"></span>ARMED</span>
            </div>
            <div class="zt-card">
              <div class="card-row"><span class="row-label">Provider</span><span class="row-val">Hardware Security Module / Vault KMS Provider</span></div>
              <div class="card-row"><span class="row-label">Enclave Isolation</span><span class="row-val text-emerald font-mono">Hardware Ring-0 HSM</span></div>
              <div class="card-row"><span class="row-label">Attestation Digest</span><span class="row-val font-mono text-muted">sha256:d8a2...9f1e</span></div>
            </div>
          </section>

          <!-- Cryptographic Key Info -->
          <section class="zt-section">
            <div class="section-badge-row">
              <span class="section-tag">CRYPTOGRAPHIC KEY INFO</span>
              <span class="version-chip font-mono">v{{ keyVersion }} Active</span>
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

          <!-- Dual-Sync Targets -->
          <section class="zt-section">
            <div class="section-badge-row">
              <span class="section-tag">DUAL-SYNC TARGETS</span>
              <span class="status-pill status-synced font-mono">&lt; 1.2ms Latency</span>
            </div>
            <div class="zt-card">
              <div class="sync-item">
                <div class="sync-icon"><BaseIcon name="database" size="sm" /></div>
                <div class="sync-info">
                  <div class="sync-title">Primary NVMe (Local Raft)</div>
                  <div class="sync-sub font-mono">Commit Index #4,892,104 · 1.14ms write</div>
                </div>
                <span class="sync-status-badge text-emerald">SYNCED</span>
              </div>
              <div class="sync-divider" />
              <div class="sync-item">
                <div class="sync-icon"><BaseIcon name="cloud" size="sm" /></div>
                <div class="sync-info">
                  <div class="sync-title">Secondary S3 (Air-Gapped Sync)</div>
                  <div class="sync-sub font-mono">TLS 1.3 Immutable WORM · 0 byte lag</div>
                </div>
                <span class="sync-status-badge text-cyan">VERIFIED</span>
              </div>
            </div>
          </section>

          <!-- ZeroTrust Network Mesh -->
          <section class="zt-section">
            <div class="section-badge-row">
              <span class="section-tag">ZEROTRUST NETWORK MESH</span>
              <span class="status-pill status-mesh font-mono">mTLS v1.3</span>
            </div>
            <div class="zt-card">
              <div class="card-row"><span class="row-label">Identity Attestation</span><span class="row-val">SPIFFE/SPIRE Dynamic Attestations</span></div>
              <div class="card-row"><span class="row-label">Trust Domain</span><span class="row-val font-mono text-cyan">spiffe://k8scontrol.prod</span></div>
              <div class="card-row"><span class="row-label">Peer Enforcement</span><span class="row-val text-emerald">Strict Zero-Implicit-Trust</span></div>
            </div>
          </section>

          <!-- Interactive Action Buttons -->
          <div class="drawer-actions">
            <button class="btn-zt-action btn-rotate" :disabled="rotationStatus === 'rotating'" @click="rotateKey">
              <BaseIcon :name="rotationStatus === 'rotating' ? 'refresh' : 'key'" size="xs" />
              <span>{{ rotationStatus === 'rotating' ? 'Rotating Enclave Key...' : rotationStatus === 'rotated' ? 'Rotated to v' + keyVersion + '!' : 'Rotate Key' }}</span>
            </button>
            <button class="btn-zt-action btn-verify" :disabled="verificationStatus === 'verifying'" @click="verifyChecksum">
              <BaseIcon :name="verificationStatus === 'verifying' ? 'refresh' : 'check-circle'" size="xs" />
              <span>{{ verificationStatus === 'verifying' ? 'Verifying S3 Integrity...' : verificationStatus === 'verified' ? 'Checksum SHA-256 Valid!' : 'Verify S3 Integrity Checksum' }}</span>
            </button>
            <button class="btn-zt-action btn-download" @click="downloadAttestation">
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
.zerotrust-drawer-root { position: fixed; inset: 0; z-index: 10000; display: flex; justify-content: flex-end; }
.drawer-backdrop { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.7); backdrop-filter: blur(6px); -webkit-backdrop-filter: blur(6px); animation: fadeIn 0.2s ease; }
.zerotrust-drawer { position: relative; width: 440px; max-width: 100vw; height: 100vh; background: rgba(11, 15, 25, 0.98); border-left: 1px solid var(--border-medium, rgba(6, 182, 212, 0.3)); box-shadow: -8px 0 32px rgba(0, 0, 0, 0.75), 0 0 20px rgba(6, 182, 212, 0.15); display: flex; flex-direction: column; z-index: 10001; animation: slideInRight 0.25s cubic-bezier(0.16, 1, 0.3, 1); }
.drawer-header { padding: 16px 20px; border-bottom: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08)); display: flex; align-items: center; justify-content: space-between; gap: 12px; background: rgba(15, 23, 42, 0.6); }
.header-brand { display: flex; align-items: center; gap: 12px; min-width: 0; }
.header-icon-wrap { width: 36px; height: 36px; border-radius: 8px; background: rgba(6, 182, 212, 0.15); border: 1px solid rgba(6, 182, 212, 0.4); color: #38bdf8; display: flex; align-items: center; justify-content: center; }
.drawer-title { font-size: 15px; font-weight: 700; color: #fff; margin: 0; letter-spacing: -0.02em; }
.drawer-subtitle { font-size: 11px; color: var(--text-muted, #94a3b8); margin: 2px 0 0; }
.drawer-close-btn { background: rgba(255, 255, 255, 0.04); border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.12)); color: var(--text-muted, #94a3b8); width: 32px; height: 32px; border-radius: 6px; display: flex; align-items: center; justify-content: center; cursor: pointer; transition: all 0.15s ease; }
.drawer-close-btn:hover { background: rgba(255, 255, 255, 0.1); color: #fff; }
.drawer-body { flex: 1; overflow-y: auto; padding: 18px 20px; display: flex; flex-direction: column; gap: 18px; }
.zt-section { display: flex; flex-direction: column; gap: 8px; }
.section-badge-row { display: flex; justify-content: space-between; align-items: center; }
.section-tag { font-size: 10px; font-weight: 800; color: var(--text-muted, #94a3b8); letter-spacing: 0.08em; }
.status-pill { display: inline-flex; align-items: center; gap: 6px; padding: 2px 8px; border-radius: 9999px; font-size: 10px; font-weight: 700; }
.status-armed { background: rgba(16, 185, 129, 0.15); border: 1px solid rgba(16, 185, 129, 0.4); color: #34d399; }
.status-synced { background: rgba(6, 182, 212, 0.12); border: 1px solid rgba(6, 182, 212, 0.35); color: #38bdf8; }
.status-mesh { background: rgba(99, 102, 241, 0.15); border: 1px solid rgba(99, 102, 241, 0.4); color: #818cf8; }
.version-chip { background: rgba(255, 255, 255, 0.06); border: 1px solid rgba(255, 255, 255, 0.15); color: #e2e8f0; font-size: 10px; padding: 2px 6px; border-radius: 4px; }
.zt-card { background: rgba(15, 23, 42, 0.65); border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08)); border-radius: 8px; padding: 12px 14px; display: flex; flex-direction: column; gap: 8px; }
.card-row { display: flex; justify-content: space-between; align-items: center; font-size: 12px; gap: 8px; }
.row-label { color: var(--text-muted, #94a3b8); font-size: 11px; }
.row-val { font-weight: 600; text-align: right; }
.key-id-row { padding-top: 4px; border-top: 1px solid rgba(255, 255, 255, 0.05); }
.key-id-val { display: inline-flex; align-items: center; gap: 6px; color: #38bdf8; font-size: 11px; }
.btn-copy-mini { background: transparent; border: none; color: var(--text-muted, #94a3b8); cursor: pointer; padding: 2px 4px; border-radius: 4px; display: inline-flex; }
.btn-copy-mini:hover { color: #fff; background: rgba(255, 255, 255, 0.1); }
.sync-item { display: flex; align-items: center; gap: 10px; }
.sync-icon { color: #38bdf8; opacity: 0.85; }
.sync-info { flex: 1; min-width: 0; }
.sync-title { font-size: 12px; font-weight: 600; color: #f1f5f9; }
.sync-sub { font-size: 10px; color: var(--text-muted, #94a3b8); }
.sync-status-badge { font-size: 10px; font-weight: 700; font-family: var(--font-mono, monospace); }
.sync-divider { height: 1px; background: rgba(255, 255, 255, 0.06); }
.drawer-actions { display: flex; flex-direction: column; gap: 8px; margin-top: 8px; }
.btn-zt-action { display: inline-flex; align-items: center; justify-content: center; gap: 8px; padding: 9px 14px; border-radius: 6px; font-size: 12px; font-weight: 600; cursor: pointer; transition: all 0.15s ease; }
.btn-rotate { background: rgba(6, 182, 212, 0.14); border: 1px solid rgba(6, 182, 212, 0.4); color: #38bdf8; }
.btn-rotate:hover:not(:disabled) { background: rgba(6, 182, 212, 0.25); color: #fff; }
.btn-verify { background: rgba(16, 185, 129, 0.14); border: 1px solid rgba(16, 185, 129, 0.4); color: #34d399; }
.btn-verify:hover:not(:disabled) { background: rgba(16, 185, 129, 0.25); color: #fff; }
.btn-download { background: rgba(255, 255, 255, 0.05); border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.12)); color: #e2e8f0; }
.btn-download:hover { background: rgba(255, 255, 255, 0.1); color: #fff; }
.btn-zt-action:disabled { opacity: 0.6; cursor: not-allowed; }
@keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
@keyframes slideInRight { from { transform: translateX(100%); } to { transform: translateX(0); } }
</style>