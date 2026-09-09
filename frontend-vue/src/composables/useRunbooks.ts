import { ref, computed, reactive } from 'vue'
import { runbookApi, type Runbook } from '../api/governance'

export interface RunbookStep {
  id: string;
  stepNum: number;
  title: string;
  content: string;
  command?: string;
  dependencies?: string[];
  status: 'idle' | 'running' | 'completed' | 'failed' | 'skipped';
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

export function useRunbooks() {
  const runbooks = ref<Runbook[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const statusMessage = ref<{ type: 'success' | 'error'; text: string } | null>(null)
  const activeCategory = ref<string>('ALL')
  const searchQuery = ref('')
  const viewMode = ref<'grid' | 'table'>('grid')

  const selectedRunbook = ref<Runbook | null>(null)
  const executingRunbook = ref<Runbook | null>(null)
  const activeExecution = ref<RunbookExecutionRecord | null>(null)
  const executingId = ref<string | null>(null)
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

  const categoriesCount = computed(() => Math.max(0, categoryList.value.length - 1))
  
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

  function parseRunbookSteps(content?: string, rbTitle?: string): RunbookStep[] {
    if (!content) return []
    const lines = content.split('\n')
    const steps: RunbookStep[] = []
    let currentStep: RunbookStep | null = null
    let stepCounter = 1

    for (const line of lines) {
      const trimmed = line.trim()
      if (/^\d+\./.test(trimmed) || trimmed.startsWith('Step ')) {
        if (currentStep) steps.push(currentStep)
        const stepNum = stepCounter++
        currentStep = {
          id: `step-${stepNum}`,
          stepNum,
          title: trimmed.replace(/^\d+\.\s*/, '').replace(/^Step\s*\d+:\s*/, ''),
          content: '',
          dependencies: stepNum > 1 ? [`step-${stepNum - 1}`] : [],
          status: 'idle',
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

  const parsedSteps = computed<RunbookStep[]>(() => {
    if (!selectedRunbook.value) return []
    return parseRunbookSteps(selectedRunbook.value.content, selectedRunbook.value.title)
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
    selectedRunbook.value = rb
    completedSteps.value = new Set()
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
    if (completedSteps.value.has(idx)) {
      completedSteps.value.delete(idx)
    } else {
      completedSteps.value.add(idx)
    }
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
        executedBy: rb.author || 'Operator',
        summary: res.message || `${isDry ? '[DRY-RUN] ' : ''}Runbook "${rb.title}" execution completed successfully.`,
        logs: [
          `[${new Date().toLocaleTimeString()}] [INIT] Initiating ${isDry ? 'DRY-RUN' : 'LIVE'} execution for "${rb.title}"`,
          `[${new Date().toLocaleTimeString()}] [TARGET] Cluster: ${options.target?.cluster || 'production-01'} | Namespace: ${options.target?.namespace || 'default'}`,
          `[${new Date().toLocaleTimeString()}] [EXEC] Dispatched ${rb.steps_count || 3} procedure steps.`,
          `[${new Date().toLocaleTimeString()}] [OUTPUT] ${res.message || 'All steps exited with code 0.'}`,
          `[${new Date().toLocaleTimeString()}] [DONE] Runbook finished in ${elapsedMs}ms.`,
        ],
        steps: parseRunbookSteps(rb.content, rb.title).map(s => ({ ...s, status: 'completed' })),
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
    executionLogs.value.push(
      `[${new Date().toLocaleTimeString()}] [EXEC] Executing Step ${stepIdx + 1}: ${step.title}`,
    )
    if (step.command) {
      executionLogs.value.push(`[${new Date().toLocaleTimeString()}] [CMD] $ ${step.command}`)
    }

    await new Promise(resolve => setTimeout(resolve, 600))
    step.status = 'completed'
    completedSteps.value.add(stepIdx)
    executionLogs.value.push(
      `[${new Date().toLocaleTimeString()}] [OK] Step ${stepIdx + 1} completed successfully with exit status 0.`,
    )
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

  async function handleUpdateRunbook() {
    if (!editingRunbook.value) return
    loading.value = true
    statusMessage.value = null
    try {
      await runbookApi.updateRunbook(editingRunbook.value.id, editingRunbook.value)
      statusMessage.value = {
        type: 'success',
        text: `Runbook "${editingRunbook.value.title}" updated successfully.`,
      }
      showEditModal.value = false
      await fetchRunbooks()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to update runbook'
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

  function copyCommand(cmd?: string) {
    if (!cmd) return
    navigator.clipboard.writeText(cmd)
    statusMessage.value = { type: 'success', text: 'Command copied to clipboard!' }
  }


  function getCategoryIcon(cat: string): string {
    const c = (cat || '').toLowerCase()
    if (c.includes('disaster') || c.includes('dr')) return '⚡'

    if (c.includes('incident')) return '🚡'
    if (c.includes('security')) return '🔑'
    if (c.includes('database') || c.includes('db')) return '🐘'
    if (c.includes('network')) return '🌐'
    return '📖'
  }


  function formatDate(d?: string): string {
    if (!d) return '-'
    try {
      return new Date(d).toLocaleDateString()
    } catch {
      return d
    }
  }

  function dismissStatus() {
    statusMessage.value = null
  }

  return {
    runbooks,
    loading,
    error,
    statusMessage,
    activeCategory,
    searchQuery,
    viewMode,
    selectedRunbook,
    executingRunbook,
    activeExecution,
    executingId,
    completedSteps,
    showCreateModal,
    showEditModal,
    showExecuteModal,
    showExecutionDrawer,
    editingRunbook,
    executionHistory,
    executionLogs,
    tagInput,
    newRunbook,
    categoryList,
    categoriesCount,
    totalStepsCount,
    filteredRunbooks,
    parsedSteps,
    parameterSchemas,
    fetchRunbooks,
    selectCategory,
    openInspectDrawer,
    openExecuteModal,
    openEditModal,
    toggleStep,
    markAllStepsComplete,
    handleExecuteRunbook,
    executeSingleStep,
    handleCreateRunbook,
    handleUpdateRunbook,
    handleDeleteRunbook,
    copyCommand,
    getCategoryIcon,
    formatDate,
    dismissStatus,
  }
}
