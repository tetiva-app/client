import { EditorView } from '@codemirror/view'
import type { EditorState, Extension, SelectionRange, StateEffect, Text, TransactionSpec } from '@codemirror/state'
import {
  SearchQuery,
  findNext,
  findPrevious,
  getSearchQuery,
  openSearchPanel,
  search,
  searchPanelOpen,
  setSearchQuery,
} from '@codemirror/search'

const MAX_COUNTED = 1000

export interface FindStatus {
  current: number
  total: number
  more: boolean
}

interface Matches {
  query: SearchQuery
  doc: Text
  ranges: { from: number; to: number }[]
  more: boolean
}

// CodeMirror highlights matches only with its search panel open; this one stays empty.
export function findInDocument(report: (status: FindStatus | null) => void): Extension {
  const count = createFindCounter()
  let last: FindStatus | null = null
  return [
    search({ top: true, createPanel: () => ({ dom: document.createElement('span') }) }),
    EditorView.updateListener.of((update) => {
      const queryChanged = getSearchQuery(update.state) !== getSearchQuery(update.startState)
      if (!queryChanged && !update.docChanged && !update.selectionSet) return
      const next = count(update.state)
      if (next?.current === last?.current && next?.total === last?.total && next?.more === last?.more) return
      last = next
      report(next)
    }),
  ]
}

export function findStatusText(status: FindStatus | null): string {
  if (!status) return ''
  if (status.total === 0) return 'No matches'
  return `${status.current || '?'}/${status.total}${status.more ? '+' : ''}`
}

export function createFindCounter(): (state: EditorState) => FindStatus | null {
  let cached: Matches | null = null
  return (state) => {
    const query = getSearchQuery(state)
    if (!query.valid) {
      cached = null
      return null
    }
    if (!cached || cached.query !== query || cached.doc !== state.doc) cached = scan(state, query)
    return { current: indexOf(cached, state.selection.main), total: cached.ranges.length, more: cached.more }
  }
}

function scan(state: EditorState, query: SearchQuery): Matches {
  const ranges: Matches['ranges'] = []
  const cursor = query.getCursor(state)
  for (let match = cursor.next(); !match.done; match = cursor.next()) {
    if (ranges.length === MAX_COUNTED) return { query, doc: state.doc, ranges, more: true }
    ranges.push(match.value)
  }
  return { query, doc: state.doc, ranges, more: false }
}

function indexOf(matches: Matches, selection: SelectionRange): number {
  let lo = 0
  let hi = matches.ranges.length - 1
  while (lo <= hi) {
    const mid = (lo + hi) >> 1
    const range = matches.ranges[mid]
    if (range.from < selection.from) lo = mid + 1
    else if (range.from > selection.from) hi = mid - 1
    else return range.to === selection.to ? mid + 1 : 0
  }
  return 0
}

export function findQuery(state: EditorState, text: string): TransactionSpec {
  const query = new SearchQuery({ search: text, caseSensitive: false, literal: true })
  const effects: StateEffect<unknown>[] = [setSearchQuery.of(query)]
  const match = query.valid ? matchFrom(state, query, state.selection.main.from) : null
  if (!match) return { effects }
  return {
    effects: [...effects, EditorView.scrollIntoView(match.from, { y: 'center' })],
    selection: { anchor: match.from, head: match.to },
  }
}

function matchFrom(state: EditorState, query: SearchQuery, from: number): { from: number; to: number } | null {
  const after = query.getCursor(state, from).next()
  if (!after.done) return after.value
  const first = query.getCursor(state).next()
  return first.done ? null : first.value
}

export function applyFind(view: EditorView, text: string): void {
  if (text && !searchPanelOpen(view.state)) openSearchPanel(view)
  view.dispatch(findQuery(view.state, text))
}

export function stepFind(view: EditorView, forward: boolean): void {
  if (!getSearchQuery(view.state).valid) return
  if (forward) findNext(view)
  else findPrevious(view)
}
