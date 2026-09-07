<script setup lang="ts">
import YamlEditorModal from '../k8s/YamlEditorModal.vue'

withDefaults(
  defineProps<{
    show: boolean
    cluster: string
    namespace?: string
    initialYaml?: string
    title?: string
    mode?: 'create' | 'edit'
  }>(),
  {
    namespace: 'default',
    initialYaml: '',
    title: 'Apply Kubernetes Manifest (YAML)',
    mode: 'create'
  }
)

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'applied', result: { message: string }): void
}>()
</script>

<template>
  <YamlEditorModal
    :show="show"
    :cluster="cluster"
    :namespace="namespace"
    :initial-yaml="initialYaml"
    :title="title"
    :mode="mode"
    @close="emit('close')"
    @applied="(res) => emit('applied', res)"
  />
</template>
