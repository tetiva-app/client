import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useResponseStore } from './responses'

describe('useResponseStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('getResponseState returns idle for unknown id', () => {
    const store = useResponseStore()
    expect(store.getResponseState('nonexistent')).toEqual({ status: 'idle' })
  })

  it('setResponse stores and retrieves state', () => {
    const store = useResponseStore()
    const state = { status: 'loading' as const, startedAt: 1000 }
    store.setResponse('req-1', state)
    expect(store.getResponseState('req-1')).toEqual(state)
  })

  it('setResponse reassigns map to stay reactive', () => {
    const store = useResponseStore()
    const mapBefore = store.responseMap
    store.setResponse('req-1', { status: 'idle' })
    expect(store.responseMap).not.toBe(mapBefore)
  })

  it('cancelRequest sets state to idle', () => {
    const store = useResponseStore()
    store.setResponse('req-1', { status: 'loading', startedAt: 123 })
    store.cancelRequest('req-1')
    expect(store.getResponseState('req-1')).toEqual({ status: 'idle' })
  })

  it('deleteResponse removes the entry', () => {
    const store = useResponseStore()
    store.setResponse('req-1', { status: 'loading', startedAt: 123 })
    store.deleteResponse('req-1')
    expect(store.getResponseState('req-1')).toEqual({ status: 'idle' })
  })

  it('deleteResponse reassigns map to stay reactive', () => {
    const store = useResponseStore()
    store.setResponse('req-1', { status: 'idle' })
    const mapBefore = store.responseMap
    store.deleteResponse('req-1')
    expect(store.responseMap).not.toBe(mapBefore)
  })

  it('stores success state with data', () => {
    const store = useResponseStore()
    const data = {
      statusCode: 200,
      statusText: 'OK',
      url: 'http://example.com',
      headers: {},
      body: '{}',
      size: 2,
      durationMs: 42,
    }
    store.setResponse('req-1', { status: 'success', data })
    const result = store.getResponseState('req-1')
    expect(result.status).toBe('success')
    if (result.status === 'success') {
      expect(result.data.statusCode).toBe(200)
    }
  })

  it('stores error state', () => {
    const store = useResponseStore()
    store.setResponse('req-1', {
      status: 'error',
      error: { title: 'Failed', detail: 'connection refused', suggestions: ['check server'] },
    })
    const result = store.getResponseState('req-1')
    expect(result.status).toBe('error')
    if (result.status === 'error') {
      expect(result.error.title).toBe('Failed')
    }
  })

  it('remembers the duration of the previous successful send', () => {
    const store = useResponseStore()
    const ok = (durationMs: number) => ({
      status: 'success' as const,
      data: { statusCode: 200, statusText: 'OK', url: 'u', headers: {}, body: '', size: 0, durationMs },
    })

    store.setResponse('req-1', ok(142))
    expect(store.previousDurationMs('req-1')).toBeUndefined()

    store.setResponse('req-1', { status: 'loading', startedAt: 1 })
    store.setResponse('req-1', ok(318))
    expect(store.previousDurationMs('req-1')).toBe(142)

    store.setResponse('req-1', { status: 'error', error: { title: 't', detail: 'd', suggestions: [] } })
    store.setResponse('req-1', ok(139))
    expect(store.previousDurationMs('req-1')).toBe(318)
    expect(store.previousDurationMs('req-2')).toBeUndefined()
  })
})
