import { describe, it, expect, vi, afterEach } from 'vitest'
import { nextTick, type App } from 'vue'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))

import EnvironmentSelector from './EnvironmentSelector.vue'
import { useEnvironmentStore } from '@/stores/environments'
import { setCurrentLocale } from '@/lib/locale'
import { mountTree, textOf, type El } from '@/test-utils/tree'

let mounted: App | null = null

afterEach(() => {
  setCurrentLocale('en')
  mounted?.unmount()
  mounted = null
  vi.restoreAllMocks()
})

function env(id: string, name: string, isActive: boolean) {
  return { id, name, isActive, version: 1, createdAt: '', updatedAt: '' }
}

function firstClickable(n: El): El | null {
  if (typeof n.props.onClick === 'function') return n
  for (const child of n.children) {
    const hit = firstClickable(child)
    if (hit) return hit
  }
  return null
}

async function openPicker(...envs: ReturnType<typeof env>[]): Promise<El> {
  const pinia = createPinia()
  setActivePinia(pinia)
  useEnvironmentStore().environments = envs
  const { app, root } = mountTree(EnvironmentSelector, pinia)
  mounted = app
  ;(firstClickable(root)!.props.onClick as () => void)()
  await nextTick()
  return root
}

describe('environment picker', () => {
  it('keeps its English labels', async () => {
    const text = textOf(await openPicker(env('e1', 'Staging', false)))

    expect(text.match(/No Environment/g)).toHaveLength(2)
    expect(text).toContain('Manage Environments...')
    expect(text).toContain('Staging')
  })

  it('speaks Russian when the app does', async () => {
    setCurrentLocale('ru')
    const text = textOf(await openPicker(env('e1', 'Staging', false)))

    expect(text.match(/Без окружения/g)).toHaveLength(2)
    expect(text).toContain('Настроить окружения…')
    expect(text).not.toContain('No Environment')
  })

  it('shows the active environment name untouched', async () => {
    setCurrentLocale('ru')
    const text = textOf(await openPicker(env('e1', 'Staging', true)))

    expect(text.match(/Staging/g)).toHaveLength(2)
    expect(text.match(/Без окружения/g)).toHaveLength(1)
  })
})
