import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import {
  settingsApi,
  apiKeysApi,
  type Setting,
  type SettingUpdate,
  type IntegrationTestResult,
  type ApiKeyItem,
} from '../api/settings'
import { authApi, type TOTPStatusResponse } from '../api/auth'

export type TabKey = 'general' | 'security' | 'tenancy' | 'notifications' | 'apikeys' | 'telemetry' | 'backup' | 'integrations' | 'about'
export type CategoryKey = 'platform' | 'security' | 'tenancy' | 'notifications' | 'apikeys' | 'telemetry' | 'backup' | 'integrations'

export interface SettingsTabItem {
  id: TabKey
  label: string
  icon: string
  category?: CategoryKey
  count?: number
}

export function buildSettingsTabsList(keys: ApiKeyItem[] = []): SettingsTabItem[] {
  return [
    { id: 'general', label: 'General', icon: 'sliders', category: 'platform' },
    { id: 'security', label: 'Security', icon: 'shield', category: 'security' },
    { id: 'tenancy', label: 'Tenancy', icon: 'server', category: 'tenancy' },
    { id: 'notifications', label: 'Notifications', icon: 'bell', category: 'notifications' },
    { id: 'apikeys', label: 'API Keys', icon: 'key', category: 'apikeys', count: keys.length },
    { id: 'backup', label: 'Backup & DR', icon: 'archive', category: 'backup' },
    { id: 'about', label: 'About', icon: 'help-circle' },
  ]
}

export interface TestState {
  testing: boolean
  result: IntegrationTestResult | null
  error?: string
}

export interface SettingsFormState {
  // General / Platform
  name: string
  environment: 'production' | 'staging' | 'development' | 'edge'
  maintenance_mode: boolean
  timezone: string
  language: string
  // Security & Access
  require_2fa: boolean
  session_timeout_minutes: number
  jwt_session_duration_hours: number
  password_min_length: number
  rate_limit_enabled: boolean
  rate_limit_requests_per_min: number
  rate_limit_burst: number
  ip_allowlist: string
  // Telemetry & Notifications
  prometheus_endpoint: string
  prometheus_scrape_interval_sec: number
  loki_endpoint: string
  loki_retention_days: number
  alertmanager_endpoint: string
  alertmanager_webhook_url: string
  smtp_enabled: boolean
  smtp_host: string
  smtp_port: number
  webhook_url: string
  // Disaster Recovery & Automated Backups
  backup_provider: 's3' | 'minio' | 'local_nvme' | 'gcs'
  backup_s3_endpoint: string
  backup_s3_bucket: string
  backup_s3_region: string
  backup_schedule_cron: string
  backup_retention_days: number
  backup_encryption_enabled: boolean
  backup_compression_level: 'none' | 'fast' | 'high'
  backup_auto_verify: boolean
  // DevOps Toolchain Integrations
  argocd_url: string
  trivy_url: string
  vault_url: string
  grafana_url: string
  // Tenancy & API Keys
  isolation_mode: string
  default_cpu_limit: string
  default_memory_limit: string
  default_storage_limit: string
  strict_network_isolation: boolean
  auto_provision_ingress: boolean
  apikeys_rotation_days: number
  apikeys_require_expiry: boolean
}

export const timezoneOptions = [
  { value: 'UTC', label: 'UTC — Coordinated Universal Time' },
  { value: 'Asia/Ho_Chi_Minh', label: 'Asia/Ho_Chi_Minh — Indochina Time (UTC+7)' },
  { value: 'Asia/Singapore', label: 'Asia/Singapore — Singapore Time (UTC+8)' },
  { value: 'Asia/Tokyo', label: 'Asia/Tokyo — Japan Standard Time (UTC+9)' },
  { value: 'Europe/London', label: 'Europe/London — Greenwich Mean Time (UTC+0/+1)' },
  { value: 'Europe/Berlin', label: 'Europe/Berlin — Central European Time (UTC+1/+2)' },
  { value: 'America/New_York', label: 'America/New_York — Eastern Time (UTC-5/-4)' },
  { value: 'America/Chicago', label: 'America/Chicago — Central Time (UTC-6/-5)' },
  { value: 'America/Los_Angeles', label: 'America/Los_Angeles — Pacific Time (UTC-8/-7)' },
  { value: 'Australia/Sydney', label: 'Australia/Sydney — Eastern Australia (UTC+10/+11)' },
]

export const languageOptions = [
  { value: 'en', label: 'English (United States)' }, { value: 'vi', label: 'Vietnamese (Vietnam)' },
]

