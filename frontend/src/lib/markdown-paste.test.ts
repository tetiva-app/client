import { describe, it, expect } from 'vitest'
import { readTable } from '@susisu/mte-kernel'
import { TABLE_OPTIONS } from './markdown-table-editor'
import { isTsv, tsvToMarkdownTable, tableFromRows } from './markdown-paste'

describe('isTsv', () => {
  it('needs at least two tab-separated lines', () => {
    expect(isTsv('a\tb\n1\t2')).toBe(true)
    expect(isTsv('a\tb\r\n1\t2\r\n')).toBe(true)
    expect(isTsv('a\tb\n\n1\t2')).toBe(true)
    expect(isTsv('a\tb')).toBe(false)
    expect(isTsv('a,b\n1,2')).toBe(false)
    expect(isTsv('a\tb\nplain')).toBe(false)
  })

  it('leaves tab-indented code alone', () => {
    expect(isTsv('\tif err != nil {\n\t\treturn err\n\t}')).toBe(false)
    expect(isTsv('\tgo build ./...\n\tgo test ./...')).toBe(false)
    expect(isTsv('build:\n\tgo build ./...')).toBe(false)
  })

  it('needs a rectangle: a spreadsheet never copies ragged rows', () => {
    expect(isTsv('a\tb\tc\n1\t2')).toBe(false)
    expect(isTsv('a\tb\n1\t2\t3')).toBe(false)
  })
})

describe('tsvToMarkdownTable', () => {
  it('turns the first line into the header and formats columns', () => {
    expect(tsvToMarkdownTable('name\tage\nAnn\t30\nBob\t4')).toBe(
      '| name | age |\n| ---- | --- |\n| Ann  | 30  |\n| Bob  | 4   |',
    )
  })
  it('drops blank rows', () => {
    expect(tsvToMarkdownTable('h\tv\n\n1\t2\n')).toBe('| h   | v   |\n| --- | --- |\n| 1   | 2   |')
  })
  it('keeps a data row made of hyphens as data', () => {
    expect(tsvToMarkdownTable('h\tv\n---\t---\n1\t2')).toBe('| h   | v   |\n| --- | --- |\n| --- | --- |\n| 1   | 2   |')
  })
  it('escapes pipes, trims cells and pads ragged rows', () => {
    expect(tsvToMarkdownTable('k\tv\ta|b\nx\t y ')).toBe(
      '| k   | v   | a\\|b |\n| --- | --- | ---- |\n| x   | y   |      |',
    )
  })
})

describe('tableFromRows', () => {
  it('formats an arbitrary matrix', () => {
    expect(tableFromRows([['h'], ['1'], ['22']])).toBe('| h   |\n| --- |\n| 1   |\n| 22  |')
  })
  it('escapes backslashes and pipes so the kernel reads one cell back', () => {
    const md = tableFromRows([['h'], ['a\\|b']])
    expect(md.split('\n')[2]).toBe('| a\\\\\\|b |')
    const back = readTable(md.split('\n'), TABLE_OPTIONS)
    expect(back.getRows()[2].getWidth()).toBe(1)
  })
})
