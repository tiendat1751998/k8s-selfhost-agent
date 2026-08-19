<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import {
  scaffoldApi,
  type Template,
  type TemplateVariable,
  type RenderResponse,
  type TemplateCategory
} from '../api/scaffold'

// State
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

// Wizard State
const showWizardModal = ref(false)
const activeTemplate = ref<Template | null>(null)
const formVariables = reactive<Record<string, string>>({})
const registerInCatalog = ref(true)
const ownerTeam = ref('')
const ownerEmail = ref('')
const renderResult = ref<RenderResponse | null>(null)
const activeOutputTab = ref<'yaml' | 'compose' | 'helm'>('yaml')
const copySuccess = ref(false)

// Custom Template Modal State
const showCustomModal = ref(false)
const customModalMode = ref<'create' | 'edit'>('create')
const customTemplate = reactive<{
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
}>({
  name: '',
  description: '',
  category: 'web',
  framework: 'go-chi',
  manifest_yaml: '',
  helm_values: '',
  docker_compose: '',
  variables: [],
  tagsInput: '',
})

// Categories definition
const categories: { key: TemplateCategory; label: string; icon: string }[] = [
  { key: 'all', label: 'All Templates', icon: '✨' },
  { key: 'web', label: 'Web Applications', icon: '🌐' },
  { key: 'api', label: 'REST & gRPC APIs', icon: '⚡' },
  { key: 'database', label: 'Databases & Storage', icon: '🗄️' },
  { key: 'worker', label: 'Background Workers', icon: '⏳' },
  { key: 'fullstack', label: 'Full-Stack Apps', icon: '🚀' },
]

function showToast(text: string, type: 'success' | 'error' = 'success') {
  if (toastTimer) clearTimeout(toastTimer)
  toastMessage.value = { text, type }
  toastTimer = setTimeout(() => {
    toastMessage.value = null
  }, 4000)
}

function getFrameworkIcon(framework: string): string {
  switch (framework.toLowerCase()) {
    case 'go-chi':
    case 'golang':
    case 'go':
      return '🐹'
    case 'node-express':
    case 'nodejs':
    case 'node':
      return '🟩'
    case 'python-fastapi':
    case 'fastapi':
    case 'python':
      return '🐍'
    case 'postgres':
    case 'postgresql':
      return '🐘'
    case 'react':
      return '⚛️'
    case 'vue':
      return '💚'
    default:
      return '📦'
  }
}

function getCategoryBadgeClass(category: string): string {
  switch (category.toLowerCase()) {
    case 'web':
      return 'badge-cyan'
    case 'api':
      return 'badge-indigo'
    case 'database':
      return 'badge-amber'
    case 'worker':
      return 'badge-emerald'
    case 'fullstack':
      return 'badge-violet'
    default:
      return 'badge-slate'
  }
}

async function loadTemplates() {
  loading.value = true
  error.value = null
  try {
    const filter: Record<string, string> = {}
    if (selectedCategory.value !== 'all') {
      filter.category = selectedCategory.value
    }
    if (searchQuery.value.trim()) {
      filter.search = searchQuery.value.trim()
    }
    templates.value = await scaffoldApi.list(filter)
  } catch (err: any) {
    error.value = err?.message || 'Failed to load scaffold templates'
    showToast(error.value || 'Error loading templates', 'error')
  } finally {
    loading.value = false
  }
}

