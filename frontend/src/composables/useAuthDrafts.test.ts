import { describe, it, expect, beforeEach } from 'vitest'
import { switchAuthDraft, clearAuthDrafts } from './useAuthDrafts'
import { defaultAuthData } from '@/lib/auth-data'

describe('useAuthDrafts', () => {
  beforeEach(() => {
    clearAuthDrafts('r1')
    clearAuthDrafts('r2')
  })

  it('hands back what a scheme held when the user returns to it', () => {
    expect(switchAuthDraft('r1', 'bearer', '{"token":"abc"}', 'basic')).toBe(defaultAuthData('basic'))
    expect(switchAuthDraft('r1', 'basic', '{"username":"u","password":""}', 'bearer')).toBe('{"token":"abc"}')
    expect(switchAuthDraft('r1', 'bearer', '{"token":"abc"}', 'basic')).toBe('{"username":"u","password":""}')
  })

  it('keeps each owner apart', () => {
    switchAuthDraft('r1', 'bearer', '{"token":"abc"}', 'basic')
    expect(switchAuthDraft('r2', 'basic', '{}', 'bearer')).toBe(defaultAuthData('bearer'))
  })

  it('forgets an owner that is gone', () => {
    switchAuthDraft('r1', 'bearer', '{"token":"abc"}', 'basic')
    clearAuthDrafts('r1')
    expect(switchAuthDraft('r1', 'basic', '{}', 'bearer')).toBe(defaultAuthData('bearer'))
  })
})