export const environmentOptions = [
  { value: 'production', label: 'PROD — High Availability & Strict SLA', badgeColor: 'rose' },
  { value: 'staging', label: 'STAGE — Pre-Production Verification', badgeColor: 'amber' },
  { value: 'development', label: 'DEV — Rapid Iteration & Sandbox', badgeColor: 'cyan' },
  { value: 'edge', label: 'EDGE — Distributed Gateway Node', badgeColor: 'emerald' },
]

export const backupProviderOptions = [
  { id: 'minio', name: 'MinIO Self-Host', icon: 'database', desc: 'On-premise S3-compatible high-speed blob storage' },
  { id: 's3', name: 'AWS S3 Glacier', icon: 'cloud', desc: 'Cloud multi-AZ replicated object vault' },
  { id: 'local_nvme', name: 'Local NVMe Direct', icon: 'hard-drive', desc: 'Zero-latency local cluster volume backup' },
  { id: 'gcs', name: 'Google Cloud Storage', icon: 'globe', desc: 'Enterprise GCS Nearline/Coldline bucket' },
]

const initialDefaults: SettingsFormState = {
  name: 'K8s Self-Host Platform', environment: 'production', maintenance_mode: false, timezone: 'UTC', language: 'en',
  require_2fa: false, session_timeout_minutes: 60, jwt_session_duration_hours: 8, password_min_length: 8,
  rate_limit_enabled: true, rate_limit_requests_per_min: 600, rate_limit_burst: 120, ip_allowlist: '',
  prometheus_endpoint: 'http://prometheus-k8s.monitoring.svc:9090', prometheus_scrape_interval_sec: 15,
  loki_endpoint: 'http://loki-gateway.logging.svc:3100', loki_retention_days: 30,
  alertmanager_endpoint: 'http://alertmanager.monitoring.svc:9093', alertmanager_webhook_url: '',
  smtp_enabled: false, smtp_host: '', smtp_port: 587, webhook_url: '',
  backup_provider: 'minio', backup_s3_endpoint: 'http://minio.backup.svc:9000', backup_s3_bucket: 'k8s-platform-snapshots',
  backup_s3_region: 'us-east-1', backup_schedule_cron: '0 2 * * *', backup_retention_days: 30,
  backup_encryption_enabled: true, backup_compression_level: 'fast', backup_auto_verify: true,
  argocd_url: '', trivy_url: '', vault_url: '', grafana_url: '',
  isolation_mode: 'namespace', default_cpu_limit: '16', default_memory_limit: '32', default_storage_limit: '200',
  strict_network_isolation: true, auto_provision_ingress: false, apikeys_rotation_days: 90, apikeys_require_expiry: true,
}

