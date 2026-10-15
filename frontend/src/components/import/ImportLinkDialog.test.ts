import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import { setCurrentLocale } from '@/lib/locale'

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
    DialogFooter: inline('div'),
    DialogHeader: inline('div'),
    DialogTitle: inline('h2'),
  }
})

import ImportLinkDialog from './ImportLinkDialog.vue'
import { useImportUi, type LinkStep } from '@/stores/importUi'

afterEach(() => { setCurrentLocale('en') })

async function render(step: LinkStep): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  const ui = useImportUi()
  ui.linkOpen = true
  ui.linkStep = step
  ui.title = 'Internal API'
  ui.slug = 'internal-api-pa55word'
  const app = createSSRApp(ImportLinkDialog)
  app.use(pinia)
  return renderToString(app)
}

describe('import from link dialog', () => {
  it('asks for a link in English', async () => {
    const html = await render('url')

    expect(html).toContain('Import from link')
    expect(html).toContain('Paste a link to a collection published on share.tetiva.app.')
    expect(html).toContain('aria-label="Link or slug"')
    expect(html).toContain('Cancel')
    expect(html).toContain('Continue')
  })

  it('asks for a link in Russian', async () => {
    setCurrentLocale('ru')
    const html = await render('url')

    expect(html).toContain('Импорт по ссылке')
    expect(html).toContain('Вставьте ссылку на коллекцию, опубликованную на share.tetiva.app.')
    expect(html).toContain('aria-label="Ссылка или код коллекции"')
    expect(html).toContain('Отмена')
    expect(html).toContain('Продолжить')
  })

  it('asks for the password in English', async () => {
    const html = await render('password')

    expect(html).toContain('Password required')
    expect(html).toContain('“Internal API” is protected. Enter the password you got with the link.')
    expect(html).toContain('placeholder="Password"')
    expect(html).toContain('Unlock')
  })

  it('asks for the password in Russian', async () => {
    setCurrentLocale('ru')
    const html = await render('password')

    expect(html).toContain('Нужен пароль')
    expect(html).toContain('Коллекция «Internal API» защищена. Введите пароль, который вы получили вместе со ссылкой.')
    expect(html).toContain('placeholder="Пароль"')
    expect(html).toContain('Открыть')
  })

  it('offers to close a deep link that is being opened', async () => {
    expect(await render('opening')).toContain('Close')
    setCurrentLocale('ru')
    expect(await render('opening')).toContain('Закрыть')
  })
})
