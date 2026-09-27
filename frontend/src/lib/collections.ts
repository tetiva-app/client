import type { Collection } from '@/types/collection'

// Go omits parentId for a top-level collection, so the key is missing rather than null.
export function isRootCollection(c: Pick<Collection, 'parentId'>): boolean {
  return !c.parentId
}
