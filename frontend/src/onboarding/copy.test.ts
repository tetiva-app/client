import { describe, it, expect } from 'vitest'
import { ONBOARDING_COPY, onboardingCopy } from './copy'

function entries(value: unknown, prefix = ''): [string, string][] {
  if (Array.isArray(value)) {
    return value.flatMap((item, i) => entries(item, `${prefix}[${i}]`))
  }
  if (value && typeof value === 'object') {
    return Object.entries(value as Record<string, unknown>)
      .flatMap(([k, v]) => entries(v, prefix ? `${prefix}.${k}` : k))
  }
  return [[prefix, value as string]]
}

function keyPaths(value: unknown): string[] {
  return entries(value).map(([path]) => path)
}

describe('ONBOARDING_COPY', () => {
  it('exposes the same key set in both languages', () => {
    expect(keyPaths(ONBOARDING_COPY.en).sort()).toEqual(keyPaths(ONBOARDING_COPY.ru).sort())
  })

  it('ships four tour slides per language', () => {
    expect(ONBOARDING_COPY.ru.tour.slides).toHaveLength(4)
    expect(ONBOARDING_COPY.en.tour.slides).toHaveLength(4)
  })

  it('keeps the interpolation placeholders in both languages', () => {
    for (const locale of ['ru', 'en'] as const) {
      expect(ONBOARDING_COPY[locale].verify.sentTo).toContain('{email}')
      expect(ONBOARDING_COPY[locale].verify.resendIn).toContain('{seconds}')
    }
  })

  it('leaves no string empty', () => {
    for (const locale of ['ru', 'en'] as const) {
      for (const [, text] of entries(ONBOARDING_COPY[locale])) {
        expect(text.trim().length).toBeGreaterThan(0)
      }
    }
  })
})

describe('onboardingCopy', () => {
  it('picks Russian for any ru* tag and English otherwise', () => {
    expect(onboardingCopy('ru-RU')).toBe(ONBOARDING_COPY.ru)
    expect(onboardingCopy('de-DE')).toBe(ONBOARDING_COPY.en)
  })
})
