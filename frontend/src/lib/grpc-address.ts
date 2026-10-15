// Mirrors grpcTarget in internal/adapters/requester/grpc.go.
const SCHEME = /^(grpcs?|https?):\/\//i
const ANY_SCHEME = /^[a-z][a-z0-9+.-]*:\/\//i

export function isGrpcTls(address: string): boolean {
  return /^(grpcs|https):\/\//i.test(address.trim())
}

export function grpcTarget(address: string): { target: string; tls: boolean } {
  const trimmed = address.trim()
  const m = SCHEME.exec(trimmed)
  if (!m) return { target: trimmed, tls: false }
  const rest = trimmed.slice(m[0].length).replace(/\/$/, '')
  const tls = isGrpcTls(trimmed)
  return { target: tls && !hasPort(rest) && !rest.includes('{{') ? withPort443(rest) : rest, tls }
}

export function schemeInVariable(address: string): boolean {
  return /^\{\{[^{}]+\}\}\/?$/.test(address.trim())
}

export function toggleGrpcTls(address: string): string {
  const rest = address.trimStart().replace(ANY_SCHEME, '')
  return isGrpcTls(address) ? rest : `grpcs://${rest}`
}

function hasPort(host: string): boolean {
  const close = host.lastIndexOf(']')
  return close >= 0 ? host.slice(close + 1).startsWith(':') : host.split(':').length === 2
}

function withPort443(host: string): string {
  const bare = host.replace(/^\[(.*)\]$/, '$1')
  return bare.includes(':') ? `[${bare}]:443` : `${bare}:443`
}
