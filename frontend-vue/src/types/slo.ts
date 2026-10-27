// ==========================================
// Observability & SLO Domain Types (Enterprise)
// ==========================================

export type TimeWindowFilter = '1h' | '6h' | '24h' | '7d' | '30d'

export interface ServiceOption {
  id: string
  name: string
  desc: string
}

export interface LatencyPercentiles {
  p50: number
  p90: number
  p99: number
  p999: number
}

export interface BannerMessage {
  type: 'success' | 'warning' | 'error'
  text: string
}

export interface SLODefinition {
  id: string
  service: string
  target: number
  indicator_type: 'availability' | 'latency' | 'error_rate' | 'cache_hit_rate' | string
  window: string
  query?: string
  alert_threshold?: number
  created_at: string
  updated_at?: string
}

export interface SLOSnapshot {
  id: string
  slo_id: string
  service: string
  target: number
  actual: number
  burn_rate: number
  error_budget: number
  budget_status: 'healthy' | 'warning' | 'critical' | 'unmeasured' | 'exhausted' | string
  recorded_at: string
}

export interface CreateSLOPayload {
  service: string
  target: number
  indicator_type: string
  window: string
  query?: string
  alert_threshold?: number
}

export interface UpdateSLOPayload {
  service?: string
  target?: number
  indicator_type?: string
  window?: string
  query?: string
  alert_threshold?: number
}

export interface InspectSLOState {
  def?: SLODefinition
  snap?: SLOSnapshot
}

export interface SLOFormState {
  id?: string
  selectedService: string
  customServiceName: string
  indicator_type: 'availability' | 'latency' | 'error_rate' | 'cache_hit_rate' | string
  target: number
  window: string
  query: string
  alert_threshold: number
  target_latency_p99_ms?: number
}

export interface EnrichedSLOItem {
  id: string
  service: string
  target: number
  actual: number
  error_budget: number
  burn_rate: number
  alert_threshold: number
  budget_status: 'healthy' | 'warning' | 'critical' | 'unmeasured' | 'exhausted' | string
  indicator_type: string
  window: string
  query: string
  isFastBurn: boolean
  isExhausted: boolean
  isUnmeasured: boolean
  def: SLODefinition
  snap?: SLOSnapshot
}

export interface FleetReliabilityPulse {
  monitoredCount: number
  healthyCount: number
  warningCount: number
  criticalCount: number
  fastBurnCount: number
  exhaustedCount: number
  unmeasuredCount: number
  avgBudgetRemainingPct: number
}
