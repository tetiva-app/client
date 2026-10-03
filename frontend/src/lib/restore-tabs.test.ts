import { describe, it, expect, vi } from 'vitest'
import type { RestoreTabs } from '@/services'
import { restoreTabs } from './restore-tabs'

function fakeTabs(missing: string[] = []) {
  const opened: string[] = []
  let active: string | null = null
  const deps = {
    activeWorkspaceId: 'ws1' as string | undefined,
    openTab: vi.fn(async (id: string) => {
      await Promise.resolve()
      if (missing.includes(id)) return
      opened.push(`request:${id}`)
      active = `request:${id}`
    }),
    openCollection: vi.fn((id: string) => {
      if (missing.includes(id)) return
      opened.push(`collection:${id}`)
      active = `collection:${id}`
    }),
    setActive: vi.fn((tabId: string) => { active = tabId }),
    hasTab: (tabId: string) => opened.includes(tabId),
  }
  return { deps, opened, active: () => active }
}

function saved(over: Partial<RestoreTabs> = {}): RestoreTabs {
  return {
    workspaceId: 'ws1',
    tabs: [{ type: 'request', id: 'r1' }, { type: 'collection', id: 'c1' }, { type: 'request', id: 'r2' }],
    activeTabId: 'request:r1',
    ...over,
  }
}

describe('restoreTabs', () => {
  it('reopens the tabs in order and activates the saved one', async () => {
    const t = fakeTabs()

    await restoreTabs(saved(), t.deps)

    expect(t.opened).toEqual(['request:r1', 'collection:c1', 'request:r2'])
    expect(t.active()).toBe('request:r1')
  })

  it('skips a request that is gone and keeps going', async () => {
    const t = fakeTabs(['r1'])

    await restoreTabs(saved(), t.deps)

    expect(t.opened).toEqual(['collection:c1', 'request:r2'])
    expect(t.deps.setActive).not.toHaveBeenCalled()
    expect(t.active()).toBe('request:r2')
  })

  it('leaves the tabs of another workspace closed', async () => {
    const t = fakeTabs()
    t.deps.activeWorkspaceId = 'ws2'

    await restoreTabs(saved(), t.deps)

    expect(t.deps.openTab).not.toHaveBeenCalled()
    expect(t.deps.openCollection).not.toHaveBeenCalled()
  })

  it('does nothing without a saved set', async () => {
    const t = fakeTabs()

    await restoreTabs(null, t.deps)

    expect(t.deps.openTab).not.toHaveBeenCalled()
    expect(t.deps.setActive).not.toHaveBeenCalled()
  })
})
