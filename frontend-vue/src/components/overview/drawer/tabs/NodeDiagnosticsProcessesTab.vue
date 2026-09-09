<script setup lang="ts">
import { formatPercent, formatBytes, formatIoRate } from './nodeDiagnosticsUtils'
import { useNodeProcesses, type NodeProcessesProps } from './useNodeProcesses'

const props = defineProps<NodeProcessesProps>()

const {
  processCategoryFilter,
  processSearch,
  processSortBy,
  processPageSize,
  processCurrentPage,
  processCategoryCounts,
  filteredNodeProcesses,
  totalProcessPages,
  paginatedNodeProcesses,
  getProcessCpuColor,
  getUnifiedProcessState,
  getProcessOriginBadge,
} = useNodeProcesses(props)
</script>

<template>
  <!-- Unified Top Resource & Workload Processes -->
  <div class="node-processes-section mt-4">
    <div class="proc-header-row">
      <div class="proc-title-group">
        <h4 class="proc-section-heading">🔥 Top Resource-Consuming Apps & Processes</h4>
        <span class="badge badge-indigo font-mono" v-if="filteredNodeProcesses.length > 0">
          {{ filteredNodeProcesses.length }} active
        </span>
      </div>

      <!-- Top Category Filter Chips -->
      <div class="service-filter-chips proc-category-chips">
        <button
          class="chip-btn"
          :class="{ active: processCategoryFilter === 'all' }"
          @click="processCategoryFilter = 'all'; processCurrentPage = 1"
        >
          All ({{ processCategoryCounts.total }})
        </button>
        <button
          class="chip-btn chip-healthy"
          :class="{ active: processCategoryFilter === 'container' }"
          @click="processCategoryFilter = 'container'; processCurrentPage = 1"
        >
          📦 Workloads ({{ processCategoryCounts.container }})
        </button>
        <button
          class="chip-btn chip-traffic"
          :class="{ active: processCategoryFilter === 'host_daemon' }"
          @click="processCategoryFilter = 'host_daemon'; processCurrentPage = 1"
        >
          ⚡ Host Daemons ({{ processCategoryCounts.hostDaemon }})
        </button>
        <button
          class="chip-btn chip-degraded"
          :class="{ active: processCategoryFilter === 'kernel' }"
          @click="processCategoryFilter = 'kernel'; processCurrentPage = 1"
        >
          ⚙️ OS Kernel ({{ processCategoryCounts.kernel }})
        </button>
      </div>
    </div>

    <div class="proc-controls">
      <div class="proc-search-box wide-search">
        <span class="search-icon">🔍</span>
        <input
          v-model="processSearch"
          type="text"
          placeholder="Search processes by name, PID, user, command line..."
          class="input-proc-search"
          @input="processCurrentPage = 1"
        />
        <button v-if="processSearch" class="btn-clear-search" @click="processSearch = ''; processCurrentPage = 1">✕</button>
      </div>

      <div class="proc-sort-group">
        <select v-model="processSortBy" class="select-proc-sort" @change="processCurrentPage = 1">
          <option value="cpu">Sort: CPU % (High to Low)</option>
          <option value="mem">Sort: Memory</option>
          <option value="disk">Sort: Disk I/O (High to Low)</option>
          <option value="rps">Sort: Ingress Req/s</option>
          <option value="bandwidth">Sort: Bandwidth (Rx/Tx)</option>
          <option value="name">Sort: App Name</option>
          <option value="pid">Sort: PID</option>
        </select>
      </div>
    </div>

    <div class="processes-table-wrapper" v-if="filteredNodeProcesses.length > 0">
      <table class="processes-table">
        <thead>
          <tr>
            <th class="th-pid">PID</th>
            <th class="th-app">App / Command</th>
            <th class="th-user">User</th>
            <th class="th-cpu">CPU %</th>
            <th class="th-mem">Memory</th>
            <th class="th-disk">Disk I/O</th>
            <th class="th-rps">Req/s</th>
            <th class="th-err">Err %</th>
            <th class="th-bw">Bandwidth</th>
            <th class="th-state">State</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="proc in paginatedNodeProcesses"
            :key="proc.pid + '-' + proc.name"
            class="proc-row"
            :class="{ 'proc-row-hot': proc.cpu_percent >= 70, 'proc-row-container': proc.is_container }"
          >
            <td class="col-proc-pid">
              <span class="pid-tag font-mono" :class="{ 'pid-tag-ctr': proc.pid === 'CTR' }">
                {{ typeof proc.pid === 'number' ? '#' + proc.pid : proc.pid }}
              </span>
            </td>
            <td class="col-proc-app">
              <div class="proc-app-info" :title="proc.command_line ? `${proc.name}\nType: ${getProcessOriginBadge(proc).label}\nCommand: ${proc.command_line}` : proc.name">
                <div class="proc-name-row">
                  <span class="proc-name">{{ proc.name }}</span>
                  <span class="proc-origin-tag font-mono" :class="getProcessOriginBadge(proc).class">
                    {{ getProcessOriginBadge(proc).icon }} {{ getProcessOriginBadge(proc).label }}
                  </span>
                </div>
                <span class="proc-cmd-line font-mono" v-if="proc.command_line">
                  {{ proc.command_line }}
                </span>
              </div>
            </td>
            <td class="col-proc-user">
              <span class="user-badge font-mono">{{ proc.user || 'root' }}</span>
            </td>
            <td class="col-proc-cpu">
              <div class="proc-cpu-box">
                <div class="proc-cpu-val-row">
                  <span class="proc-cpu-val smooth-value" :class="`text-${getProcessCpuColor(proc.cpu_percent)}`">
                    {{ formatPercent(proc.cpu_percent) }}
                  </span>
                  <span v-if="proc.cpu_percent >= 70" class="badge badge-rose badge-hot-pulse">
                    HOT
                  </span>
                </div>
                <div class="proc-mini-track">
                  <div
                    class="proc-mini-fill smooth-bar"
                    :class="`bg-${getProcessCpuColor(proc.cpu_percent)}`"
                    :style="{ width: `${Math.min(100, proc.cpu_percent)}%` }"
                  ></div>
                </div>
              </div>
            </td>
            <td class="col-proc-mem">
              <div class="proc-mem-box">
                <span class="proc-mem-val smooth-value">{{ formatBytes(proc.memory_bytes) }}</span>
                <span class="proc-mem-pct font-mono" v-if="proc.memory_percent">
                  ({{ formatPercent(proc.memory_percent) }})
                </span>
              </div>
            </td>
            <td class="col-proc-disk font-mono text-slate">
              <span class="bw-split">
                <span class="bw-rx text-cyan" :title="'Process Disk Read: ' + formatIoRate(proc.disk_read_bytes_per_sec || 0)">📖 {{ formatIoRate(proc.disk_read_bytes_per_sec || 0) }}</span>
                <span class="bw-tx text-purple" :title="'Process Disk Write: ' + formatIoRate(proc.disk_write_bytes_per_sec || 0)">✍️ {{ formatIoRate(proc.disk_write_bytes_per_sec || 0) }}</span>
              </span>
            </td>
            <td class="col-proc-rps font-mono text-emerald">
              <span v-if="proc.requests_per_sec > 0">
                ⚡ {{ proc.requests_per_sec.toLocaleString() }}
              </span>
              <span v-else class="text-slate opacity-40">0</span>
            </td>
            <td class="col-proc-err font-mono">
              <span v-if="proc.error_rate > 0" class="text-rose font-bold">
                {{ proc.error_rate.toFixed(1) }}%
              </span>
              <span v-else class="text-slate opacity-40">0.0%</span>
            </td>
            <td class="col-proc-bw font-mono text-slate">
              <span class="bw-split">
                <span class="bw-rx text-cyan" :title="'Real-time Download / Read: ' + formatIoRate(proc.rx_bytes_per_sec)">↓ {{ formatIoRate(proc.rx_bytes_per_sec) }}</span>
                <span class="bw-tx text-purple" :title="'Real-time Upload / Write: ' + formatIoRate(proc.tx_bytes_per_sec)">↑ {{ formatIoRate(proc.tx_bytes_per_sec) }}</span>
              </span>
            </td>
            <td class="col-proc-state">
              <span class="badge-state-mini font-mono" :class="getUnifiedProcessState(proc).class">
                <span class="state-mini-dot" :class="getUnifiedProcessState(proc).dotClass"></span>
                {{ getUnifiedProcessState(proc).label }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Unified Page Size & Pagination Footer -->
    <div class="proc-pagination" v-if="filteredNodeProcesses.length > 0">
      <div class="pagination-info">
        Showing
        <span class="text-cyan font-mono font-bold">{{ filteredNodeProcesses.length === 0 ? 0 : (processCurrentPage - 1) * processPageSize + 1 }}–{{ Math.min(processCurrentPage * processPageSize, filteredNodeProcesses.length) }}</span>
        of
        <span class="text-slate font-mono font-bold">{{ filteredNodeProcesses.length }}</span>
        processes & workloads
      </div>

      <!-- Page Size Selector -->
      <div class="page-size-group">
        <span class="page-size-label font-mono">PAGE SIZE:</span>
        <div class="page-size-pill">
          <button
            v-for="size in [10, 25, 50, 100]"
            :key="size"
            class="btn-page-size font-mono"
            :class="{ active: processPageSize === size }"
            @click="processPageSize = size; processCurrentPage = 1"
          >
            {{ size }}
          </button>
        </div>
      </div>

      <div class="pagination-actions" v-if="totalProcessPages > 1">
        <button
          class="btn-page"
          :disabled="processCurrentPage <= 1"
          @click="processCurrentPage = 1"
          title="First Page"
        >
          «
        </button>
        <button
          class="btn-page"
          :disabled="processCurrentPage <= 1"
          @click="processCurrentPage--"
        >
          ‹ Prev
        </button>
        <span class="page-indicator font-mono">
          Page {{ processCurrentPage }} / {{ totalProcessPages }}
        </span>
        <button
          class="btn-page"
          :disabled="processCurrentPage >= totalProcessPages"
          @click="processCurrentPage++"
        >
          Next ›
        </button>
        <button
          class="btn-page"
          :disabled="processCurrentPage >= totalProcessPages"
          @click="processCurrentPage = totalProcessPages"
          title="Last Page"
        >
          »
        </button>
      </div>
    </div>

    <div class="proc-empty-state glass-panel" v-else>
      <div class="radar-glow-container">
        <div class="radar-beacon"></div>
        <span class="proc-empty-icon">📡</span>
      </div>
      <h5 class="proc-empty-title">
        {{ processSearch ? 'No Processes Matching Filter' : 'No Process Telemetry Streamed' }}
      </h5>
      <p class="proc-empty-desc" v-if="processSearch">
        No active processes matched "<span class="text-cyan font-mono">{{ processSearch }}</span>". Try searching for a different process name or PID.
      </p>
      <div class="proc-empty-telemetry-hint" v-else>
        <p class="proc-empty-desc">
          Host metrics are active, but high-resolution process inspection stream is pending agent collector broadcast.
        </p>
        <div class="troubleshooting-hint-box">
          <div class="hint-header">
            <span class="hint-icon">💡</span>
            <strong class="hint-title">Troubleshooting & Activation</strong>
          </div>
          <ul class="hint-list">
            <li>Verify <code>k8s-agent</code> daemon is running on this node.</li>
            <li>Ensure agent has process collection permissions (e.g. host <code>/proc</code> mount).</li>
            <li>Processes will populate automatically upon receiving telemetry broadcast.</li>
          </ul>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../../../assets/styles/components/node-diagnostics.css';
</style>
