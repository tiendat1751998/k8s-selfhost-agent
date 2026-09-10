import { ref, reactive, computed, onMounted } from 'vue'
import {
  settingsApi,
  type Setting,
  type SettingUpdate,
  type IntegrationTestResult,
} from '../api/settings'
import { authApi, type TOTPStatusResponse } from '../api/auth'

export type TabKey = 'general' | 'security' | 'tenancy' | 'notifications' | 'apikeys' | 'telemetry' | 'backup' | 'integrations' | 'about'
export type CategoryKey = 'platform' | 'security' | 'tenancy' | 'notifications' | 'apikeys' | 'telemetry' | 'backup' | 'integrations'

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
}

export const timezoneOptions = [
  { value: 'UTC', label: '🌐 UTC — Coordinated Universal Time' },
  { value: 'Asia/Ho_Chi_Minh', label: '🇻🇳 Asia/Ho_Chi_Minh — Indochina Time (UTC+7)' },
  { value: 'Asia/Singapore', label: '🇸🇬 Asia/Singapore — Singapore Time (UTC+8)' },
  { value: 'Asia/Tokyo', label: '🇯🇵 Asia/Tokyo — Japan Standard Time (UTC+9)' },
  { value: 'Europe/London', label: '🇬🇧 Europe/London — Greenwich Mean Time (UTC+0/+1)' },
  { value: 'Europe/Berlin', label: '🇩🇪 Europe/Berlin — Central European Time (UTC+1/+2)' },
  { value: 'America/New_York', label: '🇺🇸 America/New_York — Eastern Time (UTC-5/-4)' },
  { value: 'America/Chicago', label: '🇺🇸 America/Chicago — Central Time (UTC-6/-5)' },
  { value: 'America/Los_Angeles', label: '🇺🇸 America/Los_Angeles — Pacific Time (UTC-8/-7)' },
  { value: 'Australia/Sydney', label: '🇦🇺 Australia/Sydney — Eastern Australia (UTC+10/+11)' },
]

export const languageOptions = [
  { value: 'en', label: '🇺🇸 English (United States)' },
  { value: 'vi', label: '🇻🇳 Vietnamese (Vietnam)' },
]

export const environmentOptions = [
  { value: 'production', label: 'PROD — High Availability & Strict SLA', badgeColor: 'rose' },
  { value: 'staging', label: 'STAGE — Pre-Production Verification', badgeColor: 'amber' },
  { value: 'development', label: 'DEV — Rapid Iteration & Sandbox', badgeColor: 'cyan' },
  { value: 'edge', label: 'EDGE — Distributed Gateway Node', badgeColor: 'emerald' },
]

export const backupProviderOptions = [
  { id: 'minio', name: 'MinIO Self-Host', icon: '🗄️', desc: 'On-premise S3-compatible high-speed blob storage' },
  { id: 's3', name: 'AWS S3 Glacier', icon: '☁️', desc: 'Cloud multi-AZ replicated object vault' },
  { id: 'local_nvme', name: 'Local NVMe Direct', icon: '⚡', desc: 'Zero-latency local cluster volume backup' },
  { id: 'gcs', name: 'Google Cloud Storage', icon: '🌐', desc: 'Enterprise GCS Nearline/Coldline bucket' },
]

