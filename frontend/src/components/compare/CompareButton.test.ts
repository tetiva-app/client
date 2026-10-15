import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { ExecuteResponse } from '@/types/execute'
import { tagWith } from '@/test-utils/markup'
import { setCurrentLocale } from '@/lib/locale'

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))

import CompareButton from './CompareButton.vue'

const response: ExecuteResponse = {
  statusCode: 200, statusText: 'OK', url: 'https://api.example.com', headers: {}, body: '{}', size: 2, durationMs: 142, historyId: 'h1',
}

function render(): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  const app = createSSRApp({
    render: () => h(CompareButton, { class: 'ml-auto', requestId: 'r1', protocol: 'http', response }),
  })
  app.use(pinia)
  return renderToString(app)
}

afterEach(() => { setCurrentLocale('en') })

describe('compare button', () => {
  it('takes the status bar placement and names both of its parts', async () => {
    const html = await render()

    expect(tagWith(html, 'data-testid="compare-button"')).toContain('ml-auto')
    expect(html).toContain('title="Compare with the previous send"')
    expect(html).toContain('aria-label="Compare with another response"')
  })

  it('speaks Russian', async () => {
    setCurrentLocale('ru')
    const html = await render()

    expect(html).toContain('Сравнить')
    expect(html).toContain('title="Сравнить с предыдущей отправкой"')
    expect(html).toContain('aria-label="Сравнить с другим ответом"')
  })
})
