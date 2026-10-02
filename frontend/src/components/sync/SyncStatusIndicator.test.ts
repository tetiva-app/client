import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h, ref } from 'vue'
import { renderToString } from '@vue/server-renderer'

const status = vi.hoisted(() => ({ state: 'connected', pending: 0, parked: 0, tooLarge: 0, verify: false }))

vi.mock('@/composables/useSyncStatus', () => ({
  useSyncStatus: () => ({
    state: ref(status.state),
    pending: ref(status.pending),
    parked: ref(status.parked),
    tooLarge: ref(status.tooLarge),
    awaitingVerification: ref(status.verify),
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
  Object.assign(status, { state: 'connected', pending: 0, parked: 0, tooLarge: 0, verify: false })
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

  it('fades the calm icon but never the pending counter', async () => {
    status.pending = 3
    const html = await render()

    expect(inside(html, 'opacity-60')).not.toContain('sync-pending-badge')
    expect(inside(html, 'data-testid="sync-pending-badge"')).toContain('3')
  })

  it('keeps warnings at full strength', async () => {
    for (const state of ['auth_expired', 'plan_limit', 'update_required']) {
      status.state = state
      expect(await render(), state).not.toContain('opacity-60')
    }
    Object.assign(status, { state: 'connected', verify: true })
    expect(await render()).not.toContain('opacity-60')
    Object.assign(status, { verify: false, parked: 2 })
    expect(await render()).not.toContain('opacity-60')
  })

  it('does not pop its tooltip when a closing dialog hands focus back', async () => {
    expect(tagWith(await render(), '<div')).toContain('ignore-non-keyboard-focus')
  })
})
