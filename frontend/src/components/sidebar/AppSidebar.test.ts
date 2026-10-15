import { describe, it, expect, vi, afterEach } from 'vitest'
import type { App } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import type { Request } from '@/types/request'

const { replays, empty } = vi.hoisted(() => ({
  replays: [] as ((id: string) => void)[],
  empty: { default: { render: () => null } },
}))

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))
vi.mock('./CollectionTree.vue', () => empty)
vi.mock('./PublicationsSidebar.vue', () => empty)
vi.mock('./SidebarUpdateSlot.vue', () => empty)
vi.mock('@/components/WorkspaceSwitcher.vue', () => empty)
vi.mock('./history/HistorySidebar.vue', async () => {
  const { defineComponent } = await import('vue')
  return {
    default: defineComponent({
      setup: (_, { attrs }) => { replays.push(attrs.onReplay as (id: string) => void); return () => null },
    }),
  }
})

import AppSidebar from './AppSidebar.vue'
import { useHistoryStore } from '@/stores/history'
import { useRequestStore } from '@/stores/tabs'
import { useToast } from '@/composables/useToast'
import { setCurrentLocale } from '@/lib/locale'
import { mountTree } from '@/test-utils/tree'

let mounted: App | null = null
let switched = vi.fn()

afterEach(() => {
  setCurrentLocale('en')
  mounted?.unmount()
  mounted = null
  replays.length = 0
  const toast = useToast()
  for (const t of toast.toasts.value) toast.dismiss(t.id)
  vi.restoreAllMocks()
})

function replay(result: Request | null): Promise<string[]> {
  const pinia = createPinia()
  setActivePinia(pinia)
  vi.spyOn(useHistoryStore(), 'replay').mockResolvedValue(result)
  vi.spyOn(useRequestStore(), 'loadRequest').mockImplementation(() => {})
  vi.spyOn(useRequestStore(), 'openTab').mockResolvedValue(undefined)
  switched = vi.fn()
  mounted = mountTree(AppSidebar, pinia, { activeSection: 'history', onSwitchSection: switched }).app
  replays[0]('h1')
  return vi.waitFor(() => {
    const messages = useToast().toasts.value.map(t => t.message)
    expect(messages).toHaveLength(1)
    return messages
  })
}

describe('replaying from history', () => {
  it('keeps the English draft toast', async () => {
    expect(await replay({ id: 'r1' } as Request)).toEqual(['Opened as draft — edit and Send, or save to a collection.'])
  })

  it('keeps the English failure toast', async () => {
    expect(await replay(null)).toEqual(['Replay failed'])
  })

  it('opens the draft with a Russian toast', async () => {
    setCurrentLocale('ru')
    expect(await replay({ id: 'r1' } as Request)).toEqual(['Открыт черновик\u00a0— отправьте его или сохраните в коллекцию.'])
  })

  it('reports a failed replay in Russian', async () => {
    setCurrentLocale('ru')
    expect(await replay(null)).toEqual(['Не удалось повторить запрос'])
  })

  it('switches to collections once the draft opens', async () => {
    await replay({ id: 'r1' } as Request)
    expect(switched).toHaveBeenCalledWith('collections')
  })

  it('stays on history when the replay fails', async () => {
    await replay(null)
    expect(switched).not.toHaveBeenCalled()
  })
})
