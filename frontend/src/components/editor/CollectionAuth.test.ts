import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createSSRApp, defineComponent } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import { defaultAuthData } from '@/lib/auth-data'
import { clearAuthDrafts } from '@/composables/useAuthDrafts'

const selects = vi.hoisted(() => new Map<string, (value: string) => void>())

vi.mock('./auth/AuthSelect.vue', () => ({
  default: defineComponent({
    props: { testId: String },
    emits: ['update:modelValue'],
    setup(props, { emit }) {
      if (props.testId) selects.set(props.testId, value => emit('update:modelValue', value))
      return () => null
    },
  }),
}))
vi.mock('./AuthEditor.vue', () => ({ default: { render: () => null } }))

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

  it('says a nested collection without auth falls back to its parent', async () => {
    const html = await render({ authType: 'none', authData: '{}', nested: true })

    expect(html).toContain('use the parent collection')
    expect(html).not.toContain('without auth headers')
  })

  it('says a top-level collection without auth sends none', async () => {
    const html = await render({ authType: 'none', authData: '{}' })

    expect(html).toContain('without auth headers')
  })
})
