import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h, type Component } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'

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

import CreateRequestDialog from './CreateRequestDialog.vue'
import CreateCollectionDialog from './CreateCollectionDialog.vue'
import MoveToDialog from './MoveToDialog.vue'
import RenameDialog from '@/components/RenameDialog.vue'
import { setCurrentLocale } from '@/lib/locale'

function render(component: Component, props: Record<string, unknown>): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  const app = createSSRApp(defineComponent({ render: () => h(component, { open: true, ...props }) }))
  app.use(pinia)
  return renderToString(app)
}

afterEach(() => { setCurrentLocale('en') })

describe('dialogs opened from the tree menu', () => {
  it('keeps New Request in English', async () => {
    const html = await render(CreateRequestDialog, { collectionId: 'c1' })

    expect(html).toContain('New Request')
    expect(html).toContain('Enter a name for the request.')
    expect(html).toContain('placeholder="Request name"')
    expect(html).toContain('Create')
  })

  it('words New Request in Russian', async () => {
    setCurrentLocale('ru')
    const html = await render(CreateRequestDialog, { collectionId: 'c1' })

    expect(html).toContain('Новый запрос')
    expect(html).toContain('Введите название запроса.')
    expect(html).toContain('placeholder="Название запроса"')
    expect(html).toContain('Отмена')
    expect(html).toContain('Создать')
    expect(html).not.toContain('Cancel')
  })

  it('words New Sub-Collection and New Collection in Russian', async () => {
    setCurrentLocale('ru')
    const sub = await render(CreateCollectionDialog, { parentId: 'c1' })
    const root = await render(CreateCollectionDialog, { parentId: null })

    expect(sub).toContain('Новая подколлекция')
    expect(sub).toContain('Введите название подколлекции.')
    expect(sub).toContain('placeholder="Название коллекции"')
    expect(root).toContain('Новая коллекция')
    expect(root).toContain('Введите название коллекции.')
  })

  it('keeps Move to in English with a real plural', async () => {
    const html = await render(MoveToDialog, { selectedIds: ['a'], disabledIds: new Set() })

    expect(html).toContain('Move 1 item to…')
    expect(html).toContain('Root (top level)')
  })

  it('words Move to in Russian', async () => {
    setCurrentLocale('ru')
    const html = await render(MoveToDialog, { selectedIds: ['a', 'b', 'c', 'd', 'e'], disabledIds: new Set() })

    expect(html).toContain('Переместить 5 элементов в…')
    expect(html).toContain('Выберите коллекцию назначения.')
    expect(html).toContain('Верхний уровень')
    expect(html).toContain('>Отмена<')
    expect(html).toMatch(/>\s*Переместить\s*</)
  })

  it('words the Rename buttons in Russian', async () => {
    setCurrentLocale('ru')
    const html = await render(RenameDialog, { title: 'Переименовать запрос', initialName: 'List pets' })

    expect(html).toContain('Отмена')
    expect(html).toContain('Сохранить')
    expect(html).toContain('placeholder="Название"')
    expect(html).not.toContain('Save')
  })
})
