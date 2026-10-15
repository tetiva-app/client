import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

const env = vi.hoisted(() => ({ wails: true, openFile: vi.fn() }))

vi.mock('@/services', () => ({ isWailsEnvironment: () => env.wails }))
vi.mock('@wailsio/runtime', () => ({ Dialogs: { OpenFile: (options: unknown) => env.openFile(options) } }))

import { pickPath } from './pick-file'

const WINDOWS_CANCEL = 'Invalid dialog call: Dialog.OpenFile failed, error getting selection: cancelled by user'
const SERVER_BUILD = 'Invalid dialog call: Dialog.OpenFile failed, error getting selection: file dialogs not available in server mode'
const PROTO = { CanChooseFiles: true, Filters: [{ DisplayName: 'Proto Files', Pattern: '*.proto' }] }

describe('pickPath', () => {
  beforeEach(() => {
    env.wails = true
    env.openFile.mockReset()
    vi.stubGlobal('prompt', vi.fn(() => '/typed/api.proto'))
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('returns the path picked in the native dialog', async () => {
    env.openFile.mockResolvedValue('/protos/api.proto')

    expect(await pickPath(PROTO, 'Enter path to .proto file:')).toBe('/protos/api.proto')
    expect(env.openFile).toHaveBeenCalledWith({ ...PROTO, AllowsMultipleSelection: false })
    expect(prompt).not.toHaveBeenCalled()
  })

  it('returns null when the dialog closes without a pick', async () => {
    env.openFile.mockResolvedValue('')
    const logged = vi.spyOn(console, 'error').mockImplementation(() => {})

    expect(await pickPath(PROTO, 'Enter path to .proto file:')).toBeNull()
    expect(prompt).not.toHaveBeenCalled()
    expect(logged).not.toHaveBeenCalled()
  })

  it('returns null when Windows reports the cancel as an error', async () => {
    env.openFile.mockRejectedValue(new Error(WINDOWS_CANCEL))
    const logged = vi.spyOn(console, 'error').mockImplementation(() => {})

    expect(await pickPath(PROTO, 'Enter path to .proto file:')).toBeNull()
    expect(prompt).not.toHaveBeenCalled()
    expect(logged).not.toHaveBeenCalled()
  })

  it('asks for a path when the server build has no native dialogs', async () => {
    env.openFile.mockRejectedValue(new Error(SERVER_BUILD))

    expect(await pickPath(PROTO, 'Enter path to .proto file:')).toBe('/typed/api.proto')
    expect(prompt).toHaveBeenCalledWith('Enter path to .proto file:')
  })

  it('asks for a path in the browser without touching the runtime', async () => {
    env.wails = false

    expect(await pickPath(PROTO, 'Enter path to .proto file:')).toBe('/typed/api.proto')
    expect(env.openFile).not.toHaveBeenCalled()
  })

  it('returns null when the path prompt is dismissed', async () => {
    env.wails = false
    vi.stubGlobal('prompt', vi.fn(() => ''))

    expect(await pickPath(PROTO, 'Enter path to .proto file:')).toBeNull()
  })

  it('logs any other dialog failure and returns null without asking', async () => {
    const failure = new Error('Invalid dialog call: Dialog.OpenFile failed, error getting selection: CoInitializeEx failed')
    env.openFile.mockRejectedValue(failure)
    const logged = vi.spyOn(console, 'error').mockImplementation(() => {})

    expect(await pickPath(PROTO, 'Enter path to .proto file:')).toBeNull()
    expect(logged).toHaveBeenCalledWith('[Tetiva] File dialog failed:', failure)
    expect(prompt).not.toHaveBeenCalled()
  })
})
