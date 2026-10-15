import { describe, it, expect } from 'vitest'
import { fill } from '@/lib/locale'
import { ENVIRONMENT_COPY } from './copy'

function entries(value: unknown, prefix = ''): [string, string][] {
  if (value && typeof value === 'object') {
    return Object.entries(value as Record<string, unknown>)
      .flatMap(([k, v]) => entries(v, prefix ? `${prefix}.${k}` : k))
  }
  return [[prefix, value as string]]
}

function placeholders(text: string): string[] {
  return (text.match(/\{\w+\}/g) ?? []).sort()
}

describe('ENVIRONMENT_COPY', () => {
  it('exposes the same key set in both languages', () => {
    const en = entries(ENVIRONMENT_COPY.en).map(([k]) => k).sort()
    const ru = entries(ENVIRONMENT_COPY.ru).map(([k]) => k).sort()
    expect(ru).toEqual(en)
  })

  it('leaves no string empty', () => {
    for (const locale of ['en', 'ru'] as const) {
      for (const [key, text] of entries(ENVIRONMENT_COPY[locale])) {
        expect(typeof text, `${locale}.${key}`).toBe('string')
        expect(text.trim().length, `${locale}.${key}`).toBeGreaterThan(0)
      }
    }
  })

  it('keeps the same placeholders in both languages', () => {
    const ru = new Map(entries(ENVIRONMENT_COPY.ru))
    for (const [key, text] of entries(ENVIRONMENT_COPY.en)) {
      expect(placeholders(ru.get(key) ?? ''), key).toEqual(placeholders(text))
    }
  })

  it('keeps the English wording', () => {
    const { modal, deleteEnvironment, picker } = ENVIRONMENT_COPY.en
    expect(modal.title).toBe('Manage Environments')
    expect(fill(modal.copyName, { name: 'Staging' })).toBe('Staging (copy)')
    expect(fill(deleteEnvironment.text, { name: 'Staging' }))
      .toBe('Delete environment "Staging"? All variables in this environment will be lost.')
    expect(deleteEnvironment.failed).toBe('Action failed. Please try again.')
    expect(picker.none).toBe('No Environment')
    expect(picker.manage).toBe('Manage Environments...')
  })

  it('quotes names with guillemets in Russian', () => {
    const { modal, deleteEnvironment } = ENVIRONMENT_COPY.ru
    expect(fill(deleteEnvironment.text, { name: 'Staging' }))
      .toBe('Удалить окружение «Staging»? Все его переменные будут потеряны.')
    expect(fill(modal.copyName, { name: 'Staging' })).toBe('Staging (копия)')
  })
})
