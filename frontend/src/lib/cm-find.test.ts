import { describe, it, expect, vi } from 'vitest'
import { EditorState } from '@codemirror/state'
import { search, SearchQuery } from '@codemirror/search'
import { createFindCounter, findQuery, findStatusText } from './cm-find'

function doc(text: string, anchor = 0): EditorState {
  return EditorState.create({ doc: text, selection: { anchor }, extensions: [search()] })
}

function query(state: EditorState, text: string): EditorState {
  return state.update(findQuery(state, text)).state
}

function findStatus(state: EditorState) {
  return createFindCounter()(state)
}

describe('findQuery', () => {
  it('selects the first match at or after the cursor', () => {
    const state = query(doc('ann bob ann bob', 4), 'bob')
    expect(state.selection.main).toMatchObject({ from: 4, to: 7 })
    expect(findStatus(state)).toEqual({ current: 1, total: 2, more: false })
  })

  it('wraps to the first match when none follows the cursor', () => {
    const state = query(doc('ann bob ann', 11), 'ANN')
    expect(state.selection.main).toMatchObject({ from: 0, to: 3 })
  })

  it('keeps the current match while the query grows', () => {
    let state = query(doc('ab abc abcd', 4), 'ab')
    expect(state.selection.main.from).toBe(7)
    state = query(state, 'abc')
    expect(state.selection.main.from).toBe(7)
  })

  it('leaves the selection alone without a match', () => {
    const state = query(doc('ann bob', 2), 'zzz')
    expect(state.selection.main.head).toBe(2)
    expect(findStatus(state)).toEqual({ current: 0, total: 0, more: false })
  })
})

describe('findStatus', () => {
  it('is null without a query', () => {
    expect(findStatus(doc('ann'))).toBeNull()
    expect(findStatus(query(doc('ann'), ''))).toBeNull()
  })

  it('counts every match and has no current one off a match', () => {
    const state = query(doc('a a a'), 'a')
    const moved = state.update({ selection: { anchor: 1 } }).state
    expect(findStatus(moved)).toEqual({ current: 0, total: 3, more: false })
  })
})

describe('createFindCounter', () => {
  it('does not rescan the document when only the selection moves', () => {
    const count = createFindCounter()
    const state = query(doc('a b a b a'), 'a')
    const scans = vi.spyOn(SearchQuery.prototype, 'getCursor')
    expect(count(state)).toEqual({ current: 1, total: 3, more: false })
    const scansForQuery = scans.mock.calls.length

    expect(count(state.update({ selection: { anchor: 4, head: 5 } }).state)).toEqual({ current: 2, total: 3, more: false })
    expect(count(state.update({ selection: { anchor: 1 } }).state)).toEqual({ current: 0, total: 3, more: false })
    expect(scans.mock.calls.length).toBe(scansForQuery)
    scans.mockRestore()
  })

  it('rescans after the document changes', () => {
    const count = createFindCounter()
    const state = query(doc('a b'), 'a')
    expect(count(state)?.total).toBe(1)
    expect(count(state.update({ changes: { from: 3, insert: ' a' } }).state)?.total).toBe(2)
  })

  it('stops counting at 1000 and says there are more', () => {
    expect(findStatus(query(doc('a '.repeat(1500)), 'a'))).toMatchObject({ total: 1000, more: true })
  })
})

describe('findStatusText', () => {
  it('reads n/m, ?/m off a match, + past the cap and No matches', () => {
    expect(findStatusText(null)).toBe('')
    expect(findStatusText({ current: 2, total: 5, more: false })).toBe('2/5')
    expect(findStatusText({ current: 0, total: 5, more: false })).toBe('?/5')
    expect(findStatusText({ current: 3, total: 1000, more: true })).toBe('3/1000+')
    expect(findStatusText({ current: 0, total: 0, more: false })).toBe('No matches')
  })
})
