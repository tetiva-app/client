import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { HistoryRecord } from '@/types/history'

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))

vi.mock('@/components/ui/tabs', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  return { Tabs: inline('div'), TabsList: inline('div'), TabsTrigger: inline('button'), TabsContent: inline('section') }
})

import HistoryViewer from './HistoryViewer.vue'
import { useHistoryStore } from '@/stores/history'
import { setCurrentLocale } from '@/lib/locale'

const FAILED: HistoryRecord = {
  id: 'h1', workspaceId: 'w1', protocol: 'http', method: 'POST', url: 'https://api.example.com/pets',
  requestHeaders: {}, requestBody: '', responseStatus: 0, responseHeaders: {}, responseBody: '',
  responseSize: 48213, durationMs: 45, errorMessage: 'dial tcp: connection refused', createdAt: '',
}

function render(selected: HistoryRecord | null): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  useHistoryStore().selectedRecord = selected
  const app = createSSRApp(HistoryViewer)
  app.use(pinia)
  return renderToString(app)
}

afterEach(() => { setCurrentLocale('en') })

describe('history record view', () => {
  it('keeps its English wording', async () => {
    const html = await render(FAILED)

    expect(html).toContain('ERR')
    expect(html).toContain('45ms')
    expect(html).toContain('Replay')
    expect(html).toContain('title="Delete"')
    for (const label of ['Request', 'Response', 'Error', 'Headers', 'Body']) expect(html).toContain(label)
    expect(html).toContain('(no headers)')
    expect(html).toContain('(empty)')
    expect(html).toContain('(47 KB)')
  })

  it('speaks Russian when the app does', async () => {
    setCurrentLocale('ru')
    const html = await render(FAILED)

    expect(html).toContain('ERR')
    expect(html).toContain('45\u00a0мс')
    expect(html).toContain('Повторить')
    expect(html).toContain('title="Удалить"')
    for (const label of ['Запрос', 'Ответ', 'Ошибка', 'Заголовки', 'Тело']) expect(html).toContain(label)
    expect(html).toContain('(нет заголовков)')
    expect(html).toContain('(пусто)')
    expect(html).toContain('(47 КБ)')
    expect(html).not.toMatch(/Replay|Headers|KB/)
  })

  it('prompts to pick a record in both languages', async () => {
    expect(await render(null)).toContain('Select a history record to view details.')
    setCurrentLocale('ru')
    expect(await render(null)).toContain('Выберите запись в истории, чтобы посмотреть подробности.')
  })
})
