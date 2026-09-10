import { ref } from 'vue'
import { defineStore } from 'pinia'
import { getAuthService } from '@/services'
import { openExternal } from '@/lib/open-external'
import type { Result } from '@/types/common'
import type {
  AuthConfigReq,
  AuthServiceAPI,
  FlowInfo,
  FlowState,
  FlowStatus,
  StartFlowReq,
  TokenState,
} from '@/services/auth-api'

export interface TokenEntry {
  state: TokenState
  expiresAt: string
  loading: boolean
  error: string
  // Field-keyed messages of the last validation failure; the generic message is
  // the constant "validation failed" and says nothing on its own.
  fields: Record<string, string>
  // Something the user should read that is not a failure — a flow the editor
  // cancelled for them, say. Survives a passive refresh; a user action clears it.
  notice: string
  flowState: FlowState
  flowId: string
  authorizeUrl: string
  userCode: string
  verificationUri: string
  verificationUriComplete: string
  flowExpiresAt: string
}

// A running flow survives a reload as a bare UUID; nothing secret is stored.
const FLOW_STORAGE_KEY = 'tetiva.authFlows'

export function ownerKey(ownerKind: string, ownerId: string): string {
  return `${ownerKind}:${ownerId}`
}

export function emptyTokenEntry(): TokenEntry {
  return {
    state: 'none',
    expiresAt: '',
    loading: false,
    error: '',
    fields: {},
    notice: '',
    flowState: 'idle',
    flowId: '',
    authorizeUrl: '',
    userCode: '',
    verificationUri: '',
    verificationUriComplete: '',
    flowExpiresAt: '',
  }
}

