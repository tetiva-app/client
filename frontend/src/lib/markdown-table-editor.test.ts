import { describe, it, expect } from 'vitest'
import { EditorState } from '@codemirror/state'
import type { TransactionSpec } from '@codemirror/state'
import { markdown, markdownLanguage } from '@codemirror/lang-markdown'
import { Point, Range } from '@susisu/mte-kernel'
import {
  CodeMirrorTextEditor, createTableEditor, isInsideCodeAtLine, isInsideCodeBlock,
  TABLE_OPTIONS, type EditorHost,
} from './markdown-table-editor'

export function makeHost(doc: string, cursor = 0): EditorHost & { dispatches: number } {
  let state = EditorState.create({
    doc,
    selection: { anchor: cursor },
    extensions: [markdown({ base: markdownLanguage })],
  })
  return {
    dispatches: 0,
    get state() { return state },
    dispatch(spec: TransactionSpec) {
      state = state.update(spec).state
      this.dispatches += 1
    },
  }
}

const TABLE = '| a | b |\n| - | - |\n| 1 | 2 |'

describe('CodeMirrorTextEditor', () => {
  it('maps rows and columns to the document', () => {
    const host = makeHost('x\n' + TABLE, 2 + '| a |'.length)
    const t = new CodeMirrorTextEditor(host)
    expect(t.getLastRow()).toBe(3)
    expect(t.getLine(1)).toBe('| a | b |')
    expect(t.getCursorPosition()).toEqual(new Point(1, 5))
  })

  it('replaces, inserts and deletes lines', () => {
    const host = makeHost('a\nb\nc')
    const t = new CodeMirrorTextEditor(host)
    t.replaceLines(0, 2, ['X', 'Y', 'Z'])
    expect(host.state.doc.toString()).toBe('X\nY\nZ\nc')
    t.insertLine(4, 'end')
    expect(host.state.doc.toString()).toBe('X\nY\nZ\nc\nend')
    t.deleteLine(0)
    expect(host.state.doc.toString()).toBe('Y\nZ\nc\nend')
  })

  it('batches a transact into one dispatch and keeps reads consistent inside it', () => {
    const host = makeHost('a\nb')
    const t = new CodeMirrorTextEditor(host)
    t.transact(() => {
      t.replaceLines(0, 1, ['A'])
      expect(t.getLine(0)).toBe('A')
      t.insertLine(2, 'c')
      t.setCursorPosition(new Point(2, 1))
    })
    expect(host.dispatches).toBe(1)
    expect(host.state.doc.toString()).toBe('A\nb\nc')
    expect(host.state.selection.main.head).toBe('A\nb\nc'.length)
  })

  it('nested transact still dispatches once', () => {
    const host = makeHost('a')
    const t = new CodeMirrorTextEditor(host)
    t.transact(() => { t.transact(() => { t.replaceLines(0, 1, ['b']) }); t.insertLine(1, 'c') })
    expect(host.dispatches).toBe(1)
    expect(host.state.doc.toString()).toBe('b\nc')
  })

  it('sets a selection range with the cursor at its end', () => {
    const host = makeHost('hello world')
    const t = new CodeMirrorTextEditor(host)
    t.setSelectionRange(new Range(new Point(0, 6), new Point(0, 11)))
    expect(host.state.selection.main.from).toBe(6)
    expect(host.state.selection.main.head).toBe(11)
  })

  it('refuses table edits inside fenced code', () => {
    const doc = '```\n| a | b |\n```\n| c | d |'
    const host = makeHost(doc)
    const t = new CodeMirrorTextEditor(host)
    expect(t.acceptsTableEdit(1)).toBe(false)
    expect(t.acceptsTableEdit(3)).toBe(true)
    expect(isInsideCodeBlock(host.state, doc.indexOf('| a'))).toBe(true)
    expect(isInsideCodeBlock(host.state, doc.indexOf('| c'))).toBe(false)
  })

  it('refuses table edits inside an indented code block', () => {
    const doc = 'text\n\n    | a | b |\n\n| c | d |'
    const host = makeHost(doc)
    const t = new CodeMirrorTextEditor(host)
    expect(t.acceptsTableEdit(2)).toBe(false)
    expect(t.acceptsTableEdit(4)).toBe(true)
  })
})

