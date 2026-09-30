<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'

export interface FirmBankingTransaction {
  id: number
  type: 'Domestic' | 'Overseas'
  date: string
  accountNo: string
  currency: 'VND' | 'USD'
  amount: number
  beneficiaryName: string
  beneficiaryAccount: string
  beneficiaryBank: string
  status: 'Approved' | 'Waiting for Approval' | 'Rejected'
  requester: string
  description?: string
  swiftCode?: string
}

interface Props {
  show: boolean
  transaction: FirmBankingTransaction | null
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'approve', id: number): void
  (e: 'reject', id: number): void
}>()

function formatAmount(amt: number, ccy: string): string {
  if (ccy === 'USD') {
    return amt.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
  }
  return amt.toLocaleString('vi-VN')
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.show) {
    emit('close')
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div
    v-if="show && transaction"
    class="fb-modal-backdrop"
    role="dialog"
    aria-modal="true"
    :aria-labelledby="'modal-title-' + transaction.id"
    @click.self="emit('close')"
  >
    <div class="fb-modal-box">
      <div class="fb-modal-header">
        <h3 :id="'modal-title-' + transaction.id" class="fb-modal-title">
          Transaction Details #{{ transaction.id }}
        </h3>
        <button
          type="button"
          class="fb-modal-close-btn"
          aria-label="Close dialog"
          @click="emit('close')"
        >
          ✕
        </button>
      </div>

      <div class="fb-modal-body">
        <div class="fb-detail-grid">
          <div class="fb-detail-lbl">Transaction Type</div>
          <div class="fb-detail-val">{{ transaction.type }} Remittance</div>

          <div class="fb-detail-lbl">Transfer Date</div>
          <div class="fb-detail-val font-mono">{{ transaction.date }}</div>

          <div class="fb-detail-lbl">Withdrawal Account</div>
          <div class="fb-detail-val font-mono">{{ transaction.accountNo }}</div>

          <div class="fb-detail-lbl">Amount & Currency</div>
          <div class="fb-detail-val font-bold font-mono">
            {{ formatAmount(transaction.amount, transaction.currency) }} {{ transaction.currency }}
          </div>

          <div class="fb-detail-lbl">Beneficiary Name</div>
          <div class="fb-detail-val font-bold">{{ transaction.beneficiaryName }}</div>

          <div class="fb-detail-lbl">Beneficiary Account</div>
          <div class="fb-detail-val font-mono">{{ transaction.beneficiaryAccount }}</div>

          <div class="fb-detail-lbl">Beneficiary Bank</div>
          <div class="fb-detail-val">{{ transaction.beneficiaryBank }}</div>

          <div v-if="transaction.swiftCode" class="fb-detail-lbl">SWIFT / BIC</div>
          <div v-if="transaction.swiftCode" class="fb-detail-val font-mono">{{ transaction.swiftCode }}</div>

          <div class="fb-detail-lbl">Remittance Note</div>
          <div class="fb-detail-val">{{ transaction.description || 'Payment for commercial contract & invoices' }}</div>

          <div class="fb-detail-lbl">Requester</div>
          <div class="fb-detail-val">{{ transaction.requester }}</div>

          <div class="fb-detail-lbl">Current Status</div>
          <div class="fb-detail-val">
            <span
              class="fb-status-pill"
              :class="{
                'status-approved': transaction.status === 'Approved',
                'status-waiting': transaction.status === 'Waiting for Approval',
                'status-rejected': transaction.status === 'Rejected'
              }"
            >
              {{ transaction.status }}
            </span>
          </div>
        </div>
      </div>

      <div class="fb-modal-footer">
        <button
          type="button"
          class="fb-btn-action fb-btn-action-secondary"
          @click="emit('close')"
        >
          Close
        </button>
        <template v-if="transaction.status === 'Waiting for Approval'">
          <button
            type="button"
            class="fb-btn-action"
            style="background:#fff1f2; color:#e11d48; border:1px solid #fecdd3;"
            @click="emit('reject', transaction.id)"
          >
            Reject
          </button>
          <button
            type="button"
            class="fb-btn-action fb-btn-action-primary"
            @click="emit('approve', transaction.id)"
          >
            Approve Transaction
          </button>
        </template>
      </div>
    </div>
  </div>
</template>
