import { computed, onActivated, onDeactivated, type Ref } from 'vue'
import { useCodeSnippet } from '@/composables/useCodeSnippet'
import { useToast } from '@/composables/useToast'
import { copyText } from '@/lib/clipboard'
import { useEnvironmentStore } from '@/stores/environments'
import { useResponseStore } from '@/stores/responses'
import { useWebSocketStore } from '@/stores/websocket'
import { useWorkspaceStore } from '@/stores/workspace'
import type { Request } from '@/types/request'

// Binds the Code tab to the stores; the component itself only lays it out.
export function useCodeSnippetPanel(request: Ref<Request>) {
  const envStore = useEnvironmentStore()
  const workspaceStore = useWorkspaceStore()
  const responses = useResponseStore()
  const sockets = useWebSocketStore()
  const toast = useToast()

  const envVersion = computed(() => {
    const active = envStore.activeEnvironment
    if (!active) return ''
    return `${active.id}|${envStore.getVariables(active.id).map((v) => `${v.id}:${v.version}`).join(',')}`
  })

  const executing = computed(() => {
    const id = request.value.id
    return responses.getResponseState(id).status === 'loading' || sockets.stateFor(id).status === 'connecting'
  })

  const snippet = useCodeSnippet({
    request,
    workspaceId: computed(() => workspaceStore.activeWorkspace?.id),
    envVersion,
    executing,
  })

  // A cached editor misses collection auth and cookie changes, and should not rebuild for them while hidden.
  onActivated(snippet.resume)
  onDeactivated(snippet.pause)

  const selected = computed({
    get: () => snippet.selectedKey.value,
    set: (key: string) => snippet.select(key),
  })

  const language = computed(() => snippet.targets.value.find((t) => t.key === snippet.selectedKey.value)?.language)

  async function copy() {
    if (!snippet.canCopy.value) return
    try {
      await copyText(snippet.code.value)
      toast.success('Copied')
    } catch {
      toast.error('Could not copy to the clipboard')
    }
  }

  return {
    targets: snippet.targets,
    selected,
    language,
    code: snippet.code,
    warnings: snippet.warnings,
    error: snippet.error,
    loading: snippet.loading,
    stale: snippet.stale,
    canCopy: snippet.canCopy,
    resolveVariables: snippet.resolveVariables,
    includeSecrets: snippet.includeSecrets,
    copy,
  }
}
