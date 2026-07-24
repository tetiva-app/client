import { computed, toValue, type MaybeRefOrGetter } from 'vue'

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function escapeRegex(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

/**
 * Wraps substring matches of `query` inside `text` with <mark>. Safe for v-html
 * (both sides HTML-escaped); queries shorter than 2 chars return plain escaped text.
 */
export function useHighlight(
  text: MaybeRefOrGetter<string>,
  query: MaybeRefOrGetter<string>,
) {
  return computed(() => {
    const rawText = toValue(text)
    const rawQuery = toValue(query).trim()
    const safeText = escapeHtml(rawText)
    if (rawQuery.length < 2) return safeText

    const pattern = new RegExp(escapeRegex(escapeHtml(rawQuery)), 'gi')
    return safeText.replace(pattern, (m) => `<mark>${m}</mark>`)
  })
}
