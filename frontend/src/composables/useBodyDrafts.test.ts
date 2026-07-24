import { describe, it, expect, beforeEach } from 'vitest'
import {
  saveDraft,
  getDraft,
  clearDrafts,
  isTextFamily,
  isTextFamilyTransition,
} from './useBodyDrafts'

const REQ = 'req-1'

describe('useBodyDrafts', () => {
  beforeEach(() => {
    clearDrafts(REQ)
  })

  it('returns default content for an unsaved type', () => {
    expect(getDraft(REQ, 'json')).toBe('')
    expect(getDraft(REQ, 'form')).toBe('[]')
  })

  it('saves and restores body per type', () => {
    saveDraft(REQ, 'json', '{"a":1}')
    saveDraft(REQ, 'form', '[{"key":"x","value":"y"}]')

    expect(getDraft(REQ, 'json')).toBe('{"a":1}')
    expect(getDraft(REQ, 'form')).toBe('[{"key":"x","value":"y"}]')
  })

  it('isTextFamily: json/xml/raw are text family, others are not', () => {
    expect(isTextFamily('json')).toBe(true)
    expect(isTextFamily('xml')).toBe(true)
    expect(isTextFamily('raw')).toBe(true)
    expect(isTextFamily('form')).toBe(false)
    expect(isTextFamily('binary')).toBe(false)
    expect(isTextFamily('none')).toBe(false)
  })

  it('isTextFamilyTransition: only true when both ends are text family', () => {
    expect(isTextFamilyTransition('json', 'xml')).toBe(true)
    expect(isTextFamilyTransition('xml', 'raw')).toBe(true)
    expect(isTextFamilyTransition('raw', 'json')).toBe(true)

    expect(isTextFamilyTransition('json', 'form')).toBe(false)
    expect(isTextFamilyTransition('form', 'json')).toBe(false)
    expect(isTextFamilyTransition('binary', 'raw')).toBe(false)
    expect(isTextFamilyTransition('json', 'none')).toBe(false)
  })

  it('clearDrafts wipes all per-type drafts for a request', () => {
    saveDraft(REQ, 'json', '{"a":1}')
    saveDraft(REQ, 'xml', '<x/>')
    clearDrafts(REQ)
    expect(getDraft(REQ, 'json')).toBe('')
    expect(getDraft(REQ, 'xml')).toBe('')
  })
})
