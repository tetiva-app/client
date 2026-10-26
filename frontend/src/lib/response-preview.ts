export type TextPreviewKind = 'html' | 'svg'

export const PREVIEW_CSP = "default-src 'none'; img-src data:; style-src 'unsafe-inline'; font-src data:"

const XLINK_NS = 'http://www.w3.org/1999/xlink'

export const PREVIEW_IMAGE_MAX_BYTES = 10 * 1024 * 1024

function mediaType(contentType: string): string {
  return contentType.split(';')[0].trim().toLowerCase()
}

export function mayBeImage(contentType: string): boolean {
  const type = mediaType(contentType)
  return type === '' || type === 'application/octet-stream' || type.startsWith('image/')
}

export function textPreviewKind(contentType: string, body: string): TextPreviewKind | null {
  const type = mediaType(contentType)
  if (type === 'text/html') return 'html'
  if (type === 'image/svg+xml') return 'svg'
  if (type !== '') return null

  const head = body.slice(0, 1024).trimStart().toLowerCase()
  if (head.startsWith('<!doctype html') || head.startsWith('<html')) return 'html'
  if (/^(<\?xml[^>]*>\s*)?((<!--[\s\S]*?-->|<!doctype[^>]*>)\s*)*<svg[\s>/]/.test(head)) return 'svg'
  return null
}

// CSP covers neither preconnect nor navigation, hence no <link> and no href.
function scrub(root: Document | DocumentFragment) {
  root.querySelectorAll('meta[http-equiv], base, link').forEach(el => el.remove())
  root.querySelectorAll('a, area').forEach((el) => {
    el.removeAttribute('href')
    el.removeAttributeNS(XLINK_NS, 'href')
  })
  root.querySelectorAll('template').forEach(t => scrub(t.content))
}

export function htmlPreviewDoc(html: string): string {
  const doc = new DOMParser().parseFromString(html, 'text/html')
  scrub(doc)

  const csp = doc.createElement('meta')
  csp.httpEquiv = 'Content-Security-Policy'
  csp.content = PREVIEW_CSP
  const dns = doc.createElement('meta')
  dns.httpEquiv = 'x-dns-prefetch-control'
  dns.content = 'off'
  doc.head.prepend(csp, dns)

  return '<!doctype html>' + doc.documentElement.outerHTML
}

const IMAGE_FORMATS: Record<string, string> = {
  'image/svg+xml': 'SVG',
  'image/x-icon': 'ICO',
  'image/vnd.microsoft.icon': 'ICO',
}

export function imageFormatLabel(mimeType: string): string {
  const type = mediaType(mimeType)
  return IMAGE_FORMATS[type] ?? type.replace(/^image\//, '').toUpperCase()
}

export function base64ToBytes(b64: string): Uint8Array {
  const bin = atob(b64)
  const bytes = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
  return bytes
}
