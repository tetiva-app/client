import { describe, it, expect } from 'vitest'
import { mcpErrorText } from './mcp-error'
import { SETTINGS_COPY } from './copy'

const en = SETTINGS_COPY.en.mcp
const ru = SETTINGS_COPY.ru.mcp

describe('mcpErrorText', () => {
  it('words a rejected address in the app language, whatever Go said about it', () => {
    const err = { code: 'validation', message: 'validation failed', fields: { addr: 'port must be between 1 and 65535' } }

    expect(mcpErrorText(err, en)).toBe(en.errors.address)
    expect(mcpErrorText(err, ru)).toBe('Укажите адрес как хост:порт, порт от 1 до 65535, например 127.0.0.1:9300.')
  })

  it('says the server is managed by environment variables', () => {
    const err = { code: 'validation', message: 'validation failed', fields: { _: 'MCP is managed by environment variables' } }

    expect(mcpErrorText(err, ru)).toBe(ru.errors.envManaged)
    expect(mcpErrorText(err, en)).toBe("Can't change: the MCP server is managed by environment variables.")
  })

  it('falls back to the raw detail for anything else', () => {
    const err = { code: 'internal', message: 'settings.SetMCPConfig: database is locked' }

    expect(mcpErrorText(err, en)).toBe("Couldn't save: settings.SetMCPConfig: database is locked")
    expect(mcpErrorText(err, ru)).toBe('Не удалось сохранить: settings.SetMCPConfig: database is locked')
  })

  it('keeps a detail that looks like a replacement pattern as it is', () => {
    expect(mcpErrorText({ code: 'internal', message: 'bad $& {detail}' }, en)).toBe("Couldn't save: bad $& {detail}")
  })
})
