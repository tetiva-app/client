import { describe, it, expect } from 'vitest'
import { eventPayload } from './useWindowEvents'

describe('eventPayload', () => {
  it('unwraps a WailsEvent', () => {
    expect(eventPayload({ name: 'examples:changed', data: { requestId: 'r1' } })).toEqual({ requestId: 'r1' })
  })

  it('accepts a bare payload', () => {
    expect(eventPayload({ requestId: 'r1' })).toEqual({ requestId: 'r1' })
  })
})
