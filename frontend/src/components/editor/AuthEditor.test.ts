import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createSSRApp, defineComponent } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import { defaultAuthData } from '@/lib/auth-data'
import { clearAuthDrafts } from '@/composables/useAuthDrafts'
import type { AuthType } from '@/types/request'

const selects = vi.hoisted(() => new Map<string, (value: string) => void>())

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))

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

import AuthEditor from './AuthEditor.vue'

async function render(props: Record<string, unknown>): Promise<string> {
  const app = createSSRApp(AuthEditor, { ownerKind: 'request', ownerId: 'r1', ...props })
  app.use(createPinia())
  return renderToString(app)
}

function renderAuth(authType: AuthType, authData: object): Promise<string> {
  return render({ authType, authData: JSON.stringify(authData) })
}

beforeEach(() => {
  setActivePinia(createPinia())
  selects.clear()
  clearAuthDrafts('r1')
})

describe('AuthEditor', () => {
  it('hands the bearer token back after the Auth tab was left and reopened', async () => {
    const sent: string[] = []
    const onData = (value: string) => sent.push(value)

    await render({ authType: 'bearer', authData: '{"token":"abc"}', 'onUpdate:authData': onData })
    selects.get('auth-type-selector')!('basic')

    await render({ authType: 'basic', authData: sent[0], 'onUpdate:authData': onData })
    selects.get('auth-type-selector')!('bearer')

    expect(sent).toEqual([defaultAuthData('basic'), '{"token":"abc"}'])
  })
})

describe('auth editor secrets', () => {
  it('masks the bearer token behind a show toggle', async () => {
    const html = await renderAuth('bearer', { prefix: 'Bearer', token: 'eyJ.secret' })

    expect(html).toContain('aria-label="Show Token"')
    expect(html).not.toContain('aria-label="Show Prefix"')
  })

  it('masks the API key value but not its name', async () => {
    const html = await renderAuth('api_key', { key: 'X-API-Key', value: 'sk-live', addTo: 'header' })

    expect(html).toContain('aria-label="Show Value"')
    expect(html).not.toContain('aria-label="Show Key"')
  })

  it('shows a token or API key that is only {{variable}} references', async () => {
    expect(await renderAuth('bearer', { token: '{{accessToken}}' })).not.toContain('aria-label="Show Token"')
    expect(await renderAuth('api_key', { key: 'X-API-Key', value: '{{apiKey}}', addTo: 'header' })).not.toContain('aria-label="Show Value"')
  })
})
