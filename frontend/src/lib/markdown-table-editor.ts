// Maps @susisu/mte-kernel's line operations onto CodeMirror transactions. The host is
// structural (`EditorView` satisfies it), so tests need no DOM.

import { ChangeSet } from '@codemirror/state'
import type { EditorState, Transaction, TransactionSpec } from '@codemirror/state'
import { syntaxTree } from '@codemirror/language'
import {
  Alignment,
  DefaultAlignment,
  FormatType,
  HeaderAlignment,
  ITextEditor,
  Point,
  TableEditor,
  completeTable,
  options,
  readTable,
} from '@susisu/mte-kernel'
import type { Focus, Options, Range, Table } from '@susisu/mte-kernel'

export interface EditorHost {
  readonly state: EditorState
  dispatch(spec: TransactionSpec): void
}

export const TABLE_OPTIONS: Options = options({
  formatType: FormatType.NORMAL,
  minDelimiterWidth: 3,
  defaultAlignment: DefaultAlignment.LEFT,
  headerAlignment: HeaderAlignment.FOLLOW,
  smartCursor: false,
})

const CODE_BLOCK_NODES = new Set(['FencedCode', 'CodeBlock', 'HTMLBlock'])

export function isInsideCodeBlock(state: EditorState, pos: number): boolean {
  let node = syntaxTree(state).resolveInner(pos, 1)
  for (;;) {
    if (CODE_BLOCK_NODES.has(node.name)) return true
    const parent = node.parent
    if (parent === null) return false
    node = parent
  }
}

