import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h, type VNode } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { Request } from '@/types/request'

vi.mock('@/services', () => ({
  isWailsEnvironment: () => false,
  getWindowService: async () => null,
}))

const items = vi.hoisted(() => [] as { label: string; attrs: Record<string, unknown> }[])

vi.mock('@/components/ui/context-menu', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  const item = defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => {
      const children = slots.default?.() as VNode[] | undefined
      items.push({ label: (children ?? []).map(c => (typeof c.children === 'string' ? c.children : '')).join('').trim(), attrs })
      return h('div', attrs, children)
    },
  })
  return {
    ContextMenu: inline('div'),
    ContextMenuTrigger: inline('div'),
    ContextMenuContent: inline('div'),
    ContextMenuItem: item,
    ContextMenuSeparator: inline('hr'),
  }
})

import RequestItem from './RequestItem.vue'
import { useTreeSelection } from '@/composables/useTreeSelection'
import { useRequestStore } from '@/stores/tabs'
import { setCurrentLocale } from '@/lib/locale'
import { tagWith } from '@/test-utils/markup'

const REQUEST = {
  id: 'r1', collectionId: 'c1', name: 'List pets', protocol: 'http', method: 'GET', url: '', version: 1,
} as unknown as Request

function render(props: Record<string, unknown> = {}): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  const app = createSSRApp(RequestItem, { request: REQUEST, depth: 1, ...props })
  app.use(pinia)
  return renderToString(app)
}

afterEach(() => {
  items.length = 0
  setCurrentLocale('en')
  useTreeSelection().clearSelection()
})

describe('request context menu', () => {
  it('keeps its English labels', async () => {
    const html = await render()

    expect(html).toContain('Rename')
    expect(html).toContain('Duplicate')
    expect(html).toContain('Move to…')
    expect(html).toContain('Delete')
    expect(html).toContain('min-w-48')
    expect(html).not.toMatch(/class="([^"]* )?w-48/)
  })

  it('speaks Russian when the app does', async () => {
    setCurrentLocale('ru')
    const html = await render()

    expect(html).toContain('Переименовать')
    expect(html).toContain('Дублировать')
    expect(html).toContain('Переместить…')
    expect(html).toContain('Удалить')
    expect(html).not.toContain('Rename')
  })

  it('hands the loaded request to the tree on Duplicate', async () => {
    const onDuplicate = vi.fn()
    await render({ onDuplicate })
    useRequestStore().loadRequest(REQUEST)

    await (items.find(i => i.label === 'Duplicate')?.attrs.onClick as () => Promise<void>)()

    expect(onDuplicate).toHaveBeenCalledWith(REQUEST)
  })

  it('offers no Duplicate on a replay draft', async () => {
    const html = await render({ request: { ...REQUEST, isDraft: true } })

    expect(html).toContain('Rename')
    expect(html).not.toContain('Duplicate')
  })

  it('counts a multi-selection in Russian', async () => {
    setCurrentLocale('ru')
    useTreeSelection().setSelection(['r1', 'r2', 'r3'])
    const html = await render()

    expect(html).toContain('Удалить 3 элемента')
    expect(html).toContain('min-w-48')
  })
})

describe('request tree row', () => {
  it('shows the full name on hover when the label is cut', async () => {
    const html = await render()

    expect(tagWith(html, 'truncate flex-1')).toContain('title="List pets"')
  })
})
