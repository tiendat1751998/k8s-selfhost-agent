<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'
import type { ComputeHost } from '../../api/compute'

defineProps<{
  filteredHostsCount: number
  loading: boolean
  showMobileSearch: boolean
  searchQuery: string
  hosts: ComputeHost[]
  onlineHostsCount: number
  offlineHostsCount: number
  avgLatency: number
}>()

const emit = defineEmits<{
  (e: 'update:showMobileSearch', val: boolean): void
  (e: 'update:searchQuery', val: string): void
  (e: 'addHost'): void
  (e: 'refresh'): void
  (e: 'openFencing', nodeName: string, clusterId: string): void
}>()
</script>

<template>
  <div class="infra-hosts-header-bar-wrapper">
    <!-- 44px Mobile Command Bar (< 640px) -->
    <div class="hosts-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <BaseIcon name="server" size="sm" />
        <span class="command-bar-title font-bold">Compute Hosts ({{ filteredHostsCount }})</span>
      </div>
      <div class="command-bar-actions">
        <button class="btn-icon-cmd" title="Add Host" @click="emit('addHost')">
          <span class="font-bold text-sm">+</span>
        </button>
        <button class="btn-icon-cmd" title="Refresh" :disabled="loading" @click="emit('refresh')">
          <BaseIcon name="refresh" size="xs" :class="{ 'animate-spin': loading }" />
        </button>
        <button
          class="btn-icon-cmd"
          :class="{ active: showMobileSearch }"
          title="Search"
          @click="emit('update:showMobileSearch', !showMobileSearch)"
        >
          <BaseIcon name="search" size="xs" />
        </button>
      </div>
    </div>

    <!-- 20px Mobile Micro-Telemetry Strip (< 640px) -->
    <div class="hosts-micro-telemetry mobile-only">
      <span class="tel-item"><BaseIcon name="server" size="xs" /> {{ hosts.length }} hosts</span>
      <span>·</span>
      <span class="tel-item tel-online"><BaseIcon name="shield" size="xs" /> {{ onlineHostsCount }} online</span>
      <span>·</span>
      <span class="tel-item tel-offline"><BaseIcon name="alert-triangle" size="xs" /> {{ offlineHostsCount }} offline</span>
      <template v-if="offlineHostsCount > 0">
        <span>·</span>
        <button
          type="button"
          class="btn-micro-fence font-mono"
          @click="emit('openFencing', hosts.find(h => h.status !== 'connected')?.name || '', hosts.find(h => h.status !== 'connected')?.tenant_id || 'default')"
        >
          <BaseIcon name="shield" size="xs" /> Safe Fence
        </button>
      </template>
      <span v-if="onlineHostsCount > 0 && avgLatency > 0">·</span>
      <span v-if="onlineHostsCount > 0 && avgLatency > 0" class="tel-item tel-latency"><BaseIcon name="zap" size="xs" /> avg {{ avgLatency }}ms</span>
    </div>

    <!-- Mobile Search Expandable Input -->
    <div v-if="showMobileSearch" class="mobile-search-box mobile-only animate-fade-in">
      <input
        :value="searchQuery"
        type="text"
        placeholder="Filter hosts by name, IP, label..."
        class="input-glass"
        style="width: 100%; font-size: 12px; padding: 6px 10px;"
        @input="emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
      />
    </div>

    <!-- SRE Storage Fencing Alert Banner (When Unready/Offline Nodes Exist) -->
    <div v-if="offlineHostsCount > 0" class="fencing-alert-strip glass-panel animate-fade-in">
      <div class="fencing-alert-info">
        <span class="fencing-alert-icon-box"><BaseIcon name="alert-triangle" size="sm" /></span>
        <div class="fencing-alert-texts">
          <span class="fencing-alert-title font-bold text-amber">
            {{ offlineHostsCount }} Unready / Offline Host(s) Detected
          </span>
          <span class="fencing-alert-sub text-muted">
            Workloads with RWO CSI volumes risk split-brain corruption without STONITH fencing.
          </span>
        </div>
      </div>
      <div class="fencing-alert-actions">
        <button
          v-for="unreadyHost in hosts.filter(h => h.status !== 'connected')"
          :key="unreadyHost.id"
          type="button"
          class="btn btn-warning-sm btn-action-fence font-mono"
          @click="emit('openFencing', unreadyHost.name, unreadyHost.tenant_id || 'default')"
        >
          <BaseIcon name="shield" size="xs" />
          <span>Safe Remediation / Fence: {{ unreadyHost.name }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
