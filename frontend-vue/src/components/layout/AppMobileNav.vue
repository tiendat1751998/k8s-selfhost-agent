<script setup lang="ts">
import { useRoute } from 'vue-router'
import { useAlertStore } from '../../stores/alertStore'

const route = useRoute()
const alertStore = useAlertStore()
</script>

<template>
  <!-- Mobile Bottom Navigation Bar (Docked) -->
  <nav class="mobile-bottom-bar" aria-label="Mobile Navigation">
    <router-link to="/" class="mobile-nav-tab" :class="{ 'mobile-tab-active': route.path === '/' }">
      <span class="tab-icon">📊</span>
      <span class="tab-label font-mono">Overview</span>
    </router-link>
    <router-link to="/logs" class="mobile-nav-tab" :class="{ 'mobile-tab-active': route.path.startsWith('/logs') }">
      <span class="tab-icon">📜</span>
      <span class="tab-label font-mono">Live Logs</span>
    </router-link>
    <router-link to="/fleet" class="mobile-nav-tab" :class="{ 'mobile-tab-active': route.path.startsWith('/fleet') }">
      <span class="tab-icon">☸️</span>
      <span class="tab-label font-mono">Fleet</span>
    </router-link>
    <router-link to="/deployments" class="mobile-nav-tab" :class="{ 'mobile-tab-active': route.path.startsWith('/deployments') }">
      <span class="tab-icon">🚀</span>
      <span class="tab-label font-mono">Deploy</span>
    </router-link>
    <button 
      class="mobile-nav-tab mobile-nav-btn" 
      :class="{ 'mobile-tab-active': alertStore.showAlertCenterModal }" 
      @click="alertStore.showAlertCenterModal = true"
      aria-label="Open Alert Center"
    >
      <div class="tab-icon-wrap">
        <span class="tab-icon">🔔</span>
        <span v-if="alertStore.activeAlerts.length > 0" class="mobile-badge-count font-mono">
          {{ alertStore.activeAlerts.length > 99 ? '99+' : alertStore.activeAlerts.length }}
        </span>
      </div>
      <span class="tab-label font-mono">Alerts</span>
    </button>
  </nav>
</template>
