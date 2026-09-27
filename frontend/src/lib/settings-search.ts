import type { Locale } from '@/lib/locale'

export type SettingsSectionId = 'interface' | 'editor' | 'publishing' | 'mcp' | 'updates' | 'about'

export const SETTINGS_SECTIONS: readonly SettingsSectionId[] = [
  'interface', 'editor', 'publishing', 'mcp', 'updates', 'about',
]

export interface SettingsRowIndex {
  id: string
  section: SettingsSectionId
  text: Record<Locale, string[]>
}

function normalize(s: string): string {
  return s.trim().toLowerCase().replace(/ё/g, 'е').replace(/[‘’]/g, "'").replace(/\s+/g, ' ')
}

export function searchSettings(query: string, index: SettingsRowIndex[]): Map<SettingsSectionId, Set<string>> {
  const hits = new Map<SettingsSectionId, Set<string>>()
  const q = normalize(query)
  if (!q) return hits
  for (const row of index) {
    if (!normalize([...row.text.en, ...row.text.ru].join(' ')).includes(q)) continue
    const ids = hits.get(row.section) ?? new Set<string>()
    ids.add(row.id)
    hits.set(row.section, ids)
  }
  return hits
}
