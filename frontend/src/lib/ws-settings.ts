// Mirrors internal/domain/usecase/websocket/settings.go: the same tolerant
// reading rules, plus `extra` so keys written by a newer client survive a
// read-modify-write round trip.

export type WsFormat = 'json' | 'text' | 'binary'

export interface WsSavedMessage {
  id: string
  name: string
  format: WsFormat
  data: string
  extra?: Record<string, unknown>
}

export interface WsSettings {
  version: 1
  pingIntervalSec: number
  subprotocols: string[]
  messages: WsSavedMessage[]
  extra: Record<string, unknown>
}

// Mirrors maxPingIntervalSec in settings.go: seconds that still fit an int64 Duration.
const MAX_PING_INTERVAL_SEC = 9223372036

const KNOWN_KEYS = ['version', 'pingIntervalSec', 'subprotocols', 'messages']
const KNOWN_MESSAGE_KEYS = ['id', 'name', 'format', 'data']
const FORMATS: readonly string[] = ['json', 'text', 'binary']

export function emptyWsSettings(): WsSettings {
  return { version: 1, pingIntervalSec: 0, subprotocols: [], messages: [], extra: {} }
}

// parseWsSettings never throws: an empty body, a replay draft or an imported
// payload all read as the defaults.
export function parseWsSettings(body: string): WsSettings {
  const settings = emptyWsSettings()
  if (!body || !body.trim()) return settings

  let doc: unknown
  try {
    doc = JSON.parse(body)
  } catch {
    return settings
  }
  if (!isRecord(doc)) return settings

  const ping = doc.pingIntervalSec
  // Whole seconds only: a fractional value would arm a sub-second ticker in Go.
  if (typeof ping === 'number' && Number.isInteger(ping) && ping >= 1 && ping <= MAX_PING_INTERVAL_SEC) {
    settings.pingIntervalSec = ping
  }

  const subs = doc.subprotocols
  if (Array.isArray(subs) && subs.every((v) => typeof v === 'string')) {
    settings.subprotocols = subs as string[]
  }

  const messages = doc.messages
  if (Array.isArray(messages)) {
    for (const item of messages) {
      const parsed = parseMessage(item)
      if (parsed) settings.messages.push(parsed)
    }
  }

  settings.extra = unknownKeys(doc, KNOWN_KEYS)
  return settings
}

// serializeWsSettings always writes version 1 and merges `extra` back at both levels.
export function serializeWsSettings(settings: WsSettings): string {
  return JSON.stringify({
    ...settings.extra,
    version: 1,
    pingIntervalSec: settings.pingIntervalSec,
    subprotocols: settings.subprotocols,
    messages: settings.messages.map((m) => ({
      ...(m.extra ?? {}),
      id: m.id,
      name: m.name,
      format: m.format,
      data: m.data,
    })),
  })
}

function parseMessage(item: unknown): WsSavedMessage | null {
  if (!isRecord(item)) return null
  const { id, name, format, data } = item
  if (typeof id !== 'string' || !id) return null
  if (typeof format !== 'string' || !FORMATS.includes(format)) return null
  if (name !== undefined && name !== null && typeof name !== 'string') return null
  if (data !== undefined && data !== null && typeof data !== 'string') return null
  const message: WsSavedMessage = {
    id,
    name: typeof name === 'string' ? name : '',
    format: format as WsFormat,
    data: typeof data === 'string' ? data : '',
  }
  const extra = unknownKeys(item, KNOWN_MESSAGE_KEYS)
  if (Object.keys(extra).length > 0) message.extra = extra
  return message
}

function unknownKeys(doc: Record<string, unknown>, known: string[]): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(doc)) {
    if (!known.includes(key)) out[key] = value
  }
  return out
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
