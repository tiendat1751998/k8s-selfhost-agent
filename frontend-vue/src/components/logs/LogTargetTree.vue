<script setup lang="ts">
import { ref } from 'vue'
import type { LogEntry } from '../../stores/logStore'

export interface LogTarget {
  type: 'all' | 'node' | 'service'
  id: string
  name: string
  icon?: string
}

const props = defineProps<{ modelValue: LogTarget; logs: LogEntry[] }>()
const emit = defineEmits<{ (e: 'update:modelValue', target: LogTarget): void; (e: 'select', target: LogTarget): void }>()

const hostNodes = [
  { id: 'k8smater', name: 'k8smater', icon: '👑', role: 'Control Plane' },
  { id: 'worker1', name: 'worker1', icon: '🖥️', role: 'Worker Node' },
  { id: 'worker2', name: 'worker2', icon: '🖥️', role: 'Worker Node' },
  { id: 'k8sworker3', name: 'k8sworker3', icon: '🖥️', role: 'Worker Node' },
  { id: 'masterdb', name: 'masterdb', icon: '🗄️', role: 'Primary DB' },
  { id: 'workerdb1', name: 'workerdb1', icon: '🗄️', role: 'Replica DB' },
]

const services = [
  { id: 'traefik', name: 'traefik', icon: '🚦', type: 'Ingress Proxy' },
  { id: 'postgres', name: 'postgres', icon: '🐘', type: 'Stateful DB' },
  { id: 'nats', name: 'nats', icon: '⚡', type: 'Message Broker' },
  { id: 'k8s-agent', name: 'k8s-agent', icon: '🛰️', type: 'Cluster Agent' },
  { id: 'standalone', name: 'standalone', icon: '⚙️', type: 'Core Daemon' },
  { id: 'docker', name: 'docker', icon: '🐳', type: 'Container Engine' },
]

const nodesExpanded = ref(true)
const servicesExpanded = ref(true)

function selectTarget(target: LogTarget) {
  emit('update:modelValue', target)
  emit('select', target)
}

function getNodeCount(nodeId: string): number {
  const q = nodeId.toLowerCase()
  return props.logs.filter(l => (l.node && l.node.toLowerCase().includes(q)) || (l.pod && l.pod.toLowerCase().includes(q)) || (l.namespace && l.namespace.toLowerCase().includes(q))).length
}

function getServiceCount(serviceId: string): number {
  const q = serviceId.toLowerCase()
  return props.logs.filter(l => (l.service && l.service.toLowerCase().includes(q)) || (l.container && l.container.toLowerCase().includes(q)) || (l.pod && l.pod.toLowerCase().includes(q)) || (l.namespace && l.namespace.toLowerCase().includes(q))).length
}
</script>

<template>
  <aside class="log-target-tree glass-panel" aria-label="Log Stream Target Hierarchy">
    <div class="tree-header">
      <div class="tree-title"><span class="tree-icon">🌲</span><span>Log Targets</span></div>
      <span class="tree-badge font-mono">{{ logs.length }} logs</span>
    </div>

    <div class="tree-content">
      <!-- All Cluster Logs -->
      <button
        type="button"
        role="treeitem"
        :aria-selected="modelValue.type === 'all'"
        class="tree-item tree-root-item"
        :class="{ active: modelValue.type === 'all' }"
        @click="selectTarget({ type: 'all', id: 'all', name: 'All Cluster Logs', icon: '🌐' })"
      >
        <span class="tree-accent-bar"></span>
        <span class="item-icon">🌐</span>
        <div class="item-meta">
          <span class="item-name">All Cluster Logs</span>
          <span class="item-sub font-mono">Unified aggregator</span>
        </div>
        <span class="item-count font-mono" :class="{ 'has-logs': logs.length > 0 }">{{ logs.length }}</span>
      </button>

      <!-- Host Nodes Section -->
      <div class="tree-section">
        <div class="section-toggle" @click="nodesExpanded = !nodesExpanded">
          <span class="section-caret" :class="{ 'caret-down': nodesExpanded }">▸</span>
          <span class="section-label">Host Nodes</span>
          <span class="section-count font-mono">{{ hostNodes.length }}</span>
        </div>
        <div v-show="nodesExpanded" class="section-items">
          <button
            v-for="node in hostNodes"
            :key="node.id"
            type="button"
            role="treeitem"
            :aria-selected="modelValue.type === 'node' && modelValue.id === node.id"
            class="tree-item"
            :class="{ active: modelValue.type === 'node' && modelValue.id === node.id }"
            @click="selectTarget({ type: 'node', id: node.id, name: node.name, icon: node.icon })"
          >
            <span class="tree-accent-bar"></span>
            <span class="item-icon">{{ node.icon }}</span>
            <div class="item-meta">
              <span class="item-name font-mono">{{ node.name }}</span>
              <span class="item-sub font-mono">{{ node.role }}</span>
            </div>
            <span class="item-count font-mono" :class="{ 'has-logs': getNodeCount(node.id) > 0 }">{{ getNodeCount(node.id) }}</span>
          </button>
        </div>
      </div>

      <!-- Services & Containers Section -->
      <div class="tree-section">
        <div class="section-toggle" @click="servicesExpanded = !servicesExpanded">
          <span class="section-caret" :class="{ 'caret-down': servicesExpanded }">▸</span>
          <span class="section-label">Services & Containers</span>
          <span class="section-count font-mono">{{ services.length }}</span>
        </div>
        <div v-show="servicesExpanded" class="section-items">
          <button
            v-for="svc in services"
            :key="svc.id"
            type="button"
            role="treeitem"
            :aria-selected="modelValue.type === 'service' && modelValue.id === svc.id"
            class="tree-item"
            :class="{ active: modelValue.type === 'service' && modelValue.id === svc.id }"
            @click="selectTarget({ type: 'service', id: svc.id, name: svc.name, icon: svc.icon })"
          >
            <span class="tree-accent-bar"></span>
            <span class="item-icon">{{ svc.icon }}</span>
            <div class="item-meta">
              <span class="item-name font-mono">{{ svc.name }}</span>
              <span class="item-sub font-mono">{{ svc.type }}</span>
            </div>
            <span class="item-count font-mono" :class="{ 'has-logs': getServiceCount(svc.id) > 0 }">{{ getServiceCount(svc.id) }}</span>
          </button>
        </div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
@import '../../assets/styles/views/logstream.css';
</style>