function clockTime(date: Date): string {
  return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

// "Valid until 14:05" for today, with the day added when the token outlives it.
export function tokenStatusLabel(entry: TokenEntry, now: Date = new Date()): string {
  switch (entry.state) {
    case 'valid': {
      const until = new Date(entry.expiresAt)
      if (!entry.expiresAt || Number.isNaN(until.getTime())) return 'Valid'
      const sameDay = until.toDateString() === now.toDateString()
      const day = until.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
      return sameDay ? `Valid until ${clockTime(until)}` : `Valid until ${day}, ${clockTime(until)}`
    }
    case 'expired-refreshable':
      return 'Expired, will refresh'
    case 'expired':
      return 'Expired'
    default:
      return 'No token'
  }
}

function messageOf(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function webStorage(): Storage | null {
  try {
    return typeof sessionStorage === 'undefined' ? null : sessionStorage
  } catch {
    return null
  }
}

function storedFlows(): Record<string, string> {
  const store = webStorage()
  if (!store) return {}
  try {
    const parsed = JSON.parse(store.getItem(FLOW_STORAGE_KEY) ?? '{}')
    return parsed && typeof parsed === 'object' ? (parsed as Record<string, string>) : {}
  } catch {
    return {}
  }
}

function writeStoredFlows(all: Record<string, string>) {
  const store = webStorage()
  if (!store) return
  try {
    store.setItem(FLOW_STORAGE_KEY, JSON.stringify(all))
  } catch {
    // Private-mode quota; a reload then simply cannot re-attach.
  }
}

// Removes the stored id whether or not a record still points at it: a reload
// leaves the id behind with no record, and it is the only handle on that flow.
function dropStoredFlow(key: string): string {
  const all = storedFlows()
  const flowId = all[key] ?? ''
  if (!flowId) return ''
  delete all[key]
  writeStoredFlows(all)

  return flowId
}

function flowInfoFields(info: FlowInfo, flowId: string): Partial<TokenEntry> {
  return {
    flowId,
    authorizeUrl: info.authorizeUrl,
    userCode: info.userCode,
    verificationUri: info.verificationUri,
    verificationUriComplete: info.verificationUriComplete,
    flowExpiresAt: info.expiresAt,
  }
}

function clearedFlowFields(): Partial<TokenEntry> {
  return {
    flowId: '',
    authorizeUrl: '',
    userCode: '',
    verificationUri: '',
    verificationUriComplete: '',
    flowExpiresAt: '',
  }
}

// How long an adoption waits before re-reading a flow that is still starting.
const STARTING_RETRY_MS = 250
const STARTING_RETRIES = 8

// One per owner while a browser flow runs. `op` is the `begin` number the flow
// holds, so a signal from a replaced flow is recognised and dropped.
interface FlowRecord {
  flowId: string
  op: number
  off: () => void
}

export const useAuthTokenStore = defineStore('authTokens', () => {
  const entries = ref<Record<string, TokenEntry>>({})
  // Per key: newest call number and in-flight user actions (Get token / Clear).
  // Only the newest call commits; a local status read yields to a user action.
  const calls: Record<string, number> = {}
  const acting: Record<string, number> = {}
  const flows: Record<string, FlowRecord | undefined> = {}

  function entry(ownerKind: string, ownerId: string): TokenEntry {
    return entries.value[ownerKey(ownerKind, ownerId)] ?? emptyTokenEntry()
  }

  function patch(key: string, changes: Partial<TokenEntry>) {
    entries.value = {
      ...entries.value,
      [key]: { ...(entries.value[key] ?? emptyTokenEntry()), ...changes },
    }
  }

  // A quiet call keeps the buttons enabled: disabling one drops the keyboard
  // focus to <body> in Chromium, and a confirming read must not cost that.
  function begin(key: string, userAction: boolean, quiet = false): number {
    if (userAction) acting[key] = (acting[key] ?? 0) + 1
    const call = (calls[key] ?? 0) + 1
    calls[key] = call
    const changes: Partial<TokenEntry> = { error: '' }
    if (!quiet) changes.loading = true
    // A background status read must not wipe a notice nobody has read yet.
    if (userAction) changes.notice = ''
    patch(key, changes)
    return call
  }

  function commit(key: string, call: number, userAction: boolean, changes: Partial<TokenEntry>) {
    if (userAction && (acting[key] ?? 0) > 0) acting[key] -= 1
    if (calls[key] !== call) return
    patch(key, { loading: false, ...changes })
  }

  // Synchronous and at-most-once: it releases the entry before any caller can
  // await, so a signal already queued on the event bus cannot commit after it.
  function retireFlow(key: string): FlowRecord | undefined {
    const record = flows[key]
    if (!record) return undefined
    delete flows[key]
    const all = storedFlows()
    if (all[key] === record.flowId) {
      delete all[key]
      writeStoredFlows(all)
    }
    try {
      record.off()
    } catch {
      // An unsubscribe that throws must not strand the entry as loading.
    }
    if ((acting[key] ?? 0) > 0) acting[key] -= 1
    calls[key] = (calls[key] ?? 0) + 1
    patch(key, { loading: false, ...clearedFlowFields() })
    return record
  }

  function ownsFlow(key: string, flowId: string, op: number): boolean {
    const record = flows[key]
    return !!record && record.flowId === flowId && record.op === op
  }

  async function requestCancel(flowId: string) {
    try {
      await (await getAuthService()).cancelFlow(flowId)
    } catch {
      // The flow expires on its own; nothing here is worth surfacing.
    }
  }

  // The single landing point for every flow signal, from the event bus or from
  // a status read. Info never travels on an event, so it is not applied here.
  function flowSignal(req: AuthConfigReq, flowId: string, op: number, status: FlowStatus) {
    const key = ownerKey(req.ownerKind, req.ownerId)
    if (!ownsFlow(key, flowId, op)) return
    if (status.state === 'pending' || status.state === 'starting') {
      patch(key, { flowState: 'pending' })
      return
    }
    retireFlow(key)
    switch (status.state) {
      case 'done':
        patch(key, { flowState: 'done', error: '', fields: {} })
        void refresh(req)
        return
      case 'error':
      case 'cancelled':
        patch(key, { flowState: status.state, error: status.error })
        return
      default:
        // '' — the manager has never heard of the id, or forgot it again.
        patch(key, { flowState: 'idle', error: '' })
    }
  }

  function forget(ownerKind: string, ownerId: string) {
    const key = ownerKey(ownerKind, ownerId)
    const record = retireFlow(key)
    const flowId = record ? record.flowId : dropStoredFlow(key)
    if (flowId) void requestCancel(flowId)
    // Bumped so a call still in flight cannot bring the entry back.
    calls[key] = (calls[key] ?? 0) + 1
    delete acting[key]
    if (!(key in entries.value)) return
    const next = { ...entries.value }
    delete next[key]
    entries.value = next
  }

  // Status is passive: a failure keeps the last known state and records the message.
  // A throwing binding call must still release the entry, or `acting` pins refresh.
  async function run<T>(
    key: string,
    userAction: boolean,
    call: number,
    op: (service: AuthServiceAPI) => Promise<Result<T>>,
    onOk: (data: T) => Partial<TokenEntry>,
  ) {
    let result: Result<T>
    try {
      result = await op(await getAuthService())
    } catch (e) {
      commit(key, call, userAction, { error: messageOf(e), fields: {} })
      return
    }
    if (result.error) {
      commit(key, call, userAction, { error: result.error.message, fields: result.error.fields ?? {} })
      return
    }
    commit(key, call, userAction, onOk(result.data))
  }

  async function refresh(req: AuthConfigReq, quiet = false) {
    const key = ownerKey(req.ownerKind, req.ownerId)
    if (acting[key]) return
    await run(key, false, begin(key, false, quiet), (s) => s.tokenStatus(req), (d) => ({ state: d.state, expiresAt: d.expiresAt, error: '', fields: {} }))
  }

  async function fetchToken(req: AuthConfigReq) {
    const key = ownerKey(req.ownerKind, req.ownerId)
    await run(key, true, begin(key, true), (s) => s.fetchToken(req), (d) => ({ state: d.state, expiresAt: d.expiresAt, error: '', fields: {} }))
  }

  async function clearToken(req: AuthConfigReq) {
    const key = ownerKey(req.ownerKind, req.ownerId)
    await run(key, true, begin(key, true), (s) => s.clearToken(req), () => ({ state: 'none', expiresAt: '', error: '', fields: {} }))
  }

  function failFlow(key: string, flowId: string, op: number, error: string, fields: Record<string, string> = {}) {
    if (!ownsFlow(key, flowId, op)) return
    retireFlow(key)
    patch(key, { flowState: 'error', error, fields })
  }

  // Reads the status of a flow the record still owns. Returns null when the read
  // failed or when a signal settled the flow while it was in flight.
  async function readFlowStatus(
    service: AuthServiceAPI, key: string, flowId: string, op: number,
  ): Promise<FlowStatus | null> {
    let result: Result<FlowStatus>
    try {
      result = await service.flowStatus(flowId)
    } catch (e) {
      failFlow(key, flowId, op, messageOf(e))
      return null
    }
    if (!ownsFlow(key, flowId, op)) return null
    if (result.error) {
      failFlow(key, flowId, op, result.error.message, result.error.fields ?? {})
      return null
    }
    return result.data
  }

  // Subscribes an already-installed record. Returns null when the listener could
  // not be attached, or when a signal settled the flow while it was attaching.
  async function attachFlow(req: AuthConfigReq, key: string, flowId: string, op: number): Promise<AuthServiceAPI | null> {
    try {
      const service = await getAuthService()
      const off = await service.subscribeFlow(flowId, (s) => flowSignal(req, flowId, op, s))
      if (!ownsFlow(key, flowId, op)) {
        off()
        return null
      }
      flows[key]!.off = off
      return service
    } catch (e) {
      failFlow(key, flowId, op, messageOf(e))
      return null
    }
  }

  // The record is registered before anything is awaited, so no window exists
  // where `begin` is held but nothing can release it.
  async function startFlow(req: AuthConfigReq, grant: 'authorization_code' | 'device_code') {
    const key = ownerKey(req.ownerKind, req.ownerId)
    const previous = retireFlow(key)
    if (previous) void requestCancel(previous.flowId)
    const flowId = crypto.randomUUID()
    const op = begin(key, true)
    flows[key] = { flowId, op, off: () => {} }
    writeStoredFlows({ ...storedFlows(), [key]: flowId })
    patch(key, { flowState: 'idle', fields: {}, ...clearedFlowFields(), flowId })

    const service = await attachFlow(req, key, flowId, op)
    if (!service) return

    const startReq: StartFlowReq = { ...req, flowId }
    let result: Result<FlowInfo>
    try {
      result = grant === 'authorization_code'
        ? await service.startAuthCodeFlow(startReq)
        : await service.startDeviceFlow(startReq)
    } catch (e) {
      failFlow(key, flowId, op, messageOf(e))
      return
    }
    if (!ownsFlow(key, flowId, op)) {
      // The cancel that retired the record may have reached Go before the id was
      // registered, where it counts as unknown; this answer proves it is live.
      if (!result.error) void requestCancel(flowId)
      return
    }
    if (result.error) {
      failFlow(key, flowId, op, result.error.message, result.error.fields ?? {})
      return
    }
    patch(key, { flowState: 'pending', ...flowInfoFields(result.data, flowId) })
    if (grant !== 'authorization_code') return

    try {
      await openExternal(result.data.authorizeUrl)
    } catch (e) {
      if (!ownsFlow(key, flowId, op)) return
      const record = retireFlow(key)
      if (record) void requestCancel(record.flowId)
      patch(key, { flowState: 'error', error: messageOf(e) })
    }
  }

  // A cancel the user did not ask for carries a notice saying why.
  async function cancelFlow(req: AuthConfigReq, notice = '') {
    const key = ownerKey(req.ownerKind, req.ownerId)
    const record = retireFlow(key)
    if (!record) return
    patch(key, { flowState: 'cancelled', error: '', notice })
    await requestCancel(record.flowId)
    // Cancel cannot interrupt a flow that is already committing its token, so
    // ask once whether one landed in that window.
    await refresh(req, true)
  }

  // A reload destroys Pinia while the Go manager keeps listening. Same ordering
  // as startFlow: guarded placeholder, then subscribe, then ask for the status —
  // a flow that ends in between is then recovered by the event, not lost.
  async function adoptFlow(req: AuthConfigReq) {
    const key = ownerKey(req.ownerKind, req.ownerId)
    if (flows[key]) return
    const flowId = storedFlows()[key]
    if (!flowId) return
    const op = begin(key, true)
    flows[key] = { flowId, op, off: () => {} }

    const service = await attachFlow(req, key, flowId, op)
    if (!service) return

    let status = await readFlowStatus(service, key, flowId, op)
    if (!status) return
    // A flow whose start has not finished carries no authorize URL or user code
    // yet; a slow IdP or a takeOver join can hold it there for a few reads.
    for (let attempt = 0; status.state === 'starting' && attempt < STARTING_RETRIES; attempt++) {
      await new Promise(resolve => setTimeout(resolve, STARTING_RETRY_MS))
      if (!ownsFlow(key, flowId, op)) return
      status = await readFlowStatus(service, key, flowId, op)
      if (!status) return
    }
    if (status.state === 'pending' || status.state === 'starting') {
      patch(key, { flowState: 'pending', ...flowInfoFields(status.info, flowId) })
      return
    }
    flowSignal(req, flowId, op, status)
  }

  return {
    entries,
    entry,
    refresh,
    fetchToken,
    clearToken,
    forget,
    startFlow,
    cancelFlow,
    adoptFlow,
  }
})
