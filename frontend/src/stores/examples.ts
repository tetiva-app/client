import { ref } from 'vue'
import { defineStore } from 'pinia'
import type { CreateExampleInput, Example, ExampleInput, ExampleProtocol } from '@/types/example'
import type { HeaderItem } from '@/types/request'
import { getExampleService } from '@/services'
import { emitWailsEvent } from '@/composables/useWindowEvents'
import { runMutation } from '@/stores/runMutation'
import { exampleBodyTooLarge } from '@/lib/example-limits'
import { contentSaved } from '@/lib/content-saved'

export interface ExampleDraft {
  requestId: string
  protocol: ExampleProtocol
  baseVersion: number
  value: ExampleInput
  dirty: boolean
  remote: 'updated' | 'deleted' | null
  // Created by "New example" and never saved: it exists only in this window.
  isNew: boolean
}

export interface ExampleListItem {
  id: string
  name: string
  statusCode: number
  dirty: boolean
  state: 'saved' | 'new' | 'deleted'
}

function copyHeaders(headers: HeaderItem[]): HeaderItem[] {
  return headers.map(h => ({ key: h.key, value: h.value, enabled: h.enabled }))
}

function toInput(e: ExampleInput): ExampleInput {
  return {
    name: e.name,
    statusCode: e.statusCode,
    statusText: e.statusText,
    headers: copyHeaders(e.headers),
    body: e.body,
    contentType: e.contentType,
  }
}

function sameInput(a: ExampleInput, b: ExampleInput): boolean {
  return a.name === b.name
    && a.statusCode === b.statusCode
    && a.statusText === b.statusText
    && a.body === b.body
    && a.contentType === b.contentType
    && JSON.stringify(copyHeaders(a.headers)) === JSON.stringify(copyHeaders(b.headers))
}

// The backend would refuse these, so a save on leave would only repeat its error toast.
function refusedOnLeave(v: ExampleInput): boolean {
  return v.name.trim() === '' || exampleBodyTooLarge(v.body)
}

function cleanDraft(e: Example): ExampleDraft {
  return {
    requestId: e.requestId,
    protocol: e.protocol,
    baseVersion: e.version,
    value: toInput(e),
    dirty: false,
    remote: null,
    isNew: false,
  }
}

