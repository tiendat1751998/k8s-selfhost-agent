<script setup lang="ts">
import { ref, watch } from 'vue'
import type { DeploymentApp } from '../../api/compute'
import ModalDrawer from '../ui/ModalDrawer.vue'
import CanaryTrafficSection from './CanaryTrafficSection.vue'
import BlueGreenTrafficSection from './BlueGreenTrafficSection.vue'
import { formatContainerName } from '../../utils/dockerFormat'

interface Props {
  show: boolean
  app: DeploymentApp | null
  actionLoading?: string | null
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'update:strategy', strategy: string): void
  (e: 'changeStrategy', app: DeploymentApp, strategy: string): void
  (e: 'applyCanaryWeight', weight: number): void
  (e: 'promoteCanary', app: DeploymentApp): void
  (e: 'abortCanary', app: DeploymentApp): void
  (e: 'cutoverBlueGreen', app: DeploymentApp, color: 'blue' | 'green'): void
  (e: 'rollback', app: DeploymentApp): void
  (e: 'togglePause', app: DeploymentApp): void
}>()

const canarySliderWeight = ref(20)
const activeStrategy = ref<string>('RollingUpdate')

watch(
  () => props.app,
  (newApp) => {
    if (newApp) {
      canarySliderWeight.value = newApp.canaryWeight || 20
      activeStrategy.value = newApp.strategy || 'RollingUpdate'
    }
  },
  { immediate: true }
)

function onStrategyTabClick(strat: string) {
  activeStrategy.value = strat
  emit('update:strategy', strat)
  if (props.app) {
    emit('changeStrategy', props.app, strat)
  }
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    :title="`Rollout & Traffic Strategy: ${formatContainerName(app?.name).serviceName || ''}`"
    subtitle="Manage live Canary weights, Blue-Green zero-downtime cutover, and revision rollbacks"
    max-width="640px"
    @update:show="emit('update:show', $event)"
  >
    <div v-if="app" class="strategy-modal-content">
      <!-- Strategy Selector Header -->
      <div class="strategy-type-selector">
        <button
          type="button"
          class="strat-tab-btn"
          :class="{ 'strat-active': activeStrategy === 'Canary' }"
          @click="onStrategyTabClick('Canary')"
        >
          <span>🐥 Canary Traffic Split</span>
        </button>
        <button
          type="button"
          class="strat-tab-btn"
          :class="{ 'strat-active': activeStrategy === 'BlueGreen' }"
          @click="onStrategyTabClick('BlueGreen')"
        >
          <span>🔄 Blue-Green Zero-Downtime</span>
        </button>
        <button
          type="button"
          class="strat-tab-btn"
          :class="{ 'strat-active': activeStrategy === 'RollingUpdate' || !activeStrategy }"
          @click="onStrategyTabClick('RollingUpdate')"
        >
          <span>📦 Rolling Update</span>
        </button>
      </div>

      <!-- Canary Mode Controls -->
      <CanaryTrafficSection
        v-if="activeStrategy === 'Canary'"
        :app="app"
        :weight="canarySliderWeight"
        :action-loading="actionLoading"
        @update:weight="canarySliderWeight = $event"
        @abort-canary="emit('abortCanary', $event)"
        @promote-canary="emit('promoteCanary', $event)"
      />

      <!-- Blue-Green Mode Controls -->
      <BlueGreenTrafficSection
        v-else-if="activeStrategy === 'BlueGreen'"
        :app="app"
        :action-loading="actionLoading"
        @cutover-blue-green="(app, color) => emit('cutoverBlueGreen', app, color)"
      />

      <!-- Rollout Lifecycle & Revisions Panel -->
      <div class="revisions-panel glass-panel">
        <div class="revisions-header">
          <span class="rev-title font-mono">Revision History & Safe Rollbacks</span>
          <div class="revisions-btns">
            <button
              type="button"
              class="btn btn-secondary btn-xs"
              :disabled="actionLoading === 'pause'"
              @click="emit('togglePause', app)"
            >
              <span>{{ app.paused ? '▶️ Resume Rollout' : '⏸️ Pause Rollout' }}</span>
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-xs"
              :disabled="actionLoading === 'rollback'"
              @click="emit('rollback', app)"
            >
              <span>⏮️ Rollback to Previous</span>
            </button>
          </div>
        </div>

        <div class="revisions-list font-mono">
          <div class="rev-item is-current">
            <span class="rev-num">Revision #{{ app.revision || 3 }} (Current)</span>
            <span class="rev-img">{{ app.image }}</span>
            <span class="rev-tag text-emerald">Active</span>
          </div>
          <div class="rev-item">
            <span class="rev-num">Revision #{{ Math.max(1, (app.revision || 3) - 1) }}</span>
            <span class="rev-img">{{ app.image.replace(/:.*/, ':v1.8.4') }}</span>
            <button
              type="button"
              class="btn btn-secondary btn-xs"
              @click="emit('rollback', app)"
            >
              Rollback
            </button>
          </div>
        </div>
      </div>
    </div>

    <template #footer="{ close }">
      <button type="button" class="btn btn-secondary" @click="close">Close</button>
      <button
        v-if="activeStrategy === 'Canary'"
        type="button"
        class="btn btn-primary"
        :disabled="actionLoading === 'canary'"
        @click="emit('applyCanaryWeight', canarySliderWeight)"
      >
        <span>Apply Canary Weight ({{ canarySliderWeight }}%) ➔</span>
      </button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/components/canary-modal.css';
</style>