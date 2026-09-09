import { ref, reactive, computed, onMounted } from 'vue'
import {
  scaffoldApi,
  type Template,
  type TemplateVariable,
  type RenderResponse,
  type TemplateCategory,
} from '../api/scaffold'

export interface CategoryOption {
  key: TemplateCategory
  label: string
  icon: string
}

export interface RepoConfig {
  repoName: string
  gitProvider: 'github' | 'gitlab' | 'bitbucket'
  organization: string
  branch: string
  isPrivate: boolean
}

export interface CicdConfig {
  pipelineProvider: 'github-actions' | 'gitlab-ci' | 'argocd'
  triggerOnPush: boolean
  autoDeployK8s: boolean
}

export interface ScaffolderLogEntry {
  id: string
  timestamp: string
  level: 'info' | 'warn' | 'error' | 'success'
  message: string
}

export interface CustomTemplateForm {
  id?: string
  name: string
  description: string
  category: string
  framework: string
  manifest_yaml: string
  helm_values: string
  docker_compose: string
  variables: TemplateVariable[]
  tagsInput: string
}

export const SCAFFOLDER_CATEGORIES: CategoryOption[] = [
  { key: 'all', label: 'All Templates', icon: '✨' },
  { key: 'web', label: 'Web Applications', icon: '🌐' },
  { key: 'api', label: 'REST & gRPC APIs', icon: '⚡' },
  { key: 'database', label: 'Databases & Storage', icon: '🗄️' },
  { key: 'worker', label: 'Background Workers', icon: '⏳' },
  { key: 'fullstack', label: 'Full-Stack Apps', icon: '🚀' },
]

export const BUILTIN_SCAFFOLD_TEMPLATES: Template[] = [
  {
    id: 'tpl-builtin-go-microservice',
    name: 'Go Chi Microservice',
    description: 'Cloud-native Go REST microservice with Chi router, Prometheus metrics, and graceful shutdown',
    category: 'api',
    framework: 'go-chi',
    manifest_yaml: 'apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: go-service',
    helm_values: 'replicaCount: 2\nimage:\n  repository: golang\n  tag: 1.22',
    docker_compose: 'version: "3.8"\nservices:\n  app:\n    image: golang:1.22',
    variables: [
      { name: 'service_name', description: 'Name of the service', default: 'my-go-service', required: true, type: 'string' },
      { name: 'port', description: 'HTTP listen port', default: '8080', required: true, type: 'number' },
      { name: 'replicas', description: 'K8s pod replicas', default: '2', required: true, type: 'number' },
    ],
    built_in: true,
    tags: ['golang', 'chi', 'rest', 'k8s'],
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-25T00:00:00Z',
  },
  {
    id: 'tpl-builtin-node-fastify',
    name: 'Node.js Fastify API',
    description: 'High-performance TypeScript Fastify microservice with schema validation and OpenAPI spec',
    category: 'web',
    framework: 'node-fastify',
    manifest_yaml: 'apiVersion: apps/v1\nkind: Deployment',
    helm_values: 'replicaCount: 2',
    docker_compose: 'version: "3.8"',
    variables: [
      { name: 'service_name', description: 'Service name', default: 'fastify-app', required: true, type: 'string' },
      { name: 'port', description: 'Listen port', default: '3000', required: true, type: 'number' },
    ],
    built_in: true,
    tags: ['nodejs', 'fastify', 'typescript', 'docker'],
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-25T00:00:00Z',
  },
  {
    id: 'tpl-builtin-python-fastapi',
    name: 'Python FastAPI Service',
    description: 'Modern asynchronous Python 3.11 service with Pydantic v2 data models and Swagger docs',
    category: 'api',
    framework: 'python-fastapi',
    manifest_yaml: 'apiVersion: apps/v1\nkind: Deployment',
    helm_values: 'replicaCount: 2',
    docker_compose: 'version: "3.8"',
    variables: [
      { name: 'service_name', description: 'Service name', default: 'fastapi-service', required: true, type: 'string' },
      { name: 'port', description: 'Listen port', default: '8000', required: true, type: 'number' },
    ],
    built_in: true,
    tags: ['python', 'fastapi', 'async', 'rest'],
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-25T00:00:00Z',
  },
  {
    id: 'tpl-builtin-nginx-proxy',
    name: 'Nginx Reverse Proxy',
    description: 'Production-ready Nginx reverse proxy with SSL termination, gzip compression, and caching headers',
    category: 'web',
    framework: 'nginx',
    manifest_yaml: 'apiVersion: apps/v1\nkind: Deployment',
    helm_values: 'replicaCount: 2',
    docker_compose: 'version: "3.8"',
    variables: [
      { name: 'service_name', description: 'Proxy name', default: 'nginx-ingress', required: true, type: 'string' },
      { name: 'upstream_host', description: 'Upstream hostname', default: 'backend-svc:8080', required: true, type: 'string' },
    ],
    built_in: true,
    tags: ['nginx', 'ingress', 'cache', 'proxy'],
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-25T00:00:00Z',
  },
  {
    id: 'tpl-builtin-postgres-db',
    name: 'PostgreSQL HA Cluster',
    description: 'Dedicated PostgreSQL 16 database with automated backup policies and connection pooling',
    category: 'database',
    framework: 'postgres',
    manifest_yaml: 'apiVersion: apps/v1\nkind: StatefulSet',
    helm_values: 'replicaCount: 1',
    docker_compose: 'version: "3.8"',
    variables: [
      { name: 'database_name', description: 'Database name', default: 'app_db', required: true, type: 'string' },
      { name: 'username', description: 'Admin user', default: 'postgres', required: true, type: 'string' },
    ],
    built_in: true,
    tags: ['postgresql', 'database', 'sql', 'storage'],
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-25T00:00:00Z',
  },
]

