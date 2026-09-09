<template>
  <div class="recovery-codes-card-printable">
    <!-- Warning Alert -->
    <div class="alert-box-warning" role="alert">
      <span class="alert-icon" aria-hidden="true">⚠️</span>
      <div class="alert-text">
        <strong>CRITICAL:</strong> These backup codes will <u>NEVER</u> be displayed again. Store them in a secure password manager or encrypted drive.
      </div>
    </div>

    <!-- Emergency Scratch Tokens Grid (8 tokens) -->
    <div class="recovery-codes-grid" aria-label="Emergency recovery scratch tokens">
      <div
        v-for="(code, idx) in displayCodes"
        :key="idx"
        class="recovery-code-card"
      >
        <span class="code-number">#{{ idx + 1 }}</span>
        <code class="code-value font-mono">{{ code }}</code>
      </div>
    </div>

    <!-- Action Bar: [ 📋 Copy ], [ 📥 Download TXT ], [ 🖨 Print ] -->
    <div class="recovery-actions-bar">
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :aria-label="allCodesCopied ? 'All recovery codes copied' : 'Copy all recovery codes to clipboard'"
        @click="$emit('copy-codes')"
      >
        <span>{{ allCodesCopied ? '✓ All Codes Copied' : '📋 Copy All Codes' }}</span>
      </button>

      <button
        type="button"
        class="btn btn-secondary btn-sm"
        aria-label="Download recovery codes as a plaintext file"
        @click="$emit('download-txt')"
      >
        <span>📥 Download TXT</span>
      </button>

      <button
        type="button"
        class="btn btn-secondary btn-sm"
        aria-label="Print recovery codes document"
        @click="$emit('print')"
      >
        <span>🖨 Print</span>
      </button>
    </div>

    <!-- Acknowledgement Checkbox -->
    <div class="acknowledgement-group">
      <label class="custom-checkbox-label">
        <input
          type="checkbox"
          class="custom-checkbox"
          :checked="hasSavedCodes"
          @change="$emit('update:hasSavedCodes', ($event.target as HTMLInputElement).checked)"
        />
        <span class="checkbox-text">
          I confirm that I have saved these {{ displayCodes.length }} emergency recovery tokens in a secure, encrypted location.
        </span>
      </label>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  recoveryCodes: string[]
  allCodesCopied: boolean
  hasSavedCodes: boolean
}

const props = defineProps<Props>()

defineEmits<{
  (e: 'update:hasSavedCodes', value: boolean): void
  (e: 'copy-codes'): void
  (e: 'download-txt'): void
  (e: 'print'): void
}>()

const displayCodes = computed(() => {
  return props.recoveryCodes.length > 0 ? props.recoveryCodes : []
})
</script>
