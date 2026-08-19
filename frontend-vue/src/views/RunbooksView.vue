<template>
  <div class="view-container">
    <!-- View Header -->
    <div class="view-header">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>OPERATIONAL RUNBOOKS & PLAYBOOKS</span>
        </div>
        <h1 class="view-title">Standard Operating Procedures & Execution Catalog</h1>
        <p class="view-desc">
          Structured runbook library with interactive step execution, automated diagnostics commands, and disaster recovery playbooks.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="fetchRunbooks">
          <span>{{ loading ? '⏳ Syncing...' : '🔄 Refresh' }}</span>
        </button>
        <button class="btn btn-primary" @click="showCreateModal = true">
          <span>+ Create Runbook</span>
        </button>
      </div>
    </div>

    <!-- Notification Banner -->
    <div v-if="statusMessage" class="status-banner animate-fade-in" :class="'banner-' + statusMessage.type">
      <span class="banner-icon">{{ statusMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null">✕</button>
    </div>

    <!-- Metrics HUD Grid -->
    <div class="metrics-grid">
      <MetricCard
        title="Cataloged Runbooks"
        :value="runbooks.length"
        badge="AVAILABLE"
        badge-color="cyan"
        subtitle="Standard operating playbooks"
        icon="📖"
      />
      <MetricCard
        title="Operational Categories"
        :value="categoriesCount"
        badge="ORGANIZED"
        badge-color="violet"
        subtitle="Incident, DR, Security & DB"
        icon="🗂️"
      />
      <MetricCard
        title="Total Procedure Steps"
        :value="totalStepsCount"
        badge="STEPS"
        badge-color="emerald"
        subtitle="Automated & verified instructions"
        icon="🪜"
      />
      <MetricCard
        title="Execution Engine"
        value="LIVE CLI"
        badge="READY"
        badge-color="emerald"
        subtitle="One-click diagnostic run"
        icon="⚡"
      />
    </div>

    <!-- Category Filter Bar -->
    <div class="filter-bar glass-panel">
      <div class="filter-group">
        <span class="filter-label">Category:</span>
        <button 
          v-for="cat in categoryList" 
          :key="cat"
          class="filter-pill"
          :class="{ 'filter-active': activeCategory === cat }"
          @click="selectCategory(cat)"
        >
          <span>{{ cat }}</span>
        </button>
      </div>

      <div class="search-box">
        <input 
          v-model="searchQuery" 
          type="text" 
          placeholder="Search runbook title, tags, or steps..." 
          class="input-glass" 
          style="width: 260px;" 
        />
      </div>
    </div>

    <!-- Runbooks Catalog Grid -->
    <div v-if="filteredRunbooks.length > 0" class="runbooks-grid">
      <div 
        v-for="rb in filteredRunbooks" 
        :key="rb.id" 
        class="runbook-card glass-panel glass-panel-glow"
      >
        <div class="rb-header">
          <div class="rb-icon-box">{{ getCategoryIcon(rb.category) }}</div>
          <div class="rb-title-group">
            <span class="rb-category font-mono">{{ rb.category.toUpperCase() }}</span>
            <h3 class="rb-title">{{ rb.title }}</h3>
          </div>
        </div>

        <div class="rb-tags-row">
          <span v-for="tag in (rb.tags || [])" :key="tag" class="tag-pill font-mono">#{{ tag }}</span>
        </div>

        <div class="rb-body">
          <div class="rb-meta-row font-mono">
            <span>Author: <strong class="text-secondary">{{ rb.author || 'SRE Team' }}</strong></span>
            <span>Steps: <strong class="text-cyan">{{ rb.steps_count || 3 }}</strong></span>
          </div>
          <div class="rb-meta-row font-mono text-muted" style="font-size: 10px;">
            <span>Last used: {{ rb.last_used_at ? formatDate(rb.last_used_at) : 'Not recorded' }}</span>
          </div>
        </div>

        <div class="rb-actions">
          <button 
            class="btn btn-primary" 
            :disabled="executingId === rb.id"
            @click="handleExecuteRunbook(rb)"
          >
            <span>{{ executingId === rb.id ? '⚡ Running...' : '⚡ 1-Click Run' }}</span>
          </button>
          <button class="btn btn-secondary" style="flex: 1;" @click="openExecutionModal(rb)">
            <span>Inspect Steps ({{ rb.steps_count }})</span>
          </button>
          <button class="btn btn-secondary btn-sm" title="Delete Runbook" @click="handleDeleteRunbook(rb.id)">
            <span>🗑️</span>
          </button>
        </div>
      </div>
    </div>

    <div v-else class="empty-state-box glass-panel">
      <span class="empty-icon">📖</span>
      <h3 class="empty-title">No Runbooks Discovered</h3>
      <p class="empty-desc">Create your first operational runbook with step-by-step diagnostic and remediation commands.</p>
      <button class="btn btn-primary" @click="showCreateModal = true">
        <span>+ Create Runbook</span>
      </button>
    </div>

    <!-- Interactive Step Execution Modal / Drawer -->
    <div v-if="selectedRunbook" class="modal-overlay" @click.self="selectedRunbook = null">
      <div class="modal-card modal-card-wide glass-panel animate-fade-in">
        <div class="modal-header">
          <div class="modal-title-group">
            <div class="modal-badge-row">
              <span class="badge badge-cyan">{{ selectedRunbook.category.toUpperCase() }}</span>
              <span class="font-mono text-muted" style="font-size: 11px;">Author: {{ selectedRunbook.author }}</span>
            </div>
            <h3 class="modal-title">{{ selectedRunbook.title }}</h3>
          </div>
          <button class="modal-close" @click="selectedRunbook = null">✕</button>
        </div>

        <div class="modal-body">
          <!-- Step Progress Tracker -->
          <div class="execution-hud">
            <div class="hud-step-count font-mono">
              <span>Step Execution Progress:</span>
              <span class="text-cyan font-bold">{{ completedSteps.size }} of {{ parsedSteps.length }} Completed</span>
            </div>
            <div class="hud-progress-bar">
              <div 
                class="hud-progress-fill" 
                :style="{ width: `${parsedSteps.length > 0 ? (completedSteps.size / parsedSteps.length) * 100 : 0}%` }"
              ></div>
            </div>
          </div>

          <!-- Step Items List -->
          <div class="steps-list">
            <div 
              v-for="(step, idx) in parsedSteps" 
              :key="idx" 
              class="step-item"
              :class="{ 'step-completed': completedSteps.has(idx) }"
            >
              <div class="step-header">
                <label class="step-checkbox-label">
                  <input 
                    type="checkbox" 
                    :checked="completedSteps.has(idx)" 
                    @change="toggleStep(idx)" 
                  />
                  <span class="step-num font-mono">Step {{ idx + 1 }}</span>
                </label>
                <span class="step-title font-semibold">{{ step.title }}</span>
                <span v-if="completedSteps.has(idx)" class="badge badge-emerald">COMPLETED</span>
              </div>

              <div class="step-desc font-mono">{{ step.content }}</div>

              <div v-if="step.command" class="step-command-box">
                <div class="command-header">
                  <span class="font-mono text-muted">CLI Command</span>
                  <button class="copy-btn" @click="copyCommand(step.command)">📋 Copy</button>
                </div>
                <pre class="command-pre font-mono">{{ step.command }}</pre>
              </div>
            </div>
          </div>
        </div>

        <div class="modal-footer">
          <button class="btn btn-secondary" @click="selectedRunbook = null">Close</button>
          <button 
            class="btn btn-secondary" 
            :disabled="completedSteps.size === parsedSteps.length"
            @click="markAllStepsComplete"
          >
            <span>{{ completedSteps.size === parsedSteps.length ? '✅ All Steps Verified' : 'Mark All Complete' }}</span>
          </button>
          <button 
            class="btn btn-primary" 
            :disabled="executingId === selectedRunbook.id"
            @click="handleExecuteRunbook(selectedRunbook); markAllStepsComplete()"
          >
            <span>{{ executingId === selectedRunbook.id ? '⚡ Executing Runbook...' : '⚡ 1-Click Dispatch Playbook' }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Modal: Create Runbook -->
    <div v-if="showCreateModal" class="modal-overlay" @click.self="showCreateModal = false">
      <div class="modal-card glass-panel animate-fade-in">
        <div class="modal-header">
          <div class="modal-title-group">
            <span class="badge badge-cyan">NEW STANDARD OPERATING PROCEDURE</span>
            <h3 class="modal-title">Create Operational Runbook</h3>
          </div>
          <button class="modal-close" @click="showCreateModal = false">✕</button>
        </div>

        <form class="modal-body" @submit.prevent="handleCreateRunbook">
          <div class="form-group">
            <label class="form-label">Runbook Title:</label>
            <input v-model="newRunbook.title" type="text" required class="input-glass" placeholder="e.g. PostgreSQL High Memory & Slow Query Remediation" />
          </div>

          <div class="form-group-row">
            <div class="form-group" style="flex: 1;">
              <label class="form-label">Category:</label>
              <select v-model="newRunbook.category" class="input-glass">
                <option value="Incident Response">Incident Response</option>
                <option value="Disaster Recovery">Disaster Recovery</option>
                <option value="Database Ops">Database Ops</option>
                <option value="Security Remediation">Security Remediation</option>
                <option value="Networking">Networking</option>
                <option value="Capacity Scaling">Capacity Scaling</option>
              </select>
            </div>

            <div class="form-group" style="flex: 1;">
              <label class="form-label">Author / Team:</label>
              <input v-model="newRunbook.author" type="text" required class="input-glass" placeholder="SRE Platform" />
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">Tags (comma-separated):</label>
            <input v-model="tagInput" type="text" class="input-glass font-mono" placeholder="postgres, high-memory, k8s, p1" />
          </div>

          <div class="form-group">
            <label class="form-label">Runbook Steps (Markdown or Structured Text):</label>
            <textarea 
              v-model="newRunbook.content" 
              rows="6" 
              required 
              class="input-glass font-mono"
              placeholder="1. Check active database connections:&#10;`kubectl exec -it pg-0 -- psql -c 'SELECT count(*) FROM pg_stat_activity;'`&#10;2. Analyze top memory consumer pods&#10;3. Trigger rolling restart if memory leaks confirmed"
            ></textarea>
          </div>

          <div class="modal-footer" style="padding: 16px 0 0 0; background: transparent; border-top: none;">
            <button type="button" class="btn btn-secondary" @click="showCreateModal = false">Cancel</button>
            <button type="submit" class="btn btn-primary" :disabled="loading">
              <span>{{ loading ? 'Saving...' : 'Publish Runbook' }}</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, reactive } from 'vue'
import { runbookApi, type Runbook } from '../api/governance'
import MetricCard from '../components/ui/MetricCard.vue'

const runbooks = ref<Runbook[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const statusMessage = ref<{ type: 'success' | 'error'; text: string } | null>(null)
const activeCategory = ref<string>('ALL')
const searchQuery = ref('')
const selectedRunbook = ref<Runbook | null>(null)
const showCreateModal = ref(false)
const tagInput = ref('database, postgres, recovery')
const completedSteps = ref<Set<number>>(new Set())

const newRunbook = reactive({
  title: '',
  category: 'Incident Response',
  content: '1. Verify target workload status:\n`kubectl get pods -A -l app=postgres`\n2. Inspect container events and crash logs:\n`kubectl describe pod postgres-0`\n3. Execute emergency failover if primary unreachable.',
  tags: [] as string[],
  author: 'Platform SRE',
  steps_count: 3,
})

onMounted(() => {
  fetchRunbooks()
})

async function fetchRunbooks() {
  loading.value = true
  error.value = null
  try {
    const res = await runbookApi.getRunbooks(activeCategory.value === 'ALL' ? undefined : activeCategory.value)
    runbooks.value = res.data || []
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to load runbooks'
    error.value = msg
    runbooks.value = []
  } finally {
    loading.value = false
  }
}

function selectCategory(cat: string) {
  activeCategory.value = cat
  fetchRunbooks()
}

const categoryList = computed(() => {
  const cats = new Set<string>(['ALL'])
  runbooks.value.forEach(r => {
    if (r.category) cats.add(r.category)
  })
  return Array.from(cats)
})

const categoriesCount = computed(() => categoryList.value.length - 1)
const totalStepsCount = computed(() => {
  return runbooks.value.reduce((acc, r) => acc + (r.steps_count || 3), 0)
})

const filteredRunbooks = computed(() => {
  return runbooks.value.filter(r => {
    if (activeCategory.value !== 'ALL' && r.category !== activeCategory.value) {
      return false
    }
    if (searchQuery.value) {
      const q = searchQuery.value.toLowerCase()
      const matchTitle = r.title.toLowerCase().includes(q)
      const matchCat = r.category.toLowerCase().includes(q)
      const matchTags = (r.tags || []).some(t => t.toLowerCase().includes(q))
      return matchTitle || matchCat || matchTags
    }
    return true
  })
})

function getCategoryIcon(cat: string): string {
  const c = (cat || '').toLowerCase()
  if (c.includes('disaster') || c.includes('dr')) return '⚡'
  if (c.includes('incident')) return '🚨'
  if (c.includes('security')) return '🔐'
  if (c.includes('database') || c.includes('db')) return '🐘'
  if (c.includes('network')) return '🌐'
  return '📖'
}

interface ParsedStep {
  title: string
  content: string
  command?: string
}

const parsedSteps = computed<ParsedStep[]>(() => {
  if (!selectedRunbook.value || !selectedRunbook.value.content) return []
  const lines = selectedRunbook.value.content.split('\n')
  const steps: ParsedStep[] = []
  let currentStep: ParsedStep | null = null

  for (const line of lines) {
    const trimmed = line.trim()
    if (/^\d+\./.test(trimmed) || trimmed.startsWith('Step ')) {
      if (currentStep) steps.push(currentStep)
      currentStep = {
        title: trimmed.replace(/^\d+\.\s*/, '').replace(/^Step\s*\d+:\s*/, ''),
        content: '',
      }
    } else if (trimmed.startsWith('`') && trimmed.endsWith('`') && trimmed.length > 2) {
      const cmd = trimmed.slice(1, -1)
      if (currentStep) {
        currentStep.command = cmd
      }
    } else if (trimmed) {
      if (currentStep) {
        currentStep.content += (currentStep.content ? '\n' : '') + trimmed
      } else {
        currentStep = { title: 'Overview Procedure', content: trimmed }
      }
    }
  }
  if (currentStep) steps.push(currentStep)
  return steps.length > 0 ? steps : [{ title: selectedRunbook.value.title, content: selectedRunbook.value.content }]
})

const executingId = ref<string | null>(null)

function openExecutionModal(rb: Runbook) {
  selectedRunbook.value = rb
  completedSteps.value = new Set()
}

function toggleStep(idx: number) {
  if (completedSteps.value.has(idx)) {
    completedSteps.value.delete(idx)
  } else {
    completedSteps.value.add(idx)
  }
}

async function handleExecuteRunbook(rb: Runbook) {
  executingId.value = rb.id
  statusMessage.value = null
  try {
    const res = await runbookApi.executeRunbook(rb.id)
    statusMessage.value = {
      type: 'success',
      text: res.message || `Runbook "${rb.title}" execution completed successfully.`,
    }
    rb.last_used_at = res.executed_at || new Date().toISOString()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to execute runbook'
    statusMessage.value = { type: 'error', text: msg }
  } finally {
    executingId.value = null
  }
}

function markAllStepsComplete() {
  parsedSteps.value.forEach((_, idx) => completedSteps.value.add(idx))
  statusMessage.value = {
    type: 'success',
    text: `Runbook "${selectedRunbook.value?.title}" execution completed successfully.`,
  }
}

function copyCommand(cmd?: string) {
  if (!cmd) return
  navigator.clipboard.writeText(cmd)
  statusMessage.value = { type: 'success', text: 'Command copied to clipboard!' }
}

async function handleCreateRunbook() {
  loading.value = true
  statusMessage.value = null
  try {
    newRunbook.tags = tagInput.value.split(',').map(t => t.trim()).filter(Boolean)
    await runbookApi.createRunbook(newRunbook)
    statusMessage.value = {
      type: 'success',
      text: `Runbook "${newRunbook.title}" published successfully.`,
    }
    showCreateModal.value = false
    await fetchRunbooks()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to create runbook'
    statusMessage.value = { type: 'error', text: msg }
  } finally {
    loading.value = false
  }
}

async function handleDeleteRunbook(id: string) {
  statusMessage.value = null
  try {
    await runbookApi.deleteRunbook(id)
    statusMessage.value = { type: 'success', text: `Runbook #${id.slice(0, 8)} deleted.` }
    runbooks.value = runbooks.value.filter(r => r.id !== id)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to delete runbook'
    statusMessage.value = { type: 'error', text: msg }
  }
}

function formatDate(d: string): string {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleDateString()
  } catch {
    return d
  }
}
</script>

<style scoped>
.view-container {
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.view-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 16px;
}

.view-tag {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  font-weight: 700;
  color: var(--accent-cyan);
  letter-spacing: 0.05em;
  margin-bottom: 6px;
}

.view-title {
  font-size: 24px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
}

.view-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 820px;
  margin-top: 4px;
}

.highlight {
  color: #fff;
  font-weight: 600;
}

.header-actions {
  display: flex;
  gap: 12px;
}

.status-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 18px;
  border-radius: 12px;
  font-size: 13px;
}

