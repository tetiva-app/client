import { computed, ref } from 'vue'
import type { ResultError } from '@/types/common'
import type { Environment } from '@/types/environment'
import type {
  HiddenVar,
  PublicationStatus,
  PublishPlan,
  PublishPreview,
  Redaction,
  ScanWarning,
  Visibility,
} from '@/types/publication'
import { getEnvironmentService, getPublicationService } from '@/services'
import { guarded } from '@/lib/service-call'
import { formatResultError } from '@/lib/result-error'
import { publicationErrorText, type PublicationErrorText } from '@/lib/publication-errors'
import { currentLocale, type Locale } from '@/lib/locale'
import { PUBLICATION_COPY } from '@/components/publication/copy'
import { unavailableText, usePublicationsStore } from '@/stores/publications'
import { useEnvironmentStore } from '@/stores/environments'
import { useCollectionStore } from '@/stores/collections'
import { useRequestStore } from '@/stores/tabs'
import { emitWailsEvent } from '@/composables/useWindowEvents'

export interface PublishTarget {
  id: string
  workspaceId: string
  name: string
}

export interface HiddenRow extends HiddenVar { toggle: boolean; on: boolean }
export interface RemovedRow extends Redaction { toggle: boolean; on: boolean }
export interface WarningRow extends ScanWarning { toggle: boolean; on: boolean }

export type PublishError =
  | { kind: 'save-failed' }
  | { kind: 'publication'; error: ResultError }
  | { kind: 'other'; error: ResultError }

const MIN_PASSWORD_BYTES = 8
const MAX_PASSWORD_BYTES = 72

function wordError(e: PublishError, locale: Locale): PublicationErrorText {
  switch (e.kind) {
    case 'save-failed': return { text: PUBLICATION_COPY[locale].dialog.saveFailed }
    case 'publication': return publicationErrorText(e.error, locale)
    case 'other': return { text: formatResultError(e.error) }
  }
}

function warningSignature(p: PublishPreview | null): string {
  return (p?.warnings ?? []).map(w => w.selector).sort().join('\n')
}

// The server brings a revoked page back under its old slug, and a password page keeps its password.
function reopens(st: PublicationStatus | null): boolean {
  return !!st && !st.published && st.canManage && st.publicUrl !== ''
}

