import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h, ref } from 'vue'
import { renderToString } from '@vue/server-renderer'

const status = vi.hoisted(() => ({ state: 'connected', parked: 0, tooLarge: 0 }))

vi.mock('@/composables/useSyncStatus', () => ({
  useSyncStatus: () => ({
    state: ref(status.state),
    pending: ref(0),
    parked: ref(status.parked),
    tooLarge: ref(status.tooLarge),
    awaitingVerification: ref(false),
  }),
}))

vi.mock('@/components/ui/tooltip', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  return { Tooltip: inline('div'), TooltipTrigger: inline('div'), TooltipContent: inline('span') }
})

import SyncStatusIndicator from './SyncStatusIndicator.vue'
import { setCurrentLocale } from '@/lib/locale'
import { inside, tagWith } from '@/test-utils/markup'

afterEach(() => {
  setCurrentLocale('en')
  Object.assign(status, { state: 'connected', parked: 0, tooLarge: 0 })
})

function render(): Promise<string> {
  return renderToString(createSSRApp(SyncStatusIndicator))
}

describe('sync button on the rail', () => {
  it('names itself and its state in the app language', async () => {
    const en = await render()
    expect(en).toContain('aria-label="Sync"')
    expect(inside(en, 'side="right"')).toContain('Sync connected')

    setCurrentLocale('ru')
    status.state = 'plan_limit'
    const ru = await render()
    expect(ru).toContain('aria-label="Синхронизация"')
    expect(inside(ru, 'side="right"')).toContain('Синхронизация на паузе — достигнут лимит тарифа')
  })

  it('counts held-back changes in Russian', async () => {
    setCurrentLocale('ru')
    Object.assign(status, { state: 'connected', parked: 3, tooLarge: 5 })

    expect(inside(await render(), 'side="right"'))
      .toContain('3 изменения не синхронизированы — лимит тарифа · 5 элементов слишком велики для сервера')
  })

  it('does not pop its tooltip when a closing dialog hands focus back', async () => {
    expect(tagWith(await render(), '<div')).toContain('ignore-non-keyboard-focus')
  })
})
