import { describe, it, expect, afterEach } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import type { HistoryRecord } from '@/types/history'
import type { Example } from '@/types/example'
import type { Protocol } from '@/types/request'
import { setCurrentLocale } from '@/lib/locale'
import type { CandidatesStatus, EarlierSend } from './useCompareCandidates'
import CompareBaselineList from './CompareBaselineList.vue'
import LatencyDelta from './LatencyDelta.vue'

function today(hours: number, minutes: number, seconds: number): string {
  const at = new Date()
  at.setHours(hours, minutes, seconds, 0)
  return at.toISOString()
}

const record = (over: Partial<HistoryRecord> = {}): HistoryRecord => ({
  id: 'h1', requestId: 'r1', workspaceId: 'ws-1', protocol: 'http', method: 'GET',
  url: 'https://api.staging.dostavka.dev/v2/orders/ord_7Hq2kP9x', requestHeaders: {}, requestBody: '',
  responseStatus: 200, responseHeaders: {}, responseBody: '{}', responseSize: 612, durationMs: 142,
  createdAt: today(14, 2, 31), ...over,
})

const example: Example = {
  id: 'e1', requestId: 'r1', name: 'Assigned order', statusCode: 200, statusText: 'OK', headers: [],
  body: '{"id":1}', contentType: 'application/json', protocol: 'http', sortOrder: 0, version: 1, createdAt: '', updatedAt: '',
}

interface Props {
  status?: CandidatesStatus
  sends?: EarlierSend[]
  examples?: Example[]
  totalCount?: number
  showAll?: boolean
  protocol?: Protocol
}

function render(props: Props = {}): Promise<string> {
  const app = createSSRApp(CompareBaselineList, {
    name: 'Get order',
    protocol: props.protocol ?? 'http',
    status: props.status ?? 'ready',
    sends: props.sends ?? [],
    examples: props.examples ?? [],
    totalCount: props.totalCount ?? 1,
    showAll: props.showAll ?? true,
  })
  return renderToString(app)
}

const text = (html: string) => html.replace(/<!--.*?-->/g, '').replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ')

afterEach(() => { setCurrentLocale('en') })

describe('compare baseline list', () => {
  it('explains the first response and still lists the examples group', async () => {
    const html = text(await render())

    expect(html).toContain('Compare current response with…')
    expect(html).toContain('This is the first response of “Get order”.')
    expect(html).toContain('Every send is saved to History. Send once more, and the earlier response will appear here.')
    expect(html).toContain('Saved examples')
    expect(html).toContain('None for this request.')
    expect(html).not.toContain('sends in History')
  })

  it('lists earlier sends with time, status, duration, size and host', async () => {
    const html = text(await render({
      sends: [
        { record: record(), differed: false },
        { record: record({ id: 'h0', responseStatus: 404, durationMs: 38, responseSize: 96 }), differed: true },
      ],
      totalCount: 24,
    }))

    expect(html).toContain('Earlier sends of “ Get order ”')
    expect(html).toContain('Today 14:02:31 200 142ms 612 B api.staging.dostavka.dev')
    expect(html).toContain('404 38ms 96 B api.staging.dostavka.dev request differed')
    expect(html.match(/request differed/g)).toHaveLength(1)
    expect(html).toContain('All 24 sends in History, compare any two')
  })

  it('names gRPC statuses and colours NOT_FOUND as a warning', async () => {
    const html = await render({
      protocol: 'grpc',
      sends: [{ record: record({ protocol: 'grpc', url: 'grpc://orders.internal:443', responseStatus: 5 }), differed: false }],
    })

    expect(html).toMatch(/style="color:var\(--gc-warning\);[^"]*"[^>]*>NOT_FOUND</)
    expect(text(html)).toContain('orders.internal:443')
  })

  it('keeps the request name out of the upper-case group title', async () => {
    const html = await render({ sends: [{ record: record(), differed: false }] })

    expect(html).toContain('<span class="normal-case">Get order</span>')
  })

  it('links to History only when it holds two sends or more', async () => {
    const html = await render({ sends: [{ record: record(), differed: false }], totalCount: 1 })

    expect(html).not.toContain('in History')
  })

  it('hides the History link without a handler', async () => {
    const html = await render({ sends: [{ record: record(), differed: false }], totalCount: 24, showAll: false })

    expect(html).not.toContain('sends in History')
  })

  it('lists saved examples with their tag', async () => {
    const html = text(await render({ examples: [example] }))

    expect(html).toContain('Assigned order 200 — 8 B example')
    expect(html).not.toContain('None for this request.')
  })

  it('shows skeleton rows while loading', async () => {
    const html = await render({ status: 'loading' })

    expect(html.match(/data-testid="baseline-skeleton"/g)).toHaveLength(3)
    expect(html).not.toContain('first response')
  })

  it('offers a retry when History fails to load', async () => {
    const html = text(await render({ status: 'failed' }))

    expect(html).toContain('Couldn&#39;t load History')
    expect(html).toContain('Try again')
  })

  it('speaks Russian', async () => {
    setCurrentLocale('ru')
    const html = text(await render({
      sends: [{ record: record(), differed: true }],
      examples: [example],
      totalCount: 24,
    }))

    expect(html).toContain('Сравнить текущий ответ с…')
    expect(html).toContain('Прошлые отправки запроса « Get order »')
    expect(html).toContain('Сегодня 14:02:31 200 142 мс 612 Б')
    expect(html).toContain('запрос был другим')
    expect(html).toContain('Сохранённые примеры')
    expect(html).toContain('пример')
    expect(html).toContain('24 отправки в истории: сравните любые две')
  })
})

describe('latency delta', () => {
  const delta = (props: { a?: number; b?: number; against?: 'previous' | 'a' }) => renderToString(createSSRApp(LatencyDelta, props))

  it('is hidden without a previous duration', async () => {
    expect(text(await delta({ b: 318 })).trim()).toBe('')
  })

  it('is hidden when nothing changed', async () => {
    expect(text(await delta({ a: 142, b: 142 })).trim()).toBe('')
  })

  it('shows a slower send as a warning', async () => {
    const html = await delta({ a: 142, b: 318 })

    expect(text(html)).toContain('+176')
    expect(html).toContain('data-trend="up"')
    expect(html).toContain('title="176 ms slower than the previous send"')
    expect(html).toContain('aria-label="176 ms slower than the previous send"')
  })

  it('names side A as the baseline inside a compare', async () => {
    expect(await delta({ a: 318, b: 139, against: 'a' })).toContain('title="179 ms faster than A"')
  })

  it('explains the delta in Russian', async () => {
    setCurrentLocale('ru')

    expect(await delta({ a: 142, b: 318 })).toContain('title="На 176\u00a0мс медленнее прошлой отправки"')
    expect(await delta({ a: 318, b: 139, against: 'a' })).toContain('title="На 179\u00a0мс быстрее, чем A"')
  })

  it('shows a faster send with a minus sign', async () => {
    const html = await delta({ a: 318, b: 139 })

    expect(text(html)).toContain('−179')
    expect(html).toContain('data-trend="down"')
  })
})
