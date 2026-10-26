<script setup lang="ts">
import HostBladeRackGrid from './HostBladeRackGrid.vue'
import type { ComputeHost } from '../../api/compute'
import type { HostTestResult, HostTypeDefinition, HostHardwareMetrics } from '../../types/hosts'

defineProps<{
  hosts: ComputeHost[]
  hostHardwareMap?: Record<string, HostHardwareMetrics>
  hostTestResults: Record<string, HostTestResult>
  testingHostId: string | null
  getHostTypeMeta: (type?: string) => HostTypeDefinition
  formatDate: (d?: string) => string
  formatUptime: (seconds?: number) => string
}>()

const emit = defineEmits<{
  (e: 'test', host: ComputeHost): void
  (e: 'edit', host: ComputeHost): void
  (e: 'delete', host: ComputeHost): void
  (e: 'select', host: ComputeHost): void
  (e: 'copy', text: string): void
}>()
</script>

<template>
  <HostBladeRackGrid
    :hosts="hosts"
    :host-hardware-map="hostHardwareMap"
    :host-test-results="hostTestResults"
    :testing-host-id="testingHostId"
    :get-host-type-meta="getHostTypeMeta"
    :format-date="formatDate"
    :format-uptime="formatUptime"
    @test="emit('test', $event)"
    @edit="emit('edit', $event)"
    @delete="emit('delete', $event)"
    @select="emit('select', $event)"
    @copy="emit('copy', $event)"
  />
</template>
