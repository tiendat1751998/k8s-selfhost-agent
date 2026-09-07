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
.log-target-tree { display: flex; flex-direction: column; background: rgba(11, 15, 25, 0.85); border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; overflow: hidden; height: 100%; min-height: 520px; }
.tree-header { height: 36px; display: flex; align-items: center; justify-content: space-between; padding: 0 12px; background: rgba(15, 23, 42, 0.8); border-bottom: 1px solid rgba(255, 255, 255, 0.08); }
.tree-title { display: flex; align-items: center; gap: 6px; font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em; color: #94a3b8; }
.tree-badge { font-size: 10px; color: #38bdf8; background: rgba(56, 189, 248, 0.12); padding: 1px 6px; border-radius: 4px; }
.tree-content { padding: 8px; overflow-y: auto; display: flex; flex-direction: column; gap: 8px; }
.tree-item { position: relative; display: flex; align-items: center; gap: 8px; width: 100%; padding: 6px 10px; background: transparent; border: 1px solid transparent; border-radius: 6px; color: #94a3b8; text-align: left; cursor: pointer; transition: all 0.15s ease; }
.tree-item:hover { background: rgba(255, 255, 255, 0.04); color: #f1f5f9; }
.tree-item.active { background: rgba(6, 182, 212, 0.14); border-color: rgba(6, 182, 212, 0.45); color: #38bdf8; }
.tree-item.active .item-name { color: #f8fafc; font-weight: 700; }
.tree-accent-bar { display: none; position: absolute; left: 0; top: 3px; bottom: 3px; width: 3px; background: linear-gradient(180deg, #06b6d4 0%, #10b981 100%); border-radius: 0 2px 2px 0; box-shadow: 0 0 10px rgba(6, 182, 212, 0.8); }
.tree-item.active .tree-accent-bar { display: block; }
.item-icon { font-size: 13px; line-height: 1; }
.item-meta { flex: 1; display: flex; flex-direction: column; min-width: 0; }
.item-name { font-size: 12px; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.item-sub { font-size: 10px; color: #64748b; line-height: 1.1; }
.item-count { font-size: 10px; padding: 1px 5px; background: rgba(255, 255, 255, 0.05); border-radius: 4px; color: #64748b; }
.item-count.has-logs { color: #94a3b8; background: rgba(255, 255, 255, 0.08); }
.tree-item.active .item-count { background: rgba(6, 182, 212, 0.2); color: #38bdf8; border: 1px solid rgba(6, 182, 212, 0.35); }
.tree-section { display: flex; flex-direction: column; gap: 2px; }
.section-toggle { display: flex; align-items: center; gap: 6px; padding: 4px 6px; font-size: 10.5px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.04em; color: #64748b; cursor: pointer; user-select: none; border-radius: 4px; }
.section-toggle:hover { color: #cbd5e1; }
.section-caret { font-size: 10px; display: inline-block; transition: transform 0.15s ease; }
.section-caret.caret-down { transform: rotate(90deg); }
.section-label { flex: 1; }
.section-count { font-size: 9px; background: rgba(255, 255, 255, 0.05); padding: 0 4px; border-radius: 3px; }
.section-items { display: flex; flex-direction: column; gap: 2px; padding-left: 6px; }
</style>
