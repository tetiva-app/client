import { describe, it, expect, afterEach } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import ToastContainer from './ToastContainer.vue'
import { useToast } from '@/composables/useToast'
import { setCurrentLocale } from '@/lib/locale'
import { tagWith } from '@/test-utils/markup'

afterEach(() => {
  setCurrentLocale('en')
  const toast = useToast()
  for (const t of toast.toasts.value) toast.dismiss(t.id)
})

async function render(): Promise<string> {
  const ctx: { teleports?: Record<string, string> } = {}
  await renderToString(createSSRApp(ToastContainer), ctx)
  return ctx.teleports?.body ?? ''
}

describe('ToastContainer', () => {
  it('labels the dismiss button in the app language', async () => {
    useToast().info('Saved', undefined, { sticky: true })
    expect(await render()).toContain('aria-label="Dismiss"')

    setCurrentLocale('ru')
    expect(await render()).toContain('aria-label="Закрыть"')
  })

  it('centres the icon and the close button on the first line of the message', async () => {
    useToast().success('Publication updated', { label: 'Copy link', onClick: () => {} }, { sticky: true })
    const html = await render()

    const line = ['h-4', 'flex', 'items-center', 'shrink-0']
    const icon = tagWith(html, 'data-testid="toast-icon"')
    for (const cls of line) expect(icon).toContain(cls)
    const close = tagWith(html, 'aria-label="Dismiss"')
    for (const cls of line) expect(close).toContain(cls)
    const message = tagWith(html, 'data-testid="toast-message"')
    expect(message).toMatch(/^<p /)
    expect(message).toContain('text-xs')
    expect(message).toContain('leading-4')
  })
})
