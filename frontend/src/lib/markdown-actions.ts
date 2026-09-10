// Markdown editing primitives for the description editor: document + selection in,
// one replacement out. No CodeMirror types — toolbar, keymap and slash menu share it.

import { tableFromRows } from './markdown-paste'

export type MarkdownAction =
  | 'bold'
  | 'italic'
  | 'code'
  | 'link'
  | 'heading'
  | 'bulletList'
  | 'numberedList'
  | 'quote'
  | 'codeBlock'
  | 'table'

export interface MarkdownEdit {
  from: number
  to: number
  insert: string
  // Positions in the document produced by applying this edit.
  selectionFrom: number
  selectionTo: number
}

const HEADING_RE = /^(#{1,6}) +/
const BULLET_RE = /^[-*+] +/
const NUMBER_RE = /^\d+\. +/
const QUOTE_RE = /^> ?/
const URL_RE = /^https?:\/\/\S+$/

const INLINE = {
  bold: { marker: '**', placeholder: 'bold text' },
  italic: { marker: '_', placeholder: 'italic text' },
  code: { marker: '`', placeholder: 'code' },
} as const

const MAX_TABLE_COLUMNS = 8
const MAX_TABLE_ROWS = 6

function lineStart(doc: string, pos: number): number {
  if (pos <= 0) return 0
  return doc.lastIndexOf('\n', pos - 1) + 1
}

function lineEnd(doc: string, pos: number): number {
  const i = doc.indexOf('\n', pos)
  return i === -1 ? doc.length : i
}

// A selection ending exactly at a line start does not reach into that line.
function selectedLineRanges(doc: string, from: number, to: number): Array<{ from: number; to: number }> {
  const last = to > from && lineStart(doc, to) === to ? to - 1 : to
  const end = lineEnd(doc, last)
  const ranges: Array<{ from: number; to: number }> = []
  let pos = lineStart(doc, from)
  while (true) {
    const lineTo = lineEnd(doc, pos)
    ranges.push({ from: pos, to: lineTo })
    if (lineTo >= end) break
    pos = lineTo + 1
  }
  return ranges
}

function inlineEdit(doc: string, from: number, to: number, kind: 'bold' | 'italic' | 'code'): MarkdownEdit {
  const { marker, placeholder } = INLINE[kind]
  const m = marker.length

  if (from === to) {
    return {
      from,
      to,
      insert: marker + placeholder + marker,
      selectionFrom: from + m,
      selectionTo: from + m + placeholder.length,
    }
  }

  const selected = doc.slice(from, to)

  if (selected.length >= 2 * m && selected.startsWith(marker) && selected.endsWith(marker)) {
    const inner = selected.slice(m, selected.length - m)
    return { from, to, insert: inner, selectionFrom: from, selectionTo: from + inner.length }
  }

  if (from >= m && doc.slice(from - m, from) === marker && doc.slice(to, to + m) === marker) {
    return {
      from: from - m,
      to: to + m,
      insert: selected,
      selectionFrom: from - m,
      selectionTo: from - m + selected.length,
    }
  }

  return {
    from,
    to,
    insert: marker + selected + marker,
    selectionFrom: from + m,
    selectionTo: from + m + selected.length,
  }
}

function linkEdit(doc: string, from: number, to: number): MarkdownEdit {
  const selected = doc.slice(from, to)

  // Nothing selected, or a URL was selected: the label is what needs typing.
  if (from === to || URL_RE.test(selected)) {
    const insert = `[text](${from === to ? 'url' : selected})`
    return { from, to, insert, selectionFrom: from + 1, selectionTo: from + 5 }
  }

  const insert = `[${selected}](url)`
  const urlFrom = from + selected.length + 3
  return { from, to, insert, selectionFrom: urlFrom, selectionTo: urlFrom + 3 }
}

function linePrefixEdit(
  doc: string,
  from: number,
  to: number,
  action: 'heading' | 'bulletList' | 'numberedList' | 'quote',
): MarkdownEdit {
  const lines = selectedLineRanges(doc, from, to)
  const texts = lines.map(l => doc.slice(l.from, l.to))
  const allBlank = texts.every(t => t.trim() === '')

  let transform: (text: string) => string

  if (action === 'heading') {
    const current = HEADING_RE.exec(texts[0])
    const level = current ? current[1].length : 0
    const next = level >= 3 ? 0 : level + 1
    transform = (text) => {
      const body = text.replace(HEADING_RE, '')
      return next === 0 ? body : '#'.repeat(next) + ' ' + body
    }
  } else if (action === 'quote') {
    const allPrefixed = texts.every(t => QUOTE_RE.test(t))
    transform = (text) => {
      if (allPrefixed) return text.replace(QUOTE_RE, '')
      return QUOTE_RE.test(text) ? text : '> ' + text
    }
  } else {
    const own = action === 'bulletList' ? BULLET_RE : NUMBER_RE
    const other = action === 'bulletList' ? NUMBER_RE : BULLET_RE
    const allPrefixed = !allBlank && texts.every(t => t.trim() === '' || own.test(t))
    let index = 0
    transform = (text) => {
      if (allPrefixed) return text.replace(own, '')
      // Blank separators keep a list readable; numbering them would be wrong.
      if (text.trim() === '' && !allBlank) return text
      index += 1
      const body = text.replace(own, '').replace(other, '')
      return (action === 'bulletList' ? '- ' : `${index}. `) + body
    }
  }

  const next = texts.map(transform)
  const blockFrom = lines[0].from
  const blockTo = lines[lines.length - 1].to
  const insert = next.join('\n')

  const firstDelta = next[0].length - texts[0].length
  const totalDelta = insert.length - (blockTo - blockFrom)
  const blockEndAfter = blockFrom + insert.length

  const selectionFrom = clamp(from + firstDelta, blockFrom, blockEndAfter)
  const selectionTo = clamp(to + totalDelta, selectionFrom, blockEndAfter)

  return { from: blockFrom, to: blockTo, insert, selectionFrom, selectionTo }
}

// Newlines missing before `pos` for a blank line to precede it; a line of spaces
// counts as blank, and nothing is needed when only whitespace comes before.
function padBefore(doc: string, pos: number): string {
  if (doc.slice(0, pos).trim() === '') return ''
  let breaks = 0
  for (let i = pos; i > 0; i--) {
    const ch = doc[i - 1]
    if (ch === '\n') breaks += 1
    else if (ch !== ' ' && ch !== '\t') break
  }
  return '\n'.repeat(Math.max(0, 2 - breaks))
}

function padAfter(doc: string, pos: number): string {
  if (doc.slice(pos).trim() === '') return ''
  let breaks = 0
  for (let i = pos; i < doc.length; i++) {
    const ch = doc[i]
    if (ch === '\n') breaks += 1
    else if (ch !== ' ' && ch !== '\t') break
  }
  return '\n'.repeat(Math.max(0, 2 - breaks))
}

// Block constructs need their own lines: pad when the caret sits inside text. Tables
// need a blank line too — markdown-it reads the paragraph below one as another row.
function blockEdit(
  doc: string,
  from: number,
  to: number,
  body: string,
  cursorFrom: number,
  cursorTo: number,
  blank = false,
): MarkdownEdit {
  const before = blank ? padBefore(doc, from) : (from > lineStart(doc, from) ? '\n' : '')
  const after = blank ? padAfter(doc, to) : (to < lineEnd(doc, to) ? '\n' : '')
  const base = from + before.length
  return {
    from,
    to,
    insert: before + body + after,
    selectionFrom: base + cursorFrom,
    selectionTo: base + cursorTo,
  }
}

// The editor inserts tables outside of applyMarkdownAction too: picker and TSV paste.
export function blockInsert(
  doc: string,
  from: number,
  to: number,
  body: string,
  cursorFrom: number,
  cursorTo: number,
): MarkdownEdit {
  return blockEdit(doc, from, to, body, cursorFrom, cursorTo, true)
}

function codeBlockEdit(doc: string, from: number, to: number): MarkdownEdit {
  const selected = doc.slice(from, to)
  return blockEdit(doc, from, to, '```\n' + selected + '\n```', 4, 4 + selected.length)
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max)
}

