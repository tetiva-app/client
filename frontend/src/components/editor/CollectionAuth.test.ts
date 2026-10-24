import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import { defaultAuthData } from '@/lib/auth-data'
import { clearAuthDrafts } from '@/composables/useAuthDrafts'
import { setCurrentLocale } from '@/lib/locale'

const selects = vi.hoisted(() => new Map<string, (value: string) => void>())

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))

vi.mock('./auth/AuthSelect.vue', () => ({
  default: defineComponent({
    props: { testId: String, label: String, options: Array as () => { label: string }[] },
    emits: ['update:modelValue'],
    setup(props, { emit }) {
      if (props.testId) selects.set(props.testId, value => emit('update:modelValue', value))
      return () => h('div', [props.label, ...(props.options ?? []).map(o => o.label)].join(' | '))
    },
  }),
}))
vi.mock('./InheritedAuth.vue', () => ({
  default: defineComponent({ setup: (_, { slots }) => () => h('div', { 'data-stub': 'inherited' }, slots.none?.()) }),
}))

import CollectionAuth from './CollectionAuth.vue'

async function render(props: Record<string, unknown>): Promise<string> {
  const app = createSSRApp(CollectionAuth, { collectionId: 'c1', ...props })
  app.use(createPinia())
  return renderToString(app)
}

beforeEach(() => {
  setActivePinia(createPinia())
  selects.clear()
  clearAuthDrafts('c1')
})
afterEach(() => setCurrentLocale('en'))

describe('CollectionAuth', () => {
  it('hands the bearer token back after the Auth tab was left and reopened', async () => {
    const sent: string[] = []
    const onData = (value: string) => sent.push(value)

    await render({ authType: 'bearer', authData: '{"token":"abc"}', 'onUpdate:authData': onData })
    selects.get('collection-auth-type-selector')!('basic')

    await render({ authType: 'basic', authData: sent[0], 'onUpdate:authData': onData })
    selects.get('collection-auth-type-selector')!('bearer')

    expect(sent).toEqual([defaultAuthData('basic'), '{"token":"abc"}'])
  })

  it('shows what a nested collection without auth inherits', async () => {
    const html = await render({ authType: 'none', authData: '{}', nested: true })

    expect(html).toContain('inherit auth from parent collections')
    expect(html).toContain('data-stub="inherited"')
    expect(html).toContain('No parent collection sets auth')
    expect(html).not.toContain('Requests here go without auth')
  })

  it('tells a top-level collection without auth that its requests may still set their own', async () => {
    const html = await render({ authType: 'none', authData: '{}' })

    expect(html).toContain('Requests here go without auth unless they set their own')
    expect(html).not.toContain('data-stub="inherited"')
  })
})

describe('CollectionAuth language', () => {
  it('follows the app language down to the fields of the chosen scheme', async () => {
    setCurrentLocale('ru')

    const inherit = await render({ authType: 'none', authData: '{}', nested: true })
    expect(inherit).toContain('Тип авторизации | Наследовать | Basic Auth')
    expect(inherit).toContain('Запросы этой коллекции наследуют авторизацию родительских коллекций')
    expect(inherit).toContain('В родительских коллекциях авторизация не задана')

    const basic = await render({ authType: 'basic', authData: '{"username":"u","password":"p"}' })
    expect(basic).toContain('Имя пользователя')
    expect(basic).toContain('aria-label="Показать: Пароль"')
    expect(basic).not.toContain('Username')
  })
})
