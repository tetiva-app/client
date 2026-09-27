import { describe, it, expect, vi } from 'vitest'
import { createSSRApp, defineComponent, ref } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia } from 'pinia'

vi.mock('./CodeViewer.vue', () => ({ __esModule: true, default: defineComponent({ render: () => null }) }))

vi.mock('@/composables/useCodeSnippetPanel', () => ({
  useCodeSnippetPanel: () => ({
    targets: ref([
      { key: 'curl', label: 'cURL', language: 'shell' },
      { key: 'go', label: 'Go', language: 'go' },
    ]),
    selected: ref('go'),
    language: ref('go'),
    code: ref('package main'),
    warnings: ref([]),
    error: ref(null),
    loading: ref(false),
    stale: ref(false),
    canCopy: ref(true),
    resolveVariables: ref(true),
    includeSecrets: ref(false),
    copy: async () => {},
  }),
}))

import CodeSnippetPanel from './CodeSnippetPanel.vue'

async function render(): Promise<string> {
  const app = createSSRApp(CodeSnippetPanel, { request: { id: 'r1', protocol: 'http' } })
  app.use(createPinia())
  return renderToString(app)
}

function openingTag(html: string, marker: string): string {
  const at = html.indexOf(marker)
  expect(at, marker).toBeGreaterThanOrEqual(0)
  return html.slice(html.lastIndexOf('<', at), html.indexOf('>', at) + 1)
}

describe('Generate code dialog toolbar', () => {
  it('picks the language with the app select, not the system one', async () => {
    const html = await render()

    // reka-ui adds a hidden <select> for forms; only a visible one would be the system picker.
    expect(html).not.toMatch(/<select(?![^>]*aria-hidden="true")/)
    const trigger = openingTag(html, 'aria-label="Language"')
    expect(trigger).toContain('role="combobox"')
    expect(trigger).toContain('h-8')
    expect(trigger).toContain('cursor-pointer')
    expect(html).toMatch(/aria-label="Language"[^]*>Go</)
  })
})
