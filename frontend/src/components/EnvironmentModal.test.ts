import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h, nextTick, type App } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'

vi.stubGlobal('__APP_VERSION__', '1.2.0')

const { dialogs, portability } = vi.hoisted(() => ({
  dialogs: [] as Record<string, unknown>[],
  portability: {
    exportEnvironment: async () => ({ data: { path: '/tmp/staging.json', canceled: false } }),
  },
}))

vi.mock('@/services', () => ({
  isWailsEnvironment: () => false,
  getEnvironmentService: async () => ({
    listVariables: async () => ({
      data: [{ id: 'v1', environmentId: 'e1', key: 'token', value: 's3cret', isSecret: true, enabled: true, version: 1 }],
    }),
  }),
  getPortabilityService: async () => portability,
}))

vi.mock('@/components/ConfirmDialog.vue', () => ({
  default: defineComponent({
    props: ['open', 'title', 'description', 'confirmLabel', 'cancelLabel', 'destructive', 'onConfirm'],
    setup: (props) => { dialogs.push(props); return () => null },
  }),
}))

vi.mock('@/components/ui/dialog', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  return { Dialog: inline('div'), DialogContent: inline('div'), DialogHeader: inline('div'), DialogTitle: inline('h2') }
})

vi.mock('@/components/ui/context-menu', () => {
  const slot = defineComponent({ setup: (_, { slots }) => () => slots.default?.() })
  const item = defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h('div', attrs, slots.default?.()),
  })
  return {
    ContextMenu: slot,
    ContextMenuTrigger: slot,
    ContextMenuContent: item,
    ContextMenuItem: item,
    ContextMenuSeparator: item,
  }
})

vi.mock('@/components/ui/tooltip', () => {
  const inline = defineComponent({ setup: (_, { slots }) => () => slots.default?.() })
  return { Tooltip: inline, TooltipContent: inline, TooltipTrigger: inline }
})

import EnvironmentModal from './EnvironmentModal.vue'
import { useEnvironmentStore } from '@/stores/environments'
import { useToast } from '@/composables/useToast'
import { setCurrentLocale } from '@/lib/locale'
import { find, mountTree, textOf, type El } from '@/test-utils/tree'

function env(id: string, name: string, isActive: boolean) {
  return { id, name, isActive, version: 1, createdAt: '', updatedAt: '' }
}

async function render(): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  useEnvironmentStore().environments = [env('e1', 'Staging', true), env('e2', 'Production', false)]
  const app = createSSRApp(EnvironmentModal, { open: true })
  app.use(pinia)
  return renderToString(app)
}

function controlsInsideButtons(html: string): string[] {
  const found: string[] = []
  let depth = 0
  for (const [tag, closing, name] of html.matchAll(/<(\/?)(button|input|select|textarea|a)\b[^>]*>/g)) {
    if (closing) {
      if (name === 'button') depth--
      continue
    }
    if (depth > 0) found.push(tag)
    if (name === 'button') depth++
  }
  return found
}

describe('environment list rows', () => {
  it('nest no control inside a button', async () => {
    expect(controlsInsideButtons(await render())).toEqual([])
  })

  it('keep the set-active control and the name in each row', async () => {
    const html = await render()
    const rows = html.match(/data-testid="env-row"/g) ?? []

    expect(rows).toHaveLength(2)
    expect(html).toContain('title="Active environment"')
    expect(html).toContain('title="Set as active"')
    expect(html).toMatch(/<button[^>]*>Production<\/button>/)
  })
})

let mounted: App | null = null

afterEach(() => {
  setCurrentLocale('en')
  mounted?.unmount()
  mounted = null
  dialogs.length = 0
  const toast = useToast()
  for (const t of toast.toasts.value) toast.dismiss(t.id)
  vi.restoreAllMocks()
})

function toasts(): string[] {
  return useToast().toasts.value.map(t => t.message)
}

function all(n: El, hit: (e: El) => boolean): El[] {
  return [...(hit(n) ? [n] : []), ...n.children.flatMap(c => all(c, hit))]
}

function attrs(root: El, key: string): unknown[] {
  return all(root, e => key in e.props).map(e => e.props[key])
}

function item(root: El, text: string): () => void {
  const [hit] = all(root, e => typeof e.props.onClick === 'function' && textOf(e).trim() === text)
  expect(hit, text).toBeDefined()
  return hit.props.onClick as () => void
}

