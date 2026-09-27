import { describe, it, expect } from 'vitest'
import { UI_COPY } from './copy'

function placeholders(text: string): string[] {
  return (text.match(/\{\w+\}/g) ?? []).sort()
}

describe('UI_COPY', () => {
  it('has the same keys, no empty strings and the same placeholders in both languages', () => {
    expect(Object.keys(UI_COPY.ru).sort()).toEqual(Object.keys(UI_COPY.en).sort())
    for (const key of Object.keys(UI_COPY.en) as (keyof typeof UI_COPY.en)[]) {
      expect(UI_COPY.en[key].trim(), `en.${key}`).not.toBe('')
      expect(UI_COPY.ru[key].trim(), `ru.${key}`).not.toBe('')
      expect(placeholders(UI_COPY.ru[key]), key).toEqual(placeholders(UI_COPY.en[key]))
    }
  })
})
