import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import type { Request, Protocol } from '@/types/request'
import { getRequestService } from '@/services'
import { DEFAULT_PROTOCOL, DEFAULT_METHOD, DEFAULT_BODY_TYPE, DEFAULT_AUTH_TYPE, DEFAULT_AUTH_DATA } from '@/constants/defaults'
import { useWorkspaceStore } from '@/stores/workspace'
import { clearDrafts } from '@/composables/useBodyDrafts'
import { descriptionSaveBlocked } from '@/lib/description'
import { useResponseStore } from '@/stores/responses'
import { runMutation } from '@/stores/runMutation'
import { formatResultError } from '@/lib/result-error'
import { useToast } from '@/composables/useToast'

export type Tab =
  | { id: string; type: 'request'; requestId: string; name: string; method: string; protocol: string }
  | { id: string; type: 'collection'; collectionId: string; name: string }

export const AUTOSAVE_DELAY_MS = 1500

export const useRequestStore = defineStore('requests', () => {
  const requestsMap = ref<Map<string, Request>>(new Map())
  const loading = ref(false)

  const savedSnapshots = ref<Map<string, Request>>(new Map())

  const openTabs = ref<Tab[]>([])
  const activeTabId = ref<string | null>(null)

  const activeTab = computed(() => {
    if (!activeTabId.value) return null
    return openTabs.value.find(t => t.id === activeTabId.value) ?? null
  })

  const collectionInitialSections = ref(new Map<string, string>())

  const collectionEditorRefs = ref(new Map<string, { saveScripts: () => Promise<boolean>; scriptsDirty: boolean }>())

  function registerCollectionEditor(collectionId: string, editorRef: { saveScripts: () => Promise<boolean>; scriptsDirty: boolean }) {
    collectionEditorRefs.value.set(collectionId, editorRef)
  }

  function unregisterCollectionEditor(collectionId: string) {
    collectionEditorRefs.value.delete(collectionId)
  }

  function byCollection(collectionId: string): Request[] {
    return Array.from(requestsMap.value.values())
      .filter(r => r.collectionId === collectionId)
      .sort((a, b) => a.sortOrder - b.sortOrder)
  }

  function isDirty(tabId: string): boolean {
    const tab = openTabs.value.find(t => t.id === tabId)
    if (!tab) return false
    if (tab.type === 'request') {
      return isRequestDirty(tab.requestId)
    }
    if (tab.type === 'collection') {
      const editorRef = collectionEditorRefs.value.get(tab.collectionId)
      return editorRef?.scriptsDirty ?? false
    }
    return false
  }

  function getById(id: string): Request | undefined {
    return requestsMap.value.get(id)
  }

  async function fetchByCollection(collectionId: string) {
    loading.value = true
    try {
      const service = await getRequestService()
      const result = await service.list(collectionId)
      if (result.error) {
        console.error('Failed to fetch requests:', result.error.message)
        return
      }
      const dirty = new Set(Array.from(requestsMap.value.keys()).filter(isRequestDirty))
      const incoming = new Map(result.data.map(item => [item.id, item]))
      // Replay drafts are hidden from list() (is_draft = 1), so a refresh must
      // not evict them: the open replay tab would render nothing and the close
      // path could no longer find the draft to hard-delete it.
      for (const [id, req] of requestsMap.value) {
        if (req.collectionId !== collectionId || req.isDraft || incoming.has(id)) continue
        // Deleted on another device: the buffer has nothing left to save into.
        if (dirty.has(id)) {
          cancelAutosave(id)
          console.warn(`Request ${id} is gone from the server; dropping its unsaved edits`)
        }
        requestsMap.value.delete(id)
        savedSnapshots.value.delete(id)
      }
      for (const item of result.data) {
        // A list that started before the last save carries the version that save replaced.
        const existing = requestsMap.value.get(item.id)
        if (existing && existing.version >= item.version) continue
        // An unsaved buffer outranks the list: adopt the version so the next save wins.
        if (dirty.has(item.id)) {
          adoptServerVersion(item.id, item.version, item.updatedAt)
          continue
        }
        requestsMap.value.set(item.id, item)
        savedSnapshots.value.set(item.id, { ...item })
      }
      requestsMap.value = new Map(requestsMap.value)
      savedSnapshots.value = new Map(savedSnapshots.value)
    } catch (err) {
      console.error('Failed to fetch requests:', err)
    } finally {
      loading.value = false
    }
  }

  async function create(collectionId: string, name: string, options?: { protocol?: Protocol }): Promise<Request | null> {
    const protocol = options?.protocol ?? DEFAULT_PROTOCOL
    const method = (protocol === 'grpc' || protocol === 'graphql') ? 'POST' as const : DEFAULT_METHOD
    // websocket keeps its settings document in the body
    const bodyType = protocol === 'grpc'
      ? 'json' as const
      : protocol === 'graphql'
        ? 'none' as const
        : protocol === 'websocket'
          ? 'raw' as const
          : DEFAULT_BODY_TYPE
    const data = await runMutation('Failed to create request', () =>
      getRequestService().then(s => s.create({
        collectionId,
        name,
        description: '',
        protocol,
        method,
        url: '',
        headers: [],
        body: '',
        bodyType,
        authType: DEFAULT_AUTH_TYPE,
        authData: DEFAULT_AUTH_DATA,
        preScript: '',
        postScript: '',
      }))
    )
    if (!data) return null
    requestsMap.value.set(data.id, data)
    savedSnapshots.value.set(data.id, { ...data })
    requestsMap.value = new Map(requestsMap.value)
    savedSnapshots.value = new Map(savedSnapshots.value)
    return data
  }

  function loadRequest(request: Request) {
    requestsMap.value.set(request.id, request)
    savedSnapshots.value.set(request.id, { ...request })
    requestsMap.value = new Map(requestsMap.value)
    savedSnapshots.value = new Map(savedSnapshots.value)
  }

  // In-memory only; persisted later via saveToBackend (hybrid save).
  function updateLocal(id: string, partial: Partial<Request>) {
    const existing = requestsMap.value.get(id)
    if (!existing) return
    const updated = { ...existing, ...partial }
    requestsMap.value.set(id, updated)
    requestsMap.value = new Map(requestsMap.value)
    // Only docs autosave: a URL or header would push to sync on every pause in typing.
    if ('description' in partial && !updated.isDraft) scheduleAutosave(id)
  }

  // An over-cap description the backend refuses: flushing it only repeats the toast.
  function isSaveBlocked(id: string): boolean {
    const current = requestsMap.value.get(id)
    if (!current) return false
    return descriptionSaveBlocked(current.description, savedSnapshots.value.get(id)?.description ?? '')
  }

  function isRequestDirty(requestId: string): boolean {
    const saved = savedSnapshots.value.get(requestId)
    const current = requestsMap.value.get(requestId)
    if (!saved || !current) return false
    return JSON.stringify(saved) !== JSON.stringify(current)
  }

  // Dedupe concurrent saves — a parallel edit would lose the version race
  const savesInFlight = new Map<string, Promise<boolean>>()

  // Idle autosave per request; a keystroke-level cadence would push every edit through sync.
  const autosaveTimers = new Map<string, ReturnType<typeof setTimeout>>()

  function scheduleAutosave(id: string) {
    cancelAutosave(id)
    autosaveTimers.set(id, setTimeout(() => {
      autosaveTimers.delete(id)
      const req = requestsMap.value.get(id)
      // A save that can only fail would raise a toast every 1.5 s.
      if (!req || req.isDraft || isSaveBlocked(id)) return
      void flush(id)
    }, AUTOSAVE_DELAY_MS))
  }

  function cancelAutosave(id: string) {
    const timer = autosaveTimers.get(id)
    if (timer !== undefined) {
      clearTimeout(timer)
      autosaveTimers.delete(id)
    }
  }

  // Last chance on unload: every buffer the idle timer has not reached yet.
  async function flushAllDirty(): Promise<void> {
    const requestIds = Array.from(requestsMap.value.values())
      .filter(r => !r.isDraft && isRequestDirty(r.id))
      .map(r => r.id)
    const editors = Array.from(collectionEditorRefs.value.values()).filter(e => e.scriptsDirty)
    await Promise.all([
      ...requestIds.map(id => flush(id)),
      ...editors.map(e => e.saveScripts().catch(() => false)),
    ])
  }

  function saveToBackend(id: string): Promise<boolean> {
    const inFlight = savesInFlight.get(id)
    if (inFlight) return inFlight
    const promise = doSave(id).finally(() => savesInFlight.delete(id))
    savesInFlight.set(id, promise)
    return promise
  }

  // Local edits stay, only the row's identity in the version race is refreshed.
  function adoptServerVersion(id: string, version: number, updatedAt: string) {
    const current = requestsMap.value.get(id)
    if (current) requestsMap.value.set(id, { ...current, version, updatedAt })
    const snapshot = savedSnapshots.value.get(id)
    if (snapshot) savedSnapshots.value.set(id, { ...snapshot, version, updatedAt })
    requestsMap.value = new Map(requestsMap.value)
    savedSnapshots.value = new Map(savedSnapshots.value)
  }

  async function doSave(id: string, retried = false): Promise<boolean> {
    const current = requestsMap.value.get(id)
    if (!current) return true
    if (!isRequestDirty(id)) return true

    try {
      const service = await getRequestService()
      const result = await service.edit({
        id: current.id,
        name: current.name,
        description: current.description,
        method: current.method,
        url: current.url,
        headers: current.headers,
        body: current.body,
        bodyType: current.bodyType,
        authType: current.authType,
        authData: current.authData,
        preScript: current.preScript,
        postScript: current.postScript,
        version: current.version,
        grpcService: current.grpcService,
        grpcMethod: current.grpcMethod,
        grpcProtoPath: current.grpcProtoPath,
        grpcMetadata: current.grpcMetadata,
        graphqlQuery: current.graphqlQuery,
        graphqlVariables: current.graphqlVariables,
        graphqlSchemaPath: current.graphqlSchemaPath,
        graphqlOperation: current.graphqlOperation,
      })
      if (result.error) {
        // Another device won the version race; the fresh version lets this save through.
        if (result.error.code === 'conflict' && !retried) {
          const fresh = await service.getById(id)
          if (!fresh.error) {
            adoptServerVersion(id, fresh.data.version, fresh.data.updatedAt)
            return doSave(id, true)
          }
        }
        console.error('Failed to save request:', result.error.message)
        useToast().error(`Failed to save request: ${formatResultError(result.error)}`)
        return false
      }
      const latest = requestsMap.value.get(id)
      if (latest && latest !== current) {
        // User typed during the save: keep local edits, adopt only the server version
        requestsMap.value.set(id, { ...latest, version: result.data.version, updatedAt: result.data.updatedAt })
      } else {
        requestsMap.value.set(result.data.id, result.data)
      }
      savedSnapshots.value.set(result.data.id, { ...result.data })
      requestsMap.value = new Map(requestsMap.value)
      savedSnapshots.value = new Map(savedSnapshots.value)
      return true
    } catch (err) {
      console.error('Failed to save request:', err)
      useToast().error('Failed to save request')
      return false
    }
  }

  // saveToBackend may return a save that started before the last edit, so this loops until clean.
  async function flushForHandoff(requestId: string): Promise<boolean> {
    for (let round = 0; round < 3; round++) {
      if (!isRequestDirty(requestId)) return true
      if (!(await saveToBackend(requestId))) return false
    }
    return !isRequestDirty(requestId)
  }

  // A replay draft dies with its tab: only the handoff paths persist it.
  function flush(requestId: string): Promise<boolean> {
    if (requestsMap.value.get(requestId)?.isDraft) return Promise.resolve(true)
    return flushForHandoff(requestId)
  }

  async function rename(id: string, newName: string, version: number): Promise<boolean> {
    // edit() assigns every field, so a pending save has to land before the rename.
    const hadAutosave = autosaveTimers.has(id)
    cancelAutosave(id)
    if (!(await flush(id))) {
      // The timer this cancelled was the buffer's only way back to the backend.
      if (hadAutosave && isRequestDirty(id) && !isSaveBlocked(id)) scheduleAutosave(id)
      return false
    }
    const current = requestsMap.value.get(id)
    if (!current) return false

    try {
      const service = await getRequestService()
      const result = await service.edit({
        id: current.id,
        name: newName,
        // edit.go assigns every field, so anything not sent here is erased
        description: current.description,
        method: current.method,
        url: current.url,
        headers: current.headers,
        body: current.body,
        bodyType: current.bodyType,
        authType: current.authType,
        authData: current.authData,
        preScript: current.preScript,
        postScript: current.postScript,
        version: current.version ?? version,
        grpcService: current.grpcService,
        grpcMethod: current.grpcMethod,
        grpcProtoPath: current.grpcProtoPath,
        grpcMetadata: current.grpcMetadata,
        graphqlQuery: current.graphqlQuery,
        graphqlVariables: current.graphqlVariables,
        graphqlSchemaPath: current.graphqlSchemaPath,
        graphqlOperation: current.graphqlOperation,
      })
      if (result.error) {
        console.error('Failed to rename request:', result.error.message)
        useToast().error(`Failed to rename request: ${formatResultError(result.error)}`)
        return false
      }
      const latest = requestsMap.value.get(id)
      if (latest && latest !== current) {
        // User typed during the rename: keep local edits, adopt only name and version
        requestsMap.value.set(id, {
          ...latest,
          name: result.data.name,
          version: result.data.version,
          updatedAt: result.data.updatedAt,
        })
      } else {
        requestsMap.value.set(result.data.id, result.data)
      }
      savedSnapshots.value.set(result.data.id, { ...result.data })
      requestsMap.value = new Map(requestsMap.value)
      savedSnapshots.value = new Map(savedSnapshots.value)
      const tabId = `request:${id}`
      const tab = openTabs.value.find(t => t.id === tabId)
      if (tab && tab.type === 'request') {
        tab.name = newName
        openTabs.value = [...openTabs.value]
      }
      return true
    } catch (err) {
      console.error('Failed to rename request:', err)
      useToast().error('Failed to rename request')
      return false
    }
  }

  async function remove(id: string, version: number): Promise<boolean> {
    // A timer that fires mid-delete bumps the version and the delete loses the race.
    const hadAutosave = autosaveTimers.has(id)
    cancelAutosave(id)
    await savesInFlight.get(id)?.catch(() => false)
    const currentVersion = requestsMap.value.get(id)?.version ?? version
    const data = await runMutation('Failed to delete request', () =>
      getRequestService().then(s => s.delete({ id, version: currentVersion }))
    )
    if (!data) {
      if (hadAutosave) scheduleAutosave(id)
      return false
    }
    // Close tab before removing data (closeTab calls saveToBackend, but the request is already deleted)
    const tabId = `request:${id}`
    if (openTabs.value.some(t => t.id === tabId)) {
      const idx = openTabs.value.findIndex(t => t.id === tabId)
      openTabs.value = openTabs.value.filter(t => t.id !== tabId)
      if (activeTabId.value === tabId) {
        if (openTabs.value.length === 0) {
          activeTabId.value = null
        } else {
          const newIdx = Math.min(idx, openTabs.value.length - 1)
          activeTabId.value = openTabs.value[newIdx].id
        }
      }
    }
    requestsMap.value.delete(id)
    savedSnapshots.value.delete(id)
    useResponseStore().deleteResponse(id)
    clearDrafts(id)
    await forgetTokenStatus([{ kind: 'request', id }])
    requestsMap.value = new Map(requestsMap.value)
    savedSnapshots.value = new Map(savedSnapshots.value)
    return true
  }

  async function openTab(requestId: string) {
    const existing = openTabs.value.find(t => t.id === `request:${requestId}`)
    if (existing) {
      activeTabId.value = existing.id
      return
    }

    // Always re-fetch from backend to get fresh data (handles detach/reattach scenarios)
    try {
      const service = await getRequestService()
      const result = await service.getById(requestId)
      if (!result.error) {
        loadRequest(result.data)
      }
    } catch (err) {
      console.error('Failed to fetch request for tab:', err)
    }

    const req = requestsMap.value.get(requestId)
    openTabs.value.push({
      id: `request:${requestId}`,
      type: 'request',
      requestId,
      name: req?.name ?? 'Untitled',
      method: req?.method ?? 'GET',
      protocol: req?.protocol ?? 'http',
    })
    activeTabId.value = `request:${requestId}`
  }

  // The one path out of a tab: a failed save keeps it open, and no socket nor
  // browser flow may outlive the tab that owns it. Returns false when the tab
  // must stay.
  async function releaseTab(tab: Tab): Promise<boolean> {
    if (!(await saveTabBeforeClose(tab))) return false
    // WebSocket tabs are ephemeral: the connection and the log go with the tab.
    // Dynamic import avoids a static store cycle at module load.
    if (tab.type === 'request' && tab.protocol === 'websocket') {
      const { useWebSocketStore } = await import('./websocket')
      await useWebSocketStore().teardown(tab.requestId)
    }
    await forgetTokenStatus([tab.type === 'request'
      ? { kind: 'request', id: tab.requestId }
      : { kind: 'collection', id: tab.collectionId }])
    return true
  }

  async function closeTab(tabId: string) {
    const tab = openTabs.value.find(t => t.id === tabId)
    if (!tab) return
    if (!(await releaseTab(tab))) return
    const idx = openTabs.value.findIndex(t => t.id === tabId)
    if (idx === -1) return
    openTabs.value.splice(idx, 1)
    if (activeTabId.value === tabId) {
      const next = openTabs.value[Math.min(idx, openTabs.value.length - 1)]
      activeTabId.value = next?.id ?? null
    }
  }

  async function saveTabBeforeClose(tab: Tab): Promise<boolean> {
    if (tab.type === 'request') {
      cancelAutosave(tab.requestId)
      const req = requestsMap.value.get(tab.requestId)
      // Drafts (created via History → Replay) are hard-deleted on close to avoid
      // accumulating soft-deleted rows. Skip the saveToBackend path entirely.
      if (req?.isDraft) {
        try {
          const service = await getRequestService()
          await service.deleteDraft(tab.requestId)
        } catch (err) {
          console.error('Failed to delete draft request:', err)
        }
        requestsMap.value.delete(tab.requestId)
        savedSnapshots.value.delete(tab.requestId)
        useResponseStore().deleteResponse(tab.requestId)
        clearDrafts(tab.requestId)
        requestsMap.value = new Map(requestsMap.value)
        savedSnapshots.value = new Map(savedSnapshots.value)
        return true
      }
      return flush(tab.requestId)
    }
    if (tab.type === 'collection') {
      const editorRef = collectionEditorRefs.value.get(tab.collectionId)
      if (editorRef?.scriptsDirty) {
        try {
          if (!(await editorRef.saveScripts())) return false
        } catch (err) {
          console.error('Failed to save collection scripts:', err)
          useToast().error('Failed to save collection scripts')
          return false
        }
      }
      return true
    }
    return true
  }

  async function closeAllTabs() {
    const kept: Tab[] = []
    for (const tab of [...openTabs.value]) {
      if (!(await releaseTab(tab))) kept.push(tab)
    }
    openTabs.value = kept
    activeTabId.value = kept[0]?.id ?? null
  }

  async function closeOtherTabs(tabId: string) {
    const keep = openTabs.value.find(t => t.id === tabId)
    if (!keep) return
    const kept: Tab[] = [keep]
    for (const tab of openTabs.value.filter(t => t.id !== tabId)) {
      if (!(await releaseTab(tab))) kept.push(tab)
    }
    openTabs.value = kept
    activeTabId.value = tabId
  }

  function syncTabMeta(requestId: string) {
    const tab = openTabs.value.find(t => t.id === `request:${requestId}`)
    if (tab && tab.type === 'request') {
      const req = requestsMap.value.get(requestId)
      if (req) {
        tab.name = req.name
        tab.method = req.method
        tab.protocol = req.protocol
      }
    }
  }

  async function executeRequest(id: string) {
    const req = requestsMap.value.get(id)
    if (!req) return

    const responses = useResponseStore()

    if (responses.getResponseState(id).status === 'loading') return

    // Don't execute a stale version after a failed save
    if (!(await flushForHandoff(id))) return

    responses.setResponse(id, { status: 'loading', startedAt: Date.now() })

    try {
      const service = await getRequestService()
      const wsId = useWorkspaceStore().activeWorkspace?.id
      if (!wsId) return
      const result = await service.execute({ requestId: id, workspaceId: wsId })

      // Check if request still exists (may have been deleted during execution)
      if (!requestsMap.value.has(id)) return

      if (result.error) {
        responses.setResponse(id, {
          status: 'error',
          error: {
            title: result.error.code === 'request_error' ? 'Request Failed' : 'Error',
            detail: result.error.message,
            suggestions: getSuggestions(result.error.message),
          },
        })
      } else {
        responses.setResponse(id, { status: 'success', data: result.data })
      }
    } catch (err) {
      responses.setResponse(id, {
        status: 'error',
        error: {
          title: 'Unexpected Error',
          detail: String(err),
          suggestions: [],
        },
      })
    }
  }

  function getSuggestions(message: string): string[] {
    const lower = message.toLowerCase()
    if (lower.includes('connection refused')) {
      return ['Check that the server is running', 'Verify the host and port']
    }
    if (lower.includes('dns')) {
      return ['Check the URL for typos', 'Verify network connection']
    }
    if (lower.includes('timeout') || lower.includes('timed out')) {
      return ['The server took too long to respond', 'Try increasing the timeout or check server health']
    }
    return []
  }

  async function move(id: string, targetCollectionId: string, version: number): Promise<boolean> {
    const data = await runMutation('Failed to move request', () =>
      getRequestService().then(s => s.move({ id, targetCollectionId, version }))
    )
    if (!data) return false
    requestsMap.value.set(data.id, data)
    savedSnapshots.value.set(data.id, { ...data })
    requestsMap.value = new Map(requestsMap.value)
    savedSnapshots.value = new Map(savedSnapshots.value)
    return true
  }

  function openCollectionTab(
    collectionId: string,
    name: string,
    options?: { initialSection?: 'overview' | 'authorization' | 'scripts' }
  ) {
    const tabId = `collection:${collectionId}`
    const existing = openTabs.value.find(t => t.id === tabId)
    if (existing) {
      activeTabId.value = tabId
      if (options?.initialSection) {
        collectionInitialSections.value.set(collectionId, options.initialSection)
      }
      return
    }
    if (options?.initialSection) {
      collectionInitialSections.value.set(collectionId, options.initialSection)
    }
    openTabs.value.push({ id: tabId, type: 'collection', collectionId, name })
    activeTabId.value = tabId
  }

  function syncCollectionTabName(collectionId: string, newName: string) {
    const tab = openTabs.value.find(
      t => t.type === 'collection' && t.collectionId === collectionId,
    )
    if (tab && tab.type === 'collection') {
      tab.name = newName
    }
  }

  // Dead code today — releaseTab is the live close path — but a tab that leaves
  // the strip must not leave a running flow behind whichever way it goes.
  function closeCollectionTab(collectionId: string) {
    const tabId = `collection:${collectionId}`
    const idx = openTabs.value.findIndex(t => t.id === tabId)
    if (idx === -1) return
    void forgetTokenStatus([{ kind: 'collection', id: collectionId }])
    openTabs.value.splice(idx, 1)
    if (activeTabId.value === tabId) {
      const next = openTabs.value[Math.min(idx, openTabs.value.length - 1)]
      activeTabId.value = next?.id ?? null
    }
  }

  // Token status is per owner, so a deleted owner's entry is dead weight.
  async function forgetTokenStatus(owners: { kind: string; id: string }[]) {
    const { useAuthTokenStore } = await import('./auth-tokens')
    const tokens = useAuthTokenStore()
    for (const owner of owners) tokens.forget(owner.kind, owner.id)
  }

  // Close tabs and drop cached data for backend-deleted collections. Never saves.
  async function purgeCollectionSubtree(collectionIds: string[]) {
    const idSet = new Set(collectionIds)
    const doomed = Array.from(requestsMap.value.values()).filter(r => idSet.has(r.collectionId))
    // Rows are already gone on the backend: an autosave would only raise a toast.
    for (const req of doomed) cancelAutosave(req.id)
    for (const req of doomed) {
      if (req.protocol === 'websocket') {
        const { useWebSocketStore } = await import('./websocket')
        await useWebSocketStore().teardown(req.id)
      }
      requestsMap.value.delete(req.id)
      savedSnapshots.value.delete(req.id)
      useResponseStore().deleteResponse(req.id)
      clearDrafts(req.id)
    }
    const doomedRequestIds = new Set(doomed.map(r => r.id))
    await forgetTokenStatus([
      ...collectionIds.map(id => ({ kind: 'collection', id })),
      ...doomed.map(r => ({ kind: 'request', id: r.id })),
    ])
    openTabs.value = openTabs.value.filter(t => {
      if (t.type === 'request') return !doomedRequestIds.has(t.requestId)
      return !idSet.has(t.collectionId)
    })
    if (activeTabId.value && !openTabs.value.some(t => t.id === activeTabId.value)) {
      activeTabId.value = openTabs.value[0]?.id ?? null
    }
    requestsMap.value = new Map(requestsMap.value)
    savedSnapshots.value = new Map(savedSnapshots.value)
  }

  function consumeInitialSection(collectionId: string): string | undefined {
    const section = collectionInitialSections.value.get(collectionId)
    if (section) collectionInitialSections.value.delete(collectionId)
    return section
  }

  return {
    requestsMap,
    loading,
    openTabs,
    activeTabId,
    activeTab,
    byCollection,
    isDirty,
    getById,
    fetchByCollection,
    create,
    loadRequest,
    updateLocal,
    isRequestDirty,
    isSaveBlocked,
    saveToBackend,
    flush,
    flushForHandoff,
    flushAllDirty,
    cancelAutosave,
    remove,
    rename,
    move,
    openTab,
    openCollectionTab,
    consumeInitialSection,
    closeTab,
    closeAllTabs,
    closeOtherTabs,
    syncTabMeta,
    executeRequest,
    registerCollectionEditor,
    unregisterCollectionEditor,
    syncCollectionTabName,
    closeCollectionTab,
    purgeCollectionSubtree,
    forgetTokenStatus,
  }
})
