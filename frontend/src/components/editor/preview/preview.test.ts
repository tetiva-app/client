import { describe, it, expect, afterEach, vi } from 'vitest'
import { createSSRApp, h, type Component } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { tagWith } from '@/test-utils/markup'
import { setCurrentLocale } from '@/lib/locale'

vi.mock('@/lib/response-preview', async (importOriginal) => ({
  ...await importOriginal<typeof import('@/lib/response-preview')>(),
  htmlPreviewDoc: (html: string) => `<!-- scrubbed -->${html}`,
}))

import HtmlPreview from './HtmlPreview.vue'
import ImagePreview from './ImagePreview.vue'
import BodyViewToggle from './BodyViewToggle.vue'
import TooLargeNote from './TooLargeNote.vue'

function render(component: Component, props: Record<string, unknown>): Promise<string> {
  return renderToString(createSSRApp({ render: () => h(component, props) }))
}

function attr(tag: string, name: string): string | null {
  const m = new RegExp(`\\s${name}(?:="([^"]*)")?(?=[\\s>/])`).exec(tag)
  if (!m) return null
  return (m[1] ?? '').replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&quot;/g, '"').replace(/&#39;/g, "'").replace(/&amp;/g, '&')
}

afterEach(() => { setCurrentLocale('en') })

describe('HTML preview frame', () => {
  const page = '<!DOCTYPE html><html><head><script src="https://evil.test/x.js"></script></head><body onload="alert(1)">hi</body></html>'

  it('runs in an empty sandbox: no scripts, no same-origin, no forms or popups', async () => {
    const iframe = tagWith(await render(HtmlPreview, { html: page }), '<iframe')

    expect(attr(iframe, 'sandbox')).toBe('')
    expect(iframe).not.toMatch(/allow-/)
    expect(attr(iframe, 'src')).toBeNull()
  })

  it('feeds the scrubbed document through srcdoc', async () => {
    const srcdoc = attr(tagWith(await render(HtmlPreview, { html: page }), '<iframe'), 'srcdoc')

    expect(srcdoc).toBe(`<!-- scrubbed -->${page}`)
  })

  it('names the frame in both languages', async () => {
    expect(attr(tagWith(await render(HtmlPreview, { html: '' }), '<iframe'), 'title')).toBe('HTML preview')
    setCurrentLocale('ru')
    expect(attr(tagWith(await render(HtmlPreview, { html: '' }), '<iframe'), 'title')).toBe('Просмотр HTML')
  })
})

describe('image preview', () => {
  it('shows the picture as an img, never inline markup', async () => {
    const html = await render(ImagePreview, { src: 'blob:x/1', mimeType: 'image/svg+xml', size: 2048 })

    expect(attr(tagWith(html, '<img'), 'src')).toBe('blob:x/1')
    expect(html).not.toContain('<svg')
    expect(html).toContain('SVG')
    expect(html).toContain('2 KB')
    expect(html).not.toContain('Save to file')
  })

  it('offers saving when the bytes live in a file', async () => {
    setCurrentLocale('ru')
    const html = await render(ImagePreview, { src: 'blob:x/2', mimeType: 'image/png', size: 157, saveable: true })

    expect(html).toContain('Сохранить в файл')
    expect(html).toContain('157 Б')
    expect(attr(tagWith(html, '<img'), 'alt')).toBe('Картинка из ответа')
  })
})

describe('too large note', () => {
  it('names the limit in both languages', async () => {
    expect(await render(TooLargeNote, { limit: 10 * 1024 * 1024 })).toContain('Image is too large to preview (over 10 MB)')
    setCurrentLocale('ru')
    expect(await render(TooLargeNote, { limit: 10 * 1024 * 1024 })).toContain('Картинка слишком большая для превью (больше 10 МБ)')
  })
})

describe('body view toggle', () => {
  it('marks the current view and explains what preview leaves out', async () => {
    const html = await render(BodyViewToggle, { modelValue: 'source' })

    expect(html).toMatch(/aria-pressed="true"[^>]*>\s*Source\s*</)
    const preview = tagWith(html, 'title="Rendered without scripts or network requests"')
    expect(attr(preview, 'aria-pressed')).toBe('false')
    expect(html).toContain('aria-label="Body view"')
  })

  it('speaks Russian', async () => {
    setCurrentLocale('ru')
    const html = await render(BodyViewToggle, { modelValue: 'preview' })

    expect(html).toContain('Код')
    expect(attr(tagWith(html, 'title="Страница показана без скриптов и запросов в сеть"'), 'aria-pressed')).toBe('true')
  })
})