.banner-success {
  background: rgba(16, 185, 129, 0.12);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
}

.banner-error {
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fda4af;
}

.banner-icon {
  font-size: 16px;
}

.banner-text {
  flex: 1;
  font-weight: 500;
}

.banner-close {
  background: none;
  border: none;
  color: inherit;
  font-size: 14px;
  cursor: pointer;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
}

.filter-bar {
  padding: 12px 18px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 14px;
}

.filter-group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.filter-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
}

.filter-pill {
  padding: 4px 12px;
  border-radius: 9999px;
  font-size: 11px;
  font-weight: 700;
  cursor: pointer;
  border: 1px solid transparent;
  background: rgba(255, 255, 255, 0.05);
  color: var(--text-secondary);
  transition: all 0.15s ease;
}

.filter-pill:hover, .filter-active {
  background: rgba(6, 182, 212, 0.2);
  border-color: var(--accent-sky);
  color: #fff;
}

.runbooks-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}

.runbook-card {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.rb-header {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.rb-icon-box {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
}

.rb-title-group {
  flex: 1;
  min-width: 0;
}

.rb-category {
  font-size: 10px;
  font-weight: 700;
  color: var(--accent-cyan);
  letter-spacing: 0.05em;
}

.rb-title {
  font-size: 15px;
  font-weight: 700;
  color: #fff;
  line-height: 1.3;
  margin-top: 2px;
}

.rb-tags-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.tag-pill {
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.06);
  color: var(--text-muted);
}

.rb-body {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  background: rgba(0, 0, 0, 0.25);
  padding: 10px 12px;
  border-radius: 8px;
}

.rb-meta-row {
  display: flex;
  justify-content: space-between;
}

.rb-actions {
  display: flex;
  gap: 8px;
  margin-top: auto;
}

.empty-state-box {
  padding: 48px 24px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.empty-icon {
  font-size: 36px;
}

.empty-title {
  font-size: 18px;
  color: #fff;
}

.empty-desc {
  font-size: 13px;
  color: var(--text-muted);
  max-width: 460px;
  margin-bottom: 8px;
}

.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.75);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  padding: 20px;
}

