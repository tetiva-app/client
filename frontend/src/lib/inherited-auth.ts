import type { Collection } from '@/types/collection'

const MAX_COLLECTION_DEPTH = 50

export function chainSignature(collections: Map<string, Collection>, startId: string | null | undefined): string {
  const parts: string[] = []
  const seen = new Set<string>()
  let id = startId
  while (id && !seen.has(id) && parts.length < MAX_COLLECTION_DEPTH) {
    seen.add(id)
    const c = collections.get(id)
    if (!c) {
      parts.push(id)
      break
    }
    parts.push(`${c.id}|${c.parentId ?? ''}|${c.authType}|${c.authData}`)
    id = c.parentId
  }
  return parts.join('\n')
}