const filteredTemplates = computed(() => {
  return templates.value.filter(t => {
    if (selectedCategory.value !== 'all' && t.category !== selectedCategory.value) {
      return false
    }
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

function openWizard(tmpl: Template) {
  activeTemplate.value = tmpl
  renderResult.value = null
  copySuccess.value = false
  activeOutputTab.value = 'yaml'
  registerInCatalog.value = true
  ownerTeam.value = ''
  ownerEmail.value = ''

  // Populate default variables
  Object.keys(formVariables).forEach(k => delete formVariables[k])
  if (tmpl.variables) {
    for (const v of tmpl.variables) {
      formVariables[v.name] = v.default || ''
    }
  }

  showWizardModal.value = true
}

async function handleGenerate() {
  if (!activeTemplate.value) return
  rendering.value = true
  error.value = null

  try {
    const res = await scaffoldApi.render({
      template_id: activeTemplate.value.id,
      variables: { ...formVariables },
      register_catalog: registerInCatalog.value,
      owner_team: ownerTeam.value,
      owner_email: ownerEmail.value,
    })
    renderResult.value = res
    showToast('Manifests generated successfully!')
  } catch (err: any) {
    const msg = err?.response?.data?.message || err?.message || 'Generation failed'
    showToast(msg, 'error')
  } finally {
    rendering.value = false
  }
}

function getActiveCode(): string {
  if (!renderResult.value) return ''
  switch (activeOutputTab.value) {
    case 'yaml':
      return renderResult.value.rendered_yaml || ''
    case 'compose':
      return renderResult.value.rendered_compose || ''
    case 'helm':
      return renderResult.value.rendered_helm || ''
  }
}

async function copyToClipboard() {
  const code = getActiveCode()
  if (!code) return
  try {
    await navigator.clipboard.writeText(code)
    copySuccess.value = true
    showToast('Copied to clipboard!')
    setTimeout(() => {
      copySuccess.value = false
    }, 2500)
  } catch {
    showToast('Failed to copy to clipboard', 'error')
  }
}

function downloadFile() {
  const code = getActiveCode()
  if (!code || !activeTemplate.value) return

  let filename = `${activeTemplate.value.name.toLowerCase().replace(/\s+/g, '-')}`
  if (activeOutputTab.value === 'yaml') filename += '-manifest.yaml'
  else if (activeOutputTab.value === 'compose') filename += '-docker-compose.yml'
  else if (activeOutputTab.value === 'helm') filename += '-values.yaml'

  const blob = new Blob([code], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
  showToast(`Downloaded ${filename}`)
}

// Custom Template Builder
function openCustomTemplateModal(mode: 'create' | 'edit', tmpl?: Template) {
  customModalMode.value = mode
  if (mode === 'edit' && tmpl) {
    customTemplate.id = tmpl.id
    customTemplate.name = tmpl.name
    customTemplate.description = tmpl.description
    customTemplate.category = tmpl.category
    customTemplate.framework = tmpl.framework
    customTemplate.manifest_yaml = tmpl.manifest_yaml
    customTemplate.helm_values = tmpl.helm_values
    customTemplate.docker_compose = tmpl.docker_compose
    customTemplate.variables = tmpl.variables ? JSON.parse(JSON.stringify(tmpl.variables)) : []
    customTemplate.tagsInput = (tmpl.tags || []).join(', ')
  } else {
    customTemplate.id = undefined
    customTemplate.name = ''
    customTemplate.description = ''
    customTemplate.category = 'web'
    customTemplate.framework = 'go-chi'
    customTemplate.manifest_yaml = `apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: {{.app_name}}\nspec:\n  replicas: {{.replicas}}\n  template:\n    spec:\n      containers:\n      - name: {{.app_name}}\n        image: {{.image}}`
    customTemplate.helm_values = `nameOverride: {{.app_name}}\nreplicaCount: {{.replicas}}`
    customTemplate.docker_compose = `version: '3.8'\nservices:\n  {{.app_name}}:\n    image: {{.image}}`
    customTemplate.variables = [
      { name: 'app_name', label: 'Application Name', type: 'string', default: 'my-app', required: true },
      { name: 'replicas', label: 'Replicas', type: 'number', default: '2', required: true },
      { name: 'image', label: 'Image', type: 'string', default: 'alpine:latest', required: true },
    ]
    customTemplate.tagsInput = 'custom, starter'
  }
  showCustomModal.value = true
}

function addVariableToCustomTemplate() {
  customTemplate.variables.push({
    name: `var_${customTemplate.variables.length + 1}`,
    label: `Variable ${customTemplate.variables.length + 1}`,
    type: 'string',
    default: '',
    required: false,
  })
}

function removeVariableFromCustomTemplate(index: number) {
  customTemplate.variables.splice(index, 1)
}

async function handleSaveCustomTemplate() {
  if (!customTemplate.name.trim()) {
    showToast('Template name is required', 'error')
    return
  }

  saving.value = true
  const tags = customTemplate.tagsInput
    .split(',')
    .map(t => t.trim())
    .filter(Boolean)

  const payload: Partial<Template> = {
    name: customTemplate.name,
    description: customTemplate.description,
    category: customTemplate.category,
    framework: customTemplate.framework,
    manifest_yaml: customTemplate.manifest_yaml,
    helm_values: customTemplate.helm_values,
    docker_compose: customTemplate.docker_compose,
    variables: customTemplate.variables,
    tags,
  }

  try {
    if (customModalMode.value === 'create') {
      await scaffoldApi.create(payload)
      showToast('Custom template created!')
    } else if (customTemplate.id) {
      await scaffoldApi.update(customTemplate.id, payload)
      showToast('Template updated!')
    }
    showCustomModal.value = false
    await loadTemplates()
  } catch (err: any) {
    const msg = err?.response?.data?.message || err?.message || 'Failed to save template'
    showToast(msg, 'error')
  } finally {
    saving.value = false
  }
}

async function handleDeleteCustomTemplate(tmpl: Template) {
  if (tmpl.built_in) return
  if (!confirm(`Are you sure you want to delete template "${tmpl.name}"?`)) return

  deleting.value = true
  try {
    await scaffoldApi.delete(tmpl.id)
    showToast(`Template "${tmpl.name}" deleted.`)
    await loadTemplates()
  } catch (err: any) {
    const msg = err?.response?.data?.message || err?.message || 'Failed to delete template'
    showToast(msg, 'error')
  } finally {
    deleting.value = false
  }
}

onMounted(() => {
  loadTemplates()
})
</script>

<template>
  <div class="scaffolder-view">
    <!-- Toast Notification -->
    <transition name="toast">
      <div v-if="toastMessage" :class="['toast-banner', `toast-${toastMessage.type}`]">
        <span class="toast-icon">{{ toastMessage.type === 'success' ? '✅' : '⚠️' }}</span>
        <span>{{ toastMessage.text }}</span>
      </div>
    </transition>

    <!-- Header & Hero Section -->
    <header class="page-header glass-panel">
      <div class="header-content">
        <div class="header-left">
          <div class="header-icon-badge">🪄</div>
          <div>
            <h1 class="header-title">Application Scaffolder</h1>
            <p class="header-sub">
              1-Click Deploy & Template Engine • Generates Cloud-Native K8s Manifests, Helm Charts & Compose Files
            </p>
          </div>
        </div>
        <div class="header-actions">
          <button class="btn-primary" @click="openCustomTemplateModal('create')">
            <span class="btn-icon">➕</span> Create Template
          </button>
          <button class="btn-secondary" :disabled="loading" @click="loadTemplates">
            <span class="btn-icon" :class="{ 'spin-anim': loading }">🔄</span> Refresh
          </button>
        </div>
      </div>

      <!-- Controls: Category Filter Tabs + Search -->
      <div class="filter-bar">
        <div class="category-tabs">
          <button
            v-for="cat in categories"
            :key="cat.key"
            :class="['tab-btn', { active: selectedCategory === cat.key }]"
            @click="selectedCategory = cat.key"
          >
            <span class="tab-icon">{{ cat.icon }}</span>
            <span class="tab-label">{{ cat.label }}</span>
          </button>
        </div>

        <div class="search-box">
          <span class="search-icon">🔍</span>
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Search templates, frameworks, tags..."
            class="search-input"
          />
          <button v-if="searchQuery" class="search-clear" @click="searchQuery = ''">✕</button>
        </div>
      </div>
    </header>

    <!-- Gallery Content -->
    <div class="gallery-container">
      <div v-if="loading && templates.length === 0" class="loading-state glass-panel">
        <div class="spinner"></div>
        <p>Loading application templates...</p>
      </div>

      <div v-else-if="filteredTemplates.length === 0" class="empty-state glass-panel">
        <div class="empty-icon">📦</div>
        <h3>No templates found</h3>
        <p>Try adjusting your category filter or search query.</p>
        <button class="btn-secondary mt-3" @click="selectedCategory = 'all'; searchQuery = ''">
          Reset Filters
        </button>
      </div>

      <!-- Templates Grid -->
      <div v-else class="templates-grid">
        <div
          v-for="tmpl in filteredTemplates"
          :key="tmpl.id"
          class="template-card glass-panel"
        >
          <div class="card-header">
            <div class="framework-avatar">
              {{ getFrameworkIcon(tmpl.framework) }}
            </div>
            <div class="card-badges">
              <span :class="['badge', getCategoryBadgeClass(tmpl.category)]">
                {{ tmpl.category.toUpperCase() }}
              </span>
              <span v-if="tmpl.built_in" class="badge badge-system">
                SYSTEM
              </span>
              <span v-else class="badge badge-custom">
                CUSTOM
              </span>
            </div>
          </div>

          <div class="card-body">
            <h3 class="template-name">{{ tmpl.name }}</h3>
            <p class="template-framework">{{ tmpl.framework }}</p>
            <p class="template-desc">{{ tmpl.description }}</p>

            <div class="template-tags">
              <span v-for="tag in tmpl.tags" :key="tag" class="tag-pill">
                #{{ tag }}
              </span>
            </div>
          </div>

          <div class="card-footer">
            <button class="btn-deploy" @click="openWizard(tmpl)">
              <span class="btn-icon">⚡</span> 1-Click Deploy
            </button>

            <div v-if="!tmpl.built_in" class="custom-actions">
              <button
                class="btn-icon-action"
                title="Edit Custom Template"
                @click="openCustomTemplateModal('edit', tmpl)"
              >
                ✏️
              </button>
              <button
                class="btn-icon-action btn-icon-danger"
                title="Delete Template"
                :disabled="deleting"
                @click="handleDeleteCustomTemplate(tmpl)"
              >
                🗑️
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Wizard & Code Generator Modal -->
    <ModalDrawer
      v-if="showWizardModal && activeTemplate"
      :show="showWizardModal"
      :title="`Scaffold: ${activeTemplate.name}`"
      @close="showWizardModal = false"
    >
      <div class="wizard-modal-content">
        <!-- Template Summary Banner -->
        <div class="wizard-summary glass-panel">
          <div class="summary-avatar">{{ getFrameworkIcon(activeTemplate.framework) }}</div>
          <div class="summary-info">
            <h4>{{ activeTemplate.name }}</h4>
            <p>{{ activeTemplate.description }}</p>
            <div class="summary-meta">
              <span class="badge badge-cyan">{{ activeTemplate.framework }}</span>
              <span class="badge badge-indigo">{{ activeTemplate.category }}</span>
            </div>
          </div>
        </div>

        <div class="wizard-split-layout">
          <!-- Left Column: Variable Configuration Form -->
          <div class="wizard-form-section">
            <h4 class="section-title">⚙️ Template Variables</h4>
            <p class="section-sub">Configure variables to generate tailored infrastructure manifests.</p>

            <div class="variables-form-list">
              <div
                v-for="v in activeTemplate.variables"
                :key="v.name"
                class="form-group"
              >
                <label class="form-label">
                  {{ v.label || v.name }}
                  <span v-if="v.required" class="required-star">*</span>
                </label>

                <!-- Select Dropdown -->
                <select
                  v-if="v.type === 'select'"
                  v-model="formVariables[v.name]"
                  class="form-input"
                >
                  <option
                    v-for="opt in (v.options || [])"
                    :key="opt"
                    :value="opt"
                  >
                    {{ opt }}
                  </option>
                </select>

                <!-- Number / String Input -->
                <input
                  v-else
                  v-model="formVariables[v.name]"
                  :type="v.type === 'number' ? 'number' : 'text'"
                  :placeholder="v.default || ''"
                  class="form-input"
                />
              </div>
            </div>

            <!-- Service Catalog Integration -->
            <div class="catalog-integration-box glass-panel">
              <label class="checkbox-label">
                <input v-model="registerInCatalog" type="checkbox" />
                <span class="checkbox-custom"></span>
                <span>Register in Service Catalog automatically</span>
              </label>

              <transition name="fade">
                <div v-if="registerInCatalog" class="catalog-subfields">
                  <div class="form-group">
                    <label class="form-label">Owner Team</label>
                    <input
                      v-model="ownerTeam"
                      type="text"
                      placeholder="e.g. platform-team, backend"
                      class="form-input"
                    />
                  </div>
                  <div class="form-group">
                    <label class="form-label">Owner Email</label>
                    <input
                      v-model="ownerEmail"
                      type="email"
                      placeholder="devs@example.com"
                      class="form-input"
                    />
                  </div>
                </div>
              </transition>
            </div>

            <!-- Generate Button -->
            <div class="wizard-actions">
              <button
                class="btn-primary btn-block"
                :disabled="rendering"
                @click="handleGenerate"
              >
                <span v-if="rendering" class="btn-icon spin-anim">🔄</span>
                <span v-else class="btn-icon">⚡</span>
                {{ rendering ? 'Generating Manifests...' : 'Generate Manifests' }}
              </button>
            </div>
          </div>

          <!-- Right Column: Rendered Code Viewer -->
          <div class="wizard-output-section">
            <div class="output-header">
              <div class="output-tabs">
                <button
                  :class="['output-tab-btn', { active: activeOutputTab === 'yaml' }]"
                  @click="activeOutputTab = 'yaml'"
                >
                  ☸️ K8s Manifest
                </button>
                <button
                  :class="['output-tab-btn', { active: activeOutputTab === 'compose' }]"
                  @click="activeOutputTab = 'compose'"
                >
                  🐳 Docker Compose
                </button>
                <button
                  :class="['output-tab-btn', { active: activeOutputTab === 'helm' }]"
                  @click="activeOutputTab = 'helm'"
                >
                  ⛵ Helm Values
                </button>
              </div>

              <div class="output-actions">
                <button
                  class="btn-icon-action"
                  :title="copySuccess ? 'Copied!' : 'Copy Code'"
                  :disabled="!renderResult"
                  @click="copyToClipboard"
                >
                  {{ copySuccess ? '✅' : '📋' }}
                </button>
                <button
                  class="btn-icon-action"
                  title="Download File"
                  :disabled="!renderResult"
                  @click="downloadFile"
                >
                  💾
                </button>
              </div>
            </div>

            <div class="code-editor-viewer glass-panel">
              <pre v-if="renderResult" class="code-content"><code>{{ getActiveCode() }}</code></pre>
              <div v-else class="code-placeholder">
                <span class="placeholder-icon">📄</span>
                <p>Click <strong>"Generate Manifests"</strong> to render Kubernetes manifests, Helm values, or Docker Compose.</p>
              </div>
            </div>

            <div v-if="renderResult?.catalog_entry_id" class="catalog-success-alert glass-panel">
              <span>📚 Registered in Service Catalog (ID: <code>{{ renderResult.catalog_entry_id }}</code>)</span>
            </div>
          </div>
        </div>
      </div>
    </ModalDrawer>

    <!-- Custom Template Creator Modal -->
    <ModalDrawer
      v-if="showCustomModal"
      :show="showCustomModal"
      :title="customModalMode === 'create' ? 'Create Custom Scaffold Template' : 'Edit Scaffold Template'"
      @close="showCustomModal = false"
    >
      <div class="custom-template-modal">
        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label">Template Name <span class="required-star">*</span></label>
            <input
              v-model="customTemplate.name"
              type="text"
              placeholder="e.g. Rust Microservice Starter"
              class="form-input"
            />
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Category</label>
            <select v-model="customTemplate.category" class="form-input">
              <option value="web">Web Application</option>
              <option value="api">REST & gRPC API</option>
              <option value="database">Database & Storage</option>
              <option value="worker">Worker / Cron</option>
              <option value="fullstack">Full-Stack</option>
            </select>
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Framework</label>
            <input
              v-model="customTemplate.framework"
              type="text"
              placeholder="e.g. rust-actix, nextjs"
              class="form-input"
            />
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Description</label>
          <textarea
            v-model="customTemplate.description"
            rows="2"
            placeholder="Brief overview of what this scaffold provides..."
            class="form-input"
          ></textarea>
        </div>

        <div class="form-group">
          <label class="form-label">Tags (comma-separated)</label>
          <input
            v-model="customTemplate.tagsInput"
            type="text"
            placeholder="rust, actix, microservice, high-perf"
            class="form-input"
          />
        </div>

        <!-- Variable Definition Builder -->
        <div class="variable-builder-section glass-panel">
          <div class="builder-header">
            <h4>Variables (User Fillable)</h4>
            <button class="btn-secondary btn-sm" @click="addVariableToCustomTemplate">
              ➕ Add Variable
            </button>
          </div>

          <div v-if="customTemplate.variables.length === 0" class="empty-vars">
            No dynamic variables defined yet. Click "Add Variable" to create placeholders like <code>&#123;&#123;.app_name&#125;&#125;</code>.
          </div>

          <div
            v-for="(varItem, idx) in customTemplate.variables"
            :key="idx"
            class="var-builder-row"
          >
            <input
              v-model="varItem.name"
              type="text"
              placeholder="name (e.g. port)"
              class="form-input var-input"
            />
            <input
              v-model="varItem.label"
              type="text"
              placeholder="Label"
              class="form-input var-input"
            />
            <select v-model="varItem.type" class="form-input var-select">
              <option value="string">String</option>
              <option value="number">Number</option>
              <option value="boolean">Boolean</option>
              <option value="select">Select</option>
            </select>
            <input
              v-model="varItem.default"
              type="text"
              placeholder="Default"
              class="form-input var-input"
            />
            <label class="checkbox-label var-checkbox">
              <input v-model="varItem.required" type="checkbox" />
              <span>Req</span>
            </label>
            <button
              class="btn-icon-action btn-icon-danger"
              title="Remove Variable"
              @click="removeVariableFromCustomTemplate(idx)"
            >
              ✕
            </button>
          </div>
        </div>

        <!-- Manifest Editors -->
        <div class="manifest-editors-section">
          <div class="form-group">
            <label class="form-label">Kubernetes YAML Template</label>
            <textarea
              v-model="customTemplate.manifest_yaml"
              rows="6"
              class="form-input code-textarea"
              placeholder="apiVersion: apps/v1&#10;kind: Deployment..."
            ></textarea>
          </div>

          <div class="form-group">
            <label class="form-label">Helm values.yaml Template</label>
            <textarea
              v-model="customTemplate.helm_values"
              rows="4"
              class="form-input code-textarea"
              placeholder="replicaCount: {{.replicas}}..."
            ></textarea>
          </div>

          <div class="form-group">
            <label class="form-label">Docker Compose Template</label>
            <textarea
              v-model="customTemplate.docker_compose"
              rows="4"
              class="form-input code-textarea"
              placeholder="version: '3.8'&#10;services:..."
            ></textarea>
          </div>
        </div>

        <div class="modal-footer">
          <button class="btn-secondary" @click="showCustomModal = false">Cancel</button>
          <button
            class="btn-primary"
            :disabled="saving"
            @click="handleSaveCustomTemplate"
          >
            {{ saving ? 'Saving Template...' : (customModalMode === 'create' ? 'Create Template' : 'Update Template') }}
          </button>
        </div>
      </div>
    </ModalDrawer>
  </div>
</template>

<style scoped>
.scaffolder-view {
  display: flex;
  flex-direction: column;
  gap: 24px;
  padding: 24px;
  width: 100%;
  max-width: 1600px;
  margin: 0 auto;
}

/* Toast Banner */
.toast-banner {
  position: fixed;
  top: 24px;
  right: 24px;
  z-index: 9999;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 20px;
  border-radius: 12px;
  font-weight: 500;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.6);
  animation: slideIn 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}
.toast-success {
  background: rgba(16, 185, 129, 0.95);
  color: #fff;
  border: 1px solid rgba(255, 255, 255, 0.2);
}
.toast-error {
  background: rgba(244, 63, 94, 0.95);
  color: #fff;
  border: 1px solid rgba(255, 255, 255, 0.2);
}

/* Header */
.page-header {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.header-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
}
.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}
.header-icon-badge {
  font-size: 32px;
  width: 56px;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(56, 189, 248, 0.12);
  border: 1px solid var(--border-accent);
  border-radius: 14px;
}
.header-title {
  font-size: 24px;
  font-weight: 800;
  letter-spacing: -0.03em;
  background: var(--grad-cyan);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}
.header-sub {
  color: var(--text-secondary);
  font-size: 13px;
  margin-top: 4px;
}
.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

/* Filter Bar */
.filter-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--border-subtle);
}
.category-tabs {
  display: flex;
  align-items: center;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 4px;
}
.tab-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  font-weight: 600;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s ease;
  white-space: nowrap;
}
.tab-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary);
}
.tab-btn.active {
  background: var(--grad-cyan);
  color: #fff;
  border-color: transparent;
  box-shadow: 0 4px 12px rgba(6, 182, 212, 0.35);
}

