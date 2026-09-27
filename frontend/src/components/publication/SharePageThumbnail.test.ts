import { describe, it, expect, afterEach, vi } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import SharePageThumbnail from './SharePageThumbnail.vue'
import { inside, tagWith } from '@/test-utils/markup'

function render(title: string): Promise<string> {
  return renderToString(createSSRApp(SharePageThumbnail, { title }))
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('share page thumbnail', () => {
  it('is a decoration hidden from screen readers, with a caption', async () => {
    const html = await render('Petstore API')

    const root = tagWith(html, 'data-testid="share-page-thumbnail"')
    expect(root).toContain('aria-hidden="true"')
    expect(root).toContain('pointer-events-none')
    expect(inside(html, 'data-testid="share-page-thumbnail"')).toContain('How the page looks')
    expect(html).not.toContain('<img')
  })

  it('draws the page header with the collection title, both buttons and the theme toggle for either theme', async () => {
    vi.stubGlobal('navigator', { language: 'en-US' })
    const html = await render('Petstore API')

    expect(inside(html, 'data-testid="share-page-thumbnail-title"')).toContain('Petstore API')
    expect(html).toContain('share.tetiva.app')
    expect(html).toContain('>Open in Tetiva<')
    expect(html).toContain('>Download<')
    expect(html).toContain('>GET<')
    expect(html).toContain('dark:hidden')
    expect(html).toContain('dark:inline')
  })

  it('labels the buttons in Russian for an author whose system speaks it, as the page will', async () => {
    vi.stubGlobal('navigator', { language: 'ru-RU' })
    const html = await render('Petstore API')

    expect(html).toContain('>Открыть в Tetiva<')
    expect(html).toContain('>Скачать<')
    expect(html).not.toContain('>Download<')
  })

  it('shortens a long title with an ellipsis', async () => {
    const html = await render('Internal billing and payment scenarios API for partners')

    const title = inside(html, 'data-testid="share-page-thumbnail-title"').replace(/^<[^>]+>/, '')
    expect(title.endsWith('…')).toBe(true)
    expect(title.length).toBeLessThanOrEqual(34)
    expect(title.startsWith('Internal billing')).toBe(true)
  })
})
