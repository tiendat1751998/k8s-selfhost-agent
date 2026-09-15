<script setup lang="ts">
import type { Report } from '../../api/management'
import BaseIcon from '../ui/BaseIcon.vue'

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
    icon: 'shield',
    shortName: 'Executive Summary',
    subtitle: 'CIS Benchmark & Pod Security Standards',
    format: 'pdf'
  },
  {
    id: 'tpl-finops',
    title: 'FinOps Cost Allocation & Multi-Cluster Node Waste',
    category: 'cost',
    icon: 'trending-up',
    shortName: 'FinOps Cost Allocation',
    subtitle: 'Node Bin-Packing & Spot Savings Allocation',
    format: 'csv'
  },
  {
    id: 'tpl-soc2',
    title: 'SOC2 Type II Platform & Access Audit Verification',
    category: 'compliance',
    icon: 'file-text',
    shortName: 'SOC2 Audit',
    subtitle: 'RBAC, Zero-Trust & Audit Trail Logging',
    format: 'pdf'
  },
  {
    id: 'tpl-inventory',
    title: 'Cluster Inventory & Topology Resource Mapping',
    category: 'operational',
    icon: 'package',
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
  <div class="reports-quick-strip glass-panel" role="region" aria-label="Quick Report Templates">
    <div class="quick-strip-label">
      <BaseIcon name="zap" size="xs" class="strip-label-icon" />
      <span class="label-text">Quick Launch:</span>
    </div>

    <div class="quick-chips-wrapper">
      <button
        v-for="tpl in templates"
        :key="tpl.id"
        type="button"
        class="quick-template-chip"
        :title="`${tpl.title} (${tpl.format.toUpperCase()})`"
        @click="selectTemplate(tpl)"
      >
        <BaseIcon :name="tpl.icon" size="xs" class="chip-icon" />
        <span class="chip-name">{{ tpl.shortName }}</span>
        <span class="chip-format font-mono uppercase">{{ tpl.format }}</span>
      </button>
    </div>
  </div>
</template>
