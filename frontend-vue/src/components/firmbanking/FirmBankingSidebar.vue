<script setup lang="ts">
interface MenuItem {
  id: string
  label: string
}

interface Props {
  activeMenu?: string
}

withDefaults(defineProps<Props>(), {
  activeMenu: 'report-approval'
})

const emit = defineEmits<{
  (e: 'update:activeMenu', id: string): void
  (e: 'select', id: string): void
}>()

const menuItems: MenuItem[] = [
  { id: 'board', label: 'Board Management' },
  { id: 'inquiry', label: 'Inquiry Operation' },
  { id: 'collection', label: 'Collection Operation' },
  { id: 'transfer-vn', label: 'Transfer in Vietnam' },
  { id: 'remittance', label: 'Overseas Remittance' },
  { id: 'auto-pay', label: 'Automatic Payment' },
  { id: 'report-approval', label: 'Report and Approval' },
  { id: 'card-history', label: 'Card History' },
  { id: 'account', label: 'Account Management' },
  { id: 'firm', label: 'Firm Management' },
  { id: 'common', label: 'Common Management' },
  { id: 'business', label: 'Business Management' },
  { id: 'system', label: 'System Management' }
]

function handleSelect(id: string) {
  emit('update:activeMenu', id)
  emit('select', id)
}
</script>

<template>
  <aside class="fb-sidebar" aria-label="FirmBanking Main Navigation">
    <nav class="fb-sidebar-nav">
      <a
        v-for="item in menuItems"
        :key="item.id"
        href="javascript:void(0)"
        class="fb-nav-item"
        :class="{ active: activeMenu === item.id }"
        @click="handleSelect(item.id)"
      >
        <span class="fb-nav-item-text">{{ item.label }}</span>
        <span class="fb-nav-arrow" aria-hidden="true">›</span>
      </a>
    </nav>
  </aside>
</template>
