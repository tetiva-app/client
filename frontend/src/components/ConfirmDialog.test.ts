import { describe, it, expect, vi } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from '@vue/server-renderer'

vi.mock('@/components/ui/alert-dialog', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  return {
    AlertDialog: inline('div'),
    AlertDialogAction: inline('button'),
    AlertDialogCancel: inline('button'),
    AlertDialogContent: inline('div'),
    AlertDialogDescription: inline('p'),
    AlertDialogFooter: inline('div'),
    AlertDialogHeader: inline('div'),
    AlertDialogTitle: inline('h2'),
  }
})

import ConfirmDialog from './ConfirmDialog.vue'

function render(props: Record<string, unknown>): Promise<string> {
  return renderToString(createSSRApp(ConfirmDialog, { open: true, ...props }))
}

describe('confirm dialog', () => {
  it('says Cancel and Confirm unless told otherwise', async () => {
    const html = await render({ title: 'Delete?', description: 'Gone for good.' })

    expect(html).toContain('>Delete?<')
    expect(html).toContain('>Gone for good.<')
    expect(html).toContain('>Cancel<')
    expect(html).toContain('Confirm')
  })

  it('reads its texts from getters, so an open dialog can be reworded', async () => {
    const html = await render({
      title: () => 'Снять с публикации?',
      description: () => 'Страница пропадёт.',
      confirmLabel: () => 'Снять',
      cancelLabel: () => 'Отмена',
    })

    expect(html).toContain('>Снять с публикации?<')
    expect(html).toContain('>Страница пропадёт.<')
    expect(html).toContain('>Отмена<')
    expect(html).toContain('Снять')
    expect(html).not.toContain('Cancel')
    expect(html).not.toContain('Confirm')
  })
})
