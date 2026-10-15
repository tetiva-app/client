import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { ExecuteResponse } from '@/types/execute'
import type { HistoryRecord } from '@/types/history'
import type { Protocol } from '@/types/request'
import { sideFromExample, sideFromHistory, type CompareSide } from '@/lib/compare/sides'
import type { CompareViewMode } from '@/stores/responseCompare'

vi.mock('@/services', () => ({ getHistoryService: async () => ({ getById: vi.fn() }) }))

vi.mock('@/stores/workspace', () => ({
  useWorkspaceStore: () => ({ activeWorkspace: { id: 'ws-1' } }),
}))

vi.mock('./DiffPane.vue', () => ({
  default: defineComponent({
    props: ['a', 'b', 'language', 'mode'],
    setup: (props) => () => h('div', {
      'data-pane': '',
      'data-language': props.language,
      'data-mode': props.mode,
      'data-a': props.a,
      'data-b': props.b,
    }),
  }),
}))

import ResponseCompareView from './ResponseCompareView.vue'
import { useResponseStore } from '@/stores/responses'
import { useResponseCompareStore } from '@/stores/responseCompare'
import { setCurrentLocale } from '@/lib/locale'

function today(hours: number, minutes: number, seconds: number): string {
  const at = new Date()
  at.setHours(hours, minutes, seconds, 0)
  return at.toISOString()
}

const record = (over: Partial<HistoryRecord> = {}): HistoryRecord => ({
  id: 'h1',
  requestId: 'r1',
  workspaceId: 'ws-1',
  protocol: 'http',
  method: 'GET',
  url: 'https://api.example.com/v2/orders/1',
  requestHeaders: { Accept: ['application/json'], Authorization: ['Bearer a'] },
  requestBody: '',
  responseStatus: 200,
  responseHeaders: { 'Content-Type': ['application/json'] },
  responseBody: '{"id":1,"status":"assigned"}',
  responseSize: 28,
  durationMs: 142,
  createdAt: today(14, 2, 31),
  authQueryKeys: [],
  authHeaderKeys: [],
  ...over,
})

const response = (over: Partial<ExecuteResponse> = {}): ExecuteResponse => ({
  statusCode: 200,
  statusText: 'OK',
  url: 'https://api.example.com/v2/orders/1',
  headers: { 'Content-Type': ['application/json'] },
  body: '{"id":1,"status":"picked_up"}',
  size: 29,
  durationMs: 318,
  historyId: 'h2',
  ...over,
})

interface Setup {
  baseline?: CompareSide
  current?: ExecuteResponse
  currentRecord?: HistoryRecord | null
  protocol?: Protocol
  viewMode?: CompareViewMode
  loading?: boolean
  normalize?: boolean
  tab?: 'body' | 'headers' | 'request'
}

function render(setup: Setup = {}): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  const current = setup.current ?? response()
  useResponseStore().setResponse('r1', { status: 'success', data: current })
  const compare = useResponseCompareStore()
  compare.open('r1', {
    baseline: setup.baseline ?? sideFromHistory(record()),
    current,
    currentRecord: setup.currentRecord === undefined ? record({ id: 'h2', createdAt: today(14, 9, 47) }) : setup.currentRecord,
  })
  if (setup.normalize === false) compare.setNormalize('r1', false)
  if (setup.tab) compare.setTab('r1', setup.tab)
  compare.setViewMode(setup.viewMode ?? 'split')
  const app = createSSRApp(ResponseCompareView, {
    requestId: 'r1',
    protocol: setup.protocol ?? 'http',
    session: compare.sessions.get('r1'),
    loading: setup.loading ?? false,
  })
  app.use(pinia)
  return renderToString(app)
}

const text = (html: string) => html.replace(/<!--.*?-->/g, '').replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ')

afterEach(() => { setCurrentLocale('en') })

