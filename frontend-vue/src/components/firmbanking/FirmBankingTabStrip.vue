<script setup lang="ts">
interface TabItem {
  id: string
  label: string
  title: string
}

interface Props {
  activeTab?: string
}

withDefaults(defineProps<Props>(), {
  activeTab: 'approve'
})

const emit = defineEmits<{
  (e: 'update:activeTab', id: string): void
  (e: 'tab-change', id: string): void
}>()

const tabs: TabItem[] = [
  { id: 'dashboard', label: 'Dashboard', title: 'FirmBanking Dashboard' },
  { id: 'view-message', label: 'View Message...', title: 'View Messages & Notifications' },
  { id: 'approve', label: 'Approve', title: 'Approve Pending Transactions' },
  { id: 'transfer-inq', label: 'Transfer Inqu...', title: 'Transfer Inquiry' },
  { id: 'approval-req', label: 'Approval Req...', title: 'Approval Request History' }
]

function selectTab(id: string) {
  emit('update:activeTab', id)
  emit('tab-change', id)
}
</script>

<template>
  <div class="fb-vertical-tabstrip" role="tablist" aria-orientation="vertical" aria-label="Quick Navigation Tabs">
    <button
      v-for="tab in tabs"
      :key="tab.id"
      type="button"
      role="tab"
      :aria-selected="activeTab === tab.id"
      class="fb-vtab"
      :class="{ active: activeTab === tab.id }"
      :title="tab.title"
      @click="selectTab(tab.id)"
    >
      {{ tab.label }}
    </button>
  </div>
</template>
