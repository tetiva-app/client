// Tables built from plain data go through the same kernel as the editor, so the
// picker's skeleton and pasted TSV arrive already aligned.

import { Table, TableCell, TableRow, completeTable, formatTable } from '@susisu/mte-kernel'
import { TABLE_OPTIONS } from './markdown-table-editor'

// Backslashes first: escaping the pipe first would turn `a\|b` into `a\\|b`, which
// the kernel reads back as an escaped backslash plus a cell separator.
function cellText(value: string): string {
  return value.trim().replace(/\\/g, '\\\\').replace(/\|/g, '\\|')
}

function tsvLines(text: string): string[] {
  return text.split(/\r?\n/).filter(line => line.trim() !== '')
}

export function tableFromRows(rows: string[][]): string {
  const width = Math.max(1, ...rows.map(row => row.length))
  const toRow = (values: string[]) => new TableRow(
    Array.from({ length: width }, (_, i) => new TableCell(cellText(values[i] ?? ''))),
    '',
    '',
  )
  const [header = [], ...body] = rows
  // An explicit delimiter row: completeTable would otherwise take a first data row
  // of hyphens for the delimiter and lose it.
  const delimiter = new TableRow(Array.from({ length: width }, () => new TableCell('---')), '', '')
  const table = new Table([toRow(header), delimiter, ...body.map(toRow)])
  const { table: formatted } = formatTable(completeTable(table, TABLE_OPTIONS).table, TABLE_OPTIONS)
  return formatted.toLines().join('\n')
}

export function isTsv(text: string): boolean {
  const lines = tsvLines(text)
  if (lines.length < 2) return false
  // Tab-indented code would otherwise pass: leading tabs are indentation, not an empty first cell.
  if (lines.every(line => line.startsWith('\t'))) return false
  // A spreadsheet always copies a rectangle; ragged tab counts mean prose or code.
  const width = lines[0].split('\t').length
  return width >= 2 && lines.every(line => line.split('\t').length === width)
}

export function tsvToMarkdownTable(text: string): string {
  return tableFromRows(tsvLines(text).map(line => line.split('\t')))
}
