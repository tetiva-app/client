import { describe, it, expect } from 'vitest'
import { renderMarkdown, MARKDOWN_OPTIONS } from './markdown'

const VECTORS = [
  '<script>alert(1)</script>',
  '<img src=x onerror=alert(1)>',
  '[x](javascript:alert(1))',
  '[x](JaVaScRiPt:alert(1))',
  '[x](&#106;avascript:alert(1))',
  '[x](vbscript:msgbox(1))',
  '[x](data:text/html,<script>alert(1)</script>)',
  '<a href="data:text/html,x">y</a>',
  'javascript:alert(1)',
  '<details open ontoggle=alert(1)>',
]

describe('renderMarkdown', () => {
  it('never enables raw HTML', () => {
    expect(MARKDOWN_OPTIONS.html).toBe(false)
  })

  it.each(VECTORS)('renders %s inert', (src) => {
    const html = renderMarkdown(src)
    expect(html).not.toMatch(/<script/i)
    // Attribute inside a real tag, not the escaped literal text `&lt;img … onerror=…&gt;`.
    expect(html).not.toMatch(/<[^>]+\son[a-z]+\s*=/i)
    expect(html).not.toMatch(/href="(javascript|vbscript|data:text)/i)
    expect(html).not.toMatch(/<details/i)
  })

  it('still links plain urls and renders tables', () => {
    expect(renderMarkdown('see http://ok.com')).toContain('href="http://ok.com"')
    expect(renderMarkdown('| a |\n| - |\n| 1 |')).toContain('<table>')
  })

  it('treats undefined as empty', () => {
    expect(renderMarkdown(undefined as unknown as string)).toBe('')
  })
})
