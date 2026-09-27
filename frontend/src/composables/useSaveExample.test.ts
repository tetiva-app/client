import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { ExecuteResponse } from '@/types/execute'
import type { MockExampleService } from '@/services/mock-example'

vi.mock('@/services', async () => {
  const { MockExampleService } = await import('@/services/mock-example')
  let service = new MockExampleService()
  return {
    getExampleService: async () => service,
    isWailsEnvironment: () => false,
    __reset: () => { service = new MockExampleService() },
    __service: () => service,
  }
})

vi.mock('@/composables/useWindowEvents', () => ({
  emitWailsEvent: async () => {},
}))

import {
  defaultExampleName,
  exampleFromResponse,
  findContentType,
  saveExampleBlocker,
  useSaveExample,
} from './useSaveExample'
import type { SaveExampleSource } from './useSaveExample'
import { useToast } from './useToast'
import { MAX_EXAMPLE_BODY_BYTES } from '@/lib/example-limits'

async function service(): Promise<MockExampleService> {
  const mod = (await import('@/services')) as unknown as { __service: () => MockExampleService }
  return mod.__service()
}

function response(over: Partial<ExecuteResponse> = {}): ExecuteResponse {
  return {
    statusCode: 200,
    statusText: '200 OK',
    url: 'https://api.example.com/users',
    headers: { 'Content-Type': ['application/json'] },
    body: '{"ok":true}',
    size: 11,
    durationMs: 12,
    ...over,
  }
}

function source(over: Partial<SaveExampleSource> = {}): SaveExampleSource {
  return { requestId: 'r1', protocol: 'http', isDraft: false, response: response(), ...over }
}

describe('defaultExampleName', () => {
  it('uses the code and the reason phrase for HTTP', () => {
    expect(defaultExampleName('http', 404, '404 Not Found')).toBe('404 Not Found')
    expect(defaultExampleName('graphql', 200, '200 OK')).toBe('200 OK')
  })

  it('falls back to the bare code when the reason phrase is missing', () => {
    expect(defaultExampleName('http', 299, '299')).toBe('299')
  })

  it('uses the status name and code for gRPC', () => {
    expect(defaultExampleName('grpc', 5, 'NotFound')).toBe('NOT_FOUND (5)')
    expect(defaultExampleName('grpc', 0, 'OK')).toBe('OK (0)')
  })
})

describe('saveExampleBlocker', () => {
  it('allows any received response, including 4xx and 5xx', () => {
    expect(saveExampleBlocker(source())).toBeNull()
    expect(saveExampleBlocker(source({ response: response({ statusCode: 404, statusText: '404 Not Found' }) }))).toBeNull()
    expect(saveExampleBlocker(source({ response: response({ statusCode: 503, statusText: '503 Service Unavailable' }) }))).toBeNull()
  })

  it('blocks binary responses', () => {
    const src = source({ response: response({ isBinary: true, body: '' }) })
    expect(saveExampleBlocker(src)).toBe("Binary responses can't be saved as examples")
  })

  it('measures the body limit in UTF-8 bytes, not characters', () => {
    const atLimit = 'я'.repeat(MAX_EXAMPLE_BODY_BYTES / 2)
    const overLimit = 'я'.repeat(MAX_EXAMPLE_BODY_BYTES / 2 + 1)
    expect(overLimit.length).toBeLessThan(MAX_EXAMPLE_BODY_BYTES)

    expect(saveExampleBlocker(source({ response: response({ body: atLimit }) }))).toBeNull()
    expect(saveExampleBlocker(source({ response: response({ body: overLimit }) })))
      .toBe('Response is too large for an example (max 256 KB)')
    expect(saveExampleBlocker(source({ response: response({ body: 'a'.repeat(MAX_EXAMPLE_BODY_BYTES + 1) }) })))
      .toBe('Response is too large for an example (max 256 KB)')
  })

  it('blocks draft requests', () => {
    expect(saveExampleBlocker(source({ isDraft: true }))).toBe('Save the request first to keep examples')
  })
})

