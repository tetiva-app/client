import { computed, watch } from 'vue'
import type { ResultError } from '@/types/common'
import type { DeepLink, ImportConfirmRequest, ImportConfirmResult, ImportPreview } from '@/services'
import { getDeepLinkService, getPortabilityService } from '@/services'
import { guarded } from '@/lib/service-call'
import { formatResultError, isUnreachable, UNREACHABLE_TEXT } from '@/lib/result-error'
import { warningsToastMessage } from '@/lib/auth-warnings'
import { useImportUi, type ImportSource } from '@/stores/importUi'
import { useWorkspaceStore } from '@/stores/workspace'
import { useCollectionStore } from '@/stores/collections'
import { useEnvironmentStore } from '@/stores/environments'
import { useRequestStore } from '@/stores/tabs'
import { useToast } from '@/composables/useToast'

export const MAX_SNAPSHOT_FILE_BYTES = 8 * 1024 * 1024

const SLUG = /^[a-z0-9-]{1,40}-[a-z0-9]{8}$/
const SHARE_HOST = 'share.tetiva.app'
const SNAPSHOT_MARKER = '"tetiva.collection-snapshot"'
const EXPIRED_LINK = 'The link has expired — open it again from the page'

const REASON_TEXT: Record<string, string> = {
  LINK_NOT_FOUND: "This collection isn't published, or the link is wrong",
  PASSWORD_REQUIRED: 'Enter the password to import this collection',
  PASSWORD_INVALID: 'Wrong password',
  RATE_LIMITED: 'Too many attempts — try again in a minute',
  SNAPSHOT_TOO_LARGE: 'The collection is larger than 8 MiB',
  UPDATE_REQUIRED: 'Update Tetiva to import this collection',
  PREVIEW_EXPIRED: 'The download expired — import from the link again',
  UNSUPPORTED_FILE: "This file isn't a Postman or Tetiva collection",
}

// Dev builds flip together with the Go side's !production tag, which lets TETIVA_PUBLIC_API point at a
// local stand or a self-hosted server: the links that server shows live on its own host.
const ANY_SHARE_HOST = import.meta.env.MODE !== 'production'

export function parseShareLink(input: string, anyHost = ANY_SHARE_HOST): string | null {
  const text = input.trim()
  if (SLUG.test(text)) return text
  let url: URL
  try {
    url = new URL(text.startsWith(`${SHARE_HOST}/`) ? `https://${text}` : text)
  } catch {
    return null
  }
  if (url.username || url.password) return null
  const hostOk = anyHost
    ? url.protocol === 'https:' || url.protocol === 'http:'
    : url.protocol === 'https:' && url.host === SHARE_HOST
  if (!hostOk) return null
  const slug = url.pathname.replace(/^\/|\/$/g, '')
  return SLUG.test(slug) ? slug : null
}

// Go checks the size again; this spares the webview a large snapshot while a large Postman file still passes.
export async function snapshotFileTooLarge(file: Blob): Promise<boolean> {
  if (file.size <= MAX_SNAPSHOT_FILE_BYTES) return false
  return (await file.slice(0, 4096).text()).includes(SNAPSHOT_MARKER)
}

export function importErrorText(e: ResultError): string {
  if (isUnreachable(e)) return UNREACHABLE_TEXT
  if (e.reason === 'SNAPSHOT_INVALID') {
    const detail = e.fields?.snapshot
    return detail ? `The collection can't be read: ${detail}` : "The collection can't be read"
  }
  return (e.reason && REASON_TEXT[e.reason]) || formatResultError(e)
}

function plural(n: number, word: string): string {
  return `${n} ${word}${n === 1 ? '' : 's'}`
}

export function importCounts(c: Pick<ImportPreview, 'folders' | 'requests' | 'examples'>): string {
  return [plural(c.folders, 'folder'), plural(c.requests, 'request'), plural(c.examples, 'example')].join(' · ')
}

