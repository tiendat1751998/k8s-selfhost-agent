<script setup lang="ts">
import type { ProcessMetric } from '../../api/hosts'

defineProps<{
  processes: ProcessMetric[]
}>()
</script>

<template>
  <div class="glass-panel" style="overflow-x: auto; padding: 6px;">
    <table class="font-mono" style="width: 100%; border-collapse: collapse; font-size: 11px;">
      <thead>
        <tr style="color: var(--text-muted); border-bottom: 1px solid var(--color-border-subtle); text-align: left;">
          <th style="padding: 6px 8px;">PID</th>
          <th style="padding: 6px 8px;">NAME</th>
          <th style="padding: 6px 8px;">USER</th>
          <th style="padding: 6px 8px; text-align: right;">CPU %</th>
          <th style="padding: 6px 8px; text-align: right;">RAM %</th>
          <th style="padding: 6px 8px; text-align: center;">STATE</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="proc in processes" :key="proc.pid" style="border-bottom: 1px solid var(--color-hairline);">
          <td style="padding: 6px 8px; color: var(--text-muted);">{{ proc.pid }}</td>
          <td style="padding: 6px 8px; color: var(--text-primary); font-weight: 600;" :title="proc.command_line">{{ proc.name }}</td>
          <td style="padding: 6px 8px; color: var(--text-secondary);">{{ proc.user }}</td>
          <td style="padding: 6px 8px; text-align: right; color: var(--color-badge-agent-text); font-weight: 700;">{{ proc.cpu_percent.toFixed(1) }}%</td>
          <td style="padding: 6px 8px; text-align: right; color: var(--color-badge-metrics-text);">{{ proc.memory_percent.toFixed(1) }}%</td>
          <td style="padding: 6px 8px; text-align: center;"><span>{{ proc.state }}</span></td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
