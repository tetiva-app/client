import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from '@vue/server-renderer'

vi.mock('@/components/ui/tooltip', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  return { Tooltip: inline('div'), TooltipTrigger: inline('div'), TooltipContent: inline('span') }
})

import HelpLink from './HelpLink.vue'
import { setCurrentLocale } from '@/lib/locale'
import { inside, tagWith } from '@/test-utils/markup'

afterEach(() => { setCurrentLocale('en') })

function render(slug?: string): Promise<string> {
  return renderToString(createSSRApp(defineComponent({ render: () => h(HelpLink, { slug, 'data-testid': 'help' }) })))
}

describe('HelpLink', () => {
  it('names the docs page in the app language', async () => {
    expect(tagWith(await render('mcp-server'), 'data-testid="help"')).toContain('aria-label="Documentation: mcp server"')

    setCurrentLocale('ru')
    const html = await render('mcp-server')
    expect(tagWith(html, 'data-testid="help"')).toContain('aria-label="Документация: mcp server"')
    expect(inside(html, '<span')).toContain('Документация: mcp server')
    expect(tagWith(await render(), 'data-testid="help"')).toContain('aria-label="Документация"')
  })
})
