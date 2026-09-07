<script setup lang="ts">
import type { DeploymentTemplate } from '../../api/compute'
import ModalDrawer from '../ui/ModalDrawer.vue'

interface Props {
  show: boolean
  templates: DeploymentTemplate[]
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'select', tmpl: DeploymentTemplate): void
}>()
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="drawer"
    title="Application Blueprint Catalog"
    subtitle="Pre-configured enterprise service blueprints ready to instantiate"
    max-width="560px"
    @update:show="emit('update:show', $event)"
  >
    <div class="templates-list">
      <div
        v-for="tmpl in templates"
        :key="tmpl.name"
        class="template-item glass-panel"
      >
        <div class="tmpl-header">
          <div>
            <h4 class="tmpl-title">{{ tmpl.name }}</h4>
            <span class="tmpl-category font-mono">{{ tmpl.category }} · {{ tmpl.version }} · {{ tmpl.strategy || 'RollingUpdate' }}</span>
          </div>
          <button type="button" class="btn btn-primary btn-xs" @click="emit('select', tmpl)">
            <span>Use Blueprint ➔</span>
          </button>
        </div>
        <p class="tmpl-desc">{{ tmpl.desc }}</p>
        <div class="tmpl-specs font-mono">
          <span>Port: {{ tmpl.ports }}</span>
          <span>CPU: {{ tmpl.cpu }}</span>
          <span>RAM: {{ tmpl.mem }}</span>
        </div>
      </div>
    </div>

    <template #footer="{ close }">
      <button type="button" class="btn btn-secondary" @click="close">Close Catalog</button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
.templates-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.template-item {
  padding: 14px;
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tmpl-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 10px;
}

.tmpl-title {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
  margin: 0;
}

.tmpl-category {
  font-size: 11px;
  color: var(--accent-cyan);
}

.tmpl-desc {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.4;
  margin: 0;
}

.tmpl-specs {
  display: flex;
  gap: 12px;
  font-size: 11px;
  color: var(--text-muted);
  background: rgba(0, 0, 0, 0.2);
  padding: 6px 10px;
  border-radius: 6px;
}
</style>