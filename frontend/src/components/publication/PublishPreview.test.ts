import { describe, it, expect } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import type { PublishPreview as Preview } from '@/types/publication'
import PublishPreview from './PublishPreview.vue'

function preview(over: Partial<Preview> = {}): Preview {
  return {
    folders: 1, requests: 2, examples: 6, publishedVars: [], hiddenVars: [], redactions: [], warnings: [],
    errors: [], ignoredOverrides: [], sizeBytes: 1000, sizeLimitBytes: 8 << 20, gzipBytes: 300,
    gzipLimitBytes: 7 << 19, largestExamples: [], previewHash: 'hash-1',
    ...over,
  }
}

async function render(p: Preview): Promise<string> {
  const app = createSSRApp(PublishPreview, {
    preview: p, hiddenRows: [], removedRows: [], warningRows: [], environmentName: '', includeScripts: false, busy: false,
  })
  return renderToString(app)
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
