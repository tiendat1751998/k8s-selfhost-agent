<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'
import type { ComputeHost } from '../../api/compute'
import type { HostTypeDefinition } from '../../types/hosts'

const props = defineProps<{
  filteredHostsCount: number
  loading: boolean
  showMobileSearch: boolean
  searchQuery: string
  hosts: ComputeHost[]
  onlineHostsCount: number
  offlineHostsCount: number
  avgLatency: number
  selectedTypeFilter?: string
  selectedStatusFilter?: string
  hostTypeDefinitions?: HostTypeDefinition[]
  typeCounts?: Record<string, number>
}>()

const emit = defineEmits<{
  (e: 'update:showMobileSearch', val: boolean): void
  (e: 'update:searchQuery', val: string): void
  (e: 'update:selectedTypeFilter', val: string): void
  (e: 'update:selectedStatusFilter', val: string): void
  (e: 'addHost'): void
  (e: 'refresh'): void
  (e: 'openFencing', nodeName: string, clusterId: string): void
}>()

function handleFirstFencing() {
  const first = props.hosts.find(h => h.status !== 'connected')
  if (first) {
    emit('openFencing', first.name, first.tenant_id || 'default')
  }
}
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
    <div v-if="offlineHostsCount > 0" class="fencing-alert-strip animate-fade-in">
      <div class="fencing-alert-info">
        <span class="fencing-alert-dot" aria-hidden="true"></span>
        <div class="fencing-alert-texts">
          <span class="fencing-alert-title font-medium">{{ offlineHostsCount }} unreachable host(s)</span>
          <span class="fencing-alert-sub text-muted">— RWO volumes may require fencing</span>
        </div>
      </div>
      <div class="fencing-alert-actions">
        <button
          type="button"
          class="btn btn-action-fence"
          @click="handleFirstFencing"
        >
          <BaseIcon name="shield" size="xs" />
          <span>Fence Host</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.infra-hosts-header-bar-wrapper {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.hosts-mobile-command-bar {
  height: 44px;
  min-height: 44px;
  padding: 0 12px;
  background: var(--bg-card);
  border: 1px solid var(--color-border-subtle);
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.command-bar-left {
  display: flex;
  align-items: center;
  gap: 8px;
}
.command-bar-title {
  font-family: var(--font-sans);
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
  letter-spacing: -0.01em;
  line-height: 1;
}
.command-bar-count-pill {
  height: 20px;
  min-width: 22px;
  padding: 0 6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  font-weight: 600;
  font-family: var(--font-mono);
  color: var(--text-secondary);
  background: var(--color-surface-inset);
  border: 1px solid var(--color-border-subtle);
  border-radius: 6px;
  line-height: 1;
}
.command-bar-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}
.btn-icon-cmd {
  width: 30px;
  height: 30px;
  min-width: 30px;
  min-height: 30px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  background: var(--color-surface-inset);
  border: 1px solid var(--color-border-subtle);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.15s ease;
}
.btn-icon-cmd:hover {
  background: var(--bg-elevated);
  color: var(--text-primary);
  border-color: var(--color-border-strong);
}
.btn-cmd-add {
  height: 30px;
  padding: 0 12px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  font-family: var(--font-sans);
  font-size: 12px;
  font-weight: 600;
  background: var(--color-primary, #326ce5) !important;
  color: #ffffff !important;
  border: 1px solid rgba(255, 255, 255, 0.18) !important;
  cursor: pointer;
  transition: opacity 0.15s ease, background 0.15s ease;
}
.btn-cmd-add:hover {
  background: var(--color-primary-hover, #3b82f6) !important;
}
.hosts-micro-telemetry {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  font-size: 11px;
  color: var(--text-muted, #6b7280);
  background: var(--bg-app);
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
  min-height: 42px;
  padding: 8px 12px;
  margin: 0;
  background: var(--color-status-warning-bg, rgba(210, 153, 34, 0.10));
  border: 1px solid var(--color-status-warning-border, rgba(245, 158, 11, 0.35));
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: nowrap;
}
.fencing-alert-info {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  overflow: hidden;
}
.fencing-alert-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--color-status-warning, #f59e0b);
  flex-shrink: 0;
}
.fencing-alert-texts {
  display: flex;
  align-items: center;
  gap: 6px;
  font-family: var(--font-sans);
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.fencing-alert-title {
  color: var(--text-primary);
  font-weight: 600;
}
.fencing-alert-sub {
  color: var(--text-muted);
}
.fencing-alert-actions {
  display: flex;
  align-items: center;
  flex-shrink: 0;
}
.btn-action-fence {
  height: 28px;
  padding: 0 10px;
  font-family: var(--font-sans);
  font-size: 12px;
  font-weight: 500;
  background: var(--bg-card);
  color: var(--text-primary);
  border: 1px solid var(--color-border-strong);
  border-radius: 6px;
  white-space: nowrap;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  cursor: pointer;
  transition: all 0.15s ease;
}
.btn-action-fence:hover {
  background: var(--bg-elevated);
  border-color: var(--color-border-strong);
  color: var(--text-primary);
}
@media (max-width: 640px) {
  .fencing-alert-strip {
    padding: 8px 12px;
    margin: 0;
    gap: 8px;
  }
  .fencing-alert-sub {
    display: none;
  }
  .btn-action-fence {
    height: 28px;
    padding: 0 10px;
    font-size: 12px;
  }
}
</style>

