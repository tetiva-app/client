import { ref } from 'vue'
import { defineStore } from 'pinia'
import { getSyncService, isWailsEnvironment } from '@/services'
import { openExternal } from '@/lib/open-external'
import type { Result } from '@/types/common'
import type {
  AuthState,
  BrowserSignInEvent,
  BrowserSignInInfo,
  BrowserSignInStatus,
  SignInIntent,
  SyncServiceAPI,
} from '@/services/sync-api'

// The flow record lives here, not in the modal: closing and reopening the dialog,
// or reloading the webview, must not disturb a sign-in the Go manager is running.
const STORAGE_KEY = 'tetiva.browserSignIn'

// How long an adoption waits before re-reading a flow that is still starting:
// discovery, the start call and a join of the previous flow all sit before pending.
const STARTING_RETRY_MS = 250
const STARTING_TIMEOUT_MS = 30_000

// loginUrl carries the claim secret in its fragment, so it is never stored here;
// "Open again" re-reads it from the manager.
interface StoredSignIn {
  flowId: string
  host: string
  expiresAt: string
  serverUrl: string
}

export type SignInPanelState = 'idle' | 'starting' | 'pending' | 'done' | 'error' | 'cancelled'

interface FlowRecord {
  flowId: string
  op: number
  off: () => void
  // Set once a done signal began reading the account: the record still owns the
  // id, but no other answer may touch the panel any more.
  settling: boolean
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

function readStored(): StoredSignIn | null {
  const store = webStorage()
  if (!store) return null
  try {
    const parsed = JSON.parse(store.getItem(STORAGE_KEY) ?? 'null')
    if (!parsed || typeof parsed !== 'object') return null
    const entry = parsed as Partial<StoredSignIn>
    if (typeof entry.flowId !== 'string' || !entry.flowId) return null
    return {
      flowId: entry.flowId,
      host: typeof entry.host === 'string' ? entry.host : '',
      expiresAt: typeof entry.expiresAt === 'string' ? entry.expiresAt : '',
      serverUrl: typeof entry.serverUrl === 'string' ? entry.serverUrl : '',
    }
  } catch {
    return null
  }
}

function writeStored(entry: StoredSignIn) {
  const store = webStorage()
  if (!store) return
  try {
    store.setItem(STORAGE_KEY, JSON.stringify(entry))
  } catch {
    // Private-mode quota; a reload then simply cannot re-attach.
  }
}

function dropStored() {
  const store = webStorage()
  if (!store) return
  try {
    store.removeItem(STORAGE_KEY)
  } catch {
    // Same as above: nothing to recover from, and nothing to report.
  }
}

// A deadline the server moved forward while the user confirms their email. An
// answer that arrives out of order carries the old one and must not shorten it.
function movesForward(current: string, next: string): boolean {
  if (!next) return false
  const to = Date.parse(next)
  if (Number.isNaN(to)) return false
  const from = Date.parse(current)
  return Number.isNaN(from) || to > from
}

export const useBrowserSignInStore = defineStore('browserSignIn', () => {
  const flowId = ref('')
  const loginUrl = ref('')
  const host = ref('')
  const expiresAt = ref('')
  const serverUrl = ref('')
  const state = ref<SignInPanelState>('idle')
  const error = ref('')
  const emailVerificationPending = ref(false)
  const auth = ref<AuthState | null>(null)

  // `op` is bumped whenever a record is created or retired, so a signal from a
  // replaced flow is dropped; `startSeq` counts starts alone.
  let record: FlowRecord | null = null
  let op = 0
  let startSeq = 0

  function owns(id: string, myOp: number): boolean {
    return !!record && record.flowId === id && record.op === myOp
  }

  // Owned and not yet committing: everything except the done path checks this.
  function active(id: string, myOp: number): boolean {
    return owns(id, myOp) && !record!.settling
  }

  // Synchronous and at-most-once: it releases the record before any caller can
  // await, so a signal already queued on the event bus cannot commit after it.
  function retire(): FlowRecord | null {
    const current = record
    if (!current) return null
    record = null
    if (readStored()?.flowId === current.flowId) dropStored()
    try {
      current.off()
    } catch {
      // An unsubscribe that throws must not strand the panel.
    }
    op += 1
    return current
  }

  // The login URL carries the claim secret; a finished flow keeps no copy of it.
  function clearSecret() {
    flowId.value = ''
    loginUrl.value = ''
  }

  function persist() {
    if (!record) return
    writeStored({
      flowId: record.flowId,
      host: host.value,
      expiresAt: expiresAt.value,
      serverUrl: serverUrl.value,
    })
  }

  function install(id: string, target: string): number {
    op += 1
    startSeq += 1
    record = { flowId: id, op, off: () => {}, settling: false }
    flowId.value = id
    loginUrl.value = ''
    host.value = ''
    expiresAt.value = ''
    serverUrl.value = target
    state.value = 'starting'
    error.value = ''
    emailVerificationPending.value = false
    auth.value = null
    return op
  }

  function reset(): void {
    retire()
    clearSecret()
    host.value = ''
    expiresAt.value = ''
    serverUrl.value = ''
    state.value = 'idle'
    error.value = ''
    emailVerificationPending.value = false
    auth.value = null
  }

  function fail(id: string, myOp: number, message: string) {
    if (!active(id, myOp)) return
    retire()
    clearSecret()
    state.value = 'error'
    error.value = message
  }

  async function requestCancel(id: string) {
    try {
      const svc = await getSyncService()
      await svc?.cancelBrowserSignIn(id)
    } catch {
      // The request expires on its own; nothing here is worth surfacing.
    }
  }

  function applyInfo(info: BrowserSignInInfo) {
    loginUrl.value = info.loginUrl
    host.value = info.host
    if (info.expiresAt) expiresAt.value = info.expiresAt
    persist()
  }

  // Reads the account behind a done signal while the record still owns the flow,
  // and only then releases it: retiring first would invalidate the very read.
  async function finish(id: string, myOp: number, known?: BrowserSignInStatus) {
    if (!active(id, myOp)) return
    record!.settling = true

    let status: BrowserSignInStatus | null = known ?? null
    if (!status) {
      try {
        const svc = await getSyncService()
        const res = await svc?.browserSignInStatus(id)
        status = res && !res.error ? res.data : null
      } catch {
        status = null
      }
    }
    // Someone retired the record while the read ran — a cancel that lost the race
    // reapplies done itself, and a new sign-in must keep its own panel.
    if (!owns(id, myOp)) return

    const account = status?.state === 'done' ? status.auth ?? null : null
    retire()
    clearSecret()
    state.value = 'done'
    error.value = ''
    emailVerificationPending.value = false
    auth.value = account
  }

  // The single landing point for every signal, from the event bus or a status
  // read. An event never carries the account, so done goes and reads it.
  function signal(id: string, myOp: number, sig: BrowserSignInEvent, known?: BrowserSignInStatus) {
    if (!active(id, myOp)) return
    switch (sig.state) {
      case 'starting':
      case 'pending':
        state.value = sig.state
        emailVerificationPending.value = sig.emailVerificationPending
        if (movesForward(expiresAt.value, sig.expiresAt)) {
          expiresAt.value = sig.expiresAt
          persist()
        }
        return
      case 'done':
        void finish(id, myOp, known)
        return
      case 'error':
      case 'cancelled':
        retire()
        clearSecret()
        state.value = sig.state
        error.value = sig.error
        return
      default:
        // '' — the manager has never heard of the id, or forgot it again.
        reset()
    }
  }

  // Subscribes an already-installed record. Returns null when the listener could
  // not be attached, or when a signal settled the flow while it was attaching.
  async function attach(id: string, myOp: number): Promise<SyncServiceAPI | null> {
    try {
      const svc = await getSyncService()
      if (!svc) {
        fail(id, myOp, 'sync is unavailable in this build')
        return null
      }
      const off = await svc.subscribeBrowserSignIn(id, (e) => signal(id, myOp, e))
      if (!active(id, myOp)) {
        off()
        return null
      }
      record!.off = off
      return svc
    } catch (e) {
      fail(id, myOp, messageOf(e))
      return null
    }
  }

  // Reads the status of a flow the record still owns. Returns null when the read
  // failed or when a signal settled the flow while it was in flight.
  async function readStatus(
    svc: SyncServiceAPI, id: string, myOp: number,
  ): Promise<BrowserSignInStatus | null> {
    let result: Result<BrowserSignInStatus>
    try {
      result = await svc.browserSignInStatus(id)
    } catch (e) {
      fail(id, myOp, messageOf(e))
      return null
    }
    if (!active(id, myOp)) return null
    if (result.error) {
      fail(id, myOp, result.error.message)
      return null
    }
    return result.data
  }

  async function openLink(id: string, myOp: number) {
    try {
      await openExternal(loginUrl.value)
    } catch (e) {
      if (!active(id, myOp)) return
      const previous = retire()
      if (previous) void requestCancel(previous.flowId)
      clearSecret()
      state.value = 'error'
      error.value = messageOf(e)
    }
  }

  // The record is registered before anything is awaited, so no window exists
  // where a signal could arrive with nothing to attach it to.
  async function start(target: string, intent: SignInIntent, locale: string): Promise<void> {
    const previous = retire()
    if (previous) void requestCancel(previous.flowId)
    const id = crypto.randomUUID()
    const myOp = install(id, target)
    writeStored({ flowId: id, host: '', expiresAt: '', serverUrl: target })

    const svc = await attach(id, myOp)
    if (!svc) return

    let result: Result<BrowserSignInInfo>
    try {
      result = await svc.startBrowserSignIn({ flowId: id, serverUrl: target, intent, locale })
    } catch (e) {
      fail(id, myOp, messageOf(e))
      return
    }
    if (!active(id, myOp)) {
      // A cancel that reached Go before the id was registered counts as unknown
      // there; this answer proves the request is live, so ask again.
      if (!result.error) void requestCancel(id)
      return
    }
    if (result.error) {
      fail(id, myOp, result.error.message)
      return
    }
    applyInfo(result.data)
    state.value = 'pending'
    await openLink(id, myOp)
  }

  // A reload destroys Pinia while the Go manager keeps polling. Same ordering as
  // start: guarded placeholder, then subscribe, then ask for the status — a flow
  // that ends in between is then recovered by the event, not lost.
  async function adopt(): Promise<void> {
    if (record) return
    const stored = readStored()
    if (!stored) return
    const myOp = install(stored.flowId, stored.serverUrl)
    host.value = stored.host
    expiresAt.value = stored.expiresAt

    const svc = await attach(stored.flowId, myOp)
    if (!svc) return

    let status = await readStatus(svc, stored.flowId, myOp)
    if (!status) return
    // A flow whose start has not finished carries no login URL yet, and giving up
    // early strands the panel on "Contacting" with no buttons.
    const until = Date.now() + STARTING_TIMEOUT_MS
    while (status.state === 'starting' && Date.now() < until) {
      await new Promise(resolve => setTimeout(resolve, STARTING_RETRY_MS))
      if (!active(stored.flowId, myOp)) return
      status = await readStatus(svc, stored.flowId, myOp)
      if (!status) return
    }
    if (status.state === 'pending' || status.state === 'starting') {
      applyInfo(status.info)
      state.value = status.state
      emailVerificationPending.value = status.emailVerificationPending
      return
    }
    signal(stored.flowId, myOp, {
      state: status.state,
      error: status.error,
      emailVerificationPending: status.emailVerificationPending,
      expiresAt: status.info.expiresAt,
    }, status)
  }

  // Cancel cannot interrupt a flow that is already committing, so the status is
  // read once more afterwards — and applied only while nobody started another.
  async function cancel(): Promise<void> {
    const seq = startSeq
    const previous = retire()
    const id = previous ? previous.flowId : (readStored()?.flowId ?? '')
    if (!previous) dropStored()
    clearSecret()
    state.value = 'cancelled'
    error.value = ''
    emailVerificationPending.value = false
    if (!id) return

    const svc = await getSyncService()
    if (!svc) return
    try {
      await svc.cancelBrowserSignIn(id)
    } catch {
      // Already gone on the Go side, or the transport died; the read below tells.
    }
    let status: BrowserSignInStatus | null = null
    try {
      const res = await svc.browserSignInStatus(id)
      status = res.error ? null : res.data
    } catch {
      return
    }
    if (status?.state !== 'done') return
    if (startSeq !== seq || flowId.value !== '') return
    state.value = 'done'
    error.value = ''
    emailVerificationPending.value = false
    auth.value = status.auth ?? null
  }

  // The browser may have failed to launch, or landed in the wrong profile, while
  // the request itself is still waiting.
  async function openAgain(): Promise<void> {
    if (!record || !loginUrl.value) return
    await openLink(record.flowId, record.op)
  }

  async function copyLink(): Promise<boolean> {
    if (!loginUrl.value) return false
    if (isWailsEnvironment()) {
      try {
        // navigator.clipboard is unreliable in the Wails WebView after awaited
        // backend calls — prefer the runtime Clipboard with a browser fallback.
        const { Clipboard } = await import('@wailsio/runtime')
        await Clipboard.SetText(loginUrl.value)
        return true
      } catch {
        // Fall through to the browser API.
      }
    }
    try {
      await navigator.clipboard.writeText(loginUrl.value)
      return true
    } catch {
      return false
    }
  }

  return {
    flowId,
    loginUrl,
    host,
    expiresAt,
    serverUrl,
    state,
    error,
    emailVerificationPending,
    auth,
    start,
    adopt,
    cancel,
    openAgain,
    copyLink,
    reset,
  }
})
