import { ref } from 'vue'
import type { Cluster } from '../../../api/fleet'
import type { K8sNamespace, K8sResource, ResourceKind, ToastMessage } from '../types'

export function useExplorerState() {
  const loading = ref(false)
  const error = ref<string | null>(null)
  const toastMessage = ref<ToastMessage | null>(null)
  const clusterOffline = ref(false)
  const offlineErrorMessage = ref<string | null>(null)

  const clusters = ref<Cluster[]>([])
  const selectedCluster = ref<string>('primary-cluster')
  const namespaces = ref<K8sNamespace[]>([])
  const selectedNamespace = ref<string>('all')
  const selectedKind = ref<ResourceKind>('pods')
  const resources = ref<K8sResource[]>([])

  // Modals & Drawer State
  const showCreateModal = ref(false)
  const showYamlModal = ref(false)
  const yamlEditorMode = ref<'create' | 'edit'>('create')
  const yamlEditorTitle = ref('Apply YAML Manifest')
  const yamlEditorInitialContent = ref('')
  const showDetailDrawer = ref(false)
  const selectedResource = ref<K8sResource | null>(null)
  const showScaleModal = ref(false)
  const scaleTarget = ref<K8sResource | null>(null)
  const showDrainModal = ref(false)
  const drainTargetNode = ref<K8sResource | null>(null)
  const showDeleteModal = ref(false)
  const resourceToDelete = ref<K8sResource | null>(null)
  const showLogsModal = ref(false)
  const logsPod = ref<K8sResource | null>(null)
  const showTerminalModal = ref(false)
  const terminalPod = ref<K8sResource | null>(null)
  const showNewNsModal = ref(false)
  const newNsError = ref<string | null>(null)
  const showImportModal = ref(false)
  const isMobileSidebarOpen = ref(false)

  function showToast(text: string, type: 'success' | 'error' = 'success') {
    toastMessage.value = { text, type }
    setTimeout(() => {
      if (toastMessage.value?.text === text) {
        toastMessage.value = null
      }
    }, 4000)
  }

  return {
    loading,
    error,
    toastMessage,
    clusterOffline,
    offlineErrorMessage,
    clusters,
    selectedCluster,
    namespaces,
    selectedNamespace,
    selectedKind,
    resources,
    showCreateModal,
    showYamlModal,
    yamlEditorMode,
    yamlEditorTitle,
    yamlEditorInitialContent,
    showDetailDrawer,
    selectedResource,
    showScaleModal,
    scaleTarget,
    showDrainModal,
    drainTargetNode,
    showDeleteModal,
    resourceToDelete,
    showLogsModal,
    logsPod,
    showTerminalModal,
    terminalPod,
    showNewNsModal,
    newNsError,
    showImportModal,
    isMobileSidebarOpen,
    showToast,
  }
}
