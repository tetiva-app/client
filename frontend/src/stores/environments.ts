import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import type { Environment, Variable } from '@/types/environment'
import { getEnvironmentService } from '@/services'
import { emitWailsEvent } from '@/composables/useWindowEvents'
import { useWorkspaceStore } from '@/stores/workspace'
import { runMutation } from '@/stores/runMutation'

export const useEnvironmentStore = defineStore('environments', () => {
  const environments = ref<Environment[]>([])
  const variablesMap = ref<Map<string, Variable[]>>(new Map())
  const loading = ref(false)

  const activeEnvironment = computed<Environment | null>(() =>
    environments.value.find(e => e.isActive) ?? null
  )

  const activeEnvironmentId = computed<string | null>(() =>
    activeEnvironment.value?.id ?? null
  )

  const resolvedVariables = computed<Record<string, string>>(() => {
    const active = activeEnvironment.value
    if (!active) return {}
    const vars = variablesMap.value.get(active.id) ?? []
    const result: Record<string, string> = {}
    for (const v of vars) {
      if (v.enabled) {
        result[v.key] = v.value
      }
    }
    return result
  })

  async function fetchAll(workspaceId?: string) {
    loading.value = true
    try {
      const wsId = workspaceId ?? useWorkspaceStore().activeWorkspace?.id
      if (!wsId) return
      const service = await getEnvironmentService()
      const result = await service.list(wsId)
      if (result.error) {
        console.error('Failed to fetch environments:', result.error.message)
        return
      }
      environments.value = result.data

      const active = result.data.find(e => e.isActive)
      if (active) {
        await fetchVariables(active.id)
      }
    } catch (err) {
      console.error('Failed to fetch environments:', err)
    } finally {
      loading.value = false
    }
  }

  async function create(name: string): Promise<Environment | null> {
    const wsId = useWorkspaceStore().activeWorkspace?.id
    if (!wsId) return null
    const data = await runMutation('Failed to create environment', () =>
      getEnvironmentService().then(s => s.create({ name, workspaceId: wsId }))
    )
    if (!data) return null
    environments.value = [...environments.value, data]
    emitWailsEvent('env:changed')
    return data
  }

  async function duplicate(sourceId: string, newName: string): Promise<Environment | null> {
    const wsId = useWorkspaceStore().activeWorkspace?.id
    if (!wsId) return null
    const data = await runMutation('Failed to duplicate environment', () =>
      getEnvironmentService().then(s => s.duplicate({ sourceId, newName, workspaceId: wsId }))
    )
    if (!data) return null
    environments.value = [...environments.value, data]
    await fetchVariables(data.id)
    emitWailsEvent('env:changed')
    return data
  }

  async function edit(id: string, name: string, version: number): Promise<Environment | null> {
    const data = await runMutation('Failed to edit environment', () =>
      getEnvironmentService().then(s => s.edit({ id, name, version }))
    )
    if (!data) return null
    environments.value = environments.value.map(e => (e.id === id ? data : e))
    emitWailsEvent('env:changed')
    return data
  }

  async function remove(id: string, version: number) {
    const data = await runMutation('Failed to delete environment', () =>
      getEnvironmentService().then(s => s.delete({ id, version }))
    )
    if (!data) return
    environments.value = environments.value.filter(e => e.id !== id)
    variablesMap.value.delete(id)
    variablesMap.value = new Map(variablesMap.value)
    emitWailsEvent('env:changed')
  }

  async function setActive(id: string) {
    const wsId = useWorkspaceStore().activeWorkspace?.id
    if (!wsId) return
    const data = await runMutation('Failed to set active environment', () =>
      getEnvironmentService().then(s => s.setActive(wsId, id))
    )
    if (!data) return
    environments.value = environments.value.map(e => ({
      ...e,
      isActive: e.id === id,
    }))
    await fetchVariables(id)
    emitWailsEvent('env:changed')
  }

  async function clearActive() {
    // Set a non-existent UUID to deactivate all
    try {
      const wsId = useWorkspaceStore().activeWorkspace?.id
      if (!wsId) return
      const service = await getEnvironmentService()
      await service.setActive(wsId, '00000000-0000-0000-0000-000000000000')
      environments.value = environments.value.map(e => ({
        ...e,
        isActive: false,
      }))
      emitWailsEvent('env:changed')
    } catch (err) {
      console.error('Failed to clear active environment:', err)
    }
  }

  async function fetchVariables(environmentId: string) {
    const data = await runMutation('Failed to fetch variables', () =>
      getEnvironmentService().then(s => s.listVariables(environmentId))
    )
    if (!data) return
    variablesMap.value.set(environmentId, data)
    variablesMap.value = new Map(variablesMap.value)
  }

  async function addVariable(environmentId: string, key: string, value: string, isSecret: boolean): Promise<Variable | null> {
    const data = await runMutation('Failed to add variable', () =>
      getEnvironmentService().then(s => s.addVariable({ environmentId, key, value, isSecret }))
    )
    if (!data) return null
    const existing = variablesMap.value.get(environmentId) ?? []
    variablesMap.value.set(environmentId, [...existing, data])
    variablesMap.value = new Map(variablesMap.value)
    emitWailsEvent('env:changed')
    return data
  }

  async function editVariable(req: { id: string; key: string; value: string; isSecret: boolean; enabled: boolean; version: number }): Promise<Variable | null> {
    const data = await runMutation('Failed to edit variable', () =>
      getEnvironmentService().then(s => s.editVariable(req))
    )
    if (!data) return null
    const envId = data.environmentId
    const vars = variablesMap.value.get(envId) ?? []
    variablesMap.value.set(envId, vars.map(v => (v.id === req.id ? data : v)))
    variablesMap.value = new Map(variablesMap.value)
    emitWailsEvent('env:changed')
    return data
  }

  async function removeVariable(id: string, environmentId: string) {
    const data = await runMutation('Failed to delete variable', () =>
      getEnvironmentService().then(s => s.deleteVariable({ id }))
    )
    if (!data) return
    const vars = variablesMap.value.get(environmentId) ?? []
    variablesMap.value.set(environmentId, vars.filter(v => v.id !== id))
    variablesMap.value = new Map(variablesMap.value)
    emitWailsEvent('env:changed')
  }

  function getVariables(environmentId: string): Variable[] {
    return variablesMap.value.get(environmentId) ?? []
  }

  return {
    environments,
    variablesMap,
    loading,
    activeEnvironment,
    activeEnvironmentId,
    resolvedVariables,
    fetchAll,
    create,
    duplicate,
    edit,
    remove,
    setActive,
    clearActive,
    fetchVariables,
    addVariable,
    editVariable,
    removeVariable,
    getVariables,
  }
})
