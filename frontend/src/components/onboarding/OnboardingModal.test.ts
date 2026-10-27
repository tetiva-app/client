import { afterEach, describe, expect, it, vi } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))

vi.mock('@/components/ui/dialog', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  return {
    Dialog: inline('div'),
    DialogContent: inline('div'),
    DialogDescription: inline('p'),
    DialogHeader: inline('div'),
    DialogTitle: inline('h2'),
  }
})

vi.mock('@/components/ui/tooltip', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  return { Tooltip: inline('div'), TooltipTrigger: inline('div'), TooltipContent: inline('span') }
})

import OnboardingModal from './OnboardingModal.vue'
import { useBuildStore } from '@/stores/build'
import { setCurrentLocale } from '@/lib/locale'
import { find, mountTree } from '@/test-utils/tree'
import { tagWith } from '@/test-utils/markup'

afterEach(() => { setCurrentLocale('en') })

function preview() {
  const pinia = createPinia()
  setActivePinia(pinia)
  useBuildStore().flavor = 'preview'
  return pinia
}

describe('welcome screen in a preview build', () => {
  it('locks the account choice and says why in both languages', async () => {
    const pinia = preview()
    const app = createSSRApp(OnboardingModal)
    app.use(pinia)
    const en = await renderToString(app)

    expect(tagWith(en, 'data-testid="onboarding-choice-account"')).toContain('aria-disabled="true"')
    expect(tagWith(en, 'data-testid="onboarding-choice-account"')).toContain('cursor-not-allowed')
    expect(tagWith(en, 'data-testid="onboarding-choice-local"')).not.toContain('aria-disabled')
    expect(en).toContain('Preview build: sign-in is unavailable')
    expect(en).not.toContain('An account can be connected any time')

    setCurrentLocale('ru')
    const ru = createSSRApp(OnboardingModal)
    ru.use(pinia)
    expect(await renderToString(ru)).toContain('Тестовая сборка: вход недоступен')
  })

  it('stays open when the locked choice is clicked', () => {
    const selectAccount = vi.fn()
    const close = vi.fn()
    const { app, root } = mountTree(OnboardingModal, preview(), { onSelectAccount: selectAccount, onClose: close })

    ;(find(root, 'onboarding-choice-account')!.props.onClick as () => void)()

    expect(selectAccount).not.toHaveBeenCalled()
    expect(close).not.toHaveBeenCalled()
    app.unmount()
  })
})
