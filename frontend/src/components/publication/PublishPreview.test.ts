import { describe, it, expect, afterEach } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import type { PublishPreview as Preview } from '@/types/publication'
import type { RemovedRow } from '@/composables/usePublishDialog'
import PublishPreview from './PublishPreview.vue'
import { formatNumber, setCurrentLocale } from '@/lib/locale'
import { inside, tagWith } from '@/test-utils/markup'

afterEach(() => { setCurrentLocale('en') })

function preview(over: Partial<Preview> = {}): Preview {
  return {
    folders: 1, requests: 2, examples: 6, publishedVars: [], hiddenVars: [], redactions: [], warnings: [],
    errors: [], ignoredOverrides: [], sizeBytes: 1000, sizeLimitBytes: 8 << 20, gzipBytes: 300,
    gzipLimitBytes: 7 << 19, largestExamples: [], previewHash: 'hash-1',
    ...over,
  }
}

async function render(p: Preview, removedRows: RemovedRow[] = []): Promise<string> {
  const app = createSSRApp(PublishPreview, {
    preview: p, hiddenRows: [], removedRows, warningRows: [], environmentName: '', includeScripts: false, busy: false,
  })
  return renderToString(app)
}

function removed(category: string, reason: string): RemovedRow {
  return { selector: `s/${category}`, path: `r / ${category}`, category, reason, overridable: false, overridden: false, toggle: false, on: false }
}

describe('publish preview', () => {
  it('names the largest response examples when the collection is over the size limit', async () => {
    const html = await render(preview({
      gzipBytes: 5 << 20,
      largestExamples: [
        { path: 'Files / Download / Blob 0', bytes: 1_000_000 },
        { path: 'Files / Download / Blob 1', bytes: 986_667 },
      ],
    }))

    expect(html).toContain('data-testid="publish-largest-examples"')
    expect(html).toContain('Files / Download / Blob 0')
    expect(html).toContain('Files / Download / Blob 1')
    expect(html).toContain('977 KB')
  })

  it('lists no examples within the limit', async () => {
    expect(await render(preview())).not.toContain('data-testid="publish-largest-examples"')
  })
})

describe('publish preview in Russian', () => {
  it('counts folders, requests and examples with Russian plurals and sizes in КБ', async () => {
    setCurrentLocale('ru')
    const html = await render(preview({ folders: 1, requests: 2, examples: 6, sizeBytes: 18_432 }))

    expect(html).toContain('1 папка')
    expect(html).toContain('2 запроса')
    expect(html).toContain('6 примеров')
    expect(html).toContain('18 КБ из 8 МБ')
    expect(html).toContain('Блокирующих ошибок нет')
  })

  it('words a blocking error by its code', async () => {
    setCurrentLocale('ru')
    const html = await render(preview({
      errors: [
        { path: 'r / method', code: 'method_unsupported', params: { value: 'TRACE' }, message: 'HTTP method "TRACE" cannot be published' },
        { path: 'r / headers', code: 'too_many', params: { list: 'headers', count: '201', limit: '200' }, message: '201 headers; at most 200 can be published' },
        { path: 'Petstore', code: 'too_many_values', params: { count: '500123', limit: '500000' }, message: '500123 JSON values; at most 500000 can be published' },
      ],
    }))

    const errors = inside(html, 'data-testid="publish-errors"')
    expect(errors).toContain('HTTP-метод «TRACE» нельзя опубликовать')
    expect(errors).toContain('заголовков: 201, а опубликовать можно не больше 200')
    expect(errors).toContain(`значений JSON: ${formatNumber('ru', 500123)}, а опубликовать можно не больше ${formatNumber('ru', 500000)}`)
    expect(errors).not.toContain('cannot be published')
  })

  it('falls back to the English message for a code it does not know', async () => {
    setCurrentLocale('ru')
    const html = await render(preview({
      errors: [{ path: 'r', code: 'something_new', params: {}, message: 'the server says no' }],
    }))

    expect(inside(html, 'data-testid="publish-errors"')).toContain('the server says no')
  })

  it('words a redaction by its category and keeps the Go reason for an unknown one', async () => {
    setCurrentLocale('ru')
    const html = await render(preview(), [removed('cookie', 'cookies are never published'), removed('future', 'some new reason')])

    expect(html).toContain('cookie никогда не публикуются')
    expect(html).not.toContain('cookies are never published')
    expect(html).toContain('some new reason')
  })
})

describe('publish preview layout', () => {
  it('sizes the label column of the size bars by its text', async () => {
    const html = await render(preview())

    expect(tagWith(html, 'data-testid="publish-size"')).toContain('grid-cols-[auto_1fr_auto]')
  })

  it('words a known blocking error in English by its code too', async () => {
    const html = await render(preview({
      errors: [{ path: 'r / body', code: 'body_type_unsupported', params: { value: 'yaml' }, message: 'stale Go text' }],
    }))

    expect(inside(html, 'data-testid="publish-errors"')).toContain('body type &quot;yaml&quot; cannot be published')
  })
})
