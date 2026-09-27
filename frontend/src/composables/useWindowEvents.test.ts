import { describe, it, expect, vi, afterEach } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { defineComponent } from 'vue'
import { mountWindow, wailsBus } from '@/test-utils/windows'

vi.mock('@wailsio/runtime', async () => {
  const { wailsBus } = await import('@/test-utils/windows')
  return { Events: { On: wailsBus.On, Emit: wailsBus.Emit } }
})

vi.mock('@/services', () => ({ isWailsEnvironment: () => true }))

import { eventPayload, useWindowEvents } from './useWindowEvents'
import { contentSaved } from '@/lib/content-saved'

describe('eventPayload', () => {
  it('unwraps a WailsEvent', () => {
    expect(eventPayload({ name: 'examples:changed', data: { requestId: 'r1' } })).toEqual({ requestId: 'r1' })
  })

  it('accepts a bare payload', () => {
    expect(eventPayload({ requestId: 'r1' })).toEqual({ requestId: 'r1' })
  })
})

describe('saves in other windows', () => {
  const unmounts: (() => void)[] = []

  afterEach(() => {
    for (const unmount of unmounts.splice(0)) unmount()
  })

  function openWindow(config: Parameters<typeof useWindowEvents>[0]): () => void {
    const Root = defineComponent({
      setup() {
        useWindowEvents(config)
        return () => null
      },
    })
    const app = mountWindow(Root)
    const unmount = () => app.unmount()
    unmounts.push(unmount)
    return unmount
  }

  // One at a time: vitest may give the real module to a dynamic import racing the mocked one.
  async function mainAndDetached(savedElsewhere: () => void): Promise<() => void> {
    openWindow({ mode: 'main', onSavedInOtherWindow: savedElsewhere })
    await vi.waitFor(() => expect(wailsBus.listeners('content:saved')).toBe(1))
    const closeDetached = openWindow({ mode: 'detached-request', requestId: 'r1', onRequestDeleted: () => {} })
    await vi.waitFor(() => expect(wailsBus.listeners('request:deleted:r1')).toBe(1))
    return closeDetached
  }

  it('a save in a detached window reaches the main window', async () => {
    const savedElsewhere = vi.fn()
    await mainAndDetached(savedElsewhere)

    contentSaved()

    await vi.waitFor(() => expect(savedElsewhere).toHaveBeenCalledTimes(1))
  })

  it('the main window hears nothing of its own saves, nor of a closed window', async () => {
    const savedElsewhere = vi.fn()
    const closeDetached = await mainAndDetached(savedElsewhere)
    closeDetached()

    contentSaved()
    await new Promise(resolve => setTimeout(resolve, 0))

    expect(savedElsewhere).not.toHaveBeenCalled()
  })

  it('App.vue sends saves from other windows to the publications recount', () => {
    const app = readFileSync(fileURLToPath(new URL('../App.vue', import.meta.url)), 'utf8')

    expect(app).toContain('onSavedInOtherWindow: () => publications.scheduleRecount()')
  })
})