export function useSettings() {
  const activeTab = ref<TabKey>('general')
  const loading = ref(true)
  const saving = ref(false)
  const statusMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)
  let messageTimer: ReturnType<typeof setTimeout> | null = null

  // Reactive state
  const form = reactive<SettingsFormState>({ ...initialDefaults })
  const initialForm = reactive<SettingsFormState>({ ...initialDefaults })
  const defaultSettings = ref<Record<string, Record<string, string>>>({})

  // 2FA TOTP State
  const totpStatus = ref<TOTPStatusResponse | null>(null)
  const loadingTotpStatus = ref(false)
  const showDisable2FAModal = ref(false)
  const disablePassword = ref('')
  const disableTotpCode = ref('')
  const disabling2FA = ref(false)
  const disableError = ref('')

  // Reachability Tests
  const integrationTests = reactive<Record<string, TestState>>({
    argocd_url: { testing: false, result: null },
    trivy_url: { testing: false, result: null },
    vault_url: { testing: false, result: null },
    grafana_url: { testing: false, result: null },
    prometheus_endpoint: { testing: false, result: null },
    loki_endpoint: { testing: false, result: null },
    alertmanager_endpoint: { testing: false, result: null },
    backup_s3_endpoint: { testing: false, result: null },
  })

  function showMessage(text: string, type: 'success' | 'error' = 'success') {
    if (messageTimer) clearTimeout(messageTimer)
    statusMessage.value = { text, type }
    messageTimer = setTimeout(() => {
      statusMessage.value = null
    }, 4000)
  }

  function clearMessage() {
    if (messageTimer) clearTimeout(messageTimer)
    statusMessage.value = null
  }

  // Dirty state tracking
  const categoryFieldMap: Record<CategoryKey, (keyof SettingsFormState)[]> = {
    platform: ['name', 'environment', 'maintenance_mode', 'timezone', 'language'],
    security: ['require_2fa', 'session_timeout_minutes', 'jwt_session_duration_hours', 'password_min_length', 'rate_limit_enabled', 'rate_limit_requests_per_min', 'rate_limit_burst', 'ip_allowlist'],
    telemetry: ['prometheus_endpoint', 'prometheus_scrape_interval_sec', 'loki_endpoint', 'loki_retention_days', 'alertmanager_endpoint', 'alertmanager_webhook_url'],
    notifications: ['smtp_enabled', 'smtp_host', 'smtp_port', 'webhook_url'],
    tenancy: ['isolation_mode', 'default_cpu_limit', 'default_memory_limit', 'default_storage_limit', 'strict_network_isolation', 'auto_provision_ingress'],
    apikeys: ['apikeys_rotation_days', 'apikeys_require_expiry'],
    backup: ['backup_provider', 'backup_s3_endpoint', 'backup_s3_bucket', 'backup_s3_region', 'backup_schedule_cron', 'backup_retention_days', 'backup_encryption_enabled', 'backup_compression_level', 'backup_auto_verify'],
    integrations: ['argocd_url', 'trivy_url', 'vault_url', 'grafana_url'],
  }

  const dirtyFields = computed(() => {
    const keys = Object.keys(form) as (keyof SettingsFormState)[]
    return keys.filter(k => form[k] !== initialForm[k])
  })

  const isDirty = computed(() => dirtyFields.value.length > 0)
  const dirtyCount = computed(() => dirtyFields.value.length)

  function isDirtyCategory(category: CategoryKey): boolean {
    const fields = categoryFieldMap[category] || []
    return fields.some(f => form[f] !== initialForm[f])
  }

  function commitCategoryState(category: CategoryKey) {
    const fields = categoryFieldMap[category] || []
    for (const f of fields) {
      ;(initialForm as any)[f] = (form as any)[f]
    }
  }

  function commitAllState() {
    Object.assign(initialForm, JSON.parse(JSON.stringify(form)))
  }

  // Computed Metrics
  const configuredIntegrationsCount = computed(() => {
    let count = 0
    if (form.argocd_url?.trim()) count++
    if (form.trivy_url?.trim()) count++
    if (form.vault_url?.trim()) count++
    if (form.grafana_url?.trim()) count++
    return count
  })

  const activeSecurityPolicyCount = computed(() => {
    let count = 0
    if (form.require_2fa) count++
    if (form.rate_limit_enabled) count++
    if (form.session_timeout_minutes <= 60) count++
    if (form.password_min_length >= 12) count++
    return count
  })

  const telemetryHealthStatus = computed(() => {
    if (form.prometheus_endpoint && form.loki_endpoint) return 'ACTIVE'
    if (form.prometheus_endpoint || form.loki_endpoint) return 'PARTIAL'
    return 'UNCONFIGURED'
  })

  const backupStatusSummary = computed(() => {
    return `${form.backup_provider.toUpperCase()} | ${form.backup_retention_days}d Retention`
  })

  // TOTP Actions
  async function fetchTOTPStatus() {
    loadingTotpStatus.value = true
    try {
      const res = await authApi.getTOTPStatus()
      totpStatus.value = res
    } catch {
      totpStatus.value = { enabled: false, verified_at: null }
    } finally {
      loadingTotpStatus.value = false
    }
  }

  async function handleDisable2FA() {
    if (!disablePassword.value || !disableTotpCode.value) return
    disabling2FA.value = true
    disableError.value = ''
    try {
      await authApi.disableTOTP(disablePassword.value, disableTotpCode.value)
      showDisable2FAModal.value = false
      disablePassword.value = ''
      disableTotpCode.value = ''
      showMessage('Two-factor authentication disabled successfully', 'success')
      await fetchTOTPStatus()
    } catch (err: unknown) {
      disableError.value = err instanceof Error ? err.message : 'Failed to disable 2FA. Please verify credentials and code.'
    } finally {
      disabling2FA.value = false
    }
  }

  // Populate Form
  function populateFormFromSettings(settingsList: Setting[]) {
    for (const item of settingsList) {
      const val = item.value
      const k = item.key as keyof SettingsFormState
      if (!(k in form)) continue
      const defaultVal = (initialDefaults as any)[k]
      if (typeof defaultVal === 'boolean') {
        ;(form as any)[k] = val === 'false' ? false : (val === 'true' ? true : defaultVal)
      } else if (typeof defaultVal === 'number') {
        const parsed = parseInt(val, 10)
        ;(form as any)[k] = isNaN(parsed) ? defaultVal : parsed
      } else {
        ;(form as any)[k] = val || defaultVal
      }
    }
  }

  // API Keys state
  const apiKeys = ref<ApiKeyItem[]>([])
  const loadingApiKeys = ref(false)
  const apiKeysError = ref<string | null>(null)

  async function loadApiKeys() {
    loadingApiKeys.value = true
    apiKeysError.value = null
    try {
      const res = await apiKeysApi.list()
      apiKeys.value = Array.isArray(res) ? res : []
    } catch (err: unknown) {
      apiKeys.value = []
      apiKeysError.value = err instanceof Error ? err.message : 'Failed to retrieve API keys'
    } finally {
      loadingApiKeys.value = false
    }
  }

  // Load Settings
  async function loadSettings() {
    loading.value = true
    try {
      let defaults: Record<string, Record<string, string>> = {}
      try {
        defaults = await settingsApi.getDefaults()
      } catch {
        defaults = {}
      }
      defaultSettings.value = defaults

      const [settingsList] = await Promise.all([
        settingsApi.getAll(),
        loadApiKeys().catch(() => {})
      ])
      if (Array.isArray(settingsList) && settingsList.length > 0) {
        populateFormFromSettings(settingsList)
      }
      commitAllState()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load platform settings'
      showMessage(msg, 'error')
    } finally {
      loading.value = false
    }
  }

  // Save Category
  async function saveCategory(category: CategoryKey, extraConfig?: Record<string, unknown>) {
    saving.value = true
    try {
      if (extraConfig && typeof extraConfig === 'object') {
        for (const [k, v] of Object.entries(extraConfig)) {
          if (k in form) (form as any)[k] = v
        }
      }
      const updates: SettingUpdate[] = []
      const fields = categoryFieldMap[category] || []
      for (const field of fields) {
        const val = form[field]
        updates.push({
          category,
          key: field,
          value: typeof val === 'boolean' ? (val ? 'true' : 'false') : String(val ?? ''),
        })
      }

      await settingsApi.update(updates)
      commitCategoryState(category)
      showMessage(`${category.toUpperCase()} settings saved successfully`, 'success')
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : `Failed to save ${category} settings`
      showMessage(msg, 'error')
    } finally {
      saving.value = false
    }
  }

  async function saveAllSettings() {
    saving.value = true
    try {
      const updates: SettingUpdate[] = []
      const categories: CategoryKey[] = ['platform', 'security', 'telemetry', 'notifications', 'backup', 'integrations']
      for (const cat of categories) {
        const fields = categoryFieldMap[cat]
        for (const field of fields) {
          const val = form[field]
          updates.push({
            category: cat,
            key: field,
            value: typeof val === 'boolean' ? (val ? 'true' : 'false') : String(val ?? ''),
          })
        }
      }

      await settingsApi.update(updates)
      commitAllState()
      showMessage('All platform settings saved successfully', 'success')
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to save settings'
      showMessage(msg, 'error')
    } finally {
      saving.value = false
    }
  }

  function resetCategoryToDefaults(category: CategoryKey) {
    const fields = categoryFieldMap[category] || []
    for (const f of fields) {
      ;(form as any)[f] = (initialDefaults as any)[f]
    }
    showMessage(`Reset ${category} settings to defaults. Click Save to persist.`, 'success')
  }

  // Reachability Tests
  async function testService(key: string, customUrl?: string) {
    const url = customUrl || (form as any)[key]
    if (!url || !String(url).trim()) {
      if (!integrationTests[key]) {
        integrationTests[key] = { testing: false, result: null }
      }
      integrationTests[key] = {
        testing: false,
        result: null,
        error: 'Please enter a valid URL before testing connectivity',
      }
      return
    }

    if (!integrationTests[key]) {
      integrationTests[key] = { testing: false, result: null }
    }
    integrationTests[key].testing = true
    integrationTests[key].result = null
    integrationTests[key].error = undefined

    try {
      const res = await settingsApi.testIntegration(String(url).trim())
      integrationTests[key] = {
        testing: false,
        result: res,
      }
    } catch (err: unknown) {
      integrationTests[key] = {
        testing: false,
        result: null,
        error: err instanceof Error ? err.message : 'Reachability check failed',
      }
    }
  }

  onMounted(() => {
    loadSettings()
    fetchTOTPStatus()
    loadApiKeys()
  })

  onUnmounted(() => {
    if (messageTimer) clearTimeout(messageTimer)
  })

  return {
    activeTab, form, initialForm, defaultSettings,
    loading, saving, statusMessage, showMessage, clearMessage,
    isDirty, dirtyFields, dirtyCount, isDirtyCategory,
    saveCategory, saveAllSettings, resetCategoryToDefaults, loadSettings,
    totpStatus, loadingTotpStatus, showDisable2FAModal, disablePassword,
    disableTotpCode, disabling2FA, disableError, fetchTOTPStatus, handleDisable2FA,
    integrationTests, testService, configuredIntegrationsCount,
    activeSecurityPolicyCount, telemetryHealthStatus, backupStatusSummary,
    apiKeys, loadingApiKeys, apiKeysError, loadApiKeys,
  }
}

export type UseSettingsReturn = ReturnType<typeof useSettings>
