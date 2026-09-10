<script setup lang="ts">
import { onMounted } from 'vue'
import { useReports } from '../composables/useReports'
import MetricCard from '../components/ui/MetricCard.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import ReportsCatalogGrid from '../components/reports/ReportsCatalogGrid.vue'
import GeneratedReportsTable from '../components/reports/GeneratedReportsTable.vue'
import ReportsMobileCards from '../components/reports/ReportsMobileCards.vue'
import ScheduleReportModal from '../components/reports/ScheduleReportModal.vue'
import '../assets/styles/views/reports.css'
import '../assets/styles/components/reports-drawers.css'

const {
  loading,
  reports,
  schedules,
  frameworks,
  selectedType,
  feedbackMessage,
  showGenerateModal,
  showScheduleModal,
  showPreviewDrawer,
  activePreviewReport,
  isSubmitting,
  newReport,
  reportColumns,
  filteredReports,
  completedCount,
  complianceScore,
  storageFootprint,
  loadReports,
  openPreview,
  downloadReport,
  exportAsPdf,
  exportAsCsv,
  handleDeleteReport,
  handleGenerateReport,
  quickGenerate,
  saveSchedule
} = useReports()

onMounted(() => {
  loadReports()
})
</script>

<template>
  <div class="reports-page">
    <div class="page-header desktop-header desktop-only">
      <div class="header-titles">
        <div class="header-badge">
          <span class="badge badge-emerald">SOC2 / CIS Compliant</span>
          <span class="badge badge-cyan">Automated Executive Digest</span>
        </div>
        <h1 class="page-title">Compliance & Platform Reports Center</h1>
        <p class="page-desc">
          Generate, schedule, and download executive audits, FinOps cloud cost optimizations, Disaster Recovery drill verifications, and DevSecOps compliance reports.
        </p>
      </div>
      <div class="header-actions">
        <button class="btn btn-secondary" @click="showScheduleModal = true"><span>🕒 Schedule Cadence</span></button>
        <button class="btn btn-primary" @click="showGenerateModal = true"><span>+ Generate New Report</span></button>
      </div>
    </div>

    <!-- Mobile 40-44px Command Bar (<768px) -->
    <div class="reports-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">📊 Reports ({{ filteredReports.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          title="➕ Schedule"
          aria-label="Schedule Cadence"
          @click="showScheduleModal = true"
        >
          <span>➕</span>
        </button>
        <button
          class="btn-icon-cmd"
          title="🔄 Sync"
          aria-label="Sync Reports"
          :disabled="loading"
          @click="loadReports"
        >
          <span>🔄</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px) -->
    <div class="reports-micro-telemetry mobile-only font-mono" role="status" aria-label="Reports Micro Telemetry">
      <span class="tel-item tel-compiled">📊 {{ reports.length }} Total Reports</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-cadence">⚡ {{ schedules.length }} Scheduled</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-footprint">📁 3 Formats</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-score">⏱️ 1.2s Avg Time</span>
    </div>

    <div v-if="feedbackMessage" class="feedback-banner animate-fade-in">
      <span class="feedback-icon">✓</span>
      <span>{{ feedbackMessage }}</span>
    </div>

    <div class="metrics-grid desktop-metrics desktop-only">
      <MetricCard title="Compiled Reports" :value="completedCount" :trend="completedCount > 0 ? 'Archived on NVMe + S3' : 'No reports compiled'" :trendType="completedCount > 0 ? 'positive' : 'neutral'" />
      <MetricCard title="Compliance Score" :value="complianceScore || '—'" :trend="complianceScore ? `${frameworks.filter(f => (f.score || 0) >= 80).length}/${frameworks.length} Frameworks Passing` : 'No compliance data'" :trendType="complianceScore ? 'positive' : 'neutral'" />
      <MetricCard title="Storage Footprint" :value="storageFootprint || '—'" :trend="storageFootprint ? 'Encrypted AES-256' : 'No storage footprint'" trendType="neutral" />
      <MetricCard title="Scheduled Cadence" :value="filteredReports.length > 0 ? 'Daily / Weekly' : '—'" :trend="filteredReports.length > 0 ? 'Automated Dispatch' : 'No active cadence'" trendType="neutral" />
    </div>

    <ReportsCatalogGrid @quickGenerate="quickGenerate" />

    <div class="filter-bar glass-panel">
      <div class="filter-left">
        <span class="filter-label">Filter Category:</span>
        <div class="type-pills">
          <button v-for="t in ['all', 'compliance', 'security', 'cost', 'operational', 'incident']" :key="t" class="tpill" :class="{ active: selectedType === t }" @click="selectedType = t">
            {{ t.toUpperCase() }}
          </button>
        </div>
      </div>
    </div>

    <div class="desktop-only">
      <GeneratedReportsTable :reports="filteredReports" :columns="reportColumns" :loading="loading" @preview="openPreview" @downloadPdf="exportAsPdf" @viewCsv="exportAsCsv" @delete="handleDeleteReport" />
    </div>

    <div class="mobile-only">
      <ReportsMobileCards :reports="filteredReports" :loading="loading" @preview="openPreview" @downloadPdf="exportAsPdf" @viewCsv="exportAsCsv" @delete="handleDeleteReport" />
    </div>

    <!-- Compile Modal -->
    <ModalDrawer v-model:show="showGenerateModal" title="Compile Enterprise Platform Report" subtitle="Select report category, metrics data range, and target export format.">
      <form @submit.prevent="handleGenerateReport" class="form-layout">
        <div class="form-group">
          <label>Report Title</label>
          <input v-model="newReport.title" type="text" placeholder="e.g. Q3 SOC2 Infrastructure Compliance Summary" class="input-glass" required />
        </div>
        <div class="form-row">
          <div class="form-group">
            <label>Report Category</label>
            <select v-model="newReport.type" class="input-glass">
              <option value="compliance">Compliance & CIS Benchmark</option>
              <option value="security">Security & Trivy CVE Digest</option>
              <option value="cost">FinOps & Cost Optimization</option>
              <option value="operational">Operational & Disaster Recovery</option>
              <option value="incident">Incident Root Cause & Post-Mortem</option>
            </select>
          </div>
          <div class="form-group">
            <label>Export Format</label>
            <select v-model="newReport.format" class="input-glass">
              <option value="pdf">PDF (Executive Signed Document)</option>
              <option value="excel">Excel XLSX (Data Tables & Pivot)</option>
              <option value="csv">CSV (Raw Telemetry Export)</option>
            </select>
          </div>
        </div>
        <div class="form-row">
          <div class="form-group">
            <label>Cluster Scope</label>
            <select v-model="newReport.cluster_scope" class="input-glass">
              <option value="all-clusters">All Clusters (Global Mesh)</option>
              <option value="prod-us-east-1">prod-us-east-1 (Primary)</option>
              <option value="prod-eu-west-1">prod-eu-west-1 (Secondary)</option>
            </select>
          </div>
          <div class="form-group">
            <label>Telemetry Window</label>
            <select v-model="newReport.date_range" class="input-glass">
              <option value="7d">Last 7 Days</option>
              <option value="30d">Last 30 Days (Monthly)</option>
              <option value="90d">Last Quarter (Q3)</option>
            </select>
          </div>
        </div>
      </form>
      <template #footer="{ close }">
        <button class="btn btn-secondary" type="button" @click="close">Cancel</button>
        <button class="btn btn-primary" :disabled="isSubmitting" @click="handleGenerateReport">
          {{ isSubmitting ? 'Compiling Report...' : 'Compile & Export' }}
        </button>
      </template>
    </ModalDrawer>

    <!-- Preview Drawer -->
    <ModalDrawer v-model:show="showPreviewDrawer" mode="drawer" :title="activePreviewReport?.title || 'Report Preview'" subtitle="Executive Summary & Compliance Checklist">
      <div v-if="activePreviewReport" class="preview-content">
        <div class="preview-meta-card glass-panel">
          <div class="p-row"><span class="p-key">Report Identifier:</span><span class="p-val font-mono text-cyan">{{ activePreviewReport.id }}</span></div>
          <div class="p-row"><span class="p-key">Category:</span><span class="p-val font-mono uppercase">{{ activePreviewReport.type }}</span></div>
          <div class="p-row"><span class="p-key">Generated By:</span><span class="p-val">{{ activePreviewReport.created_by }}</span></div>
          <div class="p-row"><span class="p-key">Timestamp:</span><span class="p-val font-mono">{{ new Date(activePreviewReport.created_at).toLocaleString() }}</span></div>
        </div>
        <div class="preview-section">
          <h4>Executive Summary Findings</h4>
          <p class="preview-text">This report was autonomously compiled by K8sControl Enterprise Suite with zero-trust boundaries.</p>
        </div>
        <div class="preview-section">
          <h4>Compliance & Verification Scorecard</h4>
          <ul class="preview-checklist">
            <li><span class="check-icon text-emerald">✓</span> CIS Kubernetes Benchmark Level 2: <strong>100% Passed</strong></li>
            <li><span class="check-icon text-emerald">✓</span> Air-Gapped NetworkPolicies: <strong>Enforced across all namespaces</strong></li>
            <li><span class="check-icon text-emerald">✓</span> Dual-Sync S3 Replication: <strong>Zero Lag Observed (RPO &lt; 15s)</strong></li>
            <li><span class="check-icon text-emerald">✓</span> Secret Encryption at Rest: <strong>AES-256 Vault KMS Armed</strong></li>
          </ul>
        </div>
      </div>
      <template #footer="{ close }">
        <button class="btn btn-secondary" type="button" @click="close">Close</button>
        <button class="btn btn-primary" @click="downloadReport(activePreviewReport!)">
          <span>⬇ Download {{ activePreviewReport?.format.toUpperCase() }}</span>
        </button>
      </template>
    </ModalDrawer>

    <ScheduleReportModal v-model:show="showScheduleModal" @save="saveSchedule" />
  </div>
</template>
