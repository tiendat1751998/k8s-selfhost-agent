<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import '../assets/styles/views/firmbanking.css'

import FirmBankingHeader from '../components/firmbanking/FirmBankingHeader.vue'
import FirmBankingSidebar from '../components/firmbanking/FirmBankingSidebar.vue'
import FirmBankingTabStrip from '../components/firmbanking/FirmBankingTabStrip.vue'
import FirmBankingDetailModal, { type FirmBankingTransaction } from '../components/firmbanking/FirmBankingDetailModal.vue'
import { INITIAL_TRANSACTIONS } from '../components/firmbanking/mockTransactions'

const router = useRouter()

// In-App Navigation State (In-memory, NO URL path changes)
const activeSidebar = ref('report-approval')
const activeTab = ref('approve')

// Filter State (2-Row Structured Grid)
const startDate = ref('2026-09-14')
const endDate = ref('2026-09-21')
const accountNo = ref('1010-888-294103')
const filterStatus = ref('All')
const filterType = ref('All')
const filterCurrency = ref('All')
const searchKeyword = ref('')

// Transactions State
const transactions = ref<FirmBankingTransaction[]>([...INITIAL_TRANSACTIONS])
const selectedRowId = ref<number>(8) // Default selected row #8 (champagne/amber highlight)
const detailModalOpen = ref(false)
const inspectedTransaction = ref<FirmBankingTransaction | null>(null)

// Pagination State
const currentPage = ref(1)
const pageSize = ref(20)

// Quick Date Range Helpers
function setQuickDate(type: 'today' | 'week' | 'month') {
  const today = '2026-09-21'
  endDate.value = today
  if (type === 'today') {
    startDate.value = today
  } else if (type === 'week') {
    startDate.value = '2026-09-14'
  } else if (type === 'month') {
    startDate.value = '2026-08-21'
  }
}

// KPI Computations
const totalCount = computed(() => transactions.value.length)
const waitingCount = computed(() => transactions.value.filter(t => t.status === 'Waiting for Approval').length)
const approvedCount = computed(() => transactions.value.filter(t => t.status === 'Approved').length)
const rejectedCount = computed(() => transactions.value.filter(t => t.status === 'Rejected').length)

// Filter Logic
const filteredTransactions = computed(() => {
  return transactions.value.filter(tx => {
    if (filterStatus.value !== 'All' && tx.status !== filterStatus.value) return false
    if (filterType.value !== 'All' && tx.type !== filterType.value) return false
    if (filterCurrency.value !== 'All' && tx.currency !== filterCurrency.value) return false
    if (searchKeyword.value.trim()) {
      const q = searchKeyword.value.trim().toLowerCase()
      const matchName = tx.beneficiaryName.toLowerCase().includes(q)
      const matchAcc = tx.beneficiaryAccount.toLowerCase().includes(q)
      const matchBank = tx.beneficiaryBank.toLowerCase().includes(q)
      if (!matchName && !matchAcc && !matchBank) return false
    }
    return true
  })
})

const totalPages = computed(() => Math.max(1, Math.ceil(filteredTransactions.value.length / pageSize.value)))

const paginatedTransactions = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredTransactions.value.slice(start, start + pageSize.value)
})

// Number Formatter
function formatAmount(amt: number, ccy: string): string {
  if (ccy === 'USD') {
    return amt.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
  }
  return amt.toLocaleString('vi-VN')
}

// Actions
function handleRowClick(tx: FirmBankingTransaction) {
  selectedRowId.value = tx.id
}

function handleOpenDetail(tx: FirmBankingTransaction, e?: Event) {
  if (e) e.stopPropagation()
  selectedRowId.value = tx.id
  inspectedTransaction.value = tx
  detailModalOpen.value = true
}

function handleApprove(id: number) {
  const target = transactions.value.find(t => t.id === id)
  if (target) {
    target.status = 'Approved'
  }
  detailModalOpen.value = false
}

function handleReject(id: number) {
  const target = transactions.value.find(t => t.id === id)
  if (target) {
    target.status = 'Rejected'
  }
  detailModalOpen.value = false
}

function handleBatchApprove() {
  transactions.value.forEach(tx => {
    if (tx.status === 'Waiting for Approval') {
      tx.status = 'Approved'
    }
  })
}

