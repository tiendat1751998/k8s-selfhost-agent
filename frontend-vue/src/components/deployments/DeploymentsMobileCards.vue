<script setup lang="ts">
import type { DeploymentApp } from '../../api/compute'
import { formatContainerName } from '../../utils/dockerFormat'
import BaseIcon from '../ui/BaseIcon.vue'

interface Props {
  deployments: DeploymentApp[]
  loading?: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'inspect', app: DeploymentApp): void
  (e: 'logs', app: DeploymentApp): void
}>()
</script>

<template>
  <div class="mobile-cards-stream mobile-card-stream">
    <div v-if="deployments.length === 0 && !loading" class="empty-mobile-state font-mono">
      <span><BaseIcon name="box" size="lg" /></span>
      <p>0 Workloads matching active filters</p>
    </div>

    <div
      v-for="app in deployments"
      :key="app.rawId || app.id || app.name"
      class="mobile-workload-card glass-panel animate-fade-in"
      @click="emit('inspect', app)"
    >
      <!-- Row 1: Status Dot + Workload Name + Namespace Badge -->
      <div class="card-row-1">
        <div class="card-name-group">
          <span
            class="status-dot"
            :class="{
              'dot-healthy': app.status === 'healthy',
              'dot-amber': app.status === 'warning' || app.status === 'degraded',
              'dot-crimson': app.status === 'error' || app.status === 'failed'
            }"
            title="Workload Status"
          ></span>
          <span class="workload-name font-semibold text-slate-100" :title="app.name">
            {{ formatContainerName(app.name).serviceName }}
          </span>
          <span class="ns-badge font-mono" :title="`Namespace: ${app.namespace || 'default'}`">
            {{ app.namespace || 'default' }}
          </span>
        </div>
      </div>

      <!-- Row 2: Replicas, Type Badge, and 2 Quick-Action Buttons -->
      <div class="card-row-2">
        <div class="card-meta-group font-mono">
          <span class="meta-replicas">
            <span class="text-emerald font-semibold">{{ app.readyReplicas !== undefined ? app.readyReplicas : (app.status === 'healthy' ? app.replicas : 0) }}</span><span class="text-muted">/</span><span class="desired-count">{{ app.replicas }}</span>
          </span>
          <span class="runtime-badge font-mono">{{ app.type }}</span>
          <span v-if="app.strategy === 'Canary'" class="canary-mini-badge font-mono">
            <BaseIcon name="git-branch" size="xs" /> {{ app.canaryWeight || 20 }}%
          </span>
          <span v-else-if="app.strategy === 'BlueGreen'" class="bg-mini-badge font-mono">
            <BaseIcon name="refresh" size="xs" /> {{ (app.blueGreenActive || 'blue').toUpperCase() }}
          </span>
        </div>

        <div class="card-quick-actions">
          <button
            type="button"
            class="btn-quick-action btn-quick-logs"
            title="Inspect Container Logs"
            @click.stop="emit('logs', app)"
          >
            <BaseIcon name="file-text" size="xs" /> Logs
          </button>
          <button
            type="button"
            class="btn-quick-action btn-quick-inspect"
            title="Inspect Details & Operations"
            @click.stop="emit('inspect', app)"
          >
            <BaseIcon name="zap" size="xs" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/deployments.css';

.status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  display: inline-block;
  background-color: currentColor;
}
.status-dot.dot-healthy {
  background-color: #10b981;
  box-shadow: 0 0 6px rgba(16, 185, 129, 0.6);
}
.status-dot.dot-amber {
  background-color: #f59e0b;
  box-shadow: 0 0 6px rgba(245, 158, 11, 0.6);
}
.status-dot.dot-crimson {
  background-color: #ef4444;
  box-shadow: none;
}
</style>