describe('isInsideCodeAtLine', () => {
  it('holds on the empty line of a fence the parser has not closed yet', () => {
    const open = makeHost('intro\n\n```\n')
    expect(isInsideCodeAtLine(open.state, open.state.doc.length)).toBe(true)
    const tagged = makeHost('```js\n')
    expect(isInsideCodeAtLine(tagged.state, tagged.state.doc.length)).toBe(true)
    const withCode = makeHost('```\ncode\n')
    expect(isInsideCodeAtLine(withCode.state, withCode.state.doc.length)).toBe(true)
    const tilde = makeHost('~~~\n')
    expect(isInsideCodeAtLine(tilde.state, tilde.state.doc.length)).toBe(true)
  })

  it('lets a line outside a code block through, including after a closed fence', () => {
    const closed = makeHost('```\ncode\n```\n')
    expect(isInsideCodeAtLine(closed.state, closed.state.doc.length)).toBe(false)
    const plain = makeHost('intro\n\n')
    expect(isInsideCodeAtLine(plain.state, plain.state.doc.length)).toBe(false)
    // A closing fence cannot carry an info string, so this one still opens a block.
    const reopened = makeHost('```\ncode\n```js\n')
    expect(isInsideCodeAtLine(reopened.state, reopened.state.doc.length)).toBe(true)
  })

  it('probes the line indent, so an indented code block counts', () => {
    const doc = 'text\n\n    | a | b |'
    const host = makeHost(doc)
    const lineStart = doc.lastIndexOf('\n') + 1
    expect(isInsideCodeAtLine(host.state, lineStart)).toBe(true)
    expect(isInsideCodeBlock(host.state, lineStart)).toBe(false)
  })
})

describe('createTableEditor', () => {
  it('detects the cursor inside a table', () => {
    const host = makeHost('text\n' + TABLE, 'text\n| a'.length)
    const { editor } = createTableEditor(host)
    expect(editor.cursorIsInTable(TABLE_OPTIONS)).toBe(true)
  })

  it('does not detect a table on a plain line', () => {
    const host = makeHost('text\n' + TABLE, 1)
    const { editor } = createTableEditor(host)
    expect(editor.cursorIsInTable(TABLE_OPTIONS)).toBe(false)
  })
})

import { tableTab, tableShiftTab, tableEnter, inTable, tableContext, isEmptyTableRow } from './markdown-table-editor'

describe('tableContext', () => {
  it('locates the table and the focused cell, counting cells like the kernel', () => {
    const doc = 'intro\n| a | b | c |\n| - | - | - |\n| 1 | `x|y` | 3 |'
    const ctx = tableContext(makeHost(doc, doc.indexOf('`x')))!
    expect(ctx.startRow).toBe(1)
    expect(ctx.endRow).toBe(3)
    expect(ctx.width).toBe(3)
    expect(ctx.focus.row).toBe(2)
    expect(ctx.focus.column).toBe(1)
    expect(tableContext(makeHost(doc, 2))).toBeNull()
  })
  it('uses the header width for ragged rows', () => {
    const doc = '| a | b | c |\n| - | - | - |\n| 1 | 2 |'
    const ctx = tableContext(makeHost(doc, doc.lastIndexOf('2')))!
    expect(ctx.width).toBe(3)
    expect(ctx.focus.column).toBe(1)
  })
  it('works on the completed table: missing delimiter and a body wider than the header', () => {
    const noDelim = '| a | b |\n| 1 | 2 |'
    const ctx1 = tableContext(makeHost(noDelim, noDelim.indexOf('1')))!
    expect(ctx1.focus.row).toBe(2)
    expect(ctx1.table.getHeight()).toBe(3)
    const wideBody = '| a |\n| - |\n| 1 | 2 |'
    const ctx2 = tableContext(makeHost(wideBody, wideBody.indexOf('1')))!
    expect(ctx2.width).toBe(2)
    expect(ctx2.focus.column).toBe(0)
  })
  it('recognises an empty row', () => {
    expect(isEmptyTableRow('|   |  |')).toBe(true)
    expect(isEmptyTableRow('| a |  |')).toBe(false)
  })
})