async function openStaging(): Promise<El> {
  const pinia = createPinia()
  setActivePinia(pinia)
  useEnvironmentStore().environments = [env('e1', 'Staging', true), env('e2', 'Production', false)]
  const { app, root } = mountTree(EnvironmentModal, pinia, { open: true })
  mounted = app
  ;(find(root, 'env-row')!.props.onClick as () => void)()
  await vi.waitFor(() => expect(attrs(root, 'value')).toContain('token'))
  return root
}

describe('Manage Environments wording', () => {
  it('keeps the English labels', async () => {
    const root = await openStaging()
    const text = textOf(root)

    expect(text).toContain('Manage Environments')
    expect(text).toContain('Variables: ')
    expect(text).toContain('Key')
    expect(text).toContain('Value')
    expect(text).toContain('Export as Postman')
    expect(text).toContain('Duplicate')
    expect(attrs(root, 'placeholder')).toEqual(['New environment', 'Variable name', 'Value'])
    expect(attrs(root, 'title')).toEqual(expect.arrayContaining([
      'Active environment',
      'Set as active',
      'Import a Postman or Hoppscotch environment',
      'Secret (click to unset)',
      'Not secret (click to set)',
    ]))
  })

  it('speaks Russian when the app does', async () => {
    setCurrentLocale('ru')
    const root = await openStaging()
    const text = textOf(root)

    expect(text).toContain('Окружения')
    expect(text).toContain('Переменные: ')
    expect(text).toContain('Ключ')
    expect(text).toContain('Значение')
    expect(text).toContain('Переименовать')
    expect(text).toContain('Дублировать')
    expect(text).toContain('Экспорт в Postman')
    expect(text).not.toContain('Rename')
    expect(attrs(root, 'placeholder')).toEqual(['Новое окружение', 'Имя переменной', 'Значение'])
    expect(attrs(root, 'title')).toEqual(expect.arrayContaining([
      'Активное окружение',
      'Сделать активным',
      'Импорт окружения из Postman или Hoppscotch',
      'Секретная (нажмите, чтобы сделать обычной)',
      'Обычная (нажмите, чтобы сделать секретной)',
    ]))
  })

  it('shows the empty state in Russian', async () => {
    setCurrentLocale('ru')
    const html = await render()

    expect(html).toContain('Выберите или создайте окружение')
    expect(html).not.toContain('Select or create')
  })

  it('asks before deleting in the app language', async () => {
    const root = await openStaging()

    item(root, 'Delete')()
    await nextTick()
    expect(dialogs[0].title).toBe('Delete environment')
    expect(dialogs[0].description).toBe('Delete environment "Staging"? All variables in this environment will be lost.')

    setCurrentLocale('ru')
    await nextTick()
    expect(dialogs[0].title).toBe('Удалить окружение')
    expect(dialogs[0].description).toBe('Удалить окружение «Staging»? Все его переменные будут потеряны.')
    expect(dialogs[0].confirmLabel).toBe('Удалить')
    expect(dialogs[0].cancelLabel).toBe('Отмена')
  })

  it('names a duplicate and reports an export in Russian', async () => {
    setCurrentLocale('ru')
    const root = await openStaging()
    const duplicate = vi.spyOn(useEnvironmentStore(), 'duplicate').mockResolvedValue(null)

    item(root, 'Дублировать')()
    item(root, 'Экспорт в Postman')()

    expect(duplicate).toHaveBeenCalledWith('e1', 'Staging (копия)')
    await vi.waitFor(() => expect(toasts()).toContain('Экспортировано в /tmp/staging.json'))
  })

  it('reports a failed delete in Russian', async () => {
    setCurrentLocale('ru')
    const root = await openStaging()
    vi.spyOn(useEnvironmentStore(), 'remove').mockRejectedValue(new Error('locked'))
    vi.spyOn(console, 'error').mockImplementation(() => {})

    item(root, 'Удалить')()
    await nextTick()
    await (dialogs[0].onConfirm as () => Promise<void>)()

    expect(toasts()).toEqual(['Не удалось удалить окружение. Попробуйте ещё раз.'])
  })

  it('refuses an environment file without a workspace, in Russian', async () => {
    setCurrentLocale('ru')
    const root = await openStaging()
    const [input] = all(root, e => e.props.type === 'file')
    const onChange = input.props.onChange as (e: Event) => Promise<void>

    await onChange({ target: { files: [new File(['{}'], 'staging.json')], value: '' } } as unknown as Event)

    expect(toasts()).toEqual(['Нет активного пространства'])
  })
})
