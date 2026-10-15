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
    <!-- 40px Mobile Command Bar (< 640px) -->
    <div class="hosts-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title">Compute Hosts</span>
        <span class="command-bar-count-pill font-mono">{{ filteredHostsCount }}</span>
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
        <button
          type="button"
          class="btn-icon-cmd"
          :class="{ active: showMobileSearch }"
          title="Search"
          aria-label="Search"
          @click="emit('update:showMobileSearch', !showMobileSearch)"
        >
          <BaseIcon name="search" size="xs" />
        </button>
        <button
          type="button"
          class="btn-icon-cmd"
          title="Refresh"
          aria-label="Refresh"
          :disabled="loading"
          @click="emit('refresh')"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'animate-spin': loading }" />
        </button>
        <button
          type="button"
          class="btn-cmd-add"
          title="Add Host"
          aria-label="Add Host"
          @click="emit('addHost')"
        >
          + Add
        </button>
      </div>
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
.hosts-mobile-command-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: var(--bg-surface, #0f1012);
  border-bottom: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
  gap: 8px;
}
.command-bar-left {
  display: flex;
  align-items: center;
  gap: 8px;
}
.command-bar-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary, #f3f4f6);
  letter-spacing: -0.01em;
}
.command-bar-count-pill {
  font-size: 11px;
  font-family: var(--font-mono, monospace);
  color: var(--text-secondary, #9ca3af);
  background: rgba(255, 255, 255, 0.06);
  padding: 1px 6px;
  border-radius: 9999px;
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
}
.command-bar-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}
.btn-icon-cmd {
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  background: var(--bg-card, #141516);
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
  color: var(--text-secondary, #9ca3af);
  cursor: pointer;
  transition: all 0.15s ease;
}
.btn-icon-cmd:hover {
  background: var(--bg-elevated, #1a1b1e);
  color: var(--text-primary, #f3f4f6);
  border-color: rgba(255, 255, 255, 0.15);
}
.btn-cmd-add {
  height: 28px;
  padding: 0 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  background: var(--color-primary, #10b981);
  color: #08090a;
  border: 1px solid transparent;
  cursor: pointer;
  transition: opacity 0.15s ease;
}
.btn-cmd-add:hover {
  opacity: 0.9;
}
.hosts-micro-telemetry {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  font-size: 11px;
  color: var(--text-muted, #6b7280);
  background: var(--bg-canvas, #08090a);
  border-bottom: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.06));
}
.tel-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.tel-sep {
  color: var(--text-muted, #6b7280);
  opacity: 0.5;
}
.micro-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  display: inline-block;
  flex-shrink: 0;
}
.dot-emerald {
  background: #10b981;
}
.dot-rose {
  background: #ef4444;
}
.dot-slate {
  background: #6b7280;
}
.mobile-search-box {
  padding: 8px 12px;
  background: var(--bg-card, #141516);
  border-bottom: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.mobile-filter-pills-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  flex-wrap: wrap;
}
.m-filter-group {
  display: flex;
  align-items: center;
  gap: 4px;
}
.m-filter-pill {
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 500;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
  color: var(--text-secondary, #9ca3af);
  cursor: pointer;
  transition: all 0.15s ease;
}
.m-filter-pill.active {
  background: var(--bg-elevated, #1a1b1e);
  color: var(--text-primary, #f3f4f6);
  border-color: rgba(255, 255, 255, 0.18);
}
.m-filter-select {
  font-size: 11px;
  background: var(--bg-surface, #0f1012);
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
  color: var(--text-secondary, #9ca3af);
  padding: 2px 6px;
  border-radius: 4px;
}
.fencing-alert-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  margin-bottom: 12px;
  background: var(--bg-card, #141516);
  border: 1px solid rgba(245, 158, 11, 0.25);
  border-left: 3px solid var(--status-warning, #f59e0b);
  border-radius: 6px;
  gap: 12px;
  flex-wrap: wrap;
}
.fencing-alert-info {
  display: flex;
  align-items: center;
  gap: 8px;
}
.fencing-alert-icon-box {
  color: #f59e0b;
  flex-shrink: 0;
  display: flex;
  align-items: center;
}
.fencing-alert-texts {
  display: flex;
  flex-direction: column;
  font-size: 12px;
  line-height: 1.3;
}
.fencing-alert-title {
  font-size: 12px;
  font-weight: 600;
  color: #f59e0b;
}
.fencing-alert-sub {
  font-size: 11px;
  color: var(--text-muted, #6b7280);
}
.fencing-alert-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.btn-action-fence {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 5px;
  font-size: 11px;
  font-weight: 600;
  background: rgba(245, 158, 11, 0.12);
  color: #f59e0b;
  border: 1px solid rgba(245, 158, 11, 0.35);
  cursor: pointer;
  transition: all 0.15s ease;
}
.btn-action-fence:hover {
  background: #f59e0b;
  color: #08090a;
}
@media (max-width: 640px) {
  .fencing-alert-strip {
    padding: 6px 10px;
    margin-bottom: 8px;
    gap: 6px;
  }
  .fencing-alert-title {
    font-size: 11px;
  }
  .fencing-alert-sub {
    display: none;
  }
  .btn-action-fence {
    padding: 3px 6px;
    font-size: 10px;
  }
}
</style>

