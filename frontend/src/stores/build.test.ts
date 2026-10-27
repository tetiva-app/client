import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const svc = vi.hoisted(() => ({ info: vi.fn() }))

vi.mock('@/services', () => ({ getBuildService: async () => svc, isWailsEnvironment: () => false }))

import { useBuildStore } from './build'
import { useSyncModalUi } from './syncModalUi'
import { MockBuildService } from '@/services/mock-build'

beforeEach(() => {
  setActivePinia(createPinia())
  svc.info.mockReset()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('build store', () => {
  it('takes the flavor from the backend', async () => {
    svc.info.mockResolvedValue({ data: { flavor: 'preview' } })
    const build = useBuildStore()

    await build.load()

    expect(build.preview).toBe(true)
    expect(build.productName).toBe('Tetiva Preview')
  })

  it('stays the regular app when the flavor cannot be read', async () => {
    svc.info.mockRejectedValue(new Error('no bindings'))
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const build = useBuildStore()

    await build.load()

    expect(build.preview).toBe(false)
    expect(build.productName).toBe('Tetiva')
  })
})

describe('sign-in modal in a preview build', () => {
  it('refuses to open from any entry point', () => {
    useBuildStore().flavor = 'preview'
    const ui = useSyncModalUi()

    ui.show('register')

    expect(ui.open).toBe(false)
  })
})

describe('browser mock flavor', () => {
  it('turns into the preview with ?flavor=preview', async () => {
    vi.stubGlobal('window', { location: { search: '?flavor=preview' } })
    expect((await new MockBuildService().info()).data?.flavor).toBe('preview')

    vi.stubGlobal('window', { location: { search: '?mock=update-ready' } })
    expect((await new MockBuildService().info()).data?.flavor).toBe('stable')
  })
})
