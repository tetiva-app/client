import { describe, it, expect, afterEach } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import SharePageThumbnail from './SharePageThumbnail.vue'
import { inside, tagWith } from '@/test-utils/markup'
import { setCurrentLocale } from '@/lib/locale'

function render(title: string): Promise<string> {
  return renderToString(createSSRApp(SharePageThumbnail, { title }))
}

afterEach(() => {
  setCurrentLocale('en')
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
    const html = await render('Petstore API')

    expect(inside(html, 'data-testid="share-page-thumbnail-title"')).toContain('Petstore API')
    expect(html).toContain('share.tetiva.app')
    expect(html).toContain('>Open in Tetiva<')
    expect(html).toContain('>Download<')
    expect(html).toContain('>GET<')
    expect(html).toContain('dark:hidden')
    expect(html).toContain('dark:inline')
  })

  it('labels the buttons in Russian when the app is in Russian, as the page will be', async () => {
    setCurrentLocale('ru')
    const html = await render('Petstore API')

    expect(html).toContain('>Открыть в Tetiva<')
    expect(html).toContain('>Скачать<')
    expect(html).not.toContain('>Download<')
    expect(inside(html, 'data-testid="share-page-thumbnail"')).toContain('Как выглядит страница')
  })

  it('shortens a long title with an ellipsis', async () => {
    const html = await render('Internal billing and payment scenarios API for partners')

    const title = inside(html, 'data-testid="share-page-thumbnail-title"').replace(/^<[^>]+>/, '')
    expect(title.endsWith('…')).toBe(true)
    expect(title.length).toBeLessThanOrEqual(34)
    expect(title.startsWith('Internal billing')).toBe(true)
  })
})
