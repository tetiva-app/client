import { computed, watch } from 'vue'
import type { ResultError } from '@/types/common'
import type { DeepLink, ImportConfirmResult, ImportEnvironmentResult, ImportPreview, ImportWarning } from '@/services'
import { getDeepLinkService, getPortabilityService } from '@/services'
import { guarded } from '@/lib/service-call'
import { contentSaved } from '@/lib/content-saved'
import { formatResultError, isUnreachable } from '@/lib/result-error'
import { warningsToastMessage } from '@/lib/auth-warnings'
import { freeName } from '@/lib/free-name'
import { useImportUi, type ImportSource } from '@/stores/importUi'
import { useWorkspaceStore } from '@/stores/workspace'
import { useCollectionStore } from '@/stores/collections'
import { useEnvironmentStore } from '@/stores/environments'
import { useEnvModalUi } from '@/stores/envModalUi'
import { useRequestStore } from '@/stores/tabs'
import { useToast } from '@/composables/useToast'
import { currentLocale, fill, plural, type Locale } from '@/lib/locale'
import { IMPORT_COPY, type ImportErrorReason } from '@/components/import/copy'
import { importWarningText } from '@/components/import/warnings'

export const MAX_IMPORT_FILE_BYTES = 48 * 1024 * 1024
export const MAX_SNAPSHOT_FILE_BYTES = 8 * 1024 * 1024

const SLUG = /^[a-z0-9-]{1,40}-[a-z0-9]{8}$/
const SHARE_HOST = 'share.tetiva.app'
const SNAPSHOT_MARKER = '"tetiva.collection-snapshot"'

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

export async function snapshotFileTooLarge(file: Blob): Promise<boolean> {
  if (file.size <= MAX_SNAPSHOT_FILE_BYTES) return false
  return (await file.slice(0, 4096).text()).includes(SNAPSHOT_MARKER)
}

export async function fileTooLargeText(file: Blob, locale: Locale = currentLocale.value): Promise<string> {
  const errors = IMPORT_COPY[locale].errors
  if (file.size > MAX_IMPORT_FILE_BYTES) return errors.reasons.FILE_TOO_LARGE
  return (await snapshotFileTooLarge(file)) ? errors.snapshotFileTooLarge : ''
}

function isReason(reason: string | undefined): reason is ImportErrorReason {
  return !!reason && Object.prototype.hasOwnProperty.call(IMPORT_COPY.en.errors.reasons, reason)
}

export function importErrorText(e: ResultError, locale: Locale = currentLocale.value): string {
  const errors = IMPORT_COPY[locale].errors
  if (isUnreachable(e)) return errors.unreachable
  if (e.reason === 'SNAPSHOT_INVALID') {
    const detail = e.fields?.snapshot
    return detail ? fill(errors.unreadableDetail, { detail }) : errors.unreadable
  }
  return isReason(e.reason) ? errors.reasons[e.reason] : formatResultError(e)
}

export function importCounts(
  c: Pick<ImportPreview, 'folders' | 'requests' | 'examples'>,
  locale: Locale = currentLocale.value,
): string {
  const counts = IMPORT_COPY[locale].counts
  return [
    plural(locale, c.folders, counts.folders),
    plural(locale, c.requests, counts.requests),
    plural(locale, c.examples, counts.examples),
  ].join(' · ')
}

export function environmentPhrase(names: string[], locale: Locale = currentLocale.value): string {
  const copy = IMPORT_COPY[locale]
  const quoted = names.map(name => fill(copy.quoted, { name })).join(', ')
  return fill(names.length === 1 ? copy.environments.single : copy.environments.several, { names: quoted })
}

export function importedEnvironmentText(
  r: Pick<ImportEnvironmentResult, 'environmentNames' | 'variablesCreated'>,
  locale: Locale = currentLocale.value,
): string {
  const copy = IMPORT_COPY[locale]
  const text = r.environmentNames.length === 1 ? copy.results.importedEnvironments.single : copy.results.importedEnvironments.several
  return fill(text, {
    environments: environmentPhrase(r.environmentNames, locale),
    variables: plural(locale, r.variablesCreated, copy.counts.variables),
  })
}

export function importWarningsText(warnings: ImportWarning[] | undefined, locale: Locale = currentLocale.value): string {
  const more = IMPORT_COPY[locale].results.moreWarnings
  const texts = (warnings ?? []).map(w => importWarningText(w, locale))
  return warningsToastMessage(texts, { more: rest => plural(locale, rest, more) })
}

