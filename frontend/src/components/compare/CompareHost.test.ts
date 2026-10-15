import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h, nextTick, type PropType } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import type { ExecuteResponse } from '@/types/execute'
import type { CompareSession } from '@/stores/responseCompare'
import type { CompareSide } from '@/lib/compare/sides'

vi.mock('@/services', () => ({
  getHistoryService: async () => ({ getById: () => new Promise(() => {}) }),
}))

vi.mock('@/stores/workspace', () => ({
  useWorkspaceStore: () => ({ activeWorkspace: { id: 'ws-1' } }),
}))

vi.mock('./ResponseCompareView.vue', () => ({
  __esModule: true,
  default: defineComponent({
    props: {
      requestId: String,
      protocol: String,
      session: Object as PropType<CompareSession>,
      loading: Boolean,
    },
    setup: (props) => () => h('section', {
      'data-compare-view': props.requestId,
      'data-protocol': props.protocol,
      'data-current': props.session?.current.historyId,
      'data-loading': String(props.loading),
    }),
  }),
}))

import CompareHost from './CompareHost.vue'
import { useResponseStore } from '@/stores/responses'
import { useResponseCompareStore } from '@/stores/responseCompare'
import { setCurrentLocale } from '@/lib/locale'

const response = (historyId: string): ExecuteResponse => ({
  statusCode: 200, statusText: 'OK', url: 'https://api.example.com', headers: {}, body: '{}', size: 2, durationMs: 120, historyId,
})

const baseline: CompareSide = {
  origin: 'history', id: 'h0', protocol: 'http', status: 200, size: 2, headers: {}, body: '{}',
  contentType: 'application/json', isBinary: false,
}

let pinia: Pinia

function render(): Promise<string> {
  const app = createSSRApp({
    render: () => h(CompareHost, { requestId: 'r1', protocol: 'graphql' }, () => h('div', { 'data-editor': '' }, 'editor')),
  })
  app.use(pinia)
  return renderToString(app)
}

async function settle() {
  await nextTick()
  await Promise.resolve()
  await nextTick()
}

function slotHidden(html: string): boolean {
  return /<div class="[^"]*\bhidden\b[^"]*">(<!--\[-->)?<div data-editor/.test(html)
}

beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
})

afterEach(() => { setCurrentLocale('en') })

describe('compare host', () => {
  it('renders only the editor without a session', async () => {
    useResponseStore().setResponse('r1', { status: 'success', data: response('h1') })
    const html = await render()

    expect(html).toContain('data-editor')
    expect(slotHidden(html)).toBe(false)
    expect(html).not.toContain('data-compare-view')
    expect(html).not.toContain('Request editor is collapsed while comparing')
  })

  it('collapses the editor and shows the compare view for an open session', async () => {
    useResponseStore().setResponse('r1', { status: 'success', data: response('h1') })
    useResponseCompareStore().open('r1', { baseline, current: response('h1'), currentRecord: null })
    const html = await render()

    expect(slotHidden(html)).toBe(true)
    expect(html).toContain('Request editor is collapsed while comparing')
    expect(html).toContain('Show')
    expect(html).toContain('data-compare-view="r1"')
    expect(html).toContain('data-protocol="graphql"')
    expect(html).toContain('data-current="h1"')
    expect(html).toContain('data-loading="false"')
  })

  it('keeps comparing through a re-send and shows the new response', async () => {
    const responses = useResponseStore()
    responses.setResponse('r1', { status: 'success', data: response('h1') })
    useResponseCompareStore().open('r1', { baseline, current: response('h1'), currentRecord: null })

    responses.setResponse('r1', { status: 'loading', startedAt: 1 })
    await settle()
    const loading = await render()
    expect(loading).toContain('data-current="h1"')
    expect(loading).toContain('data-loading="true"')
    expect(slotHidden(loading)).toBe(true)

    responses.setResponse('r1', { status: 'success', data: response('h2') })
    await settle()
    const done = await render()
    expect(done).toContain('data-current="h2"')
    expect(done).toContain('data-loading="false"')
  })

  it('returns to the editor when the send is cancelled', async () => {
    const responses = useResponseStore()
    responses.setResponse('r1', { status: 'success', data: response('h1') })
    useResponseCompareStore().open('r1', { baseline, current: response('h1'), currentRecord: null })
    responses.setResponse('r1', { status: 'loading', startedAt: 1 })
    await settle()

    responses.cancelRequest('r1')
    await settle()
    const html = await render()

    expect(useResponseCompareStore().isOpen('r1')).toBe(false)
    expect(slotHidden(html)).toBe(false)
    expect(html).not.toContain('data-compare-view')
  })

  it('speaks Russian in the collapsed bar', async () => {
    setCurrentLocale('ru')
    useResponseStore().setResponse('r1', { status: 'success', data: response('h1') })
    useResponseCompareStore().open('r1', { baseline, current: response('h1'), currentRecord: null })
    const html = await render()

    expect(html).toContain('Редактор запроса свёрнут на время сравнения')
    expect(html).toContain('Показать')
  })
})