.modal-card-wide {
  width: 100%;
  max-width: 820px;
  background: var(--bg-sidebar);
  border: 1px solid var(--border-medium);
  border-radius: 18px;
  overflow: hidden;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.7);
  max-height: 90vh;
  display: flex;
  flex-direction: column;
}

.modal-header {
  padding: 18px 24px;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  border-bottom: 1px solid var(--border-subtle);
}

.modal-badge-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.modal-title {
  font-size: 18px;
  color: #fff;
}

.modal-close {
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 18px;
  cursor: pointer;
}

.modal-body {
  padding: 20px 24px;
  display: flex;
  flex-direction: column;
  gap: 18px;
  overflow-y: auto;
}

.execution-hud {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 14px;
  background: rgba(0, 0, 0, 0.3);
  border-radius: 10px;
}

.hud-step-count {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
}

.hud-progress-bar {
  width: 100%;
  height: 6px;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 9999px;
  overflow: hidden;
}

.hud-progress-fill {
  height: 100%;
  background: var(--grad-cyan);
  transition: width 0.25s ease;
}

.steps-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.step-item {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  transition: all 0.15s ease;
}

.step-completed {
  border-color: rgba(16, 185, 129, 0.4);
  background: rgba(16, 185, 129, 0.03);
}

.step-header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.step-checkbox-label {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
}

.step-num {
  font-size: 11px;
  font-weight: 700;
  color: var(--accent-cyan);
}

.step-title {
  flex: 1;
  font-size: 13px;
  color: #fff;
}

.step-desc {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.5;
  white-space: pre-wrap;
}

.step-command-box {
  background: rgba(0, 0, 0, 0.4);
  border: 1px solid var(--border-medium);
  border-radius: 8px;
  overflow: hidden;
}

.command-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 12px;
  background: rgba(255, 255, 255, 0.04);
  border-bottom: 1px solid var(--border-subtle);
  font-size: 11px;
}

.copy-btn {
  background: none;
  border: none;
  color: var(--accent-sky);
  cursor: pointer;
  font-size: 11px;
  font-weight: 600;
}

.command-pre {
  padding: 10px 12px;
  font-size: 11px;
  color: #38bdf8;
  overflow-x: auto;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group-row {
  display: flex;
  gap: 12px;
}

.form-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.modal-footer {
  padding: 16px 24px;
  background: rgba(0, 0, 0, 0.3);
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  border-top: 1px solid var(--border-subtle);
}
</style>
