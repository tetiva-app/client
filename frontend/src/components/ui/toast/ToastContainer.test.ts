import { describe, it, expect, afterEach } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import ToastContainer from './ToastContainer.vue'
import { useToast } from '@/composables/useToast'
import { setCurrentLocale } from '@/lib/locale'

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
})
