import { describe, it, expect } from 'vitest'
import { EditorState } from '@codemirror/state'
import { singleLine } from './cm-single-line'
import { externalReplace, isExternal } from './cm-external'

function field(doc: string, selection?: { anchor: number; head?: number }): EditorState {
  return EditorState.create({ doc, selection, extensions: [singleLine] })
}

function paste(state: EditorState, text: string) {
  return state.update(state.replaceSelection(text), { userEvent: 'input.paste' })
}

describe('singleLine', () => {
  it('drops line breaks around a paste and keeps the text beside it', () => {
    const state = field('ab', { anchor: 1 })
    const tr = paste(state, '\r\nX\n\n')
    expect(tr.newDoc.toString()).toBe('aXb')
    expect(tr.newSelection.main.head).toBe(2)
  })

  it('turns line breaks inside a paste into spaces', () => {
    const tr = paste(field(''), 'first\nsecond\r\nthird')
    expect(tr.newDoc.toString()).toBe('first second third')
  })

  it('keeps the annotations of the original transaction', () => {
    const state = field('a')
    const tr = state.update(externalReplace(state, 'b\nc')!)
    expect(tr.newDoc.toString()).toBe('b c')
    expect(isExternal(tr)).toBe(true)
  })

  it('leaves a change without line breaks untouched', () => {
    const state = field('abc', { anchor: 3 })
    const tr = paste(state, 'def')
    expect(tr.newDoc.toString()).toBe('abcdef')
  })
})
