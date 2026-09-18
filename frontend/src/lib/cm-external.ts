import { Annotation, Transaction } from '@codemirror/state'
import type { EditorState, TransactionSpec } from '@codemirror/state'

export const External = Annotation.define<boolean>()

export function isExternal(tr: Transaction): boolean {
  return tr.annotation(External) === true
}

// Only the differing whole lines are rewritten: a range ending mid-line clips an undo event in half.
export function externalReplace(state: EditorState, next: string): TransactionSpec | null {
  const current = state.doc.toString()
  if (current === next) return null
  let from = 0
  while (from < current.length && from < next.length && current[from] === next[from]) from += 1
  let to = current.length
  let nextTo = next.length
  while (to > from && nextTo > from && current[to - 1] === next[nextTo - 1]) { to -= 1; nextTo -= 1 }
  const lineFrom = state.doc.lineAt(from).from
  const endLine = state.doc.lineAt(to)
  const lineTo = to === endLine.from ? to : endLine.to
  return {
    changes: { from: lineFrom, to: lineTo, insert: next.slice(lineFrom, nextTo + lineTo - to) },
    annotations: [External.of(true), Transaction.addToHistory.of(false)],
  }
}
