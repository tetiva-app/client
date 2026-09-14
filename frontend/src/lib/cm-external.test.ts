import { describe, it, expect } from 'vitest'
import { EditorState } from '@codemirror/state'
import type { TransactionSpec, Transaction } from '@codemirror/state'
import { history, undo, undoDepth } from '@codemirror/commands'
import { externalReplace, isExternal } from './cm-external'

function host(doc: string) {
  let state = EditorState.create({ doc, extensions: [history()] })
  const log: Transaction[] = []
  return {
    get state() { return state },
    dispatch(spec: TransactionSpec | Transaction) {
      const tr = 'state' in spec && 'changes' in spec ? spec as Transaction : state.update(spec as TransactionSpec)
      log.push(tr); state = tr.state
    },
    log,
  }
}

describe('externalReplace', () => {
  it('returns null when the document already matches', () => {
    expect(externalReplace(host('same').state, 'same')).toBeNull()
  })

  it('replaces the document outside undo history and marks the transaction', () => {
    const h = host('typed')
    h.dispatch({ changes: { from: 5, insert: '!' } })
    expect(undoDepth(h.state)).toBe(1)
    h.dispatch(externalReplace(h.state, 'from sync')!)
    expect(h.state.doc.toString()).toBe('from sync')
    expect(undoDepth(h.state)).toBe(0)
    expect(undo({ state: h.state, dispatch: tr => h.dispatch(tr) })).toBe(false)
    expect(h.state.doc.toString()).toBe('from sync')
    expect(isExternal(h.log[1])).toBe(true)
    expect(isExternal(h.log[0])).toBe(false)
  })

  it('undo after an external value stops at that value', () => {
    const h = host('')
    h.dispatch(externalReplace(h.state, 'from sync')!)
    h.dispatch({ changes: { from: 9, insert: ' and mine' } })
    undo({ state: h.state, dispatch: tr => h.dispatch(tr) })
    expect(h.state.doc.toString()).toBe('from sync')
    expect(undo({ state: h.state, dispatch: tr => h.dispatch(tr) })).toBe(false)
  })

  it('parks the caret at the start of a replacement that swallowed it', () => {
    const h = host('a long line of text')
    h.dispatch({ selection: { anchor: 15 } })
    h.dispatch(externalReplace(h.state, 'short')!)
    expect(h.state.selection.main.anchor).toBe(0)
  })

  it('maps the caret through an incoming edit above it', () => {
    const h = host('first\nsecond')
    h.dispatch({ selection: { anchor: 9 } })
    h.dispatch(externalReplace(h.state, 'first line is longer\nsecond')!)
    expect(h.state.doc.toString()).toBe('first line is longer\nsecond')
    expect(h.state.selection.main.anchor).toBe(24)
  })

  it('leaves the following line undoable when the change ends on a line boundary', () => {
    const h = host('keep\nremove\nother')
    h.dispatch({ changes: { from: 12, to: 13, insert: 'O' } })
    expect(undoDepth(h.state)).toBe(1)

    h.dispatch(externalReplace(h.state, 'keep\nOther')!)
    expect(h.state.doc.toString()).toBe('keep\nOther')
    expect(undoDepth(h.state)).toBe(1)

    undo({ state: h.state, dispatch: tr => h.dispatch(tr) })
    expect(h.state.doc.toString()).toBe('keep\nother')
  })

  it('leaves an edit on another line undoable', () => {
    const h = host('line one\nline two')
    h.dispatch({ changes: { from: 0, insert: 'A' } })
    expect(undoDepth(h.state)).toBe(1)

    h.dispatch(externalReplace(h.state, 'Aline one\nline TWO')!)
    expect(h.state.doc.toString()).toBe('Aline one\nline TWO')
    expect(undoDepth(h.state)).toBe(1)

    undo({ state: h.state, dispatch: tr => h.dispatch(tr) })
    expect(h.state.doc.toString()).toBe('line one\nline TWO')
  })

  it('rewrites only the lines that differ', () => {
    const h = host('keep\nchange me\nkeep')
    const spec = externalReplace(h.state, 'keep\nchanged\nkeep')!
    expect(spec.changes).toEqual({ from: 5, to: 14, insert: 'changed' })
  })

  it('replaces a single-line document whole', () => {
    const h = host('local')
    const spec = externalReplace(h.state, 'first external')!
    expect(spec.changes).toEqual({ from: 0, to: 5, insert: 'first external' })
  })

  it('applies two consecutive external values', () => {
    const h = host('')
    h.dispatch(externalReplace(h.state, 'one')!)
    h.dispatch(externalReplace(h.state, 'two')!)
    expect(h.state.doc.toString()).toBe('two')
  })
})