const FENCE_MARK = /^ {0,3}(`{3,}|~{3,})(.*)$/

function insideUnclosedFence(state: EditorState, pos: number): boolean {
  const cursorLine = state.doc.lineAt(pos).number
  let open: string | null = null
  for (let n = 1; n <= cursorLine; n++) {
    const match = FENCE_MARK.exec(state.doc.line(n).text)
    if (match === null) continue
    if (open === null) open = match[1]
    else if (match[1][0] === open[0] && match[1].length >= open.length && match[2].trim() === '') open = null
  }
  return open !== null
}

// Guard for whole-line inserts: probes the line's first real character (an indented
// code block starts after its indent) and covers a fence the parser has not closed
// yet, where the tree stops before the trailing newline the caret sits on.
export function isInsideCodeAtLine(state: EditorState, pos: number): boolean {
  const line = state.doc.lineAt(pos)
  const indent = line.text.length - line.text.trimStart().length
  return isInsideCodeBlock(state, line.from + indent) || insideUnclosedFence(state, pos)
}

export class CodeMirrorTextEditor extends ITextEditor {
  private readonly host: EditorHost
  private depth = 0
  private working: EditorState | null = null
  private pending: Transaction[] = []

  constructor(host: EditorHost) {
    super()
    this.host = host
  }

  private get state(): EditorState {
    return this.working ?? this.host.state
  }

  private apply(spec: TransactionSpec): void {
    const tagged = { ...spec, userEvent: 'input.table', scrollIntoView: true }
    if (this.depth === 0) {
      this.host.dispatch(tagged)
      return
    }
    const tr = this.state.update(tagged)
    this.working = tr.state
    this.pending.push(tr)
  }

  private posOf(point: Point): number {
    const line = this.state.doc.line(point.row + 1)
    return Math.min(line.from + point.column, line.to)
  }

  getCursorPosition(): Point {
    const head = this.state.selection.main.head
    const line = this.state.doc.lineAt(head)
    return new Point(line.number - 1, head - line.from)
  }

  setCursorPosition(pos: Point): void {
    this.apply({ selection: { anchor: this.posOf(pos) } })
  }

  setSelectionRange(range: Range): void {
    this.apply({ selection: { anchor: this.posOf(range.start), head: this.posOf(range.end) } })
  }

  getLastRow(): number {
    return this.state.doc.lines - 1
  }

  acceptsTableEdit(row: number): boolean {
    if (row < 0 || row > this.getLastRow()) return false
    const line = this.state.doc.line(row + 1)
    // An indented code block starts after its indent, so probe the first real character.
    const indent = line.text.length - line.text.trimStart().length
    return !isInsideCodeBlock(this.state, line.from + indent)
  }

  getLine(row: number): string {
    return this.state.doc.line(row + 1).text
  }

  insertLine(row: number, line: string): void {
    const doc = this.state.doc
    if (row > this.getLastRow()) {
      this.apply({ changes: { from: doc.length, insert: '\n' + line } })
      return
    }
    this.apply({ changes: { from: doc.line(row + 1).from, insert: line + '\n' } })
  }

  deleteLine(row: number): void {
    const line = this.state.doc.line(row + 1)
    const isLast = row === this.getLastRow()
    this.apply({
      changes: {
        from: isLast && row > 0 ? line.from - 1 : line.from,
        to: isLast ? line.to : line.to + 1,
      },
    })
  }

  replaceLines(startRow: number, endRow: number, lines: string[]): void {
    const doc = this.state.doc
    const from = doc.line(startRow + 1).from
    const to = doc.line(Math.min(endRow - 1, this.getLastRow()) + 1).to
    this.apply({ changes: { from, to, insert: lines.join('\n') } })
  }

  transact(func: () => void): void {
    if (this.depth > 0) {
      this.depth += 1
      try {
        func()
      } finally {
        this.depth -= 1
      }
      return
    }
    this.depth = 1
    this.working = this.host.state
    this.pending = []
    try {
      func()
    } finally {
      const { pending, working } = this
      this.depth = 0
      this.working = null
      this.pending = []
      if (pending.length > 0 && working !== null) {
        const changes = pending.reduce(
          (acc, tr) => acc.compose(tr.changes),
          ChangeSet.empty(this.host.state.doc.length),
        )
        this.host.dispatch({
          changes,
          selection: working.selection,
          userEvent: 'input.table',
          scrollIntoView: true,
        })
      }
    }
  }
}

export interface TableEditorHandle {
  editor: TableEditor
  text: CodeMirrorTextEditor
}

export function createTableEditor(host: EditorHost): TableEditorHandle {
  const text = new CodeMirrorTextEditor(host)
  return { editor: new TableEditor(text), text }
}

export type HostCommand = (host: EditorHost) => boolean

// Same shape the kernel accepts as a table row: optional margin, then a pipe.
const TABLE_ROW = /^\s*\|/

export function isTableLine(state: EditorState, row: number): boolean {
  if (row < 0 || row >= state.doc.lines) return false
  const line = state.doc.line(row + 1)
  if (!TABLE_ROW.test(line.text)) return false
  const indent = line.text.length - line.text.trimStart().length
  return !isInsideCodeBlock(state, line.from + indent)
}

export function inTable(host: EditorHost): boolean {
  const { doc, selection } = host.state
  return isTableLine(host.state, doc.lineAt(selection.main.head).number - 1)
}

export interface TableContext {
  startRow: number
  endRow: number
  table: Table
  width: number
  focus: Focus
}

// Parsed by the kernel so that `\|` and pipes inside code spans count as content,
// and completed so that ragged rows and a missing delimiter row do not skew the focus.
export function tableContext(host: EditorHost): TableContext | null {
  const state = host.state
  const head = state.selection.main.head
  const cursorLine = state.doc.lineAt(head)
  const cursorRow = cursorLine.number - 1
  if (!isTableLine(state, cursorRow)) return null

  let startRow = cursorRow
  while (isTableLine(state, startRow - 1)) startRow -= 1
  let endRow = cursorRow
  while (isTableLine(state, endRow + 1)) endRow += 1

  const lines: string[] = []
  for (let row = startRow; row <= endRow; row++) lines.push(state.doc.line(row + 1).text)
  const raw = readTable(lines, TABLE_OPTIONS)
  const rawFocus = raw.focusOfPosition(new Point(cursorRow, head - cursorLine.from), startRow)
  if (rawFocus === undefined) return null

  const { table, delimiterInserted } = completeTable(raw, TABLE_OPTIONS)
  const focus = delimiterInserted && rawFocus.row > 0 ? rawFocus.setRow(rawFocus.row + 1) : rawFocus
  return { startRow, endRow, table, width: table.getHeaderWidth(), focus }
}

export function isEmptyTableRow(line: string): boolean {
  const row = readTable([line], TABLE_OPTIONS).getRows()[0]
  return row.getCells().every((cell) => cell.content === '')
}

export const tableTab: HostCommand = (host) => {
  const ctx = tableContext(host)
  if (ctx === null) return false
  const { editor } = createTableEditor(host)
  // The header width counts the missing cells of a ragged row, so Tab fills them
  // instead of jumping ahead; past the last column it wraps to the next row.
  if (ctx.focus.column >= ctx.width - 1) editor.nextRow(TABLE_OPTIONS)
  else editor.nextCell(TABLE_OPTIONS)
  return true
}

export const tableShiftTab: HostCommand = (host) => {
  if (!inTable(host)) return false
  createTableEditor(host).editor.previousCell(TABLE_OPTIONS)
  return true
}

export const tableEnter: HostCommand = (host) => {
  const ctx = tableContext(host)
  if (ctx === null) return false
  const { editor, text } = createTableEditor(host)
  const line = host.state.doc.lineAt(host.state.selection.main.head)
  const onLastBodyRow = ctx.focus.row >= 2 && ctx.focus.row === ctx.table.getHeight() - 1
  if (onLastBodyRow && isEmptyTableRow(line.text)) {
    // Enter on an empty last row leaves the table, the way it ends a list.
    text.transact(() => {
      editor.deleteRow(TABLE_OPTIONS)
      editor.escape(TABLE_OPTIONS)
    })
  } else {
    editor.nextRow(TABLE_OPTIONS)
  }
  return true
}

export type TableCommand =
  | 'insertRowAbove' | 'insertRowBelow' | 'insertColumnLeft' | 'insertColumnRight'
  | 'deleteRow' | 'deleteColumn'
  | 'alignLeft' | 'alignCenter' | 'alignRight'
  | 'moveRowUp' | 'moveRowDown' | 'moveColumnLeft' | 'moveColumnRight'

export interface TableCommandItem {
  command: TableCommand
  label: string
  group: 'row' | 'column' | 'align' | 'move'
}

export const TABLE_COMMANDS: readonly TableCommandItem[] = [
  { command: 'insertRowAbove', label: 'Insert row above', group: 'row' },
  { command: 'insertRowBelow', label: 'Insert row below', group: 'row' },
  { command: 'insertColumnLeft', label: 'Insert column left', group: 'column' },
  { command: 'insertColumnRight', label: 'Insert column right', group: 'column' },
  { command: 'deleteRow', label: 'Delete row', group: 'row' },
  { command: 'deleteColumn', label: 'Delete column', group: 'column' },
  { command: 'alignLeft', label: 'Align left', group: 'align' },
  { command: 'alignCenter', label: 'Align center', group: 'align' },
  { command: 'alignRight', label: 'Align right', group: 'align' },
  { command: 'moveRowUp', label: 'Move row up', group: 'move' },
  { command: 'moveRowDown', label: 'Move row down', group: 'move' },
  { command: 'moveColumnLeft', label: 'Move column left', group: 'move' },
  { command: 'moveColumnRight', label: 'Move column right', group: 'move' },
]

// On the header row the kernel blanks the cells instead of removing the row, and on
// the delimiter row it only reformats, so these are refused outside the body.
const BODY_ONLY = new Set<TableCommand>(['deleteRow', 'moveRowUp', 'moveRowDown'])

// The toolbar greys these out on the header and delimiter rows instead of offering
// menu items that silently do nothing.
export function disabledTableCommands(host: EditorHost): TableCommand[] {
  const ctx = tableContext(host)
  if (ctx === null || ctx.focus.row >= 2) return []
  return [...BODY_ONLY]
}

// The kernel would only blank every cell, so deleting the last column deletes the
// table; blank lines on both sides of it would collapse into a double one.
function deleteTable(host: EditorHost, ctx: TableContext): boolean {
  const doc = host.state.doc
  let from = doc.line(ctx.startRow + 1).from
  let to = doc.line(ctx.endRow + 1).to
  const blankAbove = ctx.startRow > 0 && doc.line(ctx.startRow).text.trim() === ''
  const blankBelow = ctx.endRow + 2 <= doc.lines && doc.line(ctx.endRow + 2).text.trim() === ''
  if (to < doc.length) to += 1
  else if (from > 0) from -= 1
  if (blankAbove && blankBelow) from = doc.line(ctx.startRow).from
  host.dispatch({
    changes: { from, to },
    selection: { anchor: from },
    userEvent: 'input.table',
    scrollIntoView: true,
  })
  return true
}

export function runTableCommand(host: EditorHost, command: TableCommand): boolean {
  const ctx = tableContext(host)
  if (ctx === null) return false
  if (ctx.focus.row < 2 && BODY_ONLY.has(command)) return false
  if (command === 'deleteColumn' && ctx.width === 1) return deleteTable(host, ctx)

  const { editor, text } = createTableEditor(host)
  switch (command) {
    case 'insertRowAbove':
      editor.insertRow(TABLE_OPTIONS)
      break
    case 'insertRowBelow':
      // On the header and delimiter rows insertRow already lands on the first body row.
      if (ctx.focus.row < 2) editor.insertRow(TABLE_OPTIONS)
      else text.transact(() => {
        editor.insertRow(TABLE_OPTIONS)
        editor.moveRow(1, TABLE_OPTIONS)
      })
      break
    case 'insertColumnLeft':
      editor.insertColumn(TABLE_OPTIONS)
      break
    case 'insertColumnRight':
      text.transact(() => {
        editor.insertColumn(TABLE_OPTIONS)
        editor.moveColumn(1, TABLE_OPTIONS)
      })
      break
    case 'deleteRow':
      editor.deleteRow(TABLE_OPTIONS)
      break
    case 'deleteColumn':
      editor.deleteColumn(TABLE_OPTIONS)
      break
    case 'alignLeft':
      editor.alignColumn(Alignment.LEFT, TABLE_OPTIONS)
      break
    case 'alignCenter':
      editor.alignColumn(Alignment.CENTER, TABLE_OPTIONS)
      break
    case 'alignRight':
      editor.alignColumn(Alignment.RIGHT, TABLE_OPTIONS)
      break
    case 'moveRowUp':
      editor.moveRow(-1, TABLE_OPTIONS)
      break
    case 'moveRowDown':
      editor.moveRow(1, TABLE_OPTIONS)
      break
    case 'moveColumnLeft':
      editor.moveColumn(-1, TABLE_OPTIONS)
      break
    case 'moveColumnRight':
      editor.moveColumn(1, TABLE_OPTIONS)
      break
  }
  return true
}
