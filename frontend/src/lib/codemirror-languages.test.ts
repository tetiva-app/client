import { describe, expect, it, vi } from 'vitest'
import { EditorState, type Extension } from '@codemirror/state'
import { language as languageFacet } from '@codemirror/language'

const loaded = vi.hoisted(() => [] as string[])

vi.mock('@codemirror/legacy-modes/mode/go', async (original) => {
  loaded.push('go')
  return original()
})
vi.mock('@codemirror/legacy-modes/mode/clike', async (original) => {
  loaded.push('clike')
  return original()
})

function languageName(ext: Extension) {
  return EditorState.create({ extensions: ext }).facet(languageFacet)?.name
}

describe('languageSupport', () => {
  it('loads a legacy mode only when a viewer asks for it', async () => {
    const { languageSupport } = await import('./codemirror-languages')
    expect(loaded).toEqual([])

    const go = languageSupport('go')

    expect(go).toBeInstanceOf(Promise)
    expect(languageName(await go)).toBe('go')
    expect(loaded).toEqual(['go'])
  })

  it('gives Java and C# their clike dialects', async () => {
    const { languageSupport } = await import('./codemirror-languages')

    expect(languageName(await languageSupport('java'))).toBe('java')
    expect(languageName(await languageSupport('csharp'))).toBe('csharp')
  })

  it('keeps the response languages synchronous', async () => {
    const { languageSupport } = await import('./codemirror-languages')

    for (const lang of ['json', 'xml', 'html', 'javascript'] as const) {
      expect(languageSupport(lang), lang).not.toBeInstanceOf(Promise)
    }
    expect(languageName(languageSupport('json') as Extension)).toBe('json')
  })

  it('leaves PHP, plain text and no language unhighlighted', async () => {
    const { languageSupport } = await import('./codemirror-languages')

    for (const lang of ['php', 'text', undefined] as const) {
      expect(languageSupport(lang), String(lang)).toEqual([])
    }
  })
})