export function useImportFlow() {
  const ui = useImportUi()
  const workspaces = useWorkspaceStore()
  const collections = useCollectionStore()
  const toast = useToast()

  // Imported requests without auth inherit it, and a folder with none passes its parent's on (request/auth_resolver.go).
  function authOwnerName(folderId: string): string {
    const seen = new Set<string>()
    let c = collections.collectionsMap.get(folderId)
    while (c && !seen.has(c.id)) {
      if (c.authType !== '' && c.authType !== 'none' && c.authType !== 'inherit') return c.name
      seen.add(c.id)
      c = c.parentId ? collections.collectionsMap.get(c.parentId) : undefined
    }
    return ''
  }

  const destination = computed(() => {
    const ws = workspaces.activeWorkspace
    const parentId = ui.source?.kind === 'file' ? ui.source.parentId : null
    const tetiva = ui.preview?.format === 'tetiva'
    return {
      workspaceName: ws?.name ?? '',
      cloud: !!ws?.remoteWorkspaceId,
      folderName: parentId && !tetiva ? collections.collectionsMap.get(parentId)?.name ?? '' : '',
      alwaysTopLevel: parentId !== null && tetiva,
      inheritedAuth: parentId && !tetiva ? authOwnerName(parentId) : '',
    }
  })

  const current = (flow: number) => ui.flow === flow

  function finish() {
    ui.reset()
    pump()
  }

  function pump() {
    if (ui.active) return
    const next = ui.queue.shift()
    if (next) void openLink(ui.reset(), next.slug, next.token, true)
  }

  function showPreview(source: ImportSource, preview: ImportPreview) {
    ui.source = source
    ui.preview = preview
    ui.includeScripts = false
    ui.confirmError = ''
    ui.notice = ''
    ui.expired = false
  }

  function openLinkDialog() {
    if (ui.active) return
    ui.reset()
    ui.linkOpen = true
  }

  async function submitLink(input: string) {
    const slug = parseShareLink(input)
    if (!slug) {
      ui.linkError = 'Paste a share.tetiva.app link or a collection slug'
      return
    }
    await openLink(ui.flow, slug, '', false)
  }

  async function openLink(flow: number, slug: string, token: string, deepLink: boolean) {
    ui.linkOpen = true
    if (deepLink) ui.linkStep = 'opening'
    ui.slug = slug
    ui.token = token
    ui.linkError = ''
    ui.busy = 'Checking the link…'
    const meta = await guarded((await getPortabilityService()).linkMeta(slug))
    if (!current(flow)) return
    ui.busy = ''
    if (meta.error) {
      ui.linkError = importErrorText(meta.error)
      return
    }
    ui.title = meta.data.title
    // Any token here is a deep link's, possibly one enqueue took from a page clicked during the check.
    if (meta.data.passwordRequired && !ui.token) {
      ui.linkStep = 'password'
      return
    }
    await fetchPreview(flow, ui.token, ui.token !== '')
  }

  async function submitPassword(password: string) {
    const flow = ui.flow
    if (password === '') {
      ui.linkError = 'Enter the password'
      return
    }
    ui.linkError = ''
    ui.busy = 'Unlocking…'
    const res = await guarded((await getPortabilityService()).linkUnlock(ui.slug, password))
    if (!current(flow)) return
    ui.busy = ''
    if (res.error) {
      ui.linkError = importErrorText(res.error)
      return
    }
    ui.token = res.data.token
    await fetchPreview(flow, res.data.token, false)
  }

  // The download counts as an import on the server and spends a one-time token, so it runs once per link.
  async function fetchPreview(flow: number, token: string, oneTimeToken: boolean) {
    ui.busy = 'Downloading the collection…'
    const res = await guarded((await getPortabilityService()).linkFetch(ui.slug, token))
    if (!current(flow)) return
    ui.busy = ''
    if (res.error) {
      if (res.error.reason === 'PASSWORD_REQUIRED') {
        ui.token = ''
        ui.linkStep = 'password'
      }
      ui.linkError = importErrorText(res.error)
      return
    }
    ui.linkOpen = false
    showPreview({ kind: 'link', slug: ui.slug, token, previewId: res.data.previewId, oneTimeToken }, res.data.preview)
  }

  async function openFile(file: Blob, parentId: string | null) {
    if (ui.active) {
      toast.info('Finish the current import first')
      return
    }
    const flow = ui.reset()
    ui.busy = 'Reading the file…'
    let content: string
    try {
      if (await snapshotFileTooLarge(file)) {
        toast.error('The file is larger than 8 MiB')
        finish()
        return
      }
      content = await file.text()
    } catch {
      toast.error("Couldn't read the file")
      finish()
      return
    }
    if (!current(flow)) return
    const res = await guarded((await getPortabilityService()).importPreview(content))
    if (!current(flow)) return
    ui.busy = ''
    if (res.error) {
      toast.error(importErrorText(res.error))
      finish()
      return
    }
    showPreview({ kind: 'file', content, parentId }, res.data)
  }

  async function redownload(flow: number, src: Extract<ImportSource, { kind: 'link' }>) {
    if (src.oneTimeToken) {
      ui.confirmError = EXPIRED_LINK
      ui.expired = true
      return
    }
    ui.importing = true
    const res = await guarded((await getPortabilityService()).linkFetch(src.slug, src.token))
    if (!current(flow)) return
    ui.importing = false
    if (res.error) {
      ui.confirmError = importErrorText(res.error)
      return
    }
    showPreview({ ...src, previewId: res.data.previewId }, res.data.preview)
    ui.notice = 'The download expired, so the collection was downloaded again. Review it and import.'
  }

  async function confirm() {
    const flow = ui.flow
    const src = ui.source
    const preview = ui.preview
    if (!src || !preview || ui.importing || ui.expired) return
    const wsId = workspaces.activeWorkspace?.id
    if (!wsId) {
      ui.confirmError = 'No active workspace'
      return
    }
    const req: ImportConfirmRequest = src.kind === 'link'
      ? { previewId: src.previewId, includeScripts: ui.includeScripts, workspaceId: wsId }
      : {
          content: src.content,
          includeScripts: ui.includeScripts,
          workspaceId: wsId,
          ...(preview.format === 'postman' && src.parentId ? { parentId: src.parentId } : {}),
        }
    ui.importing = true
    ui.confirmError = ''
    ui.notice = ''
    const res = await guarded((await getPortabilityService()).importConfirm(req))
    if (!current(flow)) return
    ui.importing = false
    if (res.error) {
      if (res.error.reason === 'PREVIEW_EXPIRED' && src.kind === 'link') {
        await redownload(flow, src)
        return
      }
      ui.confirmError = importErrorText(res.error)
      return
    }
    finish()
    await imported(wsId, preview.title, res.data)
  }

  async function imported(wsId: string, title: string, result: ImportConfirmResult) {
    toast.success(`Imported “${title}”: ${importCounts(result)}`)
    const warning = warningsToastMessage(result.warnings)
    if (warning) toast.info(warning, undefined, { sticky: true })
    await Promise.all([collections.fetchAll(wsId), useEnvironmentStore().fetchAll(wsId)])
    const created = collections.collectionsMap.get(result.collectionId)
    if (created) useRequestStore().openCollectionTab(created.id, created.name)
  }

  // The backend finishes a started import regardless, so the dialog waits for its answer.
  function cancel() {
    if (ui.importing) return
    finish()
  }

  // A flow stopped on an error or an expired link can't import, so a new click for it starts over.
  function canFinish(): boolean {
    if (ui.source) return !ui.expired && !ui.confirmError
    return ui.busy !== '' || ui.linkStep === 'password'
  }

  // Every click on the page mints a new import token, so links are matched by slug; the one with a token wins.
  function enqueue(link: DeepLink) {
    if (ui.active && ui.slug === link.slug) {
      if (!canFinish()) {
        void openLink(ui.reset(), link.slug, link.token, true)
        return
      }
      if (link.token && !ui.token && !ui.source) {
        if (ui.linkStep !== 'password') ui.token = link.token
        else if (!ui.busy) void acceptToken(link.token)
      }
      return
    }
    const queued = ui.queue.find(l => l.slug === link.slug)
    if (!queued) ui.queue.push({ ...link })
    else if (!queued.token && link.token) queued.token = link.token
  }

  async function acceptToken(token: string) {
    ui.token = token
    ui.linkError = ''
    await fetchPreview(ui.flow, token, true)
  }

  // Subscribing first means a link that arrives before the first take triggers a take of its own.
  async function listenDeepLinks(): Promise<() => void> {
    const links = await getDeepLinkService()
    const drain = async () => {
      const res = await guarded(links.takePending())
      if (res.error) return
      for (const link of res.data) enqueue(link)
      pump()
    }
    const off = await links.onReceived(() => { void drain() })
    await drain()
    return off
  }

  // Runs fn once no import is open or queued: the startup welcome must not open over a deep link's import.
  function afterImports(fn: () => void) {
    const idle = () => !ui.active && ui.queue.length === 0
    if (idle()) {
      fn()
      return
    }
    const stop = watch(idle, done => {
      if (!done) return
      stop()
      fn()
    })
  }

  return {
    ui,
    destination,
    openLinkDialog,
    submitLink,
    submitPassword,
    openFile,
    confirm,
    cancel,
    listenDeepLinks,
    afterImports,
  }
}
