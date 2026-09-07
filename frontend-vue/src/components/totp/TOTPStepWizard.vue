<template>
  <div class="stepper-card glass-panel" role="navigation" aria-label="TOTP Setup Steps">
    <div class="stepper">
      <div
        v-for="(s, index) in coreSteps"
        :key="s.id"
        class="step-item"
        :class="{
          'step-active': isStepActive(s.id),
          'step-completed': isStepCompleted(index),
          'step-disabled': isStepDisabled(index)
        }"
      >
        <div class="step-circle" :aria-current="isStepActive(s.id) ? 'step' : undefined">
          <span v-if="isStepCompleted(index)">✓</span>
          <span v-else>{{ index + 1 }}</span>
        </div>
        <div class="step-label-group">
          <span class="step-title">{{ s.title }}</span>
          <span class="step-desc">{{ s.desc }}</span>
        </div>
        <div v-if="index < coreSteps.length - 1" class="step-line" aria-hidden="true"></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { WizardStep } from '../../composables/useTOTPSetup'

interface Props {
  currentStep: WizardStep
}

const props = defineProps<Props>()

interface CoreStep {
  id: 'scan' | 'verify' | 'recovery'
  title: string
  desc: string
}

const coreSteps: CoreStep[] = [
  { id: 'scan', title: '1. Scan QR', desc: 'Authenticator App' },
  { id: 'verify', title: '2. Verify Code', desc: '6-digit sync check' },
  { id: 'recovery', title: '3. Backup Keys', desc: 'Offline recovery tokens' },
]

const currentCoreIndex = computed(() => {
  if (props.currentStep === 'intro') return -1
  if (props.currentStep === 'scan') return 0
  if (props.currentStep === 'verify') return 1
  if (props.currentStep === 'recovery') return 2
  if (props.currentStep === 'success') return 3
  return -1
})

function isStepActive(stepId: string): boolean {
  return props.currentStep === stepId
}

function isStepCompleted(index: number): boolean {
  return currentCoreIndex.value > index
}

function isStepDisabled(index: number): boolean {
  return currentCoreIndex.value < index
}
</script>
