import { describe, it, expect, vi, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { Request } from '@/types/request'

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))

import { useHistoryReplay } from './useHistoryReplay'
import { useHistoryStore } from '@/stores/history'
import { useRequestStore } from '@/stores/tabs'
import { useToast } from './useToast'

function stores(result: Request | null) {
  setActivePinia(createPinia())
  return {
    replay: vi.spyOn(useHistoryStore(), 'replay').mockResolvedValue(result),
    loadRequest: vi.spyOn(useRequestStore(), 'loadRequest').mockImplementation(() => {}),
    openTab: vi.spyOn(useRequestStore(), 'openTab').mockResolvedValue(undefined),
  }
}

afterEach(() => {
  const toast = useToast()
  for (const t of toast.toasts.value) toast.dismiss(t.id)
  vi.restoreAllMocks()
})

describe('useHistoryReplay', () => {
  it('loads the draft before opening it in a tab', async () => {
    const draft = { id: 'r1', name: 'GET /pets' } as Request
    const { replay, loadRequest, openTab } = stores(draft)

    expect(await useHistoryReplay()('h1')).toBe(true)
    expect(replay).toHaveBeenCalledWith('h1')
    expect(loadRequest).toHaveBeenCalledWith(draft)
    expect(openTab).toHaveBeenCalledWith('r1')
    expect(loadRequest.mock.invocationCallOrder[0]).toBeLessThan(openTab.mock.invocationCallOrder[0])
  })

  it('opens nothing when the replay fails', async () => {
    const { loadRequest, openTab } = stores(null)

    expect(await useHistoryReplay()('h1')).toBe(false)
    expect(loadRequest).not.toHaveBeenCalled()
    expect(openTab).not.toHaveBeenCalled()
  })
})
