// Import and Copy-as-cURL both come back with full English sentences from Go
// (one per item). A toast shows the first few and counts the rest so a large
// Postman file cannot push a wall of text on screen.
const DEFAULT_LIMIT = 3

export function warningsToastMessage(warnings: string[] | undefined, limit = DEFAULT_LIMIT): string {
  const items = (warnings ?? []).filter(w => w.trim() !== '')
  if (items.length === 0) return ''
  const listed = items.slice(0, limit).join('; ')
  const rest = items.length - Math.min(limit, items.length)
  return rest > 0 ? `${listed} (and ${rest} more)` : listed
}
