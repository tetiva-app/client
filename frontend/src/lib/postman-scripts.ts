import type { ImportScript } from '@/services'

// Browser-mode mock of postman.scriptPreviews.
export function listPostmanScripts(content: string): ImportScript[] {
  let data: unknown
  try {
    data = JSON.parse(content)
  } catch {
    return []
  }
  if (!data || typeof data !== 'object') return []
  const out: ImportScript[] = []
  const name = (data as { info?: { name?: unknown } }).info?.name
  walk(data, typeof name === 'string' ? name : '', out)
  return out
}

function walk(node: unknown, path: string, out: ImportScript[]) {
  if (!node || typeof node !== 'object') return
  const { event, item } = node as { event?: unknown; item?: unknown }
  if (Array.isArray(event)) {
    for (const ev of event) {
      const script = importedScript(ev, path)
      if (script) out.push(script)
    }
  }
  if (!Array.isArray(item)) return
  for (const child of item) {
    const childName = child && typeof child === 'object' ? (child as { name?: unknown }).name : ''
    walk(child, `${path} / ${typeof childName === 'string' ? childName : ''}`, out)
  }
}

function importedScript(ev: unknown, path: string): ImportScript | null {
  if (!ev || typeof ev !== 'object') return null
  const { listen, disabled, script } = ev as { listen?: unknown; disabled?: unknown; script?: { exec?: unknown } }
  if ((listen !== 'prerequest' && listen !== 'test') || disabled === true) return null
  const exec = script?.exec
  const text = Array.isArray(exec) ? exec.join('\n') : typeof exec === 'string' ? exec : ''
  if (text.trim() === '') return null
  return { path, phase: listen === 'prerequest' ? 'pre' : 'post', text }
}
