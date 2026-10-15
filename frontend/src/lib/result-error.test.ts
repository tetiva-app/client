import { describe, it, expect } from 'vitest'
import { formatResultError, scriptFailure, stripCallChain } from './result-error'

describe('formatResultError', () => {
  it('names the field that failed validation', () => {
    expect(formatResultError({
      code: 'validation',
      message: 'validation failed',
      fields: { description: 'must be at most 16384 bytes' },
    })).toBe('description: must be at most 16384 bytes')
  })

  it('joins several fields', () => {
    expect(formatResultError({
      code: 'validation',
      message: 'validation failed',
      fields: { name: 'required', description: 'must be at most 16384 bytes' },
    })).toBe('name: required; description: must be at most 16384 bytes')
  })

  it('falls back to the message when there are no fields', () => {
    expect(formatResultError({ code: 'conflict', message: 'version conflict' })).toBe('version conflict')
    expect(formatResultError({ code: 'conflict', message: 'version conflict', fields: {} })).toBe('version conflict')
  })
})

describe('scriptFailure', () => {
  const failed = (fields: Record<string, string>) => ({
    code: 'internal',
    message: 'request.Execute: request.prepareHTTP: pre-request script failed: ReferenceError: x is not defined (line 3, column 13)',
    reason: 'pre_script_failed',
    fields,
  })

  it('names the collection whose script failed, the error and its position', () => {
    expect(scriptFailure(failed({
      message: 'ReferenceError: notDefinedFn is not defined',
      line: '3',
      column: '13',
      collection: 'F1 Folder',
      console: '["step 1","step 2 {\\"id\\":7}"]',
    }))).toEqual({
      detail: 'The pre-request script of collection “F1 Folder” failed: ReferenceError: notDefinedFn is not defined (line 3, column 13)',
      suggestion: 'Fix it on the Scripts tab of collection “F1 Folder”',
      console: ['step 1', 'step 2 {"id":7}'],
    })
  })

  it('points at the request when the script is its own', () => {
    expect(scriptFailure(failed({ message: 'TypeError: atob: the string is not valid base64' }))).toEqual({
      detail: 'The pre-request script of this request failed: TypeError: atob: the string is not valid base64',
      suggestion: 'Fix it on the Scripts tab of this request',
      console: [],
    })
  })

  it('suggests looking for a loop when the script timed out', () => {
    expect(scriptFailure(failed({ message: 'Script timed out after 5s', timeout: 'true' }))?.suggestion)
      .toBe('Look for an endless loop or a long wait in the script')
  })

  it('ignores a console it cannot read', () => {
    expect(scriptFailure(failed({ message: 'boom', console: 'not json' }))?.console).toEqual([])
  })

  it('is null for any other error', () => {
    expect(scriptFailure({ code: 'internal', message: 'boom' })).toBeNull()
  })

  it('gives formatResultError the same sentence, without Go internals', () => {
    expect(formatResultError(failed({ message: 'boom', collection: 'Root' })))
      .toBe('The pre-request script of collection “Root” failed: boom')
  })
})

describe('stripCallChain', () => {
  it('drops the Go function names in front of the message', () => {
    expect(stripCallChain('request.Execute: request.prepareHTTP: oauth2: token response carried no access_token'))
      .toBe('oauth2: token response carried no access_token')
    expect(stripCallChain('websocket.Connect: request.ResolveWebSocket: dial failed')).toBe('dial failed')
    expect(stripCallChain('request.Execute: HTTPRequester.Execute: failed to write temp file: disk full'))
      .toBe('failed to write temp file: disk full')
  })

  it('leaves messages that do not start with a function name alone', () => {
    expect(stripCallChain('api.example.com: connection refused')).toBe('api.example.com: connection refused')
    expect(stripCallChain('example.com: connection refused')).toBe('example.com: connection refused')
    expect(stripCallChain('request.Execute: example.com: no such host')).toBe('example.com: no such host')
    expect(stripCallChain('request failed: Get "http://x": EOF')).toBe('request failed: Get "http://x": EOF')
  })

  it('applies to formatResultError messages', () => {
    expect(formatResultError({ code: 'internal', message: 'request.BuildCurl: oauth2: no token' })).toBe('oauth2: no token')
  })
})
