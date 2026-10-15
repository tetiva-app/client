import { describe, it, expect, vi } from 'vitest'
import { ref } from 'vue'
import { useFindShortcut } from './useFindShortcut'

function cmdF() {
  return { metaKey: true, ctrlKey: false, altKey: false, shiftKey: false, code: 'KeyF', key: 'f', preventDefault: vi.fn() } as unknown as KeyboardEvent
}

describe('useFindShortcut', () => {
  it('moves Cmd/Ctrl+F to the search box', () => {
    const input = { focus: vi.fn(), select: vi.fn() }
    const event = cmdF()

    useFindShortcut(ref(input as unknown as HTMLInputElement))(event)

    expect(input.focus).toHaveBeenCalled()
    expect(input.select).toHaveBeenCalled()
    expect(event.preventDefault).toHaveBeenCalled()
  })

  it('leaves the shortcut to the sidebar while the search box is hidden', () => {
    const event = cmdF()

    useFindShortcut(ref(null))(event)

    expect(event.preventDefault).not.toHaveBeenCalled()
  })
})
