<script setup lang="ts">
interface Props {
  userName?: string
  userRole?: string
  accountNo?: string
}

withDefaults(defineProps<Props>(), {
  userName: 'Tran Van A',
  userRole: 'Approver',
  accountNo: '1010-888-294103'
})

const emit = defineEmits<{
  (e: 'logout'): void
}>()
</script>

<template>
  <header class="fb-header" role="banner">
    <div class="fb-brand-group">
      <div class="fb-logo-mark" aria-hidden="true">
        <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
          <path d="M12 2L2 7l10 5 10-5-10-5z" />
          <path d="M2 17l10 5 10-5" />
          <path d="M2 12l10 5 10-5" />
        </svg>
      </div>
      <div class="fb-brand-title">
        <span class="fb-bank-name">WOORI BANK</span>
        <span class="fb-app-name">FirmBanking System</span>
      </div>
    </div>

    <div class="fb-header-user-meta">
      <div class="fb-meta-item">
        <span>Account:</span>
        <span class="fb-meta-val font-mono">{{ accountNo }}</span>
      </div>
      <div class="fb-meta-item">
        <span>User:</span>
        <span class="fb-meta-val">{{ userName }} ({{ userRole }})</span>
      </div>
      <div class="fb-meta-item">
        <span class="fb-status-pill status-approved">
          <span style="display:inline-block; width:6px; height:6px; border-radius:50%; background:#059669;"></span>
          Online
        </span>
      </div>
      <button
        type="button"
        class="fb-btn-logout"
        title="Logout"
        aria-label="Logout"
        @click="emit('logout')"
      >
        <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
          <polyline points="16 17 21 12 16 7" />
          <line x1="21" y1="12" x2="9" y2="12" />
        </svg>
        <span>Logout</span>
      </button>
    </div>
  </header>
</template>
