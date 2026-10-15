import { EditorState } from '@codemirror/state'
import type { ChangeSpec } from '@codemirror/state'

export const singleLine = EditorState.transactionFilter.of(tr => {
  if (!tr.docChanged) return tr
  const breaks: ChangeSpec[] = []
  tr.changes.iterChanges((_fromA, _toA, fromB, _toB, inserted) => {
    if (inserted.lines === 1) return
    const lines = inserted.toJSON()
    let first = 0
    let last = lines.length - 1
    while (first < last && lines[first] === '') first++
    while (last > first && lines[last] === '') last--
    let pos = fromB
    for (let i = 0; i < lines.length - 1; i++) {
      pos += lines[i].length
      breaks.push({ from: pos, to: pos + 1, insert: i >= first && i < last ? ' ' : '' })
      pos += 1
    }
  })
  return breaks.length ? [tr, { changes: breaks, sequential: true }] : tr
})
