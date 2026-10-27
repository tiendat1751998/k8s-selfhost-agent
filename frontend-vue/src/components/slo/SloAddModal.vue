<script setup lang="ts">
import SloCreateModal from './SloCreateModal.vue'
import type { CreateSLOPayload } from '../../api/compute'

interface ServiceOption {
  id: string
  name: string
  desc: string
}

defineProps<{
  show: boolean
  realServices: ServiceOption[]
  actionInProgress?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'create', payload: CreateSLOPayload): void
}>()
</script>

<template>
  <SloCreateModal
    :show="show"
    :real-services="realServices"
    :action-in-progress="actionInProgress"
    @update:show="(val) => emit('update:show', val)"
    @create="(payload) => emit('create', payload)"
  />
</template>
