import type { Collection } from '@/types/collection'

// Go omits parentId for a top-level collection: the key is missing, not null.
export function isRootCollection(c: Pick<Collection, 'parentId'>): boolean {
  return !c.parentId
}

// Same rule as entities.SortRequests in Go, so the published page and exports match the sidebar.
const nameCollator = new Intl.Collator('en', { numeric: true, sensitivity: 'base' })

interface Sibling {
  id: string
  name: string
  createdAt?: string
}

export function compareSiblings(a: Sibling, b: Sibling): number {
  return nameCollator.compare(a.name, b.name)
    || (Date.parse(a.createdAt ?? '') || 0) - (Date.parse(b.createdAt ?? '') || 0)
    || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0)
}