.search-box {
  position: relative;
  min-width: 280px;
}
.search-icon {
  position: absolute;
  left: 12px;
  top: 50%;
  transform: translateY(-50%);
  font-size: 14px;
  color: var(--text-muted);
}
.search-input {
  width: 100%;
  padding: 8px 36px 8px 36px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  color: var(--text-primary);
  font-size: 13px;
  outline: none;
  transition: border-color 0.2s ease;
}
.search-input:focus {
  border-color: var(--accent-cyan);
}
.search-clear {
  position: absolute;
  right: 12px;
  top: 50%;
  transform: translateY(-50%);
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
}

/* Gallery Grid */
.gallery-container {
  min-height: 400px;
}
.templates-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 20px;
}
.template-card {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: 24px;
  border-radius: 16px;
  border: 1px solid var(--border-subtle);
  background: var(--bg-card);
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  overflow: hidden;
}
.template-card:hover {
  transform: translateY(-4px);
  border-color: var(--border-accent);
  box-shadow: 0 12px 30px -10px rgba(6, 182, 212, 0.25);
  background: var(--bg-card-hover);
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 16px;
}
.framework-avatar {
  font-size: 32px;
  width: 52px;
  height: 52px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-medium);
  border-radius: 14px;
}
.card-badges {
  display: flex;
  gap: 6px;
  align-items: center;
}
.template-name {
  font-size: 18px;
  font-weight: 700;
  color: var(--text-primary);
  margin-bottom: 4px;
}
.template-framework {
  font-size: 12px;
  font-family: var(--font-mono);
  color: var(--accent-cyan);
  margin-bottom: 10px;
}
.template-desc {
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.5;
  margin-bottom: 16px;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.template-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 20px;
}
.tag-pill {
  font-size: 11px;
  font-family: var(--font-mono);
  background: rgba(255, 255, 255, 0.05);
  color: var(--text-muted);
  padding: 2px 8px;
  border-radius: 6px;
  border: 1px solid var(--border-subtle);
}

