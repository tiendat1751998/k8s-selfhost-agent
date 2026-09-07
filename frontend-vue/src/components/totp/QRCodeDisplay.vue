<template>
  <div class="qr-showcase">
    <div class="qr-box-wrapper">
      <div v-if="qrImageSrc" class="qr-image-container">
        <img :src="qrImageSrc" alt="TOTP QR Code" class="qr-img" />
      </div>
      <div v-else class="qr-loading-placeholder" role="status" aria-live="polite">
        <span class="spinner" aria-hidden="true"></span>
        <span>Generating QR Code...</span>
      </div>
    </div>

    <div class="manual-secret-box">
      <span class="manual-label">Can't scan the QR code? Enter this key manually:</span>
      <div class="secret-display-pill">
        <code class="secret-key font-mono">{{ secretKey || '...' }}</code>
        <button
          type="button"
          class="btn btn-secondary btn-sm copy-btn"
          :aria-label="secretCopied ? 'Security key copied to clipboard' : 'Copy security key'"
          @click="$emit('copy-secret')"
        >
          <span>{{ secretCopied ? '✓ Copied' : '📋 Copy Key' }}</span>
        </button>
      </div>
      <p class="secret-hint">
        Account: <strong>{{ accountEmail || 'Operator' }}</strong> | Type: <strong>Time-based (30s)</strong>
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
interface Props {
  qrImageSrc: string
  secretKey: string
  secretCopied: boolean
  accountEmail?: string
}

defineProps<Props>()

defineEmits<{
  (e: 'copy-secret'): void
}>()
</script>
