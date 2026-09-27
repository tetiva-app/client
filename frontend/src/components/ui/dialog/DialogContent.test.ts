import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from '@vue/server-renderer'

vi.mock('reka-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('reka-ui')>()
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  return {
    ...actual,
    DialogPortal: inline('div'),
    DialogOverlay: inline('div'),
    DialogContent: inline('div'),
    DialogClose: inline('button'),
  }
})

import DialogContent from './DialogContent.vue'
import { setCurrentLocale } from '@/lib/locale'
import { inside } from '@/test-utils/markup'

afterEach(() => { setCurrentLocale('en') })

function render(): Promise<string> {
  return renderToString(createSSRApp(defineComponent({ render: () => h(DialogContent, null, () => 'body') })))
}

describe('DialogContent', () => {
  it('labels its close button in the app language', async () => {
    expect(inside(await render(), 'data-slot="dialog-close"')).toContain('<span class="sr-only">Close</span>')

    setCurrentLocale('ru')
    expect(inside(await render(), 'data-slot="dialog-close"')).toContain('<span class="sr-only">Закрыть</span>')
  })
})
