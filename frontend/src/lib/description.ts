// Kept apart from markdown-actions.ts so the eagerly loaded description view does
// not pull the table kernel out of the lazy editor chunk.
export function isDescriptionEmpty(description: string | null | undefined): boolean {
  return !description || description.trim() === ''
}

export function adoptStoreValue(local: string, prevStore: string | undefined, nextStore: string): string {
  if (prevStore === undefined) return nextStore
  return local === prevStore ? nextStore : local
}

export function adoptStashedValue(local: string, base: string, current: string): string {
  return base === current ? local : current
}

// Mirrors domain.MaxDescriptionLen: the backend rejects anything longer.
export const MAX_DESCRIPTION_BYTES = 16384

export function descriptionBytes(s: string): number {
  return new TextEncoder().encode(s).length
}

export function descriptionSaveBlocked(next: string, saved: string): boolean {
  const bytes = descriptionBytes(next)
  // The backend rejects growth only, so an over-cap doc still saves while it shrinks.
  return bytes > MAX_DESCRIPTION_BYTES && bytes > descriptionBytes(saved)
}