function handleExportExcel() {
  const csvHeader = 'No,Type,Date,Account,Currency,Amount,Beneficiary,BeneficiaryAccount,Bank,Status,Requester\n'
  const csvRows = filteredTransactions.value.map(t =>
    `"${t.id}","${t.type}","${t.date}","${t.accountNo}","${t.currency}","${t.amount}","${t.beneficiaryName}","${t.beneficiaryAccount}","${t.beneficiaryBank}","${t.status}","${t.requester}"`
  ).join('\n')
  const blob = new Blob([csvHeader + csvRows], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.setAttribute('download', `firmbanking_approve_${Date.now()}.csv`)
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

function handleLogout() {
  router.push('/login')
}
</script>

<template>
  <div class="fb-portal">
    <!-- Top Header -->
    <FirmBankingHeader
      user-name="Tran Van A"
      user-role="Approver"
      account-no="1010-888-294103"
      @logout="handleLogout"
    />

    <!-- Main Split Body Layout -->
    <div class="fb-body-layout">
      <!-- 13-Item Left Navy Navigation -->
      <FirmBankingSidebar
        v-model:active-menu="activeSidebar"
      />

      <!-- In-Memory Vertical Tab Strip -->
      <FirmBankingTabStrip
        v-model:active-tab="activeTab"
      />

      <!-- Workspace Area -->
      <main class="fb-workspace" role="main">
        <!-- Breadcrumb Bar -->
        <div class="fb-breadcrumb-bar">
          <div class="fb-breadcrumb">
            <span>Home</span>
            <span class="sep">›</span>
            <span>Report and Approval</span>
            <span class="sep">›</span>
            <span class="current">Approve</span>
          </div>
          <div class="fb-screen-title">Approve Transactions</div>
        </div>

        <!-- 2-Row Structured Filter Grid Card -->
        <section class="fb-filter-card" aria-label="Filter Transactions">
          <!-- Row 1 -->
          <div class="fb-filter-grid">
            <label class="fb-filter-label" for="filter-start-date">Transfer Date</label>
            <div class="fb-date-range-wrap">
              <input
                id="filter-start-date"
                v-model="startDate"
                type="date"
                class="fb-input-text font-mono"
              />
              <span class="fb-date-sep">~</span>
              <input
                id="filter-end-date"
                v-model="endDate"
                type="date"
                class="fb-input-text font-mono"
              />
              <button type="button" class="fb-quick-date-btn" @click="setQuickDate('today')">Today</button>
              <button type="button" class="fb-quick-date-btn" @click="setQuickDate('week')">1 Week</button>
              <button type="button" class="fb-quick-date-btn" @click="setQuickDate('month')">1 Month</button>
            </div>

            <label class="fb-filter-label" for="filter-status">Status</label>
            <select id="filter-status" v-model="filterStatus" class="fb-select">
              <option value="All">All Statuses</option>
              <option value="Waiting for Approval">Waiting for Approval</option>
              <option value="Approved">Approved</option>
              <option value="Rejected">Rejected</option>
            </select>

            <label class="fb-filter-label" for="filter-type">Type</label>
            <select id="filter-type" v-model="filterType" class="fb-select">
              <option value="All">All Types</option>
              <option value="Domestic">Domestic</option>
              <option value="Overseas">Overseas</option>
            </select>

            <button type="button" class="fb-btn-view" @click="currentPage = 1">
              <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="11" cy="11" r="8" />
                <line x1="21" y1="21" x2="16.65" y2="16.65" />
              </svg>
              <span>View</span>
            </button>
          </div>

          <!-- Row 2 -->
          <div class="fb-filter-grid">
            <label class="fb-filter-label" for="filter-account">Account No</label>
            <div>
              <input
                id="filter-account"
                v-model="accountNo"
                type="text"
                class="fb-input-text font-mono"
                style="width: 100%; max-width: 240px;"
              />
            </div>

            <label class="fb-filter-label" for="filter-currency">Currency</label>
            <select id="filter-currency" v-model="filterCurrency" class="fb-select">
              <option value="All">All</option>
              <option value="VND">VND</option>
              <option value="USD">USD</option>
            </select>

            <label class="fb-filter-label" for="filter-search">Beneficiary</label>
            <input
              id="filter-search"
              v-model="searchKeyword"
              type="text"
              class="fb-input-text"
              placeholder="Search beneficiary or account..."
              style="width: 100%;"
            />
          </div>
        </section>

        <!-- Table Action Bar -->
        <div class="fb-table-action-bar">
          <div class="fb-kpi-summary">
            <span>Total: <span class="fb-kpi-pill font-mono">{{ totalCount }}</span></span>
            <span>Waiting: <span class="fb-kpi-pill font-mono fb-kpi-highlight">{{ waitingCount }}</span></span>
            <span>Approved: <span class="fb-kpi-pill font-mono" style="color:#059669;">{{ approvedCount }}</span></span>
            <span>Rejected: <span class="fb-kpi-pill font-mono" style="color:#e11d48;">{{ rejectedCount }}</span></span>
          </div>

          <div class="fb-table-btns">
            <button
              type="button"
              class="fb-btn-action fb-btn-action-primary"
              :disabled="waitingCount === 0"
              @click="handleBatchApprove"
            >
              Batch Approve ({{ waitingCount }})
            </button>
            <button
              type="button"
              class="fb-btn-action fb-btn-action-secondary"
              @click="handleExportExcel"
            >
              Export Excel
            </button>
          </div>
        </div>

        <!-- Modern Data Table -->
        <div class="fb-table-card firmbanking-table-wrap">
          <div class="fb-table-scroll-wrap">
            <table class="fb-table" aria-label="FirmBanking Transactions Table">
              <thead>
                <tr>
                  <th class="text-center" style="width: 40px;">No</th>
                  <th style="width: 70px;">Type</th>
                  <th style="width: 85px;">Transfer Date</th>
                  <th style="width: 110px;">Withdrawal Acc</th>
                  <th class="text-center" style="width: 45px;">Curr</th>
                  <th class="text-right" style="width: 105px;">Amount</th>
                  <th style="width: 130px;">Beneficiary Name</th>
                  <th style="width: 115px;">Beneficiary Acc</th>
                  <th style="width: 125px;">Beneficiary Bank</th>
                  <th class="text-center" style="width: 95px;">Status</th>
                  <th style="width: 85px;">Requester</th>
                  <th class="text-center" style="width: 60px;">Action</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="tx in paginatedTransactions"
                  :key="tx.id"
                  :class="{ 'row-selected': selectedRowId === tx.id }"
                  @click="handleRowClick(tx)"
                >
                  <td class="text-center font-mono">{{ tx.id }}</td>
                  <td class="cell-ellipsis" :title="tx.type">{{ tx.type }}</td>
                  <td class="font-mono cell-ellipsis" :title="tx.date">{{ tx.date }}</td>
                  <td class="font-mono cell-ellipsis" :title="tx.accountNo">{{ tx.accountNo }}</td>
                  <td class="text-center font-mono font-bold">{{ tx.currency }}</td>
                  <td class="text-right font-mono font-bold">
                    {{ formatAmount(tx.amount, tx.currency) }}
                  </td>
                  <td class="cell-ellipsis font-bold" :title="tx.beneficiaryName">{{ tx.beneficiaryName }}</td>
                  <td class="font-mono cell-ellipsis" :title="tx.beneficiaryAccount">{{ tx.beneficiaryAccount }}</td>
                  <td class="cell-ellipsis" :title="tx.beneficiaryBank">{{ tx.beneficiaryBank }}</td>
                  <td class="text-center">
                    <span
                      class="fb-status-pill"
                      :class="{
                        'status-approved': tx.status === 'Approved',
                        'status-waiting': tx.status === 'Waiting for Approval',
                        'status-rejected': tx.status === 'Rejected'
                      }"
                    >
                      {{ tx.status }}
                    </span>
                  </td>
                  <td class="cell-ellipsis" :title="tx.requester">{{ tx.requester }}</td>
                  <td class="text-center">
                    <button
                      type="button"
                      class="fb-btn-detail"
                      @click="handleOpenDetail(tx, $event)"
                    >
                      Detail
                    </button>
                  </td>
                </tr>
                <tr v-if="paginatedTransactions.length === 0">
                  <td colspan="12" class="text-center" style="padding: 24px; color: var(--fb-text-muted);">
                    No transactions found matching the selected filters.
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- Pagination Bar -->
          <div class="fb-pagination">
            <button
              type="button"
              class="fb-page-item"
              :disabled="currentPage === 1"
              @click="currentPage--"
            >
              ‹
            </button>
            <button
              v-for="p in totalPages"
              :key="p"
              type="button"
              class="fb-page-item"
              :class="{ active: currentPage === p }"
              @click="currentPage = p"
            >
              {{ p }}
            </button>
            <button
              type="button"
              class="fb-page-item"
              :disabled="currentPage === totalPages"
              @click="currentPage++"
            >
              ›
            </button>
          </div>
        </div>

        <!-- Transaction Detail Modal -->
        <FirmBankingDetailModal
          :show="detailModalOpen"
          :transaction="inspectedTransaction"
          @close="detailModalOpen = false"
          @approve="handleApprove"
          @reject="handleReject"
        />
      </main>
    </div>
  </div>
</template>