describe('response compare view', () => {
  it('shows both sides in the bar with status, duration and the latency delta', async () => {
    const html = text(await render())

    expect(html).toContain('A Today 14:02:31 200 142ms')
    expect(html).toContain('B Now 200 318ms +176')
  })

  it('captions the split columns', async () => {
    const html = text(await render())

    expect(html).toContain('A Today 14:02:31 · from History')
    expect(html).toContain('B Current response · Today 14:09:47')
  })

  it('diffs the normalized JSON bodies in the chosen mode', async () => {
    const html = await render({ viewMode: 'unified' })

    expect(html).toContain('data-language="json"')
    expect(html).toContain('data-mode="unified"')
    expect(text(html)).not.toContain('from History')
  })

  it('waits for the panel width before picking a mode on its own', async () => {
    const html = await render({ viewMode: 'auto' })

    expect(html).not.toContain('data-pane')
  })

  it('says the bodies are identical', async () => {
    const html = await render({ current: response({ body: '{"id":1,"status":"assigned"}' }) })

    expect(html).toContain('Bodies are identical')
    expect(html).not.toContain('data-pane')
  })

  it('offers the raw compare when only normalization makes the bodies equal', async () => {
    const html = await render({
      baseline: sideFromHistory(record({ responseBody: '{"status":"assigned","id":1,"updated_at":"2026-10-10T14:02:31Z"}' })),
      current: response({ body: '{"id":1,"updated_at":"2026-10-10T14:09:47Z","status":"assigned"}' }),
    })

    expect(html).toContain('No differences once keys are sorted and volatile values are masked')
    expect(html).toContain('Compare without normalization')
    expect(html).toContain('Masked:')
  })

  it('offers the raw compare when the bodies differ only in number formatting', async () => {
    const html = await render({
      baseline: sideFromHistory(record({ responseBody: '{"id":1.0,"status":"assigned"}' })),
      current: response({ body: '{"id":1,"status":"assigned"}' }),
    })

    expect(html).toContain('Compare without normalization')
  })

  it('does not compare a binary body', async () => {
    const html = text(await render({ current: response({ body: '', isBinary: true, size: 2048 }) }))

    expect(html).toContain('Binary body: not compared (28 B → 2 KB)')
    expect(html).toMatch(/Body —/)
  })

  it('does not compare bodies over 1 MB', async () => {
    const html = text(await render({ current: response({ size: 3 * 1024 * 1024 }) }))

    expect(html).toContain('Bodies over 1 MB are not compared here (28 B, 3 MB). Headers and the request still are.')
  })

  it('says both bodies are empty', async () => {
    const html = await render({
      baseline: sideFromHistory(record({ responseBody: '', responseSize: 0 })),
      current: response({ body: '', size: 0 }),
    })

    expect(html).toContain('Both bodies are empty')
  })

  it('compares JSON against HTML as plain text', async () => {
    const html = await render({
      current: response({ headers: { 'Content-Type': ['text/html'] }, body: '<html>oops</html>' }),
    })

    expect(html).toContain('data-language="text"')
  })

  it('marks the request as the same when the compared fields match', async () => {
    const html = text(await render())

    expect(html).toContain('Request same')
  })

  it('counts changed request fields', async () => {
    const html = text(await render({
      currentRecord: record({ id: 'h2', url: 'http://localhost:8080/v2/orders/1', requestHeaders: { Accept: ['text/plain'], Authorization: ['Bearer b'] } }),
    }))

    expect(html).toContain('Request 2')
  })

  it('marks the request as unavailable when the current record is missing', async () => {
    const html = await render({ currentRecord: null, current: response({ historyId: undefined }), tab: 'request' })

    expect(text(html)).toContain('Request —')
    expect(html).toContain("The request of the current response isn&#39;t in History")
  })

  it('names a saved example and says it keeps no request', async () => {
    const example = sideFromExample({
      id: 'e1', requestId: 'r1', name: 'Assigned order', protocol: 'http', statusCode: 200, statusText: 'OK',
      headers: [{ key: 'Content-Type', value: 'application/json', enabled: true }],
      body: '{"id":1,"status":"assigned"}', contentType: 'application/json',
      sortOrder: 0, version: 1, createdAt: '', updatedAt: '',
    })

    expect(text(await render({ baseline: example }))).toContain('A Assigned order · example')
    expect(await render({ baseline: example, tab: 'request' })).toContain("Saved examples don&#39;t keep the request")
  })

  it('lists the request fields, hiding secret ones', async () => {
    const html = text(await render({ tab: 'request' }))

    expect(html).toContain('Header · Authorization secret, not compared')
    expect(html).not.toContain('Bearer a')
  })

  it('shows the metadata tab and gRPC status names', async () => {
    const html = text(await render({
      protocol: 'grpc',
      baseline: sideFromHistory(record({ protocol: 'grpc', responseStatus: 5 })),
      current: response({ statusCode: 0 }),
    }))

    expect(html).toContain('Metadata')
    expect(html).not.toContain('Headers')
    expect(html).toContain('NOT_FOUND')
    expect(html).toContain('OK')
  })

  it('compares the headers when their tab is open', async () => {
    const html = await render({
      tab: 'headers',
      current: response({ headers: { 'Content-Type': ['application/json'], ETag: ['W/"c9"'] } }),
    })

    expect(html).toContain('data-a="Content-Type: application/json"')
    expect(html).toContain('ETag: W/&quot;c9&quot;')
  })

  it('says the headers are identical', async () => {
    expect(await render({ tab: 'headers' })).toContain('Headers are identical')
  })

  it('says the headers differ only in masked values and offers the raw compare', async () => {
    const html = await render({
      tab: 'headers',
      baseline: sideFromHistory(record({ responseHeaders: { 'Content-Type': ['application/json'], Date: ['Sat, 10 Oct 2026 14:02:31 GMT'] } })),
      current: response({ headers: { 'Content-Type': ['application/json'], Date: ['Sat, 10 Oct 2026 14:09:47 GMT'] } }),
    })

    expect(html).toContain('No differences apart from Date and X-Request-Id')
    expect(html).toContain('Compare without normalization')
  })

  it('switches Split and Unified with named icon buttons', async () => {
    const html = await render()

    expect(html).toContain('aria-label="Split"')
    expect(html).toContain('aria-label="Unified"')
    expect(text(html)).not.toMatch(/Split|Unified/)
  })

  it('keeps the change navigation in place on a tab without a diff', async () => {
    const nav = (html: string) => /<span[^>]*data-testid="compare-nav"[^>]*>/.exec(html)?.[0] ?? ''

    expect(nav(await render({ tab: 'request' }))).toContain('invisible')
    expect(nav(await render({ tab: 'headers' }))).toContain('invisible')
  })

  it('lists only the masked fields that occur', async () => {
    const html = text(await render({
      baseline: sideFromHistory(record({ responseBody: '{"id":1,"updated_at":"2026-10-10T14:02:31Z"}' })),
      current: response({ body: '{"id":2,"updated_at":"2026-10-10T14:09:47Z"}' }),
    }))
    const headersOnly = text(await render({
      current: response({ headers: { 'Content-Type': ['application/json'], 'X-Request-Id': ['r-2'] } }),
    }))

    expect(html).toContain('Masked: *_at , *At')
    expect(html).not.toContain('X-Request-Id')
    expect(headersOnly).toContain('Masked: X-Request-Id')
    expect(headersOnly).not.toContain('*_at')
  })

  it('shows no masked note on the request tab', async () => {
    const html = await render({
      tab: 'request',
      baseline: sideFromHistory(record({ responseBody: '{"id":1,"updated_at":"2026-10-10T14:02:31Z"}' })),
      current: response({ body: '{"id":2,"updated_at":"2026-10-10T14:09:47Z"}' }),
    })

    expect(html).not.toContain('Masked:')
  })

  it('shows a progress bar while the response reloads', async () => {
    expect(await render({ loading: true })).toContain('data-testid="compare-progress"')
    expect(await render()).not.toContain('data-testid="compare-progress"')
  })

  it('speaks Russian', async () => {
    setCurrentLocale('ru')
    const html = await render()

    for (const label of ['Сейчас', 'Тело', 'Заголовки', 'Запрос', 'Нормализовать JSON', 'Две колонки', 'Одна колонка', 'Текущий ответ', 'из истории']) {
      expect(html).toContain(label)
    }
    expect(html).toContain('title="Закрыть сравнение"')
    expect(html).toContain('title="Поменять местами"')
    expect(html).toContain('318 мс')
  })

  it('says in Russian what normalization leaves out', async () => {
    setCurrentLocale('ru')
    const html = text(await render({
      baseline: sideFromHistory(record({ responseBody: '{"status":"assigned","id":1,"updated_at":"2026-10-10T14:02:31Z"}' })),
      current: response({ body: '{"id":1,"updated_at":"2026-10-10T14:09:47Z","status":"assigned"}' }),
    }))

    expect(html).toContain('Не учитываются: *_at , *At')
    expect(html).toContain('Различий нет без учёта порядка ключей и изменчивых значений')
  })
})
