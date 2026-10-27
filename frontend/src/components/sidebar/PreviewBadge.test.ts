import { afterEach, describe, expect, it, vi } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'

vi.stubGlobal('__APP_VERSION__', '1.3.1')

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))

vi.mock('@/components/ui/tooltip', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  return { Tooltip: inline('div'), TooltipTrigger: inline('div'), TooltipContent: inline('span') }
})

import PreviewBadge from './PreviewBadge.vue'
import { useBuildStore } from '@/stores/build'
import { setCurrentLocale, type Locale } from '@/lib/locale'
import { inside } from '@/test-utils/markup'

afterEach(() => { setCurrentLocale('en') })

function render(flavor: 'stable' | 'preview', locale: Locale = 'en'): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  setCurrentLocale(locale)
  useBuildStore().flavor = flavor
  const app = createSSRApp(PreviewBadge)
  app.use(pinia)
  return renderToString(app)
}

describe('preview marker in the sidebar footer', () => {
  it('names the build and its version in both languages', async () => {
    const en = await render('preview')
    expect(en).toContain('Preview build 1.3.1')
    expect(inside(en, 'side="top"')).toContain('Separate data, no sign-in, no updates')

    const ru = await render('preview', 'ru')
    expect(ru).toContain('Тестовая сборка 1.3.1')
    expect(inside(ru, 'side="top"')).toContain('Свои данные, без входа и обновлений')
  })

  it('is absent from the regular build', async () => {
    expect(await render('stable')).not.toContain('preview-badge')
  })
})
