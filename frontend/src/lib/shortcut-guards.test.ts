import { describe, it, expect } from 'vitest'
import { isModShortcut } from './shortcut-guards'

function ev(over: Partial<KeyboardEvent>): KeyboardEvent {
  return { metaKey: false, ctrlKey: false, key: '', code: '', ...over } as KeyboardEvent
}

describe('isModShortcut', () => {
  it('matches the physical key on a Cyrillic layout', () => {
    expect(isModShortcut(ev({ metaKey: true, key: 'ы', code: 'KeyS' }), 'KeyS', 's')).toBe(true)
  })
  it('matches by key when code is missing (synthetic events, Dvorak)', () => {
    expect(isModShortcut(ev({ ctrlKey: true, key: 'S' }), 'KeyS', 's')).toBe(true)
  })
  it('requires a modifier', () => {
    expect(isModShortcut(ev({ key: 's', code: 'KeyS' }), 'KeyS', 's')).toBe(false)
  })
  it('ignores other keys', () => {
    expect(isModShortcut(ev({ metaKey: true, key: 'ф', code: 'KeyA' }), 'KeyS', 's')).toBe(false)
  })
  it('ignores AltGr (Ctrl+Alt types ß/ś on Windows layouts)', () => {
    expect(isModShortcut(ev({ ctrlKey: true, altKey: true, key: 'ś', code: 'KeyS' }), 'KeyS', 's')).toBe(false)
  })
  it('ignores Shift so Cmd+Shift+S stays free', () => {
    expect(isModShortcut(ev({ metaKey: true, shiftKey: true, key: 'S', code: 'KeyS' }), 'KeyS', 's')).toBe(false)
  })
})
