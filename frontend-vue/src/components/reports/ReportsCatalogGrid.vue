<script setup lang="ts">
import type { Report } from '../../api/management'

interface ReportTemplate {
  id: string
  title: string
  category: Report['type']
  icon: string
  shortName: string
  subtitle: string
  format: Report['format']
}

const templates: ReportTemplate[] = [
  {
    id: 'tpl-exec',
    title: 'Executive Summary: CIS Platform & Pod Security Audit',
    category: 'compliance',
    icon: '🛡️',
    shortName: 'Executive Summary',
    subtitle: 'CIS Benchmark & Pod Security Standards',
    format: 'pdf'
  },
  {
    id: 'tpl-finops',
    title: 'FinOps Cost Allocation & Multi-Cluster Node Waste',
    category: 'cost',
    icon: '💰',
    shortName: 'FinOps Cost Allocation',
    subtitle: 'Node Bin-Packing & Spot Savings Allocation',
    format: 'csv'
  },
  {
    id: 'tpl-soc2',
    title: 'SOC2 Type II Platform & Access Audit Verification',
    category: 'compliance',
    icon: '📋',
    shortName: 'SOC2 Audit',
    subtitle: 'RBAC, Zero-Trust & Audit Trail Logging',
    format: 'pdf'
  },
  {
    id: 'tpl-inventory',
    title: 'Cluster Inventory & Topology Resource Mapping',
    category: 'operational',
    icon: '📦',
    shortName: 'Cluster Inventory',
    subtitle: 'Global Workloads, Nodes & DaemonSets',
    format: 'pdf'
  }
]

const emit = defineEmits<{
  quickGenerate: [type: Report['type'], title: string, format: Report['format']]
}>()

function selectTemplate(tpl: ReportTemplate) {
  emit('quickGenerate', tpl.category, tpl.title, tpl.format)
}
</script>

<template>
  <div class="templates-card glass-panel">
    <div class="template-header">
      <h3>⚡ One-Click Report Templates</h3>
      <span class="text-muted text-xs">Pre-configured executive & compliance templates</span>
    </div>

    <div class="template-grid">
      <div 
        v-for="tpl in templates" 
        :key="tpl.id" 
        class="template-box"
        role="button"
        tabindex="0"
        :title="`Generate ${tpl.shortName}`"
        @click="selectTemplate(tpl)"
        @keydown.enter="selectTemplate(tpl)"
      >
        <span class="t-icon">{{ tpl.icon }}</span>
        <div class="t-info">
          <span class="t-name">{{ tpl.shortName }}</span>
          <small class="t-sub">{{ tpl.subtitle }}</small>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.template-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}

@media (min-width: 768px) and (max-width: 1023px) {
  .template-grid {
    grid-template-columns: repeat(2, 1fr) !important;
    gap: 10px !important;
  }
}

@media (max-width: 767px) {
  .template-grid {
    grid-template-columns: 1fr !important;
    gap: 8px !important;
  }
}
</style>
