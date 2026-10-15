<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'
import type { ComputeHost } from '../../api/compute'
import type { HostTypeDefinition } from '../../types/hosts'

defineProps<{
  filteredHostsCount: number
  loading: boolean
  showMobileSearch: boolean
  searchQuery: string
  hosts: ComputeHost[]
  onlineHostsCount: number
  offlineHostsCount: number
  avgLatency: number
  showTelemetry?: boolean
  selectedTypeFilter?: string
  selectedStatusFilter?: string
  hostTypeDefinitions?: HostTypeDefinition[]
  typeCounts?: Record<string, number>
}>()

const emit = defineEmits<{
  (e: 'update:showMobileSearch', val: boolean): void
  (e: 'update:searchQuery', val: string): void
  (e: 'update:showTelemetry', val: boolean): void
  (e: 'update:selectedTypeFilter', val: string): void
  (e: 'update:selectedStatusFilter', val: string): void
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
        <button
          type="button"
          class="btn-icon-cmd"
          :class="{ active: showTelemetry }"
          title="Toggle Fleet Telemetry"
          aria-label="Toggle Fleet Telemetry"
          @click="emit('update:showTelemetry', !showTelemetry)"
        >
          <BaseIcon name="activity" size="xs" />
        </button>
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
      <template v-if="onlineHostsCount > 0 && avgLatency > 0">
        <span>·</span>
        <span class="tel-item tel-latency"><BaseIcon name="zap" size="xs" /> avg {{ avgLatency }}ms</span>
      </template>
    </div>

    <!-- Mobile Search Expandable Input & Filter Controls -->
    <div v-if="showMobileSearch" class="mobile-search-box mobile-only animate-fade-in">
      <input
        :value="searchQuery"
        type="text"
        placeholder="Filter hosts by name, IP, label..."
        class="input-glass"
        style="width: 100%; font-size: 12px; padding: 6px 10px;"
        @input="emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
      />
      <div class="mobile-filter-pills-row">
        <div class="m-filter-group" role="group" aria-label="Filter hosts by status">
          <button
            type="button"
            class="m-filter-pill"
            :class="{ active: (selectedStatusFilter || 'all') === 'all' }"
            @click="emit('update:selectedStatusFilter', 'all')"
          >
            All
          </button>
          <button
            type="button"
            class="m-filter-pill m-pill-connected"
            :class="{ active: selectedStatusFilter === 'connected' }"
            @click="emit('update:selectedStatusFilter', 'connected')"
          >
            Online ({{ onlineHostsCount }})
          </button>
          <button
            v-if="offlineHostsCount > 0"
            type="button"
            class="m-filter-pill m-pill-disconnected"
            :class="{ active: selectedStatusFilter === 'disconnected' }"
            @click="emit('update:selectedStatusFilter', 'disconnected')"
          >
            Offline ({{ offlineHostsCount }})
          </button>
        </div>
        <select
          v-if="hostTypeDefinitions && hostTypeDefinitions.length > 0"
          :value="selectedTypeFilter || 'all'"
          class="m-filter-select font-mono"
          aria-label="Filter hosts by type"
          @change="emit('update:selectedTypeFilter', ($event.target as HTMLSelectElement).value)"
        >
          <option value="all">All Types</option>
          <option v-for="def in hostTypeDefinitions" :key="def.type" :value="def.type">
            {{ def.label }} {{ typeCounts?.[def.type] ? `(${typeCounts[def.type]})` : '' }}
          </option>
        </select>
      </div>
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

<style scoped>
.fencing-alert-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  margin-bottom: 12px;
  background: rgba(245, 158, 11, 0.08);
  border: 1px solid rgba(245, 158, 11, 0.3);
  border-radius: 8px;
  gap: 12px;
  flex-wrap: wrap;
}
.fencing-alert-info { display: flex; align-items: center; gap: 10px; }
.fencing-alert-icon-box { color: #f59e0b; flex-shrink: 0; }
.fencing-alert-texts { display: flex; flex-direction: column; font-size: 12px; line-height: 1.3; }
.fencing-alert-title { font-size: 12px; }
.fencing-alert-sub { font-size: 11px; }
.fencing-alert-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.btn-action-fence {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 700;
  background: rgba(245, 158, 11, 0.15);
  color: #f59e0b;
  border: 1px solid rgba(245, 158, 11, 0.4);
  cursor: pointer;
  transition: all 0.15s ease;
}
.btn-action-fence:hover {
  background: #f59e0b;
  color: #0f172a;
}
@media (max-width: 640px) {
  .fencing-alert-strip {
    padding: 8px 10px;
    margin-bottom: 10px;
    gap: 8px;
  }
  .fencing-alert-title {
    font-size: 11px;
  }
  .fencing-alert-sub {
    display: none;
  }
  .btn-action-fence {
    padding: 4px 8px;
    font-size: 10.5px;
  }
}
</style>