// Expected strings below come from mte-kernel 2.1.1 with TABLE_OPTIONS: columns are
// at least three characters wide, cells padded with one space on each side.
describe('table keymap', () => {
  it('Tab formats and moves to the next cell', () => {
    const host = makeHost('| a | bb |\n| - | - |\n| 1 | 2 |', 2)
    expect(tableTab(host)).toBe(true)
    expect(host.state.doc.toString()).toBe('| a   | bb  |\n| --- | --- |\n| 1   | 2   |')
    // the next cell's content is selected
    expect(host.state.sliceDoc(host.state.selection.main.from, host.state.selection.main.to)).toBe('bb')
    expect(host.dispatches).toBe(1)
  })

  it('Tab in the last cell goes to the first cell of the next row', () => {
    const host = makeHost('| a | b |\n| - | - |\n| 1 | 2 |', 6)
    tableTab(host)
    expect(host.state.doc.toString()).toBe('| a   | b   |\n| --- | --- |\n| 1   | 2   |')
    const sel = host.state.selection.main
    expect(host.state.doc.lineAt(sel.head).number).toBe(3)
    expect(host.state.sliceDoc(sel.from, sel.to)).toBe('1')
  })

  it('Tab in the last cell of the last row appends a row', () => {
    const doc = '| a | b |\n| - | - |\n| 1 | 2 |'
    const host = makeHost(doc, doc.length - 2)
    tableTab(host)
    expect(host.state.doc.toString()).toBe('| a   | b   |\n| --- | --- |\n| 1   | 2   |\n|     |     |')
    expect(host.state.doc.lineAt(host.state.selection.main.head).number).toBe(4)
  })

  it('Tab on a ragged row fills the missing cell instead of jumping to the next row', () => {
    const doc = '| a | b | c |\n| - | - | - |\n| 1 | 2 |'
    const host = makeHost(doc, doc.lastIndexOf('2'))
    tableTab(host)
    expect(host.state.doc.toString()).toBe('| a   | b   | c   |\n| --- | --- | --- |\n| 1   | 2   |     |')
    expect(host.state.doc.lineAt(host.state.selection.main.head).number).toBe(3)
  })

  it('Tab on a body row wider than the header widens the header and moves right', () => {
    const doc = '| a |\n| - |\n| 1 | 2 |'
    const host = makeHost(doc, doc.indexOf('1'))
    tableTab(host)
    expect(host.state.doc.toString()).toBe('| a   |     |\n| --- | --- |\n| 1   | 2   |')
    expect(host.state.sliceDoc(host.state.selection.main.from, host.state.selection.main.to)).toBe('2')
  })

  it('Enter in a table without a delimiter row completes it and appends a row', () => {
    const doc = '| a | b |\n| 1 | 2 |'
    const host = makeHost(doc, doc.indexOf('1'))
    tableEnter(host)
    expect(host.state.doc.toString()).toBe('| a   | b   |\n| --- | --- |\n| 1   | 2   |\n|     |     |')
  })

  it('Shift-Tab goes back, wrapping to the previous row', () => {
    const doc = '| a | b |\n| - | - |\n| 1 | 2 |'
    const host = makeHost(doc, doc.indexOf('1'))
    tableShiftTab(host)
    expect(host.state.sliceDoc(host.state.selection.main.from, host.state.selection.main.to)).toBe('b')
  })

  it('Enter moves to the next row, appending at the end', () => {
    const doc = '| a | b |\n| - | - |\n| 1 | 2 |'
    const host = makeHost(doc, doc.indexOf('2'))
    tableEnter(host)
    expect(host.state.doc.toString()).toBe('| a   | b   |\n| --- | --- |\n| 1   | 2   |\n|     |     |')
  })

  it('Enter from the header skips the delimiter row', () => {
    const host = makeHost('| a | b |\n| - | - |\n| 1 | 2 |', 2)
    tableEnter(host)
    expect(host.state.sliceDoc(host.state.selection.main.from, host.state.selection.main.to)).toBe('1')
  })

  it('Enter on an empty last row removes it and leaves the table', () => {
    const doc = '| a | b |\n| - | - |\n| 1 | 2 |\n|   |   |'
    const host = makeHost(doc, doc.length - 3)
    tableEnter(host)
    expect(host.state.doc.toString()).toBe('| a   | b   |\n| --- | --- |\n| 1   | 2   |\n')
    expect(host.state.selection.main.head).toBe(host.state.doc.length)
    expect(host.dispatches).toBe(1)
  })

  it('falls through outside tables and inside fenced code', () => {
    const plain = makeHost('hello', 2)
    expect(tableTab(plain)).toBe(false)
    expect(tableEnter(plain)).toBe(false)
    expect(plain.dispatches).toBe(0)
    const fenced = makeHost('```\n| a | b |\n```', 6)
    expect(inTable(fenced)).toBe(false)
    expect(tableTab(fenced)).toBe(false)
  })
})

