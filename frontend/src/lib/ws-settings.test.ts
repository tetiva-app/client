import { describe, it, expect } from 'vitest'
import { parseWsSettings, serializeWsSettings, emptyWsSettings } from './ws-settings'

describe('parseWsSettings', () => {
  it('returns defaults for an empty body', () => {
    expect(parseWsSettings('')).toEqual(emptyWsSettings())
    expect(parseWsSettings('   ')).toEqual(emptyWsSettings())
  })

  it('returns defaults for garbage and for non-objects', () => {
    expect(parseWsSettings('not json at all')).toEqual(emptyWsSettings())
    expect(parseWsSettings('[1,2]')).toEqual(emptyWsSettings())
    expect(parseWsSettings('42')).toEqual(emptyWsSettings())
  })

  it('reads a partial document', () => {
    const s = parseWsSettings('{"version":1,"pingIntervalSec":30}')
    expect(s.pingIntervalSec).toBe(30)
    expect(s.subprotocols).toEqual([])
    expect(s.messages).toEqual([])
  })

  it('salvages a document with a wrong version', () => {
    const s = parseWsSettings('{"version":7,"pingIntervalSec":5}')
    expect(s.version).toBe(1)
    expect(s.pingIntervalSec).toBe(5)
  })

  it('drops a negative ping interval and a malformed subprotocol list', () => {
    const s = parseWsSettings('{"version":1,"pingIntervalSec":-3,"subprotocols":[1,"a"]}')
    expect(s.pingIntervalSec).toBe(0)
    expect(s.subprotocols).toEqual([])
  })

  it('clamps a sub-second or fractional ping interval to off', () => {
    for (const ping of [1e-9, 0.5, 1.5, 1e30]) {
      expect(parseWsSettings(`{"version":1,"pingIntervalSec":${ping}}`).pingIntervalSec).toBe(0)
    }
  })

  it('skips messages without an id or with an unknown format', () => {
    const s = parseWsSettings(JSON.stringify({
      version: 1,
      messages: [
        { id: '', name: 'no id', format: 'text', data: 'x' },
        { id: 'm2', name: 'bad format', format: 'xml', data: 'x' },
        { id: 'm3', name: 'ok', format: 'binary', data: 'aGk=' },
      ],
    }))
    expect(s.messages).toEqual([{ id: 'm3', name: 'ok', format: 'binary', data: 'aGk=' }])
  })

  it('keeps unknown keys at both levels', () => {
    const s = parseWsSettings(JSON.stringify({
      version: 1,
      pingIntervalSec: 10,
      futureFlag: { deep: true },
      messages: [{ id: 'm1', name: 'Login', format: 'json', data: '{}', pinned: true }],
    }))
    expect(s.extra).toEqual({ futureFlag: { deep: true } })
    expect(s.messages[0].extra).toEqual({ pinned: true })
  })
})

describe('serializeWsSettings', () => {
  it('round-trips a full document including extra keys', () => {
    const body = JSON.stringify({
      version: 1,
      pingIntervalSec: 15,
      subprotocols: ['graphql-ws', 'json'],
      messages: [{ id: 'm1', name: 'Login', format: 'json', data: '{"op":"login"}', pinned: true }],
      futureFlag: 'keep me',
    })
    const parsed = parseWsSettings(body)
    const again = parseWsSettings(serializeWsSettings(parsed))
    expect(again).toEqual(parsed)
    expect(JSON.parse(serializeWsSettings(parsed)).futureFlag).toBe('keep me')
    expect(JSON.parse(serializeWsSettings(parsed)).messages[0].pinned).toBe(true)
  })

  it('always writes version 1', () => {
    const s = parseWsSettings('{"version":9,"pingIntervalSec":1}')
    expect(JSON.parse(serializeWsSettings(s)).version).toBe(1)
  })

  it('serializes empty settings into a document the Go parser accepts', () => {
    expect(JSON.parse(serializeWsSettings(emptyWsSettings()))).toEqual({
      version: 1,
      pingIntervalSec: 0,
      subprotocols: [],
      messages: [],
    })
  })
})