export function useImportFlow() {
  const ui = useImportUi()
  const workspaces = useWorkspaceStore()
  const collections = useCollectionStore()
  const environments = useEnvironmentStore()
  const envModalUi = useEnvModalUi()
  const toast = useToast()
  const copy = () => IMPORT_COPY[currentLocale.value]

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
    const ownAuth = tetiva || ui.preview?.format === 'har'
    const taken = environments.environments.map(e => e.name)
    const envNames = (ui.preview?.environmentNames ?? []).map(name => {
      const free = freeName(name, taken)
      taken.push(free)
      return free
    })
    return {
      workspaceName: ws?.name ?? '',
      cloud: !!ws?.remoteWorkspaceId,
      folderName: parentId && !tetiva ? collections.collectionsMap.get(parentId)?.name ?? '' : '',
      alwaysTopLevel: parentId !== null && tetiva,
      inheritedAuth: parentId && !ownAuth ? authOwnerName(parentId) : '',
      environmentNames: envNames,
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
      ui.linkError = copy().errors.badLink
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
    ui.busy = copy().busy.checking
    const meta = await guarded((await getPortabilityService()).linkMeta(slug))
    if (!current(flow)) return
    ui.busy = ''
    if (meta.error) {
      ui.linkError = importErrorText(meta.error)
      return
    }
    ui.title = meta.data.title
    // Any token here is a deep link's, possibly one enqueue took during the check.
    if (meta.data.passwordRequired && !ui.token) {
      ui.linkStep = 'password'
      return
    }
    await fetchPreview(flow, ui.token, ui.token !== '')
  }

  async function submitPassword(password: string) {
    const flow = ui.flow
    if (password === '') {
      ui.linkError = copy().errors.enterPassword
      return
    }
    ui.linkError = ''
    ui.busy = copy().busy.unlocking
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

  async function fetchPreview(flow: number, token: string, oneTimeToken: boolean) {
    ui.busy = copy().busy.downloading
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
      toast.info(copy().errors.importInProgress)
      return
    }
    const flow = ui.reset()
    const wsId = workspaces.activeWorkspace?.id ?? ''
    ui.busy = copy().busy.reading
    let content: string
    try {
      const tooLarge = await fileTooLargeText(file)
      if (tooLarge) {
        toast.error(tooLarge)
        finish()
        return
      }
      content = await file.text()
    } catch {
      toast.error(copy().errors.readFailed)
      finish()
      return
    }
    if (!current(flow)) return
    const fileName = file instanceof File ? file.name : ''
    const res = await guarded((await getPortabilityService()).importPreview(content, fileName))
    if (!current(flow)) return
    if (res.error?.reason === 'ENVIRONMENT_FILE') {
      await importEnvironmentFile(flow, content, wsId)
      return
    }
    ui.busy = ''
    if (res.error) {
      toast.error(importErrorText(res.error))
      finish()
      return
    }
    showPreview({ kind: 'file', previewId: res.data.previewId, content, fileName, parentId }, res.data.preview)
  }

  async function importEnvironmentFile(flow: number, content: string, wsId: string) {
    if (!wsId) {
      toast.error(copy().errors.noWorkspace)
      finish()
      return
    }
    ui.busy = copy().busy.importingEnvironment
    const res = await guarded((await getPortabilityService()).importEnvironment(content, wsId))
    if (current(flow)) finish()
    if (res.error) {
      toast.error(importErrorText(res.error))
      return
    }
    const id = res.data.environmentId
    const stillHere = workspaces.activeWorkspace?.id === wsId
    toast.success(
      importedEnvironmentText(res.data),
      stillHere ? { label: copy().results.open, onClick: () => envModalUi.openForEnvironment(id) } : undefined,
    )
    const warning = importWarningsText(res.data.warnings)
    if (warning) toast.info(warning, undefined, { sticky: true })
    if (stillHere) await environments.fetchAll(wsId)
  }

  async function redownload(flow: number, src: Extract<ImportSource, { kind: 'link' }>) {
    if (src.oneTimeToken) {
      ui.confirmError = copy().errors.expiredLink
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
    ui.notice = copy().results.downloadedAgain
  }

  async function confirm() {
    const flow = ui.flow
    const src = ui.source
    const preview = ui.preview
    if (!src || !preview || ui.importing || ui.expired) return
    const wsId = workspaces.activeWorkspace?.id
    if (!wsId) {
      ui.confirmError = copy().errors.noWorkspace
      return
    }
    const opts = {
      includeScripts: ui.includeScripts,
      workspaceId: wsId,
      ...(src.kind === 'file' && preview.format !== 'tetiva' && src.parentId ? { parentId: src.parentId } : {}),
    }
    ui.importing = true
    ui.confirmError = ''
    ui.notice = ''
    const svc = await getPortabilityService()
    let res = await guarded(svc.importConfirm({ previewId: src.previewId, ...opts }))
    if (!current(flow)) return
    if (res.error?.reason === 'PREVIEW_EXPIRED' && src.kind === 'file') {
      res = await guarded(svc.importConfirm({ content: src.content, fileName: src.fileName, ...opts }))
      if (!current(flow)) return
    }
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
    await imported(wsId, preview, res.data)
  }

  async function imported(wsId: string, preview: ImportPreview, result: ImportConfirmResult) {
    const names = result.environmentNames
    const active = result.activeEnvironment
      ? fill(copy().results.activeEnvironment, { name: fill(copy().quoted, { name: result.activeEnvironment }) })
      : ''
    const env = active ? ` · ${active}` : names.length > 0 ? ` · ${environmentPhrase(names)}` : ''
    const title = preview.title || copy().confirm.untitled
    toast.success(fill(copy().results.imported, { title, counts: importCounts(result) }) + env)
    const warning = importWarningsText(result.warnings)
    if (warning) toast.info(warning, undefined, { sticky: true })
    await Promise.all([collections.fetchAll(wsId), environments.fetchAll(wsId)])
    contentSaved()
    const created = collections.collectionsMap.get(result.collectionId)
    if (created) useRequestStore().openCollectionTab(created.id, created.name)
  }

  // A started import finishes in the backend regardless; the dialog waits for its answer.
  function cancel() {
    if (ui.importing) return
    finish()
  }

  function canFinish(): boolean {
    if (ui.source) return !ui.expired && !ui.confirmError
    return ui.busy !== '' || ui.linkStep === 'password'
  }

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
