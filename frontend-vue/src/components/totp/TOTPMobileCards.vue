<template>
  <div class="totp-mobile-cards" role="region" aria-label="Mobile TOTP Setup Cards">
    <!-- Card 1: QR & Secret Key (~65px) -->
    <div
      class="totp-mobile-card-item"
      :class="{
        'card-active': currentStep === 'scan',
        'card-completed': isStepDone('scan')
      }"
      role="button"
      tabindex="0"
      @click="$emit('select-step', 'scan')"
      @keydown.enter="$emit('select-step', 'scan')"
    >
      <div class="mobile-card-left">
        <div class="mobile-card-icon" aria-hidden="true">📱</div>
        <div class="mobile-card-content">
          <span class="mobile-card-title">1. QR & Secret Key</span>
          <span class="mobile-card-desc">Scan authenticator or copy Base32</span>
        </div>
      </div>
      <span
        class="mobile-card-badge"
        :class="isStepDone('scan') ? 'badge-emerald' : currentStep === 'scan' ? 'badge-cyan' : 'badge'"
      >
        {{ isStepDone('scan') ? 'Done ✓' : currentStep === 'scan' ? 'Active' : 'Step 1' }}
      </span>
    </div>

    <!-- Card 2: 6-Digit Verify Code (~65px) -->
    <div
      class="totp-mobile-card-item"
      :class="{
        'card-active': currentStep === 'verify',
        'card-completed': isStepDone('verify')
      }"
      role="button"
      tabindex="0"
      @click="$emit('select-step', 'verify')"
      @keydown.enter="$emit('select-step', 'verify')"
    >
      <div class="mobile-card-left">
        <div class="mobile-card-icon" aria-hidden="true">🔢</div>
        <div class="mobile-card-content">
          <span class="mobile-card-title">2. Verify 6-Digit Code</span>
          <span class="mobile-card-desc">Time-based HMAC-SHA1 sync</span>
        </div>
      </div>
      <span
        class="mobile-card-badge"
        :class="isStepDone('verify') ? 'badge-emerald' : currentStep === 'verify' ? 'badge-cyan' : 'badge'"
      >
        {{ isStepDone('verify') ? 'Done ✓' : currentStep === 'verify' ? 'Active' : 'Step 2' }}
      </span>
    </div>

    <!-- Card 3: Emergency Backup Keys (~65px) -->
    <div
      class="totp-mobile-card-item"
      :class="{
        'card-active': currentStep === 'recovery',
        'card-completed': isStepDone('recovery')
      }"
      role="button"
      tabindex="0"
      @click="$emit('select-step', 'recovery')"
      @keydown.enter="$emit('select-step', 'recovery')"
    >
      <div class="mobile-card-left">
        <div class="mobile-card-icon" aria-hidden="true">📦</div>
        <div class="mobile-card-content">
          <span class="mobile-card-title">3. Backup Scratch Tokens</span>
          <span class="mobile-card-desc">8 emergency recovery codes</span>
        </div>
      </div>
      <span
        class="mobile-card-badge"
        :class="isStepDone('recovery') ? 'badge-emerald' : currentStep === 'recovery' ? 'badge-cyan' : 'badge'"
      >
        {{ isStepDone('recovery') ? 'Done ✓' : currentStep === 'recovery' ? 'Active' : 'Step 3' }}
      </span>
    </div>

    <!-- Card 4: MFA Enforcement State (~65px) -->
    <div
      class="totp-mobile-card-item"
      :class="{ 'card-completed': isEnforced }"
      role="status"
    >
      <div class="mobile-card-left">
        <div class="mobile-card-icon" aria-hidden="true">🛡️</div>
        <div class="mobile-card-content">
          <span class="mobile-card-title">MFA Enforcement</span>
          <span class="mobile-card-desc">{{ isEnforced ? 'Cluster perimeter enforced' : 'Setup in progress' }}</span>
        </div>
      </div>
      <span
        class="mobile-card-badge"
        :class="isEnforced ? 'badge-emerald' : 'badge'"
      >
        {{ isEnforced ? 'Enforced' : 'Pending' }}
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { WizardStep } from '../../composables/useTOTPSetup'

interface Props {
  currentStep: WizardStep
  isEnforced: boolean
}

const props = defineProps<Props>()

defineEmits<{
  (e: 'select-step', step: WizardStep): void
}>()

const stepOrder: WizardStep[] = ['intro', 'scan', 'verify', 'recovery', 'success']

function isStepDone(step: WizardStep): boolean {
  const currentIndex = stepOrder.indexOf(props.currentStep)
  const targetIndex = stepOrder.indexOf(step)
  return currentIndex > targetIndex
}
</script>
