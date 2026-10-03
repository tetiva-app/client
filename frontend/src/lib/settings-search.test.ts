import { describe, it, expect } from 'vitest'
import { searchSettings, type SettingsRowIndex } from './settings-search'
import { SETTINGS_SEARCH_INDEX, settingsSearchIndex } from '@/components/settings/copy'

const INDEX: SettingsRowIndex[] = [
  { id: 'theme', section: 'interface', text: { en: ['Theme', 'Light, dark'], ru: ['Тема', 'Светлая, тёмная'] } },
  { id: 'mcp-token', section: 'mcp', text: { en: ['Access token'], ru: ['Токен доступа'] } },
  { id: 'mcp-address', section: 'mcp', text: { en: ['Address'], ru: ['Адрес'] } },
]

describe('searchSettings', () => {
  it('returns nothing for an empty or blank query', () => {
    expect(searchSettings('', INDEX).size).toBe(0)
    expect(searchSettings('   ', INDEX).size).toBe(0)
  })

  it('matches case-insensitively after trimming', () => {
    expect(searchSettings('  THEME ', INDEX)).toEqual(new Map([['interface', new Set(['theme'])]]))
  })

  it('matches either language whichever one the app is in', () => {
    expect(searchSettings('токен', INDEX).get('mcp')).toEqual(new Set(['mcp-token']))
    expect(searchSettings('address', INDEX).get('mcp')).toEqual(new Set(['mcp-address']))
    expect(searchSettings('адрес', INDEX).get('mcp')).toEqual(new Set(['mcp-address']))
  })

  it('treats ё and е as the same letter', () => {
    expect(searchSettings('темная', INDEX).get('interface')).toEqual(new Set(['theme']))
  })

  it('groups matches by section', () => {
    const hits = searchSettings('a', INDEX)
    expect([...hits.keys()].sort()).toEqual(['interface', 'mcp'])
    expect(hits.get('mcp')).toEqual(new Set(['mcp-token', 'mcp-address']))
  })
})

describe('the settings index', () => {
  it('finds the MCP access-token row by its English and Russian names', () => {
    expect(searchSettings('token', SETTINGS_SEARCH_INDEX).get('mcp')).toContain('mcp-token')
    expect(searchSettings('токен', SETTINGS_SEARCH_INDEX).get('mcp')).toContain('mcp-token')
  })

  it('finds the theme row', () => {
    expect(searchSettings('  THEME ', SETTINGS_SEARCH_INDEX).get('interface')).toContain('theme')
    expect(searchSettings('тема', SETTINGS_SEARCH_INDEX).get('interface')).toContain('theme')
  })

  it('finds a row by a Russian word while its English text does not contain it', () => {
    const hits = searchSettings('перенос', SETTINGS_SEARCH_INDEX)
    expect(hits.get('editor')).toContain('word-wrap')
  })

  it('finds a row by a synonym that is not in its label', () => {
    expect(searchSettings('port', SETTINGS_SEARCH_INDEX).get('mcp')).toContain('mcp-address')
    expect(searchSettings('кабинет', SETTINGS_SEARCH_INDEX).get('publishing')).toContain('publishing')
  })

  it('gives every row a unique id', () => {
    const ids = SETTINGS_SEARCH_INDEX.map((r) => r.id)
    expect(new Set(ids).size).toBe(ids.length)
  })

  it('does not lead to the automatic-download row on Linux, where it is not shown', () => {
    expect(searchSettings('background', settingsSearchIndex('darwin')).get('updates')).toContain('updates-download')
    expect(searchSettings('фон', settingsSearchIndex('windows')).get('updates')).toContain('updates-download')
    expect(searchSettings('background', settingsSearchIndex('linux')).size).toBe(0)
    expect(searchSettings('фон', settingsSearchIndex('linux')).size).toBe(0)
  })
})
