import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { Protocol } from '@/types/request'

const { getById, settings } = vi.hoisted(() => ({
  getById: vi.fn(),
  settings: { setSnippetTarget: vi.fn() },
}))

vi.mock('@/stores/tabs', () => ({ useRequestStore: () => ({ getById }) }))
vi.mock('@/stores/settings', () => ({ useSettingsStore: () => settings }))

import { useCodeDialogUi } from './codeDialog'

function request(protocol: Protocol) {
  return { id: 'r1', protocol }
}

describe('code dialog ui', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getById.mockReset()
    getById.mockReturnValue(request('http'))
    settings.setSnippetTarget.mockReset()
  })

  it('opens on the given language for the request family', () => {
    const ui = useCodeDialogUi()

    ui.open('r1', 'go')

    expect(getById).toHaveBeenCalledWith('r1')
    expect(settings.setSnippetTarget).toHaveBeenCalledWith('http', 'go')
    expect(ui.requestId).toBe('r1')
  })

  it('shares the http language with GraphQL', () => {
    getById.mockReturnValue(request('graphql'))
    const ui = useCodeDialogUi()

    ui.open('r1', 'python-requests')

    expect(settings.setSnippetTarget).toHaveBeenCalledWith('http', 'python-requests')
  })

  it('keeps the remembered language when none is given', () => {
    const ui = useCodeDialogUi()

    ui.open('r1')

    expect(settings.setSnippetTarget).not.toHaveBeenCalled()
    expect(ui.requestId).toBe('r1')
  })

  it('stays closed for a request that is not loaded', () => {
    getById.mockReturnValue(undefined)
    const ui = useCodeDialogUi()

    ui.open('gone', 'go')

    expect(settings.setSnippetTarget).not.toHaveBeenCalled()
    expect(ui.requestId).toBeNull()
  })

  it('closes', () => {
    const ui = useCodeDialogUi()
    ui.open('r1')

    ui.close()

    expect(ui.requestId).toBeNull()
  })
})