// Numbered header cells so every column can be told apart while filling the table.
export function tableSkeleton(columns: number, rows: number): string {
  const width = clamp(Math.round(columns), 1, MAX_TABLE_COLUMNS)
  const height = clamp(Math.round(rows), 1, MAX_TABLE_ROWS)
  const header = Array.from({ length: width }, (_, i) => `Column ${i + 1}`)
  const body = Array.from({ length: height }, () => Array.from({ length: width }, () => ''))
  return tableFromRows([header, ...body])
}

export function applyMarkdownAction(
  action: MarkdownAction,
  doc: string,
  selFrom: number,
  selTo: number,
): MarkdownEdit {
  const from = Math.min(selFrom, selTo)
  const to = Math.max(selFrom, selTo)

  switch (action) {
    case 'bold':
    case 'italic':
    case 'code':
      return inlineEdit(doc, from, to, action)
    case 'link':
      return linkEdit(doc, from, to)
    case 'heading':
    case 'bulletList':
    case 'numberedList':
    case 'quote':
      return linePrefixEdit(doc, from, to, action)
    case 'codeBlock':
      return codeBlockEdit(doc, from, to)
    case 'table':
      return blockInsert(doc, from, to, tableSkeleton(2, 1), 2, 10)
  }
}

export interface SlashMenuItem {
  label: string
  action: MarkdownAction
  detail: string
}

// Declaration order is the menu order: most useful for docs first. The slash
// menu turns it into per-option boosts, since CodeMirror otherwise breaks
// score ties alphabetically.
export const SLASH_MENU_ITEMS: SlashMenuItem[] = [
  { label: '/heading', action: 'heading', detail: 'Section heading' },
  { label: '/list', action: 'bulletList', detail: 'Bullet list' },
  { label: '/code', action: 'codeBlock', detail: 'Code block' },
  { label: '/table', action: 'table', detail: 'Table' },
  { label: '/quote', action: 'quote', detail: 'Block quote' },
  { label: '/link', action: 'link', detail: 'Link' },
]

