import { Annotation, Transaction } from '@codemirror/state'
import type { EditorState, TransactionSpec } from '@codemirror/state'

// Marks a document replacement that came from props, so the editor neither echoes nor undoes it.
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
  // A range already ending on a line boundary is whole; growing it would eat the next line's undo.
  const endLine = state.doc.lineAt(to)
  const lineTo = to === endLine.from ? to : endLine.to
  // No explicit selection: the caret is mapped through the change so it moves with the text.
  return {
    changes: { from: lineFrom, to: lineTo, insert: next.slice(lineFrom, nextTo + lineTo - to) },
    annotations: [External.of(true), Transaction.addToHistory.of(false)],
  }
}
