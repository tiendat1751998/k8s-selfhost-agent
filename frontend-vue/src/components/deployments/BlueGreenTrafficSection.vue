<script setup lang="ts">
import type { DeploymentApp } from '../../api/compute'

interface Props {
  app: DeploymentApp
  actionLoading?: string | null
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'cutoverBlueGreen', app: DeploymentApp, color: 'blue' | 'green'): void
}>()
</script>

<template>
  <div class="bluegreen-config-panel glass-panel">
    <div class="bg-header-row">
      <div>
        <h4 class="strat-title">Active / Standby Cutover Router</h4>
        <p class="strat-desc">Instantly toggle live ingress traffic between isolated Blue & Green deployments.</p>
      </div>
      <span class="bg-active-tag font-mono">
        ACTIVE: {{ (app.blueGreenActive || 'blue').toUpperCase() }}
      </span>
    </div>

    <div class="bg-tracks-row">
      <!-- Blue Environment Card -->
      <div
        class="bg-track-box"
        :class="{ 'is-active-track': (app.blueGreenActive || 'blue') === 'blue' }"
      >
        <div class="bg-track-top">
          <span class="bg-color-indicator dot-blue"></span>
          <span class="bg-env-name font-mono">BLUE TRACK</span>
          <span v-if="(app.blueGreenActive || 'blue') === 'blue'" class="live-pill">LIVE ROUTING</span>
          <span v-else class="standby-pill">STANDBY</span>
        </div>
        <span class="bg-track-img font-mono">{{ app.blueVersion || app.image }}</span>
        <div class="bg-track-actions">
          <button
            type="button"
            class="btn btn-secondary btn-xs w-full"
            :disabled="(app.blueGreenActive || 'blue') === 'blue' || actionLoading === 'cutover'"
            @click="emit('cutoverBlueGreen', app, 'blue')"
          >
            <span>{{ (app.blueGreenActive || 'blue') === 'blue' ? '✓ Currently Live' : '⚡ Cutover to Blue' }}</span>
          </button>
        </div>
      </div>

      <!-- Green Environment Card -->
      <div
        class="bg-track-box"
        :class="{ 'is-active-track': app.blueGreenActive === 'green' }"
      >
        <div class="bg-track-top">
          <span class="bg-color-indicator dot-green"></span>
          <span class="bg-env-name font-mono">GREEN TRACK</span>
          <span v-if="app.blueGreenActive === 'green'" class="live-pill">LIVE ROUTING</span>
          <span v-else class="standby-pill">STANDBY</span>
        </div>
        <span class="bg-track-img font-mono">{{ app.greenVersion || `${app.image}-v2` }}</span>
        <div class="bg-track-actions">
          <button
            type="button"
            class="btn btn-secondary btn-xs w-full"
            :disabled="app.blueGreenActive === 'green' || actionLoading === 'cutover'"
            @click="emit('cutoverBlueGreen', app, 'green')"
          >
            <span>{{ app.blueGreenActive === 'green' ? '✓ Currently Live' : '⚡ Cutover to Green' }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/canary-modal.css';
</style>
