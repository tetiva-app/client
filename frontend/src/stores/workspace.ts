import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import type { Workspace } from '@/types/workspace'
import { getWorkspaceService } from '@/services'
import { emitWailsEvent } from '@/composables/useWindowEvents'
import { useCollectionStore } from '@/stores/collections'
import { useEnvironmentStore } from '@/stores/environments'
import { useRequestStore } from '@/stores/tabs'
import { runMutation } from '@/stores/runMutation'

export const useWorkspaceStore = defineStore('workspaces', () => {
  const workspaces = ref<Workspace[]>([])
  const loading = ref(false)

  const activeWorkspace = computed<Workspace | null>(() =>
    workspaces.value.find(w => w.isActive) ?? null
  )

  async function fetchAll() {
    loading.value = true
    try {
      const service = await getWorkspaceService()
      const result = await service.list()
      if (result.error) {
        console.error('Failed to fetch workspaces:', result.error.message)
        return
      }
      workspaces.value = result.data
    } catch (err) {
      console.error('Failed to fetch workspaces:', err)
    } finally {
      loading.value = false
    }
  }

  async function switchWorkspace(id: string) {
    // Not via runMutation: the whole post-activate chain (refetch, close tabs,
    // reload data) has to stay inside one try/catch or a failure escapes to the UI.
    try {
      const service = await getWorkspaceService()
      const result = await service.setActive({ workspaceId: id })
      if (result.error) {
        console.error('Failed to switch workspace:', result.error.message)
        return
      }

      await fetchAll()

      const tabStore = useRequestStore()
      await tabStore.closeAllTabs()

      const collectionStore = useCollectionStore()
      await collectionStore.fetchAll(id)

      const envStore = useEnvironmentStore()
      await envStore.fetchAll(id)

      emitWailsEvent('workspace:switched')
    } catch (err) {
      console.error('Failed to switch workspace:', err)
    }
  }

  async function createWorkspace(name: string): Promise<Workspace | null> {
    const data = await runMutation('Failed to create workspace', () =>
      getWorkspaceService().then(s => s.create({ name }))
    )
    if (!data) return null
    await switchWorkspace(data.id)
    return data
  }

  async function editWorkspace(id: string, name: string, version: number): Promise<Workspace | null> {
    const data = await runMutation('Failed to edit workspace', () =>
      getWorkspaceService().then(s => s.edit({ id, name, version }))
    )
    if (!data) return null
    workspaces.value = workspaces.value.map(w => (w.id === id ? data : w))
    return data
  }

  async function deleteWorkspace(id: string, version: number) {
    const wasActive = workspaces.value.find(w => w.id === id)?.isActive
    // Not via runMutation: the post-delete refetch + tab/collection reload must
    // stay inside one try/catch (same reason as switchWorkspace).
    try {
      const service = await getWorkspaceService()
      const result = await service.delete({ id, version })
      if (result.error) {
        console.error('Failed to delete workspace:', result.error.message)
        return
      }

      // Refetch — backend may have auto-activated another workspace.
      await fetchAll()

      if (wasActive) {
        const active = activeWorkspace.value
        if (active) {
          const tabStore = useRequestStore()
          await tabStore.closeAllTabs()

          const collectionStore = useCollectionStore()
          await collectionStore.fetchAll(active.id)

          const envStore = useEnvironmentStore()
          await envStore.fetchAll(active.id)
        }
      }
    } catch (err) {
      console.error('Failed to delete workspace:', err)
    }
  }

  return {
    workspaces,
    loading,
    activeWorkspace,
    fetchAll,
    switchWorkspace,
    createWorkspace,
    editWorkspace,
    deleteWorkspace,
  }
})
