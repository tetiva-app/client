const OS_NAMES: Record<string, string> = {
  darwin: 'macOS',
  windows: 'Windows',
  linux: 'Linux',
}

const TETIVA_UA = /^Tetiva\/(\S+)\s+\(([^;)]+);([^)]*)\)/
const MAX_FALLBACK = 60

// Wire user-agent is `Tetiva/0.17.0 (darwin; mbp.local) grpc-go/1.79.3` — grpc-go appends its own
// token after ours, so the pattern cannot anchor at the end. Host leads: it names the machine.
export function deviceLabel(userAgent: string): string {
  const ua = userAgent.trim()
  if (ua === '') return 'Unknown device'

  const match = TETIVA_UA.exec(ua)
  if (!match) {
    return ua.length > MAX_FALLBACK ? ua.slice(0, MAX_FALLBACK - 1) + '…' : ua
  }

  const [, version, rawOs, rawHost] = match
  const os = rawOs.trim()
  return [rawHost.trim(), OS_NAMES[os] ?? os, `Tetiva ${version}`].filter(Boolean).join(' · ')
}