.card-footer {
  display: flex;
  align-items: center;
  gap: 10px;
  padding-top: 16px;
  border-top: 1px solid var(--border-subtle);
}
.btn-deploy {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  background: var(--grad-cyan);
  color: #fff;
  border: none;
  padding: 10px 16px;
  border-radius: 10px;
  font-weight: 700;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s ease;
  box-shadow: 0 4px 14px rgba(6, 182, 212, 0.3);
}
.btn-deploy:hover {
  filter: brightness(1.1);
  transform: translateY(-1px);
}
.custom-actions {
  display: flex;
  gap: 6px;
}

/* Badges */
.badge {
  font-size: 10px;
  font-weight: 700;
  padding: 3px 8px;
  border-radius: 6px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.badge-cyan { background: rgba(6, 182, 212, 0.15); color: #38bdf8; border: 1px solid rgba(6, 182, 212, 0.3); }
.badge-indigo { background: rgba(99, 102, 241, 0.15); color: #818cf8; border: 1px solid rgba(99, 102, 241, 0.3); }
.badge-amber { background: rgba(245, 158, 11, 0.15); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.3); }
.badge-emerald { background: rgba(16, 185, 129, 0.15); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.3); }
.badge-violet { background: rgba(139, 92, 246, 0.15); color: #a78bfa; border: 1px solid rgba(139, 92, 246, 0.3); }
.badge-slate { background: rgba(100, 116, 139, 0.15); color: #94a3b8; border: 1px solid rgba(100, 116, 139, 0.3); }
.badge-system { background: rgba(255, 255, 255, 0.08); color: var(--text-primary); border: 1px solid var(--border-medium); }
.badge-custom { background: rgba(236, 72, 153, 0.15); color: #f472b6; border: 1px solid rgba(236, 72, 153, 0.3); }

/* Buttons */
.btn-primary {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  background: var(--grad-cyan);
  color: #fff;
  border: none;
  padding: 9px 18px;
  border-radius: 10px;
  font-weight: 600;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s ease;
}
.btn-primary:hover:not(:disabled) {
  filter: brightness(1.1);
}
.btn-secondary {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  background: rgba(255, 255, 255, 0.06);
  color: var(--text-primary);
  border: 1px solid var(--border-subtle);
  padding: 9px 18px;
  border-radius: 10px;
  font-weight: 600;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s ease;
}
.btn-secondary:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.1);
}
.btn-icon-action {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  color: var(--text-primary);
  cursor: pointer;
  transition: all 0.2s ease;
}
.btn-icon-action:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.12);
}
.btn-icon-danger:hover:not(:disabled) {
  background: rgba(244, 63, 94, 0.2);
  border-color: rgba(244, 63, 94, 0.4);
}
.btn-block {
  width: 100%;
}
.btn-sm {
  padding: 6px 12px;
  font-size: 12px;
}

