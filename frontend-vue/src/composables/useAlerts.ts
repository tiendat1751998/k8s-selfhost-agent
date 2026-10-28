import { useAlertManager } from './useAlertManager'
import { api } from '../api/client'

export function useAlerts() {
  const manager = useAlertManager()

  async function deleteChannel(id: string) {
    if (!id) return
    try {
      await api.delete(`/alerts/channels/${id}`)
      manager.channels.value = manager.channels.value.filter(c => c.ID !== id)
      manager.showFeedback('Delivery channel deleted successfully.')
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Unknown error'
      manager.showFeedback(`Failed to delete channel: ${msg}`)
      throw err
    }
  }

  return {
    ...manager,
    deleteChannel,
    handleDeleteChannel: deleteChannel
  }
}

export { useAlertManager }
