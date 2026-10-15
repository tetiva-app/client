import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { HistoryRecord } from '@/types/history'
import type { Request } from '@/types/request'

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))

vi.mock('@/stores/workspace', () => ({
  useWorkspaceStore: () => ({ activeWorkspace: { id: 'ws-1' } }),
}))

vi.mock('./DiffPane.vue', () => ({
  default: defineComponent({
    props: ['a', 'b', 'language', 'mode'],
    setup: (props) => () => h('div', { 'data-pane': '', 'data-mode': props.mode, 'data-a': props.a, 'data-b': props.b }),
  }),
}))

import HistoryCompareView from './HistoryCompareView.vue'
import { useHistoryStore } from '@/stores/history'
import { useRequestStore } from '@/stores/tabs'
import { useResponseCompareStore } from '@/stores/responseCompare'
import { setCurrentLocale } from '@/lib/locale'

function today(hours: number, minutes: number, seconds: number): string {
  const at = new Date()
  at.setHours(hours, minutes, seconds, 0)
  return at.toISOString()
}

const record = (id: string, over: Partial<HistoryRecord> = {}): HistoryRecord => ({
  id, requestId: 'r1', workspaceId: 'ws-1', protocol: 'http', method: 'GET',
  url: 'https://api.staging.dostavka.dev/v2/orders/ord_7?expand=courier,items',
  requestHeaders: { 'Accept-Language': ['ru'], Authorization: ['Bearer secret-a'] }, requestBody: '',
  responseStatus: 200, responseHeaders: { 'Content-Type': ['application/json'] },
  responseBody: '{"id":"ord_7","status":"assigned"}', responseSize: 612, durationMs: 142,
  createdAt: today(14, 2, 31), ...over,
})

const older = record('h1', {
  url: 'http://localhost:8080/v2/orders/ord_7?expand=courier,items',
  requestHeaders: { 'Accept-Language': ['en'], Authorization: ['Bearer secret-b'] },
  responseBody: '{"id":"ord_7","status":"picked_up"}', durationMs: 121, createdAt: today(9, 5, 22),
})
const newer = record('h2')

interface Setup {
  pair?: [HistoryRecord, HistoryRecord]
  request?: Partial<Request> | null
}

function render(setup: Setup = {}): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  const history = useHistoryStore()
  history.items = setup.pair ?? [newer, older]
  for (const r of setup.pair ?? [older, newer]) history.toggleCompareSelection(r)
  history.openComparePair()
  if (setup.request !== null) {
    useRequestStore().loadRequest({ id: 'r1', name: 'Get order', ...setup.request } as Request)
  }
  useResponseCompareStore().setViewMode('split')
  const app = createSSRApp(HistoryCompareView)
  app.use(pinia)
  return renderToString(app)
}

const text = (html: string) => html.replace(/<!--.*?-->/g, '').replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ')

afterEach(() => { setCurrentLocale('en') })

describe('history compare view', () => {
  it('titles the pair with the request name and shows method and path', async () => {
    const html = text(await render())

    expect(html).toContain('Compare · Get order')
    expect(html).toContain('GET /v2/orders/ord_7')
  })

  it('falls back to method and path for a draft or a missing request', async () => {
    expect(text(await render({ request: { isDraft: true } }))).toContain('Compare · GET /v2/orders/ord_7')
    expect(text(await render({ request: null }))).toContain('Compare · GET /v2/orders/ord_7')
  })

  it('puts the older record on A with its time, host, status and duration', async () => {
    const html = text(await render())

    expect(html).toMatch(/A Today 09:05:22 · localhost:8080 · 200 121ms .*B Today 14:02:31 · api\.staging\.dostavka\.dev · 200 142ms/)
  })

  it('opens on the request tab with the changed fields counted', async () => {
    const html = await render()

    expect(html).toContain('data-testid="request-diff"')
    expect(text(html)).toContain('Request 2')
    expect(html).not.toContain('Bearer secret')
  })

  it('highlights only the changed part of a request value', async () => {
    const html = await render()

    expect(html).toMatch(/<span class="[^"]*gc-error[^"]*">localhost:8080<\/span>/)
    expect(html).toMatch(/<span class="[^"]*gc-success[^"]*">api\.staging\.dostavka\.dev<\/span>/)
    expect(html).toMatch(/<span class="[^"]*gc-error[^"]*">en<\/span>/)
  })

  it('says the requests differ, so a different response is expected', async () => {
    const html = text(await render())

    expect(html).toContain('The requests differ in 2 fields, so differences in the response are expected.')
    expect(html).toContain('Response: 200 = 200 · body: 1 change · headers: no changes · 121ms vs 142ms')
    expect(html).toContain('Open response body')
    expect(html).toContain('Secret headers and parameters are hidden and not compared.')
  })

  it('says the compared fields match', async () => {
    const html = text(await render({ pair: [record('h2'), record('h1', { createdAt: today(9, 0, 0), responseStatus: 404 })] }))

    expect(html).toContain('The compared request fields match.')
    expect(html).toContain('Response: 404 ≠ 200')
  })

  it('says when the body was not compared', async () => {
    const binary = record('h1', { createdAt: today(9, 0, 0), responseBody: '', responseSize: 2048 })
    const html = text(await render({ pair: [record('h2'), binary] }))

    expect(html).toContain('· body not compared ·')
  })

  it('shows a record with no response', async () => {
    const failed = record('h1', { createdAt: today(9, 0, 0), responseStatus: 0, errorMessage: 'connection refused', responseBody: '', responseSize: 0 })
    const html = text(await render({ pair: [record('h2'), failed] }))

    expect(html).toContain('No response: connection refused')
  })

  it('offers normalization, Split and Unified for the response tabs', async () => {
    const html = await render()

    expect(html).toContain('Normalize JSON')
    expect(html).toContain('aria-label="Split"')
    expect(html).toContain('aria-label="Unified"')
    expect(html).toContain('data-testid="compare-nav"')
  })

  it('offers a close button', async () => {
    expect(await render()).toContain('aria-label="Close compare"')
  })

  it('speaks Russian', async () => {
    setCurrentLocale('ru')
    const html = text(await render())

    expect(html).toContain('Сравнение · Get order')
    expect(html).toContain('Запрос')
    expect(html).toContain('Тело ответа')
    expect(html).toContain('Заголовки ответа')
    expect(html).toContain('Запросы различаются в 2 полях, поэтому разница в ответах ожидаема.')
    expect(html).toContain('Открыть тело ответа')
    expect(html).toContain('тело: 1 изменение · заголовки: без изменений · 121 мс и 142 мс')
  })
})
