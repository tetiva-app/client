// Kept apart from markdown-actions.ts so the eagerly loaded description view does
// not pull the table kernel out of the lazy editor chunk.
export function isDescriptionEmpty(description: string | null | undefined): boolean {
  return !description || description.trim() === ''
}
