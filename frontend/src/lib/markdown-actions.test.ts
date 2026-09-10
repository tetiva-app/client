import { describe, it, expect } from 'vitest'
import {
  applyMarkdownAction,
  blockInsert,
  SLASH_MENU_ITEMS,
  tableSkeleton,
  type MarkdownAction,
} from './markdown-actions'

interface Applied {
  doc: string
  selected: string
  from: number
  to: number
}

function run(action: MarkdownAction, doc: string, from = 0, to = from): Applied {
  const edit = applyMarkdownAction(action, doc, from, to)
  const next = doc.slice(0, edit.from) + edit.insert + doc.slice(edit.to)
  return {
    doc: next,
    selected: next.slice(edit.selectionFrom, edit.selectionTo),
    from: edit.selectionFrom,
    to: edit.selectionTo,
  }
}

function span(doc: string, needle: string): [number, number] {
  const i = doc.indexOf(needle)
  if (i === -1) throw new Error(`"${needle}" not in ${JSON.stringify(doc)}`)
  return [i, i + needle.length]
}

describe('inline actions', () => {
  it('inserts a placeholder and selects it when nothing is selected', () => {
    const r = run('bold', 'ab', 1)
    expect(r.doc).toBe('a**bold text**b')
    expect(r.selected).toBe('bold text')
  })

  it('wraps a selection and keeps the text selected', () => {
    const r = run('bold', 'one two', ...span('one two', 'two'))
    expect(r.doc).toBe('one **two**')
    expect(r.selected).toBe('two')
  })

  it('unwraps when the markers are inside the selection', () => {
    const r = run('bold', 'a **two** b', ...span('a **two** b', '**two**'))
    expect(r.doc).toBe('a two b')
    expect(r.selected).toBe('two')
  })

  it('unwraps when the markers sit just outside the selection', () => {
    const r = run('bold', 'a **two** b', ...span('a **two** b', 'two'))
    expect(r.doc).toBe('a two b')
    expect(r.selected).toBe('two')
  })

  it('does not mistake a leading marker for a wrapped selection', () => {
    const r = run('code', 'x', 0, 1)
    expect(r.doc).toBe('`x`')
  })

  it('uses underscores for italic so it never collides with bold', () => {
    expect(run('italic', 'x', 0, 1).doc).toBe('_x_')
    expect(run('italic', '**x**', 0, 5).doc).toBe('_**x**_')
  })

  it('uses backticks for inline code', () => {
    expect(run('code', 'x', 0, 1).doc).toBe('`x`')
    expect(run('code', '', 0).selected).toBe('code')
  })

  it('does not swallow a lone marker character', () => {
    expect(run('code', '`', 0, 1).doc).toBe('```')
  })

  it('does not unwrap when only one side carries the marker', () => {
    expect(run('code', '`a b', 1, 2).doc).toBe('``a` b')
  })

  it('normalizes a reversed selection', () => {
    expect(run('bold', 'one two', 7, 4).doc).toBe('one **two**')
  })
})

describe('link', () => {
  it('selects the url slot when a label was selected', () => {
    const r = run('link', 'see docs', ...span('see docs', 'docs'))
    expect(r.doc).toBe('see [docs](url)')
    expect(r.selected).toBe('url')
  })

  it('selects the label when a url was selected', () => {
    const r = run('link', 'https://a.dev/x', 0, 15)
    expect(r.doc).toBe('[text](https://a.dev/x)')
    expect(r.selected).toBe('text')
  })

  it('does not treat text with spaces as a url', () => {
    const r = run('link', 'https://a.dev x', 0, 15)
    expect(r.doc).toBe('[https://a.dev x](url)')
  })

  it('inserts a full template when nothing is selected', () => {
    const r = run('link', '', 0)
    expect(r.doc).toBe('[text](url)')
    expect(r.selected).toBe('text')
  })
})

describe('heading', () => {
  it('cycles h1 to h2 to h3 and back to plain text', () => {
    expect(run('heading', 'Title', 0).doc).toBe('# Title')
    expect(run('heading', '# Title', 2).doc).toBe('## Title')
    expect(run('heading', '## Title', 3).doc).toBe('### Title')
    expect(run('heading', '### Title', 4).doc).toBe('Title')
  })

  it('keeps the caret after the marker it just added', () => {
    const r = run('heading', 'Title', 0)
    expect(r.from).toBe(2)
    expect(r.to).toBe(2)
  })

  it('applies the level of the first line to every selected line', () => {
    const r = run('heading', '# a\nb', 0, 5)
    expect(r.doc).toBe('## a\n## b')
  })
})