/* Wizard Modal Layout */
.wizard-modal-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.wizard-summary {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 16px;
}
.summary-avatar {
  font-size: 36px;
  width: 56px;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-medium);
  border-radius: 12px;
}
.summary-info h4 {
  font-size: 16px;
  font-weight: 700;
}
.summary-info p {
  font-size: 12px;
  color: var(--text-secondary);
  margin: 2px 0 6px 0;
}
.summary-meta {
  display: flex;
  gap: 8px;
}

.wizard-split-layout {
  display: grid;
  grid-template-columns: 1fr 1.3fr;
  gap: 24px;
}
@media (max-width: 960px) {
  .wizard-split-layout {
    grid-template-columns: 1fr;
  }
}

.wizard-form-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.section-title {
  font-size: 15px;
  font-weight: 700;
}
.section-sub {
  font-size: 12px;
  color: var(--text-secondary);
}
.variables-form-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.form-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}
.required-star {
  color: var(--accent-rose);
}
.form-input {
  width: 100%;
  padding: 8px 12px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  color: var(--text-primary);
  font-size: 13px;
  outline: none;
  transition: border-color 0.2s ease;
}
.form-input:focus {
  border-color: var(--accent-cyan);
}

.catalog-integration-box {
  padding: 14px;
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  user-select: none;
}
.catalog-subfields {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--border-subtle);
}

