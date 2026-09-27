import { openExternal } from '@/lib/open-external'
import { currentLocale, type Locale } from '@/lib/locale'

// Docs live under /en and /ru; the link follows the app language, not Accept-Language.
export const SITE_URL = 'https://tetiva.app'

export function buildDocsUrl(slug?: string, locale: Locale = currentLocale.value): string {
  const base = `${SITE_URL}/${locale}/docs${slug ? `/${slug}` : ''}`
  return `${base}?utm_source=app&utm_medium=help`
}

export function openDocs(slug?: string): Promise<void> {
  // Swallow rejections here so call sites can stay fire-and-forget.
  return openExternal(buildDocsUrl(slug)).catch(() => {})
}
