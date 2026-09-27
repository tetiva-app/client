import { describe, it, expect, vi } from 'vitest'
import { ref } from 'vue'
import { useConfirmDelete } from './useConfirmDelete'

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

  it('keeps the dialog open when onConfirm rejects', async () => {
    const onConfirm = vi.fn().mockRejectedValue(new Error('server error'))
    const { open, ask, confirm } = useConfirmDelete(onConfirm)

    ask({ payload: 'item-2', title: 'Delete?', description: 'sure?' })
    expect(open.value).toBe(true)

    await confirm() // must not throw out of the handler
    expect(open.value).toBe(true)
  })
})
