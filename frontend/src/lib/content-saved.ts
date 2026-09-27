type Listener = () => void

const listeners = new Set<Listener>()

export function onContentSaved(fn: Listener): () => void {
  listeners.add(fn)
  return () => { listeners.delete(fn) }
}

export function contentSaved(): void {
  for (const fn of listeners) fn()
}
