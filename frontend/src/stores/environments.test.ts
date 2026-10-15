import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/services', () => {
  const fail = async () => ({ data: null, error: { code: 'E', message: 'locked' } })
  return {
    isWailsEnvironment: () => false,
    getEnvironmentService: async () => ({
      create: fail,
      duplicate: fail,
      edit: fail,
      delete: fail,
      setActive: fail,
      listVariables: fail,
      addVariable: fail,
      editVariable: fail,
      deleteVariable: fail,
    }),
  }
})

import { useEnvironmentStore } from './environments'
import { useWorkspaceStore } from './workspace'
import { useToast } from '@/composables/useToast'
import { setCurrentLocale } from '@/lib/locale'

type Store = ReturnType<typeof useEnvironmentStore>

const failures: { run: (s: Store) => Promise<unknown>; en: string; ru: string }[] = [
  { run: s => s.create('Staging'), en: 'Failed to create environment', ru: 'Не удалось создать окружение' },
  { run: s => s.duplicate('e1', 'Staging (copy)'), en: 'Failed to duplicate environment', ru: 'Не удалось дублировать окружение' },
  { run: s => s.edit('e1', 'Production', 1), en: 'Failed to edit environment', ru: 'Не удалось переименовать окружение' },
  { run: s => s.remove('e1', 1), en: 'Failed to delete environment', ru: 'Не удалось удалить окружение' },
  { run: s => s.setActive('e1'), en: 'Failed to set active environment', ru: 'Не удалось сделать окружение активным' },
  { run: s => s.fetchVariables('e1'), en: 'Failed to fetch variables', ru: 'Не удалось загрузить переменные' },
  { run: s => s.addVariable('e1', 'token', 'x', false), en: 'Failed to add variable', ru: 'Не удалось добавить переменную' },
  {
    run: s => s.editVariable({ id: 'v1', key: 'token', value: 'x', isSecret: false, enabled: true, version: 1 }),
    en: 'Failed to edit variable',
    ru: 'Не удалось изменить переменную',
  },
  { run: s => s.removeVariable('v1', 'e1'), en: 'Failed to delete variable', ru: 'Не удалось удалить переменную' },
]

beforeEach(() => {
  setActivePinia(createPinia())
  useWorkspaceStore().workspaces = [{
    id: 'w1', name: 'Default', isActive: true, version: 1, remoteWorkspaceId: null, createdAt: '', updatedAt: '',
  }]
  vi.spyOn(console, 'error').mockImplementation(() => {})
})

afterEach(() => {
  setCurrentLocale('en')
  const toast = useToast()
  for (const t of toast.toasts.value) toast.dismiss(t.id)
  vi.restoreAllMocks()
})

describe('environment store failures', () => {
  for (const locale of ['en', 'ru'] as const) {
    it(`toast in ${locale}`, async () => {
      setCurrentLocale(locale)
      const store = useEnvironmentStore()

      for (const f of failures) await f.run(store)

      expect(useToast().toasts.value.map(t => t.message)).toEqual(failures.map(f => `${f[locale]}: locked`))
    })
  }

  it('toast follows a language switch after the store exists', async () => {
    setCurrentLocale('en')
    const store = useEnvironmentStore()
    setCurrentLocale('ru')

    for (const f of failures) await f.run(store)

    expect(useToast().toasts.value.map(t => t.message)).toEqual(failures.map(f => `${f.ru}: locked`))
  })
})
