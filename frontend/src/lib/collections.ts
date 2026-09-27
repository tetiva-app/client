import type { Collection } from '@/types/collection'

// Go omits parentId for a top-level collection: the key is missing, not null.
export function isRootCollection(c: Pick<Collection, 'parentId'>): boolean {
  return !c.parentId
}