describe('exampleFromResponse', () => {
  it('turns every header value into its own row', () => {
    const src = source({
      response: response({
        headers: { 'Set-Cookie': ['a=1', 'b=2'], 'X-Request-Id': ['abc'] },
      }),
    })

    expect(exampleFromResponse(src, 'x').headers).toEqual([
      { key: 'Set-Cookie', value: 'a=1', enabled: true },
      { key: 'Set-Cookie', value: 'b=2', enabled: true },
      { key: 'X-Request-Id', value: 'abc', enabled: true },
    ])
  })

  it('finds the content type whatever the header case', () => {
    const src = source({ response: response({ headers: { 'content-type': ['text/xml; charset=utf-8'] } }) })

    expect(exampleFromResponse(src, 'x').contentType).toBe('text/xml; charset=utf-8')
    expect(findContentType([{ key: 'CONTENT-TYPE', value: 'text/plain', enabled: true }])).toBe('text/plain')
    expect(findContentType([])).toBe('')
  })

  it('keeps the raw body, code and reason phrase', () => {
    const src = source({ response: response({ statusCode: 404, statusText: '404 Not Found', body: '{"error":"nope"}' }) })

    expect(exampleFromResponse(src, 'Missing user')).toEqual({
      requestId: 'r1',
      protocol: 'http',
      name: 'Missing user',
      statusCode: 404,
      statusText: 'Not Found',
      headers: [{ key: 'Content-Type', value: 'application/json', enabled: true }],
      body: '{"error":"nope"}',
      contentType: 'application/json',
    })
  })

  it('stores the gRPC status name as the status text', () => {
    const src = source({ protocol: 'grpc', response: response({ statusCode: 5, statusText: 'NotFound', headers: {} }) })

    const example = exampleFromResponse(src, 'x')
    expect(example.statusCode).toBe(5)
    expect(example.statusText).toBe('NOT_FOUND')
    expect(example.protocol).toBe('grpc')
  })
})

describe('useSaveExample', () => {
  beforeEach(async () => {
    setActivePinia(createPinia())
    const mod = (await import('@/services')) as unknown as { __reset: () => void }
    mod.__reset()
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('creates the example and confirms with a toast', async () => {
    const { save, defaultName } = useSaveExample(() => source({
      response: response({ statusCode: 404, statusText: '404 Not Found' }),
    }))
    expect(defaultName.value).toBe('404 Not Found')

    expect(await save('User missing')).toBe(true)

    const listed = await (await service()).list('r1')
    expect(listed.data?.map(e => [e.name, e.statusCode, e.statusText])).toEqual([['User missing', 404, 'Not Found']])
    expect(useToast().toasts.value.some(t => t.kind === 'success' && t.message === 'Saved as example')).toBe(true)
  })

  it('falls back to the default name for a blank one', async () => {
    const { save } = useSaveExample(() => source())

    await save('   ')

    const listed = await (await service()).list('r1')
    expect(listed.data?.[0].name).toBe('200 OK')
  })

  it('does not create an example for a blocked response', async () => {
    const { save, blocker } = useSaveExample(() => source({ isDraft: true }))
    expect(blocker.value).toBe('Save the request first to keep examples')

    expect(await save('x')).toBe(false)

    const listed = await (await service()).list('r1')
    expect(listed.data).toEqual([])
  })

  it('asks before saving a response that looks like it holds a credential', async () => {
    const jwt = 'eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.c2lnbmF0dXJl'
    const src = source({ response: response({ body: `{"access_token":"${jwt}"}` }) })
    const { save, saveAnyway, secretsOpen, secretLabels } = useSaveExample(() => src)
    const svc = await service()
    const scan = vi.spyOn(svc, 'scanSecrets')

    expect(await save('Login')).toBe(false)

    expect(scan).toHaveBeenCalledWith({ headers: [{ key: 'Content-Type', value: 'application/json', enabled: true }], body: `{"access_token":"${jwt}"}` })
    expect(secretsOpen.value).toBe(true)
    expect(secretLabels.value).toEqual(['JWT', 'OAuth token'])
    expect((await svc.list('r1')).data).toEqual([])

    expect(await saveAnyway()).toBe(true)

    expect(secretsOpen.value).toBe(false)
    expect((await svc.list('r1')).data?.map(e => e.name)).toEqual(['Login'])
  })

  it('saves nothing when the secrets prompt is dismissed', async () => {
    const src = source({ response: response({ body: '{"key":"AKIAIOSFODNN7EXAMPLE"}' }) })
    const { save, secretsOpen } = useSaveExample(() => src)

    expect(await save('Keys')).toBe(false)
    secretsOpen.value = false

    expect((await (await service()).list('r1')).data).toEqual([])
  })

  it('saves at once when the scan finds nothing', async () => {
    const { save, secretsOpen } = useSaveExample(() => source())

    expect(await save('Plain')).toBe(true)
    expect(secretsOpen.value).toBe(false)
  })

  it('reports failure when the backend rejects the example', async () => {
    const { save } = useSaveExample(() => source({ requestId: 'r2' }))
    const svc = await service()
    vi.spyOn(svc, 'create').mockResolvedValue({ data: null as never, error: { code: 'validation', message: 'example is too large (max 480 KB)' } })

    expect(await save('x')).toBe(false)
  })
})