export function getFrameworkIcon(framework: string): string {
  switch (framework.toLowerCase()) {
    case 'go-chi':
    case 'golang':
    case 'go': return '🐹'
    case 'node-fastify':
    case 'fastify': return '⚡'
    case 'node-express':
    case 'nodejs':
    case 'node': return '💚'
    case 'python-fastapi':
    case 'fastapi':
    case 'python': return '🐍'
    case 'rust':
    case 'rust-actix': return '🦀'
    case 'nginx': return '🌐'
    case 'postgres':
    case 'postgresql': return '🐘'
    case 'react': return '⚛️'
    case 'vue': return '💚'
    default: return '📦'
  }
}

export function getCategoryBadgeClass(category: string): string {
  switch (category.toLowerCase()) {
    case 'web': return 'badge-cyan'
    case 'api': return 'badge-indigo'
    case 'database': return 'badge-amber'
    case 'worker': return 'badge-emerald'
    case 'fullstack': return 'badge-violet'
    default: return 'badge-slate'
  }
}

export function useScaffolder() {
  const loading = ref(false)
  const rendering = ref(false)
  const saving = ref(false)
  const deleting = ref(false)
  const error = ref<string | null>(null)
  const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)
  let toastTimer: ReturnType<typeof setTimeout> | null = null

  const templates = ref<Template[]>([])
  const selectedCategory = ref<TemplateCategory>('all')
  const searchQuery = ref('')

  const showWizardModal = ref(false)
  const currentStep = ref<1 | 2 | 3>(1)
  const activeTemplate = ref<Template | null>(null)
  const formVariables = reactive<Record<string, string>>({})
  const registerInCatalog = ref(true)
  const ownerTeam = ref('')
  const ownerEmail = ref('')

  const repoConfig = reactive<RepoConfig>({
    repoName: '', gitProvider: 'github', organization: 'cloud-org', branch: 'main', isPrivate: true,
  })

  const cicdConfig = reactive<CicdConfig>({
    pipelineProvider: 'github-actions', triggerOnPush: true, autoDeployK8s: true,
  })

  const showLogsDrawer = ref(false)
  const renderResult = ref<RenderResponse | null>(null)
  const activeOutputTab = ref<'yaml' | 'compose' | 'helm' | 'logs'>('yaml')
  const copySuccess = ref(false)
  const logs = ref<ScaffolderLogEntry[]>([])
  const isDryRun = ref(false)

  const showCustomModal = ref(false)
  const customModalMode = ref<'create' | 'edit'>('create')
  const customTemplate = reactive<CustomTemplateForm>({
    name: '', description: '', category: 'web', framework: 'go-chi',
    manifest_yaml: '', helm_values: '', docker_compose: '', variables: [], tagsInput: '',
  })

  function showToast(text: string, type: 'success' | 'error' = 'success') {
    if (toastTimer) clearTimeout(toastTimer)
    toastMessage.value = { text, type }
    toastTimer = setTimeout(() => { toastMessage.value = null }, 4000)
  }

  function addLog(message: string, level: 'info' | 'warn' | 'error' | 'success' = 'info') {
    const timestamp = new Date().toLocaleTimeString()
    logs.value.push({
      id: `log-${Date.now().toString(36)}-${logs.value.length}`,
      timestamp, level, message,
    })
  }

  async function loadTemplates() {
    loading.value = true
    error.value = null
    try {
      const filter: Record<string, string> = {}
      if (selectedCategory.value !== 'all') filter.category = selectedCategory.value
      if (searchQuery.value.trim()) filter.search = searchQuery.value.trim()
      const res = await scaffoldApi.list(filter).catch(() => null)
      if (Array.isArray(res) && res.length > 0) {
        templates.value = res
      } else {
        // Fallback to built-in system templates for resilience
        templates.value = BUILTIN_SCAFFOLD_TEMPLATES
      }
    } catch (err: unknown) {
      templates.value = BUILTIN_SCAFFOLD_TEMPLATES
      showToast(err instanceof Error ? err.message : 'Failed to load scaffold templates', 'error')
    } finally {
      loading.value = false
    }
  }

  const filteredTemplates = computed(() => {
    return templates.value.filter(t => {
      if (selectedCategory.value !== 'all' && t.category !== selectedCategory.value) return false
      if (searchQuery.value.trim()) {
        const q = searchQuery.value.toLowerCase()
        const matchName = t.name.toLowerCase().includes(q)
        const matchDesc = t.description.toLowerCase().includes(q)
        const matchFramework = t.framework.toLowerCase().includes(q)
        const matchTags = t.tags && t.tags.some(tag => tag.toLowerCase().includes(q))
        if (!matchName && !matchDesc && !matchFramework && !matchTags) return false
      }
      return true
    })
  })

  function parseTemplateVariables(tmpl: Template) {
    Object.keys(formVariables).forEach(k => delete formVariables[k])
    if (tmpl.variables) {
      for (const v of tmpl.variables) formVariables[v.name] = v.default || ''
    }
  }

  function openWizard(tmpl: Template) {
    activeTemplate.value = tmpl; currentStep.value = 1; renderResult.value = null
    copySuccess.value = false; activeOutputTab.value = 'yaml'; registerInCatalog.value = true
    ownerTeam.value = 'platform-team'; ownerEmail.value = 'devs@example.com'; isDryRun.value = false
    logs.value = []; parseTemplateVariables(tmpl)
    repoConfig.repoName = tmpl.name.toLowerCase().replace(/[^a-z0-9]/g, '-')
    showWizardModal.value = true
  }

  function closeWizard() { showWizardModal.value = false; activeTemplate.value = null }
  function nextStep() { if (currentStep.value < 3) currentStep.value = (currentStep.value + 1) as 1 | 2 | 3 }
  function prevStep() { if (currentStep.value > 1) currentStep.value = (currentStep.value - 1) as 1 | 2 | 3 }
  function goToStep(s: 1 | 2 | 3) { currentStep.value = s }

  async function triggerScaffoldJob(dryRun = false) {
    if (!activeTemplate.value) return
    isDryRun.value = dryRun
    rendering.value = true
    error.value = null
    logs.value = []
    showWizardModal.value = false
    showLogsDrawer.value = true
    activeOutputTab.value = 'logs'
    addLog(`Initiating scaffolding job for ${activeTemplate.value.name}...`, 'info')

    try {
      const res = await scaffoldApi.render({
        template_id: activeTemplate.value.id, variables: { ...formVariables },
        register_catalog: registerInCatalog.value, owner_team: ownerTeam.value, owner_email: ownerEmail.value,
      })
      renderResult.value = res
      addLog('Template rendering completed successfully!', 'success')
      showToast('Scaffolding job completed successfully!', 'success')
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Render failed'
      addLog(msg, 'error')
      showToast(msg, 'error')
    } finally {
      rendering.value = false
    }
  }

  async function copyToClipboard(text: string) {
    try {
      await navigator.clipboard.writeText(text)
      copySuccess.value = true
      setTimeout(() => { copySuccess.value = false }, 2000)
    } catch { /* fallback */ }
  }

  function downloadFile(content: string, filename: string) {
    const blob = new Blob([content], { type: 'text/plain;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
    URL.revokeObjectURL(url)
  }

  function openCustomTemplateModal(mode: 'create' | 'edit', tmpl?: Template) {
    customModalMode.value = mode
    if (mode === 'edit' && tmpl) {
      customTemplate.id = tmpl.id; customTemplate.name = tmpl.name
      customTemplate.description = tmpl.description; customTemplate.category = tmpl.category
      customTemplate.framework = tmpl.framework; customTemplate.manifest_yaml = tmpl.manifest_yaml || ''
      customTemplate.helm_values = tmpl.helm_values || ''; customTemplate.docker_compose = tmpl.docker_compose || ''
      customTemplate.variables = tmpl.variables ? [...tmpl.variables] : []
      customTemplate.tagsInput = (tmpl.tags || []).join(', ')
    } else {
      customTemplate.id = undefined; customTemplate.name = ''; customTemplate.description = ''
      customTemplate.category = 'web'; customTemplate.framework = 'go-chi'; customTemplate.manifest_yaml = ''
      customTemplate.helm_values = ''; customTemplate.docker_compose = ''; customTemplate.variables = []
      customTemplate.tagsInput = ''
    }
    showCustomModal.value = true
  }

  function addVariableToCustomTemplate() {
    customTemplate.variables.push({ name: '', label: '', description: '', default: '', required: false, type: 'string' })
  }

  function removeVariableFromCustomTemplate(index: number) { customTemplate.variables.splice(index, 1) }

  async function handleSaveCustomTemplate() {
    saving.value = true
    try {
      const tags = customTemplate.tagsInput.split(',').map(t => t.trim()).filter(Boolean)
      const payload: Partial<Template> = {
        name: customTemplate.name.trim(), description: customTemplate.description.trim(),
        category: customTemplate.category as TemplateCategory, framework: customTemplate.framework.trim(),
        manifest_yaml: customTemplate.manifest_yaml, helm_values: customTemplate.helm_values,
        docker_compose: customTemplate.docker_compose, variables: customTemplate.variables, tags,
      }
      if (customModalMode.value === 'create') {
        await scaffoldApi.create(payload)
        showToast('Custom template created successfully!')
      } else if (customTemplate.id) {
        await scaffoldApi.update(customTemplate.id, payload)
        showToast('Custom template updated successfully!')
      }
      showCustomModal.value = false
      await loadTemplates()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to save custom template', 'error')
    } finally {
      saving.value = false
    }
  }

  async function handleDeleteCustomTemplate(tmpl: Template) {
    if (!confirm(`Are you sure you want to delete template "${tmpl.name}"?`)) return
    deleting.value = true
    try {
      await scaffoldApi.delete(tmpl.id)
      showToast(`Template "${tmpl.name}" deleted.`)
      await loadTemplates()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to delete template', 'error')
    } finally {
      deleting.value = false
    }
  }

  function resetFilters() { selectedCategory.value = 'all'; searchQuery.value = ''; loadTemplates() }

  onMounted(() => { loadTemplates() })

  return {
    loading, rendering, saving, deleting, error, toastMessage, templates, selectedCategory,
    searchQuery, filteredTemplates, categories: SCAFFOLDER_CATEGORIES, showWizardModal,
    currentStep, activeTemplate, formVariables, repoConfig, cicdConfig, registerInCatalog,
    ownerTeam, ownerEmail, showLogsDrawer, renderResult, activeOutputTab, copySuccess,
    logs, isDryRun, showCustomModal, customModalMode, customTemplate, loadTemplates,
    openWizard, closeWizard, nextStep, prevStep, goToStep, triggerScaffoldJob, copyToClipboard,
    downloadFile, openCustomTemplateModal, addVariableToCustomTemplate,
    removeVariableFromCustomTemplate, handleSaveCustomTemplate, handleDeleteCustomTemplate, resetFilters,
  }
}
