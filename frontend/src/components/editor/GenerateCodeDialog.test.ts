import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h, nextTick, type App } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { HTTPMethod, Protocol, Request } from '@/types/request'
import { mountWindow } from '@/test-utils/windows'

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))

vi.mock('@/components/ui/dialog', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  return {
    Dialog: inline('div'),
    DialogContent: inline('div'),
    DialogDescription: inline('p'),
    DialogHeader: inline('div'),
    DialogTitle: inline('h2'),
  }
})

vi.mock('./CodeSnippetPanel.vue', () => ({ default: { render: () => null } }))

import GenerateCodeDialog from './GenerateCodeDialog.vue'
import { useCodeDialogUi } from '@/stores/codeDialog'
import { useRequestStore } from '@/stores/tabs'

function makeRequest(protocol: Protocol, method: HTTPMethod): Request {
  return {
    id: 'r1', collectionId: 'c1', name: 'List pets', description: '', protocol, method,
    url: '/pets', headers: [], body: '', bodyType: 'none', authType: 'none', authData: '{}',
    preScript: '', postScript: '', grpcService: '', grpcMethod: '', grpcProtoPath: '', grpcMetadata: {},
    graphqlQuery: '', graphqlVariables: '', graphqlSchemaPath: '', graphqlOperation: '',
    sortOrder: 0, version: 1, createdAt: '2026-01-01', updatedAt: '2026-01-01',
  }
}

async function subtitle(protocol: Protocol, method: HTTPMethod): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  useRequestStore().requestsMap.set('r1', makeRequest(protocol, method))
  useCodeDialogUi().open('r1')
  const ssr = createSSRApp(GenerateCodeDialog)
  ssr.use(pinia)
  const html = await renderToString(ssr)
  return html.replace(/<!--[^]*?-->/g, '').match(/<p[^>]*>([^]*?)<\/p>/)![1]
}

let app: App | null = null

afterEach(() => {
  app?.unmount()
  app = null
})

describe('generate code dialog', () => {
  it.each([
    ['http', 'GET'],
    ['graphql', 'POST'],
  ] as const)('names a %s request by method and name', async (protocol, method) => {
    const text = await subtitle(protocol, method)

    expect(text).toContain(`${method} · List pets`)
    expect(text).not.toMatch(/GQL|gRPC|WS/)
  })

  it.each([
    ['grpc', 'gRPC'],
    ['websocket', 'WS'],
  ] as const)('marks a %s request with its protocol badge', async (protocol, badge) => {
    const text = await subtitle(protocol, 'GET')

    expect(text).toContain(badge)
    expect(text).toContain('List pets')
    expect(text).not.toMatch(/GET|·/)
  })

  it('closes when its request goes away', async () => {
    let ui!: ReturnType<typeof useCodeDialogUi>
    let requests!: ReturnType<typeof useRequestStore>
    app = mountWindow(GenerateCodeDialog, {}, a => a.runWithContext(() => {
      requests = useRequestStore()
      ui = useCodeDialogUi()
      requests.requestsMap.set('r1', makeRequest('http', 'GET'))
      ui.open('r1')
    }))
    await nextTick()
    expect(ui.requestId).toBe('r1')

    requests.requestsMap.delete('r1')
    await nextTick()

    expect(ui.requestId).toBeNull()
  })

  it('closes on mount when its request is already gone', async () => {
    let ui!: ReturnType<typeof useCodeDialogUi>
    app = mountWindow(GenerateCodeDialog, {}, a => a.runWithContext(() => {
      ui = useCodeDialogUi()
      ui.requestId = 'gone'
    }))
    await nextTick()

    expect(ui.requestId).toBeNull()
  })
})
