import { openExternal } from '@/lib/open-external'

// Landing serves /docs without a locale prefix and redirects by Accept-Language,
// so the app links to the bare path. utm marks help traffic coming from the app.
export const DOCS_BASE_URL = 'https://tetiva.app/docs'

export function buildDocsUrl(slug?: string): string {
  const base = slug ? `${DOCS_BASE_URL}/${slug}` : DOCS_BASE_URL
  return `${base}?utm_source=app&utm_medium=help`
}

export function openDocs(slug?: string): Promise<void> {
  // Swallow rejections here so call sites can stay fire-and-forget.
  return openExternal(buildDocsUrl(slug)).catch(() => {})
}
