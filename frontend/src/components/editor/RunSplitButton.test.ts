import { describe, it, expect, vi } from 'vitest'
import { KeepAlive, createSSRApp, defineComponent, h, nextTick, ref, watchEffect } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { targetMetaFor } from '@/lib/snippets/targets'
import type { SnippetProtocol } from '@/lib/snippets/types'
import { mountWindow } from '@/test-utils/windows'

const menu = vi.hoisted(() => ({ open: false, request: (_open: boolean) => {} }))

vi.mock('@/components/ui/dropdown-menu', () => {
  const inline = (tag: string, slot: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, { ...attrs, 'data-slot': slot }, slots.default?.()),
  })
  return {
    DropdownMenu: defineComponent({
      inheritAttrs: false,
      props: { open: Boolean },
      emits: ['update:open'],
      setup(props, { slots, attrs, emit }) {
        menu.request = (open) => emit('update:open', open)
        watchEffect(() => { menu.open = props.open })
        return () => h('div', { ...attrs, 'data-slot': 'menu' }, slots.default?.())
      },
    }),
    DropdownMenuTrigger: defineComponent({ inheritAttrs: false, setup: (_, { slots }) => () => slots.default?.() }),
    DropdownMenuContent: inline('div', 'content'),
    DropdownMenuItem: inline('div', 'item'),
    DropdownMenuSeparator: inline('hr', 'separator'),
    DropdownMenuSub: inline('div', 'sub'),
    DropdownMenuSubTrigger: inline('div', 'sub-trigger'),
    DropdownMenuSubContent: inline('div', 'sub-content'),
  }
})

import RunSplitButton from './RunSplitButton.vue'

async function render(props: Record<string, unknown> = {}, protocol: SnippetProtocol = 'http'): Promise<string> {
  return renderToString(createSSRApp(RunSplitButton, { label: 'Send', targets: targetMetaFor(protocol), ...props }))
}

function openingTag(html: string, marker: string): string {
  const at = html.indexOf(marker)
  expect(at, marker).toBeGreaterThanOrEqual(0)
  return html.slice(html.lastIndexOf('<', at), html.indexOf('>', at) + 1)
}

function slotTexts(html: string, slot: string): string[] {
  const re = new RegExp(`data-slot="${slot}"[^>]*>([^]*?)</div>`, 'g')
  return [...html.matchAll(re)].map((m) => m[1].replace(/<[^>]*>/g, '').replace(/\s+/g, ' ').trim())
}

describe('RunSplitButton', () => {
  it('shows the run label', async () => {
    const html = await render({ label: 'Invoke' })
    expect(html).toContain('Invoke')
    expect(html).not.toContain('Cancel')
  })

  it('turns the run button into Cancel while loading', async () => {
    const html = await render({ loading: true })
    expect(html).toContain('Cancel')
    expect(html).not.toMatch(/>\s*Send\s*</)
  })

  it('disables only the run button: copying needs no host', async () => {
    const html = await render({ disabled: true })
    const [run, chevron, ...rest] = [...html.matchAll(/<button[^>]*>/g)].map((m) => m[0])
    expect(rest).toEqual([])
    expect(run).toMatch(/\sdisabled(?=[\s>=])/)
    expect(chevron).toContain('aria-label="More actions"')
    expect(chevron).toContain('rounded-l-none')
    expect(chevron).not.toMatch(/\sdisabled(?=[\s>=])/)
  })

  it('lists cURL on top and the other HTTP languages in a Copy as submenu without checkmarks', async () => {
    const html = await render()
    expect(openingTag(html, 'data-slot="content"')).toContain('min-w-[180px]')
    expect(openingTag(html, 'data-slot="content"')).toContain('align="end"')
    expect(slotTexts(html, 'item')).toEqual([
      'Copy as cURL', 'Python', 'JavaScript', 'Go', 'Java (HttpClient)', 'Java (OkHttp)', 'C#', 'PHP', 'Generate code…',
    ])
    expect(slotTexts(html, 'sub-trigger')).toEqual(['Copy as'])
    expect(html).not.toContain('lucide-check')
  })

  it('puts both WebSocket languages on top with no submenu', async () => {
    const html = await render({ label: 'Connect' }, 'websocket')
    expect(slotTexts(html, 'item')).toEqual(['Copy as websocat', 'Copy as JavaScript', 'Generate code…'])
    expect(html).not.toContain('data-slot="sub-trigger"')
    expect(html).toMatch(/<svg[^>]*lucide-terminal[^>]*>[^]*?<\/svg>\s*Copy as websocat/)
    expect(html).toMatch(/<svg[^>]*lucide-code[^>]*>[^]*?<\/svg>\s*Copy as JavaScript/)
  })

  it('closes its menu when the editor under KeepAlive is deactivated', async () => {
    const active = ref(true)
    const app = mountWindow(defineComponent({
      setup: () => () => h(KeepAlive, null, [
        active.value
          ? h(RunSplitButton, { key: 'editor', label: 'Send', targets: targetMetaFor('http') })
          : h({ render: () => null }, { key: 'other' }),
      ]),
    }))

    menu.request(true)
    await nextTick()
    expect(menu.open).toBe(true)

    active.value = false
    await nextTick()
    await nextTick()
    expect(menu.open).toBe(false)
    app.unmount()
  })
})