const initialDefaults: SettingsFormState = {
  name: 'K8s Self-Host Platform',
  environment: 'production',
  maintenance_mode: false,
  timezone: 'UTC',
  language: 'en',
  require_2fa: false,
  session_timeout_minutes: 60,
  jwt_session_duration_hours: 8,
  password_min_length: 8,
  rate_limit_enabled: true,
  rate_limit_requests_per_min: 600,
  rate_limit_burst: 120,
  ip_allowlist: '',
  prometheus_endpoint: 'http://prometheus-k8s.monitoring.svc:9090',
  prometheus_scrape_interval_sec: 15,
  loki_endpoint: 'http://loki-gateway.logging.svc:3100',
  loki_retention_days: 30,
  alertmanager_endpoint: 'http://alertmanager.monitoring.svc:9093',
  alertmanager_webhook_url: '',
  smtp_enabled: false,
  smtp_host: '',
  smtp_port: 587,
  webhook_url: '',
  backup_provider: 'minio',
  backup_s3_endpoint: 'http://minio.backup.svc:9000',
  backup_s3_bucket: 'k8s-platform-snapshots',
  backup_s3_region: 'us-east-1',
  backup_schedule_cron: '0 2 * * *',
  backup_retention_days: 30,
  backup_encryption_enabled: true,
  backup_compression_level: 'fast',
  backup_auto_verify: true,
  argocd_url: '',
  trivy_url: '',
  vault_url: '',
  grafana_url: '',
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
    tenancy: [],
    apikeys: [],
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
      switch (item.key) {
        // Platform
        case 'name': form.name = val || 'K8s Self-Host Platform'; break
        case 'environment': form.environment = (val as any) || 'production'; break
        case 'maintenance_mode': form.maintenance_mode = val === 'true'; break
        case 'timezone': form.timezone = val || 'UTC'; break
        case 'language': form.language = val || 'en'; break

        // Security
        case 'session_timeout_minutes': form.session_timeout_minutes = parseInt(val, 10) || 60; break
        case 'jwt_session_duration_hours': form.jwt_session_duration_hours = parseInt(val, 10) || 8; break
        case 'password_min_length': form.password_min_length = parseInt(val, 10) || 8; break
        case 'require_2fa': form.require_2fa = val === 'true'; break
        case 'rate_limit_enabled': form.rate_limit_enabled = val !== 'false'; break
        case 'rate_limit_requests_per_min': form.rate_limit_requests_per_min = parseInt(val, 10) || 600; break
        case 'rate_limit_burst': form.rate_limit_burst = parseInt(val, 10) || 120; break
        case 'ip_allowlist': form.ip_allowlist = val || ''; break

        // Telemetry
        case 'prometheus_endpoint': form.prometheus_endpoint = val || ''; break
        case 'prometheus_scrape_interval_sec': form.prometheus_scrape_interval_sec = parseInt(val, 10) || 15; break
        case 'loki_endpoint': form.loki_endpoint = val || ''; break
        case 'loki_retention_days': form.loki_retention_days = parseInt(val, 10) || 30; break
        case 'alertmanager_endpoint': form.alertmanager_endpoint = val || ''; break
        case 'alertmanager_webhook_url': form.alertmanager_webhook_url = val || ''; break

        // Notifications
        case 'smtp_enabled': form.smtp_enabled = val === 'true'; break
        case 'smtp_host': form.smtp_host = val || ''; break
        case 'smtp_port': form.smtp_port = parseInt(val, 10) || 587; break
        case 'webhook_url': form.webhook_url = val || ''; break

        // Backup
        case 'backup_provider': form.backup_provider = (val as any) || 'minio'; break
        case 'backup_s3_endpoint': form.backup_s3_endpoint = val || ''; break
        case 'backup_s3_bucket': form.backup_s3_bucket = val || ''; break
        case 'backup_s3_region': form.backup_s3_region = val || 'us-east-1'; break
        case 'backup_schedule_cron': form.backup_schedule_cron = val || '0 2 * * *'; break
        case 'backup_retention_days': form.backup_retention_days = parseInt(val, 10) || 30; break
        case 'backup_encryption_enabled': form.backup_encryption_enabled = val !== 'false'; break
        case 'backup_compression_level': form.backup_compression_level = (val as any) || 'fast'; break
        case 'backup_auto_verify': form.backup_auto_verify = val !== 'false'; break

        // Integrations
        case 'argocd_url': form.argocd_url = val || ''; break
        case 'trivy_url': form.trivy_url = val || ''; break
        case 'vault_url': form.vault_url = val || ''; break
        case 'grafana_url': form.grafana_url = val || ''; break
      }
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

      const settingsList = await settingsApi.getAll()
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
  async function saveCategory(category: CategoryKey) {
    saving.value = true
    try {
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
  })

  return {
    activeTab,
    form,
    initialForm,
    defaultSettings,
    loading,
    saving,
    statusMessage,
    showMessage,
    clearMessage,
    isDirty,
    dirtyFields,
    dirtyCount,
    isDirtyCategory,
    saveCategory,
    saveAllSettings,
    resetCategoryToDefaults,
    loadSettings,
    totpStatus,
    loadingTotpStatus,
    showDisable2FAModal,
    disablePassword,
    disableTotpCode,
    disabling2FA,
    disableError,
    fetchTOTPStatus,
    handleDisable2FA,
    integrationTests,
    testService,
    configuredIntegrationsCount,
    activeSecurityPolicyCount,
    telemetryHealthStatus,
    backupStatusSummary,
  }
}

export type UseSettingsReturn = ReturnType<typeof useSettings>
