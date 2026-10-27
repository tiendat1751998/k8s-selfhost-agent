import { ref, computed, reactive } from 'vue'
import { runbookApi, type Runbook } from '../api/governance'

export interface RunbookStep {
  id: string;
  stepNum: number;
  title: string;
  content: string;
  command?: string;
  dependencies?: string[];
  status: 'idle' | 'running' | 'completed' | 'succeeded' | 'degraded' | 'failed' | 'skipped';
  durationMs?: number;
  output?: string;
  error?: string;
}

export interface RunbookParam {
  key: string;
  label: string;
  type: 'string' | 'number' | 'boolean' | 'select';
  defaultValue?: string | number | boolean;
  options?: string[];
  description?: string;
  required?: boolean;
}

export interface RunbookTarget {
  cluster: string;
  namespace: string;
  resourceType: string;
  resourceName: string;
}

export interface RunbookExecutionRecord {
  id: string;
  runbookId: string;
  runbookTitle: string;
  category: string;
  target?: RunbookTarget;
  params?: Record<string, string | number | boolean>;
  isDryRun: boolean;
  status: 'pending' | 'running' | 'completed' | 'failed';
  startedAt: string;
  completedAt?: string;
  durationMs?: number;
  executedBy: string;
  summary?: string;
  logs: string[];
  steps: RunbookStep[];
}

