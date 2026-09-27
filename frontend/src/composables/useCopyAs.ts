import type { Ref } from 'vue'
import { useToast } from '@/composables/useToast'
import { warningsToastMessage } from '@/lib/auth-warnings'
import { copyText } from '@/lib/clipboard'
import { formatResultError } from '@/lib/result-error'
import { copySnippetAs } from '@/lib/snippets/copy'
import { getRequestService } from '@/services'
import { useCodeDialogUi } from '@/stores/codeDialog'
import { useSettingsStore } from '@/stores/settings'
import { useRequestStore } from '@/stores/tabs'
import { useWorkspaceStore } from '@/stores/workspace'

function warningCount(n: number): string {
  return `${n} ${n === 1 ? 'warning' : 'warnings'}`
}

export function useCopyAs(requestId: Ref<string>) {
  const store = useRequestStore()
  const workspace = useWorkspaceStore()
  const codeDialog = useCodeDialogUi()
  const toast = useToast()

  // The backend reads the stored row and dry-runs the pre-request script, so unsaved edits go first.
  async function copyCurlFromBackend(id: string, workspaceId: string) {
    if (!(await store.flushForHandoff(id))) {
      toast.error('Save the request first — it could not be saved')
      return
    }
    const result = await (await getRequestService()).generateCurl({ requestId: id, workspaceId })
    if (result.error) {
      toast.error(formatResultError(result.error))
      return
    }
    try {
      await copyText(result.data.command)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
      return
    }
    const warning = warningsToastMessage(result.data.warnings)
    if (warning) toast.info(`Copied as cURL · ${warning}`)
    else toast.success('Copied as cURL')
    useSettingsStore().setSnippetTarget('http', 'curl')
  }

  async function copy(targetKey: string) {
    const id = requestId.value
    const request = store.getById(id)
    if (!request) return
    const workspaceId = workspace.activeWorkspace?.id
    if (!workspaceId) {
      toast.error('Open a workspace first')
      return
    }
    if (request.protocol === 'http' && targetKey === 'curl') {
      await copyCurlFromBackend(id, workspaceId)
      return
    }
    const out = await copySnippetAs(request, workspaceId, targetKey)
    const details = { label: 'Details', onClick: () => codeDialog.open(id, targetKey) }
    if (!out.ok) toast.error(out.error, details)
    else if (out.warnings.length === 0) toast.success(`Copied as ${out.label}`)
    else toast.info(`Copied as ${out.label} · ${warningCount(out.warnings.length)}`, details)
  }

  function generate() {
    codeDialog.open(requestId.value)
  }

  return { copy, generate }
}