export function usePublishDialog() {
  const publications = usePublicationsStore()

  let target: PublishTarget | null = null
  let previewSeq = 0
  let confirmedPublic = false

  const status = ref<PublicationStatus | null>(null)
  const loading = ref(false)
  const loadFailed = ref<string | null>(null)
  const environments = ref<Environment[]>([])

  const visibility = ref<Visibility>('public')
  const password = ref('')
  const environmentId = ref('')
  const environmentMissing = ref(false)
  const includeScripts = ref(true)
  const publishAsIs = ref<string[]>([])

  const preview = ref<PublishPreview | null>(null)
  const previewing = ref(false)
  const previewFailure = ref<PublishError | null>(null)
  const acknowledged = ref(false)

  const featureLocked = ref(false)
  const plan = ref<PublishPlan | null>(null)
  const confirmPublicOpen = ref(false)
  const publishing = ref(false)
  const error = ref<PublishError | null>(null)

  const copy = computed(() => PUBLICATION_COPY[currentLocale.value].dialog)
  const errorText = computed(() => (error.value ? wordError(error.value, currentLocale.value) : null))
  const previewError = computed(() => (previewFailure.value ? wordError(previewFailure.value, currentLocale.value).text : ''))
  const loadError = computed(() => {
    const id = loadFailed.value
    return id === null ? '' : publications.errorOf(id) || copy.value.loadFailed
  })

  const isUpdate = computed(() => status.value?.published === true)

  const unavailable = computed(() => {
    const st = status.value
    return st && !st.available ? unavailableText(st.reasonUnavailable) : ''
  })

  const manageText = computed(() =>
    status.value?.published && !status.value.canManage ? copy.value.manage : '')

  const publishable = computed(() => status.value?.available === true && !manageText.value)

  const reopenUrl = computed(() => (reopens(status.value) ? status.value!.publicUrl : ''))

  const keepsPassword = computed(() => {
    const st = status.value
    return !!st && (st.published || reopens(st)) && st.visibility === 'password'
  })

  const passwordError = computed(() => {
    if (visibility.value !== 'password' || password.value === '') return ''
    const bytes = new TextEncoder().encode(password.value).length
    return bytes < MIN_PASSWORD_BYTES || bytes > MAX_PASSWORD_BYTES ? copy.value.passwordInvalid : ''
  })

  const passwordMissing = computed(() =>
    visibility.value === 'password' && password.value === '' && !keepsPassword.value)

  // The server lets a page keep its visibility after a downgrade, but a new password needs the feature again.
  function keepsVisibility(v: Visibility): boolean {
    const st = status.value
    return !!st && (st.published || reopens(st)) && st.visibility === v && !(v === 'password' && password.value !== '')
  }

  const planLocked = computed<Visibility[]>(() => {
    const p = plan.value
    if (!p) return []
    const lacking: Visibility[] = []
    if (!p.unlisted) lacking.push('unlisted')
    if (!p.password) lacking.push('password')
    return lacking.filter(v => !keepsVisibility(v))
  })

  const lockedVisibilities = computed<Visibility[]>(() =>
    featureLocked.value ? ['unlisted', 'password'] : planLocked.value)

  const visibilityLocked = computed(() => lockedVisibilities.value.includes(visibility.value))

  // Unlike the locks, an unknown plan offers nothing: Free must not be pointed at a locked option.
  const unlistedOffered = computed(() => plan.value !== null && !lockedVisibilities.value.includes('unlisted'))

  const hiddenRows = computed<HiddenRow[]>(() =>
    (preview.value?.hiddenVars ?? []).map(h => ({ ...h, toggle: h.overridable, on: publishAsIs.value.includes(h.selector) })))

  // A hidden variable shows up in both lists; its switch lives in the variables section.
  const removedRows = computed<RemovedRow[]>(() =>
    (preview.value?.redactions ?? [])
      .filter(r => r.category !== 'var')
      .map(r => ({ ...r, toggle: r.overridable, on: publishAsIs.value.includes(r.selector) })))

  const warningRows = computed<WarningRow[]>(() =>
    (preview.value?.warnings ?? []).map(w => ({ ...w, toggle: true, on: publishAsIs.value.includes(w.selector) })))

  const toggleable = computed(() => new Set([
    ...hiddenRows.value.filter(r => r.toggle).map(r => r.selector),
    ...removedRows.value.filter(r => r.toggle).map(r => r.selector),
    ...warningRows.value.map(r => r.selector),
  ]))

  const canPublish = computed(() => {
    const p = preview.value
    if (!target || !p || loading.value || previewing.value || publishing.value) return false
    if (!publishable.value) return false
    if (p.errors.length > 0) return false
    if (p.sizeBytes > p.sizeLimitBytes || p.gzipBytes > p.gzipLimitBytes) return false
    if (p.warnings.length > 0 && !acknowledged.value) return false
    if (planLocked.value.includes(visibility.value)) return false
    return passwordError.value === '' && !passwordMissing.value
  })

  function reset() {
    status.value = null
    loadFailed.value = null
    environments.value = []
    visibility.value = 'public'
    password.value = ''
    environmentId.value = ''
    environmentMissing.value = false
    includeScripts.value = true
    publishAsIs.value = []
    preview.value = null
    previewing.value = false
    previewFailure.value = null
    acknowledged.value = false
    featureLocked.value = false
    plan.value = null
    confirmPublicOpen.value = false
    publishing.value = false
    error.value = null
    confirmedPublic = false
  }

  async function loadEnvironments(workspaceId: string): Promise<Environment[]> {
    const res = await guarded((await getEnvironmentService()).list(workspaceId))
    return res.error ? [] : res.data
  }

  // An unknown plan locks nothing: the server's refusal is the fallback.
  async function loadPlan(collectionId: string): Promise<PublishPlan | null> {
    const res = await guarded((await getPublicationService()).plan(collectionId))
    return res.error ? null : res.data
  }

  function applyStatus(st: PublicationStatus | null, envs: Environment[]) {
    visibility.value = st && (st.published || reopens(st)) && st.visibility ? st.visibility : 'public'
    const s = st?.settings
    if (!s) return
    const missing = s.environmentMissing || (s.environmentId !== '' && !envs.some(e => e.id === s.environmentId))
    environmentMissing.value = missing
    environmentId.value = missing ? '' : s.environmentId
    includeScripts.value = s.includeScripts
    publishAsIs.value = [...s.publishAsIs]
  }

  async function start(t: PublishTarget) {
    reset()
    target = t
    loading.value = true
    // Not awaited: a slow plan lookup must not hold the dialog, and until it lands the server's refusal still works.
    void loadPlan(t.id).then(p => { if (target === t) plan.value = p })
    const [st, envs] = await Promise.all([publications.refresh(t.id), loadEnvironments(t.workspaceId)])
    if (target !== t) return
    loading.value = false
    status.value = st
    if (!st) loadFailed.value = t.id
    environments.value = envs
    applyStatus(st, envs)
    if (publishable.value) await refreshPreview()
  }

  // The preview and the publish read the database, so edits still open in the editor go first.
  function saveEdits(t: PublishTarget): Promise<boolean> {
    return useRequestStore().flushCollections(useCollectionStore().collectSubtreeIds(t.id))
  }

  async function refreshPreview() {
    const t = target
    if (!t) return
    const seq = ++previewSeq
    previewing.value = true
    previewFailure.value = null
    const saved = await saveEdits(t)
    if (seq !== previewSeq || target !== t) return
    if (!saved) {
      previewing.value = false
      preview.value = null
      previewFailure.value = { kind: 'save-failed' }
      return
    }
    const res = await guarded((await getPublicationService()).preview({
      collectionId: t.id,
      workspaceId: t.workspaceId,
      environmentId: environmentId.value,
      includeScripts: includeScripts.value,
      publishAsIs: [...publishAsIs.value],
    }))
    if (seq !== previewSeq || target !== t) return
    previewing.value = false
    if (res.error) {
      preview.value = null
      previewFailure.value = { kind: 'publication', error: res.error }
      return
    }
    if (warningSignature(res.data) !== warningSignature(preview.value)) acknowledged.value = false
    preview.value = res.data
  }

  function setVisibility(v: Visibility) {
    visibility.value = v
    confirmedPublic = false
    error.value = null
  }

  async function setEnvironment(id: string) {
    environmentId.value = id
    environmentMissing.value = false
    await refreshPreview()
  }

  async function setIncludeScripts(on: boolean) {
    includeScripts.value = on
    await refreshPreview()
  }

  async function toggleOverride(selector: string, on: boolean) {
    if (!toggleable.value.has(selector)) return
    const rest = publishAsIs.value.filter(s => s !== selector)
    publishAsIs.value = on ? [...rest, selector] : rest
    await refreshPreview()
  }

  async function makeSecret(variableId: string) {
    const envId = environmentId.value
    if (!envId) return
    const res = await guarded((await getPublicationService()).markVariableSecret(envId, variableId))
    if (res.error) {
      error.value = { kind: 'other', error: res.error }
      return
    }
    const selector = preview.value?.hiddenVars.find(h => h.variableId === variableId)?.selector
    if (selector) publishAsIs.value = publishAsIs.value.filter(s => s !== selector)
    void emitWailsEvent('env:changed')
    const envStore = useEnvironmentStore()
    if (envStore.variablesMap.has(envId)) void envStore.fetchVariables(envId)
    await refreshPreview()
  }

  function needsPublicConfirm(): boolean {
    const st = status.value
    return !confirmedPublic && visibility.value === 'public' && st?.published === true
      && (st.visibility === 'unlisted' || st.visibility === 'password')
  }

  async function send(t: PublishTarget, p: PublishPreview): Promise<PublicationStatus | null> {
    publishing.value = true
    error.value = null
    if (!(await saveEdits(t))) {
      publishing.value = false
      if (target === t) error.value = { kind: 'save-failed' }
      return null
    }
    const res = await guarded((await getPublicationService()).publish({
      collectionId: t.id,
      workspaceId: t.workspaceId,
      environmentId: environmentId.value,
      includeScripts: includeScripts.value,
      publishAsIs: [...publishAsIs.value],
      visibility: visibility.value,
      password: visibility.value === 'password' && password.value !== '' ? password.value : null,
      locale: currentLocale.value,
      confirmMakePublic: confirmedPublic,
      acknowledgedWarnings: acknowledged.value,
      previewHash: p.previewHash,
    }))
    publishing.value = false
    publications.notePublishResult(res.error ?? null)
    if (!res.error) void publications.refreshList({ remote: true })
    if (target !== t) return null
    if (!res.error) {
      publications.setStatus(t.id, res.data)
      status.value = res.data
      return res.data
    }
    const { action } = publicationErrorText(res.error)
    if (action === 'confirm-public') {
      confirmPublicOpen.value = true
      return null
    }
    if (res.error.reason === 'PUBLISH_FEATURE_REQUIRED') featureLocked.value = true
    error.value = { kind: 'publication', error: res.error }
    if (action === 'review-again') await refreshPreview()
    return null
  }

  async function publish(): Promise<PublicationStatus | null> {
    const t = target
    const p = preview.value
    if (!t || !p || !canPublish.value) return null
    if (needsPublicConfirm()) {
      confirmPublicOpen.value = true
      return null
    }
    return send(t, p)
  }

  async function confirmPublic(): Promise<PublicationStatus | null> {
    confirmPublicOpen.value = false
    confirmedPublic = true
    return publish()
  }

  function cancelConfirmPublic() {
    confirmPublicOpen.value = false
  }

  return {
    status,
    loading,
    loadError,
    environments,
    visibility,
    password,
    environmentId,
    environmentMissing,
    includeScripts,
    publishAsIs,
    preview,
    previewing,
    previewError,
    acknowledged,
    featureLocked,
    plan,
    lockedVisibilities,
    visibilityLocked,
    unlistedOffered,
    confirmPublicOpen,
    publishing,
    error,
    errorText,
    isUpdate,
    unavailableText: unavailable,
    manageText,
    reopenUrl,
    keepsPassword,
    passwordError,
    hiddenRows,
    removedRows,
    warningRows,
    canPublish,
    start,
    refreshPreview,
    setVisibility,
    setEnvironment,
    setIncludeScripts,
    toggleOverride,
    makeSecret,
    publish,
    confirmPublic,
    cancelConfirmPublic,
  }
}
