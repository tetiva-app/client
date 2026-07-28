import { onScopeDispose, ref } from 'vue'

const DEFAULT_INTERVAL_MS = 5000
const DEFAULT_TIMEOUT_MS = 300000
const RESEND_COOLDOWN_SECONDS = 60

// Polls GetMe while the waiting screen is open. The caller drives the lifecycle:
// SyncConnectModal stays mounted, so reopening it has to resume a stopped poll.
export function useVerificationPolling(opts: {
  intervalMs?: number
  timeoutMs?: number
  onVerified: () => void
}) {
  const intervalMs = opts.intervalMs ?? DEFAULT_INTERVAL_MS
  const timeoutMs = opts.timeoutMs ?? DEFAULT_TIMEOUT_MS

  const exhausted = ref(false)
  let ticker: ReturnType<typeof setInterval> | null = null
  let ceiling: ReturnType<typeof setTimeout> | null = null
  let inFlight: Promise<void> | null = null
  let verified = false

  function stop() {
    if (ticker) {
      clearInterval(ticker)
      ticker = null
    }
    if (ceiling) {
      clearTimeout(ceiling)
      ceiling = null
    }
  }

  function start() {
    stop()
    exhausted.value = false
    verified = false
    ticker = setInterval(() => { void checkNow() }, intervalMs)
    ceiling = setTimeout(() => {
      stop()
      exhausted.value = true
    }, timeoutMs)
  }

  async function runCheck(): Promise<void> {
    try {
      const { getSyncService } = await import('@/services')
      const svc = await getSyncService()
      if (!svc) return
      const res = await svc.getMe()
      if (res.error || !res.data?.emailVerified || verified) return
      verified = true
      stop()
      opts.onVerified()
    } catch {
      // Waiting for a confirmation click is exactly when the network is flaky;
      // a failed poll is not worth a message.
    }
  }

  // Concurrent callers share one request: the ticker and the "I confirmed"
  // button can land together, and onVerified must fire once.
  function checkNow(): Promise<void> {
    if (!inFlight) {
      inFlight = runCheck().finally(() => { inFlight = null })
    }
    return inFlight
  }

  onScopeDispose(stop)

  return { start, stop, checkNow, exhausted }
}

// Countdown that keeps the resend button disabled after a successful send.
export function useResendCooldown(seconds = RESEND_COOLDOWN_SECONDS) {
  const secondsLeft = ref(0)
  let ticker: ReturnType<typeof setInterval> | null = null

  function stop() {
    if (ticker) {
      clearInterval(ticker)
      ticker = null
    }
    secondsLeft.value = 0
  }

  function start() {
    stop()
    secondsLeft.value = seconds
    ticker = setInterval(() => {
      secondsLeft.value -= 1
      if (secondsLeft.value <= 0) stop()
    }, 1000)
  }

  onScopeDispose(stop)

  return { secondsLeft, start, stop }
}
