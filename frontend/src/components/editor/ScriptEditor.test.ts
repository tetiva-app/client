import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from '@vue/server-renderer'

vi.mock('./CodeEditor.vue', () => ({ __esModule: true, default: { render: () => null } }))
vi.mock('@/components/ui/HelpLink.vue', () => ({ default: { render: () => null } }))

import ScriptEditor from './ScriptEditor.vue'
import { setCurrentLocale } from '@/lib/locale'

afterEach(() => { setCurrentLocale('en') })

async function render(props: Record<string, unknown> = {}): Promise<string> {
  const html = await renderToString(createSSRApp(defineComponent({
    render: () => h(ScriptEditor, { entityId: 'c1', preScript: '', postScript: '', ...props }),
  })))
  return html.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim()
}

describe('ScriptEditor', () => {
  it('names the phases in English', async () => {
    expect(await render()).toBe('Pre-request Post-response Runs before each request')
  })

  it('names the phases in Russian', async () => {
    setCurrentLocale('ru')
    expect(await render()).toBe('Перед запросом После ответа Выполняется перед каждым запросом')
  })

  it('names the WebSocket phase in one language', async () => {
    expect(await render({ phases: ['pre'], connection: true })).toBe('Pre-connect Runs before each connection')
    setCurrentLocale('ru')
    expect(await render({ phases: ['pre'], connection: true })).toBe('Перед подключением Выполняется перед каждым подключением')
  })
})