/* Wizard Output Code Section */
.wizard-output-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.output-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.output-tabs {
  display: flex;
  gap: 6px;
}
.output-tab-btn {
  padding: 6px 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}
.output-tab-btn.active {
  background: rgba(56, 189, 248, 0.15);
  border-color: var(--border-accent);
  color: var(--accent-cyan);
}
.code-editor-viewer {
  min-height: 360px;
  max-height: 480px;
  overflow: auto;
  padding: 16px;
  background: var(--bg-terminal);
  border-radius: 12px;
  border: 1px solid var(--border-medium);
}
.code-content {
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.6;
  color: #38bdf8;
  margin: 0;
  white-space: pre;
}
.code-placeholder {
  height: 320px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  color: var(--text-muted);
  gap: 12px;
  padding: 24px;
}
.placeholder-icon {
  font-size: 40px;
}
.catalog-success-alert {
  padding: 10px 14px;
  font-size: 12px;
  color: #34d399;
  border-left: 3px solid #10b981;
}

/* Custom Template Modal Styles */
.custom-template-modal {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.form-row {
  display: flex;
  gap: 12px;
}
.flex-1 {
  flex: 1;
}
.variable-builder-section {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.builder-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.var-builder-row {
  display: grid;
  grid-template-columns: 1.2fr 1.2fr 1fr 1.2fr auto auto;
  gap: 8px;
  align-items: center;
}
.code-textarea {
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.4;
  resize: vertical;
}
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding-top: 16px;
  border-top: 1px solid var(--border-subtle);
}

/* Utilities */
.spinner {
  width: 32px;
  height: 32px;
  border: 3px solid rgba(255, 255, 255, 0.1);
  border-top-color: var(--accent-cyan);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  margin: 0 auto 12px auto;
}
.spin-anim {
  animation: spin 0.8s linear infinite;
}
@keyframes spin {
  to { transform: rotate(360deg); }
}
.loading-state, .empty-state {
  text-align: center;
  padding: 60px 24px;
}
.empty-icon {
  font-size: 48px;
  margin-bottom: 12px;
}
</style>