import { disabledTableCommands, runTableCommand, TABLE_COMMANDS } from './markdown-table-editor'

const T = '| a | b |\n| - | - |\n| 1 | 2 |\n| 3 | 4 |'
function at(needle: string) { return T.indexOf(needle) }

describe('runTableCommand', () => {
  it('lists thirteen commands in menu order', () => {
    expect(TABLE_COMMANDS.map(c => c.command)).toEqual([
      'insertRowAbove', 'insertRowBelow', 'insertColumnLeft', 'insertColumnRight',
      'deleteRow', 'deleteColumn', 'alignLeft', 'alignCenter', 'alignRight',
      'moveRowUp', 'moveRowDown', 'moveColumnLeft', 'moveColumnRight',
    ])
  })

  // Expected strings generated by mte-kernel 2.1.1 with TABLE_OPTIONS.
  it('inserts rows above and below', () => {
    const above = makeHost(T, at('3')); runTableCommand(above, 'insertRowAbove')
    expect(above.state.doc.toString()).toBe('| a   | b   |\n| --- | --- |\n| 1   | 2   |\n|     |     |\n| 3   | 4   |')
    const below = makeHost(T, at('1')); runTableCommand(below, 'insertRowBelow')
    expect(below.state.doc.toString()).toBe('| a   | b   |\n| --- | --- |\n| 1   | 2   |\n|     |     |\n| 3   | 4   |')
    expect(below.dispatches).toBe(1)
    const last = makeHost(T, at('3')); runTableCommand(last, 'insertRowBelow')
    expect(last.state.doc.toString()).toBe('| a   | b   |\n| --- | --- |\n| 1   | 2   |\n| 3   | 4   |\n|     |     |')
    const header = makeHost(T, at('a')); runTableCommand(header, 'insertRowBelow')
    expect(header.state.doc.toString()).toBe('| a   | b   |\n| --- | --- |\n|     |     |\n| 1   | 2   |\n| 3   | 4   |')
  })

  it('inserts columns left and right, including after the last column', () => {
    const left = makeHost(T, at('b')); runTableCommand(left, 'insertColumnLeft')
    expect(left.state.doc.toString()).toBe('| a   |     | b   |\n| --- | --- | --- |\n| 1   |     | 2   |\n| 3   |     | 4   |')
    const right = makeHost(T, at('b')); runTableCommand(right, 'insertColumnRight')
    expect(right.state.doc.toString()).toBe('| a   | b   |     |\n| --- | --- | --- |\n| 1   | 2   |     |\n| 3   | 4   |     |')
    expect(right.dispatches).toBe(1)
    const middle = makeHost(T, at('a')); runTableCommand(middle, 'insertColumnRight')
    expect(middle.state.doc.toString()).toBe('| a   |     | b   |\n| --- | --- | --- |\n| 1   |     | 2   |\n| 3   |     | 4   |')
  })

  it('deletes the current row and column', () => {
    const row = makeHost(T, at('1')); runTableCommand(row, 'deleteRow')
    expect(row.state.doc.toString()).toBe('| a   | b   |\n| --- | --- |\n| 3   | 4   |')
    const col = makeHost(T, at('b')); runTableCommand(col, 'deleteColumn')
    expect(col.state.doc.toString()).toBe('| a   |\n| --- |\n| 1   |\n| 3   |')
  })

  it('deleting the only body row leaves the header and the delimiter', () => {
    const one = '| a | b |\n| - | - |\n| 1 | 2 |'
    const host = makeHost(one, one.indexOf('1'))
    expect(runTableCommand(host, 'deleteRow')).toBe(true)
    expect(host.state.doc.toString()).toBe('| a   | b   |\n| --- | --- |')
  })

  it('deleting the only column deletes the table and keeps the text around it', () => {
    const doc = 'before\n\n| a |\n| - |\n| 1 |\n\nafter'
    const host = makeHost(doc, doc.indexOf('1'))
    expect(runTableCommand(host, 'deleteColumn')).toBe(true)
    expect(host.state.doc.toString()).toBe('before\n\nafter')
    expect(host.state.doc.lineAt(host.state.selection.main.head).number).toBe(2)
    expect(host.dispatches).toBe(1)
  })

  it('deletes a one-column table sitting at either end of the document', () => {
    const only = makeHost('| a |\n| - |', 2)
    runTableCommand(only, 'deleteColumn')
    expect(only.state.doc.toString()).toBe('')
    const trailing = makeHost('intro\n\n| a |\n| - |', 9)
    runTableCommand(trailing, 'deleteColumn')
    expect(trailing.state.doc.toString()).toBe('intro\n')
    const leading = makeHost('| a |\n| - |\nafter', 2)
    runTableCommand(leading, 'deleteColumn')
    expect(leading.state.doc.toString()).toBe('after')
  })

  it('aligns a column', () => {
    const host = makeHost(T, at('b')); runTableCommand(host, 'alignCenter')
    expect(host.state.doc.toString()).toBe('| a   |  b  |\n| --- |:---:|\n| 1   |  2  |\n| 3   |  4  |')
  })

  it('moves rows and columns', () => {
    const rowDown = makeHost(T, at('1')); runTableCommand(rowDown, 'moveRowDown')
    expect(rowDown.state.doc.toString()).toBe('| a   | b   |\n| --- | --- |\n| 3   | 4   |\n| 1   | 2   |')
    const colLeft = makeHost(T, at('b')); runTableCommand(colLeft, 'moveColumnLeft')
    expect(colLeft.state.doc.toString()).toBe('| b   | a   |\n| --- | --- |\n| 2   | 1   |\n| 4   | 3   |')
  })

  it('does nothing outside a table', () => {
    const host = makeHost('plain', 1)
    expect(runTableCommand(host, 'deleteRow')).toBe(false)
    expect(host.dispatches).toBe(0)
  })

  it('refuses to delete or move the header and delimiter rows', () => {
    const del = makeHost(T, at('a'))
    expect(runTableCommand(del, 'deleteRow')).toBe(false)
    expect(del.state.doc.toString()).toBe(T)
    const move = makeHost(T, at('a'))
    expect(runTableCommand(move, 'moveRowDown')).toBe(false)
    expect(move.dispatches).toBe(0)
    const delim = makeHost(T, at('-'))
    expect(runTableCommand(delim, 'deleteRow')).toBe(false)
    expect(delim.dispatches).toBe(0)
  })

  it('reports the refused commands so the menu can grey them out', () => {
    expect(disabledTableCommands(makeHost(T, at('a')))).toEqual(['deleteRow', 'moveRowUp', 'moveRowDown'])
    expect(disabledTableCommands(makeHost(T, at('-')))).toEqual(['deleteRow', 'moveRowUp', 'moveRowDown'])
    expect(disabledTableCommands(makeHost(T, at('1')))).toEqual([])
    expect(disabledTableCommands(makeHost('plain', 1))).toEqual([])
  })

  it('inserts below the delimiter row as the first body row', () => {
    const host = makeHost(T, at('-')); runTableCommand(host, 'insertRowBelow')
    expect(host.state.doc.toString()).toBe('| a   | b   |\n| --- | --- |\n|     |     |\n| 1   | 2   |\n| 3   | 4   |')
  })
})
