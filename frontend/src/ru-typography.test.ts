import { describe, it, expect } from 'vitest'
import { SETTINGS_COPY } from '@/components/settings/copy'
import { TREE_COPY } from '@/components/sidebar/copy'
import { PUBLICATION_COPY } from '@/components/publication/copy'
import { UI_COPY } from '@/components/ui/copy'
import { PUBLICATION_ERROR_COPY } from '@/lib/publication-errors'
import { PARKED_COPY } from '@/lib/sync-notices'
import { ONBOARDING_COPY } from '@/onboarding/copy'
import { RELEASE_NOTES } from '@/whats-new/notes'

function strings(value: unknown, path: string): [string, string][] {
  if (typeof value === 'string') return [[path, value]]
  if (Array.isArray(value)) return value.flatMap((v, i) => strings(v, `${path}[${i}]`))
  if (value && typeof value === 'object') {
    return Object.entries(value as Record<string, unknown>).flatMap(([k, v]) => strings(v, `${path}.${k}`))
  }
  return []
}

const RU: [string, unknown][] = [
  ['settings', SETTINGS_COPY.ru],
  ['tree', TREE_COPY.ru],
  ['publication', PUBLICATION_COPY.ru],
  ['ui', UI_COPY.ru],
  ['publicationErrors', PUBLICATION_ERROR_COPY.ru],
  ['parked', PARKED_COPY.ru],
  ['onboarding', ONBOARDING_COPY.ru],
  ['whatsNew', RELEASE_NOTES.map((n) => n.ru)],
]

describe('Russian copy', () => {
  it('never lets a line start with an em dash', () => {
    for (const [name, dict] of RU) {
      for (const [path, text] of strings(dict, name)) {
        expect(text, path).not.toMatch(/ —/)
      }
    }
  })
})
