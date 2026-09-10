import type { WsDir } from '@/types/websocket'

export function fmtTime(ts: number): string {
  const d = new Date(ts)
  return d.toLocaleTimeString(undefined, { hour12: false }) + '.' + String(d.getMilliseconds()).padStart(3, '0')
}

export function dirArrow(dir: WsDir): string {
  return dir === 'in' ? '↓' : dir === 'out' ? '↑' : '•'
}

// atob follows the forgiving decode and takes unpadded input; the backend uses
// Go's StdEncoding, which does not, so padding is checked here.
export function isBase64(value: string): boolean {
  const compact = value.replace(/\s/g, '')
  if (compact === '' || compact.length % 4 !== 0) return false
  if (!/^[A-Za-z0-9+/]*={0,2}$/.test(compact)) return false
  try {
    atob(compact)
    return true
  } catch {
    return false
  }
}
