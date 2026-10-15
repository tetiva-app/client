import { describe, it, expect, afterEach } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { hideOthers } from 'aria-hidden'
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

interface FakeElement {
  attrs: Record<string, string>
  children: FakeElement[]
  parentNode: FakeElement | null
  getAttribute(name: string): string | null
  setAttribute(name: string, value: string): void
  removeAttribute(name: string): void
  contains(node: FakeElement): boolean
  querySelectorAll(selector: string): FakeElement[]
}

function element(attrs: Record<string, string> = {}, children: FakeElement[] = []): FakeElement {
  const el: FakeElement = {
    attrs,
    children,
    parentNode: null,
    getAttribute: name => el.attrs[name] ?? null,
    setAttribute: (name, value) => { el.attrs[name] = value },
    removeAttribute: name => { delete el.attrs[name] },
    contains: node => node === el || el.children.some(c => c.contains(node)),
    querySelectorAll: selector => {
      expect(selector).toBe('[aria-live], script')
      return el.children.flatMap(c => ['aria-live' in c.attrs ? [c] : [], c.querySelectorAll(selector)].flat())
    },
  }
  for (const child of children) child.parentNode = el
  return el
}

function attributes(tag: string): Record<string, string> {
  return Object.fromEntries([...tag.matchAll(/([\w-]+)="([^"]*)"/g)].map(m => [m[1], m[2]]))
}

describe('ToastContainer', () => {
  it('stays visible to screen readers while a modal hides the rest of the page', async () => {
    useToast().info('Saved', { label: 'Open', onClick: () => {} }, { sticky: true })
    const toasts = element(attributes(tagWith(await render(), 'data-toasts')))
    const page = element()
    const dialog = element()
    const body = element({}, [page, toasts, dialog])

    const undo = hideOthers(dialog as unknown as Element, body as unknown as HTMLElement)

    expect(page.attrs['aria-hidden']).toBe('true')
    expect(toasts.attrs['aria-hidden']).toBeUndefined()
    undo()
  })

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