export const useExamplesStore = defineStore('examples', () => {
  const byRequest = ref<Record<string, Example[]>>({})
  const drafts = ref<Record<string, ExampleDraft>>({})
  // A new or orphaned draft is saved under a fresh id; the editor follows it here.
  const savedAs = ref<Record<string, string>>({})
  // A list that answers after a newer one was applied must not win.
  const listSeq: Record<string, number> = {}
  // A second save of the same draft would create it twice or lose the version race.
  const savesInFlight = new Map<string, Promise<string | null>>()

  function nextSeq(requestId: string): number {
    listSeq[requestId] = (listSeq[requestId] ?? 0) + 1
    return listSeq[requestId]
  }

  function locate(id: string): { requestId: string; example: Example } | null {
    for (const [requestId, list] of Object.entries(byRequest.value)) {
      const example = list.find(e => e.id === id)
      if (example) return { requestId, example }
    }
    return null
  }

  function applyServerList(requestId: string, list: Example[]) {
    nextSeq(requestId)
    byRequest.value[requestId] = list
    for (const [id, draft] of Object.entries(drafts.value)) {
      if (draft.requestId !== requestId || draft.isNew) continue
      const example = list.find(e => e.id === id)
      if (!example) {
        if (draft.dirty) draft.remote = 'deleted'
        else delete drafts.value[id]
      } else if (!draft.dirty) {
        drafts.value[id] = cleanDraft(example)
      } else if (example.version !== draft.baseVersion) {
        draft.remote = 'updated'
      } else if (draft.remote === 'deleted') {
        draft.remote = null
      }
    }
  }

  function listFor(requestId: string): ExampleListItem[] {
    const saved: ExampleListItem[] = (byRequest.value[requestId] ?? []).map(e => ({
      id: e.id,
      name: e.name,
      statusCode: e.statusCode,
      dirty: drafts.value[e.id]?.dirty ?? false,
      state: 'saved',
    }))
    const listed = new Set(saved.map(e => e.id))
    const unsaved: ExampleListItem[] = Object.entries(drafts.value)
      .filter(([id, d]) => d.requestId === requestId && !listed.has(id) && (d.isNew || d.remote === 'deleted'))
      .map(([id, d]) => ({
        id,
        name: d.value.name,
        statusCode: d.value.statusCode,
        dirty: d.dirty,
        state: d.isNew ? 'new' : 'deleted',
      }))
    return [...saved, ...unsaved]
  }

  function hasUnsaved(requestId: string): boolean {
    return Object.values(drafts.value).some(d => d.requestId === requestId && d.dirty)
  }

  async function fetch(requestId: string): Promise<void> {
    const seq = nextSeq(requestId)
    try {
      const result = await (await getExampleService()).list(requestId)
      if (listSeq[requestId] !== seq) return
      if (result.error) {
        console.error('Failed to load examples:', result.error.message)
        return
      }
      applyServerList(requestId, result.data ?? [])
    } catch (err) {
      console.error('Failed to load examples:', err)
    }
  }

  async function refreshLoaded(): Promise<void> {
    await Promise.all(Object.keys(byRequest.value).map(fetch))
  }

  async function refreshIfLoaded(requestId: string): Promise<void> {
    if (requestId in byRequest.value) await fetch(requestId)
  }

  function openDraft(id: string) {
    if (drafts.value[id]) return
    const found = locate(id)
    if (!found) return
    drafts.value[id] = cleanDraft(found.example)
  }

  function newDraft(requestId: string, protocol: ExampleProtocol, value: ExampleInput): string {
    const id = crypto.randomUUID()
    drafts.value[id] = { requestId, protocol, baseVersion: 0, value: toInput(value), dirty: false, remote: null, isNew: true }
    return id
  }

  function updateDraft(id: string, patch: Partial<ExampleInput>) {
    const draft = drafts.value[id]
    if (!draft) return
    const next = toInput({ ...draft.value, ...patch })
    if (sameInput(next, draft.value)) return
    draft.value = next
    draft.dirty = true
  }

  function discardDraft(id: string) {
    delete drafts.value[id]
  }

  // The request is gone, so a save on leave could only fail.
  function dropDrafts(requestId: string) {
    for (const [id, d] of Object.entries(drafts.value)) {
      if (d.requestId === requestId) delete drafts.value[id]
    }
  }

  function keepMine(id: string) {
    const draft = drafts.value[id]
    const found = locate(id)
    if (!draft || !found) return
    draft.baseVersion = found.example.version
    draft.remote = null
  }

  function addToList(requestId: string, example: Example): Promise<void> | void {
    const loaded = byRequest.value[requestId]
    if (!loaded) return fetch(requestId)
    applyServerList(requestId, [...loaded.filter(e => e.id !== example.id), example])
  }

  // Resolves to the id the draft is now saved under, or null when nothing was saved.
  function saveDraft(id: string): Promise<string | null> {
    const inFlight = savesInFlight.get(id)
    if (inFlight) return inFlight
    const draft = drafts.value[id]
    if (!draft) return Promise.resolve(null)
    const promise = (draft.isNew || draft.remote === 'deleted' ? saveAsNew(id, draft) : saveEdit(id, draft))
      .finally(() => savesInFlight.delete(id))
    savesInFlight.set(id, promise)
    return promise
  }

  async function saveAsNew(id: string, draft: ExampleDraft): Promise<string | null> {
    const sent = draft.value
    const created = await runMutation('Failed to save example', () =>
      getExampleService().then(s => s.create({ ...toInput(sent), requestId: draft.requestId, protocol: draft.protocol })),
    )
    if (!created) return null

    // No await until the example is listed: a watcher running in between would see the draft gone.
    const current = drafts.value[id]
    delete drafts.value[id]
    if (current && current.value !== sent) {
      drafts.value[created.id] = { ...cleanDraft(created), value: current.value, dirty: true }
    }
    savedAs.value[id] = created.id
    const listing = addToList(draft.requestId, created)
    void emitWailsEvent('examples:changed', { requestId: draft.requestId })
    contentSaved()
    if (listing) await listing
    return created.id
  }

  async function saveEdit(id: string, draft: ExampleDraft): Promise<string | null> {
    const { requestId } = draft
    const sent = draft.value
    const rejection = { code: '' }
    const saved = await runMutation('Failed to save example', async () => {
      const result = await (await getExampleService()).edit({ id, ...toInput(sent), version: draft.baseVersion })
      // The draft banner explains these; a toast on top would be noise.
      if (result.error?.code === 'conflict' || result.error?.code === 'not_found') {
        rejection.code = result.error.code
        return { data: null }
      }
      return result
    })
    if (rejection.code) {
      const current = drafts.value[id]
      if (current) current.remote = rejection.code === 'conflict' ? 'updated' : 'deleted'
      await fetch(requestId)
      return null
    }
    if (!saved) return null

    const current = drafts.value[id]
    if (current && current.value === sent) {
      drafts.value[id] = cleanDraft(saved)
    } else if (current) {
      current.baseVersion = saved.version
      current.remote = null
    }
    applyServerList(requestId, (byRequest.value[requestId] ?? []).map(e => (e.id === id ? saved : e)))
    void emitWailsEvent('examples:changed', { requestId })
    contentSaved()
    return id
  }

  // Leaving a request saves its edited drafts the way it saves the request; a draft that
  // conflicts, lost its example or would be refused waits for the user. False when a save failed.
  async function flushDrafts(requestId?: string): Promise<boolean> {
    const ids = Object.entries(drafts.value)
      .filter(([, d]) => (requestId === undefined || d.requestId === requestId)
        && d.dirty && d.remote === null && !refusedOnLeave(d.value))
      .map(([id]) => id)
    const saved = await Promise.all(ids.map(saveDraft))
    return saved.every(id => id !== null)
  }

  // Rejects after runMutation has shown the error, so callers only toast success.
  async function create(input: CreateExampleInput): Promise<Example> {
    const created = await runMutation('Failed to create example', () =>
      getExampleService().then(s => s.create({
        ...toInput(input),
        requestId: input.requestId,
        protocol: input.protocol,
      })),
    )
    if (!created) throw new Error('Failed to create example')

    await addToList(input.requestId, created)
    void emitWailsEvent('examples:changed', { requestId: input.requestId })
    contentSaved()
    return created
  }

  async function remove(id: string): Promise<void> {
    if (drafts.value[id]?.isNew) {
      discardDraft(id)
      return
    }
    const found = locate(id)
    if (!found) return
    const { requestId, example } = found
    const ok = await runMutation('Failed to delete example', () =>
      getExampleService().then(s => s.delete({ id, version: example.version })),
    )
    if (ok === null) {
      await fetch(requestId)
      return
    }
    discardDraft(id)
    applyServerList(requestId, (byRequest.value[requestId] ?? []).filter(e => e.id !== id))
    void emitWailsEvent('examples:changed', { requestId })
    contentSaved()
  }

  return {
    byRequest,
    drafts,
    savedAs,
    fetch,
    applyServerList,
    listFor,
    hasUnsaved,
    refreshLoaded,
    refreshIfLoaded,
    openDraft,
    newDraft,
    updateDraft,
    discardDraft,
    dropDrafts,
    keepMine,
    saveDraft,
    flushDrafts,
    create,
    remove,
  }
})

// Both windows subscribe with these, so an edit or a sync in one reaches the other.
export function exampleWindowEvents(store: ReturnType<typeof useExamplesStore>) {
  return {
    onExamplesChanged: (requestId: string) => { void store.refreshIfLoaded(requestId) },
    onSyncChanged: () => { void store.refreshLoaded() },
  }
}