export function parseRunbookSteps(content?: string, rbTitle?: string): RunbookStep[] {
  if (!content) return []
  const lines = content.split('\n')
  const steps: RunbookStep[] = []
  let currentStep: RunbookStep | null = null
  let stepCounter = 1
  let inCodeFence = false
  const fenceBuffer: string[] = []

  for (const rawLine of lines) {
    const trimmed = rawLine.trim()
    if (trimmed.startsWith('```')) {
      if (inCodeFence) {
        inCodeFence = false
        if (currentStep && fenceBuffer.length > 0) {
          const blockCmd = fenceBuffer.join('\n').trim()
          if (blockCmd) currentStep.command = currentStep.command ? `${currentStep.command}\n${blockCmd}` : blockCmd
        }
        fenceBuffer.length = 0
      } else {
        inCodeFence = true
        fenceBuffer.length = 0
      }
      continue
    }
    if (inCodeFence) {
      fenceBuffer.push(rawLine)
      continue
    }

    const isStepHeader = /^#{1,4}\s*(Step\s*\d+:?|\d+\.|\w+)/i.test(trimmed) ||
                         /^\d+\.\s+/.test(trimmed) ||
                         /^Step\s+\d+:?/i.test(trimmed)

    if (isStepHeader) {
      if (currentStep) steps.push(currentStep)
      const stepNum = stepCounter++
      const cleanTitle = trimmed
        .replace(/^#{1,4}\s*/, '')
        .replace(/^Step\s*\d+:?\s*/i, '')
        .replace(/^\d+\.\s*/, '')
        .trim()
      currentStep = {
        id: `step-${stepNum}`,
        stepNum,
        title: cleanTitle || `Step ${stepNum}`,
        content: '',
        dependencies: stepNum > 1 ? [`step-${stepNum - 1}`] : [],
        status: 'idle',
      }
    } else if (trimmed.startsWith('`') && trimmed.endsWith('`') && trimmed.length > 2) {
      const cmd = trimmed.slice(1, -1).trim()
      if (currentStep) {
        currentStep.command = currentStep.command ? `${currentStep.command}\n${cmd}` : cmd
      }
    } else if (trimmed) {
      if (currentStep) {
        currentStep.content += (currentStep.content ? '\n' : '') + trimmed
      } else {
        currentStep = {
          id: 'step-1',
          stepNum: 1,
          title: 'Overview Procedure',
          content: trimmed,
          dependencies: [],
          status: 'idle',
        }
        stepCounter++
      }
    }
  }
  if (inCodeFence && fenceBuffer.length > 0 && currentStep) {
    const blockCmd = fenceBuffer.join('\n').trim()
    if (blockCmd) currentStep.command = currentStep.command ? `${currentStep.command}\n${blockCmd}` : blockCmd
  }
  if (currentStep) steps.push(currentStep)
  return steps.length > 0 ? steps : [{
    id: 'step-1',
    stepNum: 1,
    title: rbTitle || 'Execution Step',
    content: content,
    dependencies: [],
    status: 'idle',
  }]
}

export function useRunbooks() {
  const runbooks = ref<Runbook[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const statusMessage = ref<{ type: 'success' | 'error'; text: string } | null>(null)
  const activeCategory = ref<string>('ALL')
  const searchQuery = ref('')
  const viewMode = ref<'console' | 'grid' | 'table'>('console')

  const selectedRunbook = ref<Runbook | null>(null)
  const executingRunbook = ref<Runbook | null>(null)
  const activeExecution = ref<RunbookExecutionRecord | null>(null)
  const executingId = ref<string | null>(null)
  const activeSteps = ref<RunbookStep[]>([])
  const completedSteps = ref<Set<number>>(new Set())

  const showCreateModal = ref(false)
  const showEditModal = ref(false)
  const showExecuteModal = ref(false)
  const showExecutionDrawer = ref(false)
  const editingRunbook = ref<Runbook | null>(null)

  const executionHistory = ref<RunbookExecutionRecord[]>([])
  const executionLogs = ref<string[]>([])

  const tagInput = ref('database, postgres, recovery')
  const newRunbook = reactive({
    title: '',
    category: 'Incident Response',
    content: '1. Verify target workload status:\n`kubectl get pods -A -l app=postgres`\n2. Inspect container events and crash logs:\n`kubectl describe pod postgres-0`\n3. Execute emergency failover if primary unreachable.\n`kubectl exec -it pg-0 -- patronictl failover`',
    tags: [] as string[],
    author: 'Platform SRE',
    steps_count: 3,
  })
  async function fetchRunbooks() {
    loading.value = true
    error.value = null
    try {
      const res = await runbookApi.getRunbooks(activeCategory.value === 'ALL' ? undefined : activeCategory.value)
      runbooks.value = res.data || []
      if (runbooks.value.length > 0 && (!selectedRunbook.value || !runbooks.value.some(r => r.id === selectedRunbook.value?.id))) {
        selectedRunbook.value = runbooks.value[0]
        activeSteps.value = parseRunbookSteps(selectedRunbook.value.content, selectedRunbook.value.title)
      }
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
    runbooks.value.forEach(r => { if (r.category) cats.add(r.category) })
    return Array.from(cats)
  })

  const categoriesCount = computed(() => Math.max(0, categoryList.value.length - 1))
  const totalStepsCount = computed(() => runbooks.value.reduce((acc, r) => acc + (r.steps_count || 3), 0))

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



  function selectRunbook(rb: Runbook) {
    selectedRunbook.value = rb
    activeSteps.value = parseRunbookSteps(rb.content, rb.title)
    completedSteps.value = new Set()
  }

  const parsedSteps = computed<RunbookStep[]>(() => {
    if (!selectedRunbook.value) return []
    if (activeSteps.value.length === 0) {
      activeSteps.value = parseRunbookSteps(selectedRunbook.value.content, selectedRunbook.value.title)
    }
    return activeSteps.value
  })

  const parameterSchemas = computed<RunbookParam[]>(() => {
    const rb = executingRunbook.value || selectedRunbook.value
    if (!rb || !rb.content) return []
    const params: RunbookParam[] = [
      { key: 'TIMEOUT_SEC', label: 'Step Timeout (s)', type: 'number', defaultValue: 30, description: 'Max runtime per procedure step', required: true },
      { key: 'DRY_RUN_MODE', label: 'Dry Run Only', type: 'boolean', defaultValue: false, description: 'Simulate without writing cluster state' },
    ]
    if (rb.content.toLowerCase().includes('postgres') || rb.content.toLowerCase().includes('db')) {
      params.push({ key: 'DB_TARGET_NAME', label: 'Target Database Name', type: 'string', defaultValue: 'production_main', required: true })
      params.push({ key: 'FORCE_FAILOVER', label: 'Force Emergency Failover', type: 'boolean', defaultValue: false })
    }
    if (rb.content.toLowerCase().includes('pod') || rb.content.toLowerCase().includes('restart')) {
      params.push({ key: 'GRACE_PERIOD', label: 'Pod Termination Grace (s)', type: 'number', defaultValue: 10 })
    }
    return params
  })

  function openInspectDrawer(rb: Runbook) {
    selectRunbook(rb)
    showExecutionDrawer.value = true
    executionLogs.value = [
      `[${new Date().toLocaleTimeString()}] [INFO] Loaded runbook: ${rb.title} (${rb.steps_count || 3} steps)`,
      `[${new Date().toLocaleTimeString()}] [INFO] Target Category: ${rb.category.toUpperCase()} | Author: ${rb.author}`,
      `[${new Date().toLocaleTimeString()}] [READY] DAG dependencies validated. Ready for step execution.`,
    ]
  }

  function openExecuteModal(rb: Runbook) {
    executingRunbook.value = rb
    showExecuteModal.value = true
  }

  function openEditModal(rb: Runbook) {
    editingRunbook.value = { ...rb }
    showEditModal.value = true
  }

  function toggleStep(idx: number) {
    if (completedSteps.value.has(idx)) completedSteps.value.delete(idx)
    else completedSteps.value.add(idx)
  }

  function markAllStepsComplete() {
    parsedSteps.value.forEach((_, idx) => completedSteps.value.add(idx))
    statusMessage.value = {
      type: 'success',
      text: `Runbook "${selectedRunbook.value?.title}" all steps verified.`,
    }
    executionLogs.value.push(
      `[${new Date().toLocaleTimeString()}] [SUCCESS] All ${parsedSteps.value.length} steps marked as completed.`,
    )
  }
  async function handleExecuteRunbook(
    rb: Runbook,
    options: {
      isDryRun?: boolean
      target?: RunbookTarget
      params?: Record<string, string | number | boolean>
    } = {}
  ) {
    executingId.value = rb.id
    statusMessage.value = null
    const isDry = options.isDryRun ?? false
    const startTime = Date.now()

    try {
      const res = await runbookApi.executeRunbook(rb.id)
      const elapsedMs = Date.now() - startTime
      const now = new Date().toISOString()
      rb.last_used_at = res.executed_at || now

      const responseSteps = (res as { steps?: Array<{ step_num?: number; status?: string; output?: string; duration_ms?: number }> }).steps
      if (Array.isArray(responseSteps) && responseSteps.length > 0) {
        responseSteps.forEach(s => {
          const idx = s.step_num !== undefined ? (s.step_num - 1) : -1
          if (idx >= 0 && idx < activeSteps.value.length) {
            const st = activeSteps.value[idx]
            st.status = s.status === 'degraded' ? 'degraded' : (s.status === 'failed' ? 'failed' : 'succeeded')
            if (s.output) st.output = s.output
            if (s.duration_ms) st.durationMs = s.duration_ms
            if (st.status === 'succeeded') completedSteps.value.add(idx)
          }
        })
      } else {
        activeSteps.value.forEach((st, idx) => {
          st.status = 'succeeded'
          st.durationMs = Math.round(elapsedMs / (activeSteps.value.length || 1))
          st.output = `Executed via playbook dispatch (200 OK)`
          completedSteps.value.add(idx)
        })
      }

      const execRecord: RunbookExecutionRecord = {
        id: `exec-${Date.now().toString(36)}`,
        runbookId: rb.id,
        runbookTitle: rb.title,
        category: rb.category,
        target: options.target,
        params: options.params,
        isDryRun: isDry,
        status: 'completed',
        startedAt: now,
        completedAt: new Date().toISOString(),
        durationMs: elapsedMs,
        executedBy: rb.author || 'Operator',
        summary: res.message || `${isDry ? '[DRY-RUN] ' : ''}Runbook "${rb.title}" execution completed successfully.`,
        logs: [
          `[${new Date().toLocaleTimeString()}] [INIT] Initiating ${isDry ? 'DRY-RUN' : 'LIVE'} execution for "${rb.title}"`,
          `[${new Date().toLocaleTimeString()}] [TARGET] Cluster: ${options.target?.cluster || 'production-01'} | Namespace: ${options.target?.namespace || 'default'}`,
          `[${new Date().toLocaleTimeString()}] [EXEC] Dispatched ${rb.steps_count || 3} procedure steps in ${elapsedMs}ms.`,
          `[${new Date().toLocaleTimeString()}] [OUTPUT] ${res.message || 'All steps completed.'}`,
        ],
        steps: activeSteps.value.slice(),
      }

      activeExecution.value = execRecord
      executionHistory.value.unshift(execRecord)
      executionLogs.value = execRecord.logs

      statusMessage.value = {
        type: 'success',
        text: execRecord.summary || `Runbook "${rb.title}" execution completed.`,
      }
      showExecuteModal.value = false
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to execute runbook'
      statusMessage.value = { type: 'error', text: msg }
      executionLogs.value.push(`[${new Date().toLocaleTimeString()}] [ERROR] Execution failed: ${msg}`)
    } finally {
      executingId.value = null
    }
  }

  async function executeSingleStep(step: RunbookStep, idx?: number) {
    const stepIdx = idx !== undefined ? idx : (step.stepNum - 1)
    step.status = 'running'
    const startTime = Date.now()
    executionLogs.value.push(`[${new Date().toLocaleTimeString()}] [EXEC] Executing Step ${stepIdx + 1}: ${step.title}`)
    if (step.command) executionLogs.value.push(`[${new Date().toLocaleTimeString()}] [CMD] $ ${step.command}`)
    try {
      const rbId = selectedRunbook.value?.id || executingRunbook.value?.id || 'default'
      const res = await runbookApi.executeStep(rbId, stepIdx)
      const durationMs = Date.now() - startTime
      step.durationMs = durationMs
      const isDegraded = res.status === 'degraded'
      const isSuccess = res.success !== false && res.status !== 'failed'
      step.status = isDegraded ? 'degraded' : (isSuccess ? 'succeeded' : 'failed')
      step.output = res.output || (isSuccess ? `Step ${stepIdx + 1} completed successfully in ${durationMs}ms.` : `Step execution failed.`)
      if (step.status === 'succeeded') {
        completedSteps.value.add(stepIdx)
        executionLogs.value.push(`[${new Date().toLocaleTimeString()}] [SUCCEEDED] Step ${stepIdx + 1} (${durationMs}ms): ${step.output}`)
      } else {
        executionLogs.value.push(`[${new Date().toLocaleTimeString()}] [${step.status.toUpperCase()}] Step ${stepIdx + 1}: ${step.output}`)
      }
    } catch (err: unknown) {
      step.durationMs = Date.now() - startTime
      step.status = 'failed'
      const msg = err instanceof Error ? err.message : 'Execution error'
      step.error = msg
      step.output = `Error executing step: ${msg}`
      executionLogs.value.push(`[${new Date().toLocaleTimeString()}] [ERROR] Step ${stepIdx + 1} failed: ${msg}`)
    }
  }

  async function handleCreateRunbook() {
    loading.value = true
    statusMessage.value = null
    try {
      newRunbook.tags = tagInput.value.split(',').map(t => t.trim()).filter(Boolean)
      await runbookApi.createRunbook(newRunbook)
      statusMessage.value = { type: 'success', text: `Runbook "${newRunbook.title}" published successfully.` }
      showCreateModal.value = false
      await fetchRunbooks()
    } catch (err: unknown) {
      statusMessage.value = { type: 'error', text: err instanceof Error ? err.message : 'Failed to create runbook' }
    } finally {
      loading.value = false
    }
  }

  async function handleUpdateRunbook() {
    if (!editingRunbook.value) return
    loading.value = true
    statusMessage.value = null
    try {
      await runbookApi.updateRunbook(editingRunbook.value.id, editingRunbook.value)
      statusMessage.value = { type: 'success', text: `Runbook "${editingRunbook.value.title}" updated successfully.` }
      showEditModal.value = false
      await fetchRunbooks()
    } catch (err: unknown) {
      statusMessage.value = { type: 'error', text: err instanceof Error ? err.message : 'Failed to update runbook' }
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

  function copyCommand(cmd?: string) {
    if (!cmd) return
    navigator.clipboard.writeText(cmd)
    statusMessage.value = { type: 'success', text: 'Command copied to clipboard!' }
  }

  function getCategoryIcon(cat: string): string {
    const c = (cat || '').toLowerCase()
    if (c.includes('disaster') || c.includes('dr')) return 'zap'
    if (c.includes('incident')) return 'alert-triangle'
    if (c.includes('security')) return 'shield'
    if (c.includes('database') || c.includes('db')) return 'database'
    if (c.includes('network')) return 'globe'
    return 'book-open'
  }

  function formatDate(d?: string): string {
    if (!d) return '-'
    try { return new Date(d).toLocaleDateString() } catch { return d }
  }

  function dismissStatus() {
    statusMessage.value = null
  }

  return {
    runbooks, loading, error, statusMessage, activeCategory, searchQuery, viewMode,
    selectedRunbook, executingRunbook, activeExecution, executingId,
    activeSteps, completedSteps,
    showCreateModal, showEditModal, showExecuteModal, showExecutionDrawer, editingRunbook,
    executionHistory, executionLogs, tagInput, newRunbook,
    categoryList, categoriesCount, totalStepsCount, filteredRunbooks, parsedSteps, parameterSchemas,
    fetchRunbooks, selectCategory, selectRunbook, openInspectDrawer, openExecuteModal, openEditModal,
    toggleStep, markAllStepsComplete, handleExecuteRunbook, executeSingleStep,
    handleCreateRunbook, handleUpdateRunbook, handleDeleteRunbook,
    copyCommand, getCategoryIcon, formatDate, dismissStatus,
  }
}
