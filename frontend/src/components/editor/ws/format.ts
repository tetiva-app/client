import type { WsDir } from '@/types/websocket'

export function fmtTime(ts: number): string {
  const d = new Date(ts)
  return d.toLocaleTimeString(undefined, { hour12: false }) + '.' + String(d.getMilliseconds()).padStart(3, '0')
}

export function dirArrow(dir: WsDir): string {
  return dir === 'in' ? '↓' : dir === 'out' ? '↑' : '•'
}
