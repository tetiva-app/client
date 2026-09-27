import { describe, it, expect, vi, afterEach } from 'vitest'
import { ref } from 'vue'
import { useConfirmDelete } from './useConfirmDelete'
import { useToast } from './useToast'

afterEach(() => { vi.restoreAllMocks() })

describe('useConfirmDelete', () => {
  it('closes the dialog after onConfirm resolves', async () => {
    const onConfirm = vi.fn().mockResolvedValue(undefined)
    const { open, ask, confirm } = useConfirmDelete(onConfirm)

    ask({ payload: 'item-1', title: 'Delete?', description: 'sure?' })
    expect(open.value).toBe(true)

    await confirm()
    expect(onConfirm).toHaveBeenCalledWith('item-1')
    expect(open.value).toBe(false)
  })

  it('keeps a getter description live while the dialog is open', () => {
    const note = ref('')
    const { ask, description } = useConfirmDelete(vi.fn())

    ask({ payload: 'item-3', title: 'Delete?', description: () => `sure?${note.value}` })
    expect(description.value).toBe('sure?')

    note.value = ' It is published.'
    expect(description.value).toBe('sure? It is published.')
  })

  it('keeps a getter title and confirm label live too', () => {
    const lang = ref<'en' | 'ru'>('en')
    const { ask, title, confirmLabel } = useConfirmDelete(vi.fn())

    ask({
      payload: 'item-4',
      title: () => (lang.value === 'ru' ? 'Удалить?' : 'Delete?'),
      description: 'sure?',
      confirmLabel: () => (lang.value === 'ru' ? 'Удалить' : 'Delete'),
    })
    expect([title.value, confirmLabel.value]).toEqual(['Delete?', 'Delete'])

    lang.value = 'ru'
    expect([title.value, confirmLabel.value]).toEqual(['Удалить?', 'Удалить'])
  })

  it('keeps the dialog open when onConfirm rejects', async () => {
    const onConfirm = vi.fn().mockRejectedValue(new Error('server error'))
    const { open, ask, confirm } = useConfirmDelete(onConfirm)

    ask({ payload: 'item-2', title: 'Delete?', description: 'sure?' })
    expect(open.value).toBe(true)

    await confirm() // must not throw out of the handler
    expect(open.value).toBe(true)
  })

  it('reports a failure with the caller text, read when it happens', async () => {
    const lang = ref<'en' | 'ru'>('en')
    const { ask, confirm } = useConfirmDelete(vi.fn().mockRejectedValue(new Error('server error')))
    vi.spyOn(console, 'error').mockImplementation(() => {})

    ask({
      payload: 'item-5', title: 'Delete?', description: 'sure?',
      failed: () => (lang.value === 'ru' ? 'Не удалось удалить' : "Couldn't delete"),
    })
    lang.value = 'ru'
    await confirm()

    expect(useToast().toasts.value.at(-1)?.message).toBe('Не удалось удалить')
  })

  it('keeps the generic failure text for callers that give none', async () => {
    const { ask, confirm } = useConfirmDelete(vi.fn().mockRejectedValue(new Error('server error')))
    vi.spyOn(console, 'error').mockImplementation(() => {})

    ask({ payload: 'item-6', title: 'Delete?', description: 'sure?' })
    await confirm()

    expect(useToast().toasts.value.at(-1)?.message).toBe('Action failed. Please try again.')
  })
})