describe('lists', () => {
  it('adds and removes bullets', () => {
    expect(run('bulletList', 'a\nb', 0, 3).doc).toBe('- a\n- b')
    expect(run('bulletList', '- a\n- b', 0, 7).doc).toBe('a\nb')
  })

  it('converts a numbered list into bullets', () => {
    expect(run('bulletList', '1. a\n2. b', 0, 9).doc).toBe('- a\n- b')
  })

  it('numbers lines sequentially and leaves blank separators alone', () => {
    expect(run('numberedList', 'a\n\nb', 0, 4).doc).toBe('1. a\n\n2. b')
  })

  it('removes numbering when every line already has it', () => {
    expect(run('numberedList', '1. a\n2. b', 0, 9).doc).toBe('a\nb')
  })

  it('bullets a blank line when that is all there is', () => {
    expect(run('bulletList', '', 0).doc).toBe('- ')
  })

  it('ignores the line after a selection that ends at a line start', () => {
    expect(run('bulletList', 'a\nb', 0, 2).doc).toBe('- a\nb')
  })

  it('keeps the whole selection covered after prefixing every line', () => {
    const r = run('bulletList', 'a\nb', 0, 3)
    expect(r.doc).toBe('- a\n- b')
    expect(r.selected).toBe('a\n- b')
  })

  it('does not treat a dash without a space as a list marker', () => {
    expect(run('bulletList', '-item', 0, 5).doc).toBe('- -item')
  })

  it('works on the first line of a document that starts with a newline', () => {
    expect(run('bulletList', '\na', 0).doc).toBe('- \na')
  })

  it('extends the caret past the inserted marker', () => {
    const r = run('bulletList', 'a', 1)
    expect(r.doc).toBe('- a')
    expect(r.from).toBe(3)
  })
})

describe('quote', () => {
  it('toggles the quote marker on every line', () => {
    expect(run('quote', 'a\nb', 0, 3).doc).toBe('> a\n> b')
    expect(run('quote', '> a\n> b', 0, 7).doc).toBe('a\nb')
  })

  it('only adds the marker to lines that lack it', () => {
    expect(run('quote', '> a\nb', 0, 5).doc).toBe('> a\n> b')
  })

  it('quotes blank lines too so the block stays contiguous', () => {
    expect(run('quote', 'a\n\nb', 0, 4).doc).toBe('> a\n> \n> b')
  })
})

describe('block inserts', () => {
  it('fences a selection and keeps it selected', () => {
    const r = run('codeBlock', 'GET /x', 0, 6)
    expect(r.doc).toBe('```\nGET /x\n```')
    expect(r.selected).toBe('GET /x')
  })

  it('opens an empty fence with the caret inside', () => {
    const r = run('codeBlock', '', 0)
    expect(r.doc).toBe('```\n\n```')
    expect(r.from).toBe(4)
    expect(r.to).toBe(4)
  })

  it('starts the fence on its own line when the caret is mid-line', () => {
    expect(run('codeBlock', 'note', 4).doc).toBe('note\n```\n\n```')
    expect(run('codeBlock', 'note', 2).doc).toBe('no\n```\n\n```\nte')
  })

  it('inserts a 2×1 skeleton and selects the first header cell', () => {
    const r = run('table', '')
    expect(r.doc).toBe('| Column 1 | Column 2 |\n| -------- | -------- |\n|          |          |')
    expect(r.selected).toBe('Column 1')
  })

  it('keeps a blank line between the table and the text around it', () => {
    const t = tableSkeleton(2, 1)
    expect(run('table', 'intro notes', 6).doc).toBe('intro \n\n' + t + '\n\nnotes')
    expect(run('table', 'intro\nnotes', 5).doc).toBe('intro\n\n' + t + '\n\nnotes')
    expect(run('table', 'intro', 5).doc).toBe('intro\n\n' + t)
    expect(run('table', '', 0).doc).toBe(t)
  })

  it('does not stack a second blank line on one that is already there', () => {
    const t = tableSkeleton(2, 1)
    expect(run('table', 'intro\n\n\nnotes', 7).doc).toBe('intro\n\n' + t + '\n\nnotes')
    expect(run('table', '\n\n', 2).doc).toBe('\n\n' + t)
  })

  it('keeps the first header cell selected through the padding', () => {
    expect(run('table', 'intro', 5).selected).toBe('Column 1')
  })

  it('pads a table the editor inserts outside an action', () => {
    const doc = 'intro\nnotes'
    const table = '| a |\n| - |'
    const edit = blockInsert(doc, 5, 5, table, table.length, table.length)
    const next = doc.slice(0, edit.from) + edit.insert + doc.slice(edit.to)
    expect(next).toBe('intro\n\n| a |\n| - |\n\nnotes')
    expect(next.slice(0, edit.selectionFrom)).toBe('intro\n\n| a |\n| - |')
  })
})

describe('tableSkeleton', () => {
  it('builds a skeleton with the requested size', () => {
    expect(tableSkeleton(3, 2).split('\n')).toEqual([
      '| Column 1 | Column 2 | Column 3 |',
      '| -------- | -------- | -------- |',
      '|          |          |          |',
      '|          |          |          |',
    ])
    expect(tableSkeleton(0, 0).split('\n')).toHaveLength(3)
  })
})

describe('slash menu', () => {
  it('exposes the documented commands', () => {
    expect(SLASH_MENU_ITEMS.map(i => i.label)).toEqual([
      '/heading', '/list', '/code', '/table', '/quote', '/link',
    ])
  })

  it('every entry maps to a real action and carries a description', () => {
    for (const item of SLASH_MENU_ITEMS) {
      expect(item.label.startsWith('/')).toBe(true)
      expect(item.detail.length).toBeGreaterThan(0)
      expect(() => applyMarkdownAction(item.action, '', 0, 0)).not.toThrow()
    }
  })
})
