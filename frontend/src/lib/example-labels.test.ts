import { describe, it, expect } from 'vitest'
import { exampleDeleteDescription, exampleStatusLabel } from './example-labels'

describe('example labels', () => {
  it('names a gRPC status next to its code and keeps HTTP codes bare', () => {
    expect(exampleStatusLabel('grpc', 5)).toBe('NOT_FOUND (5)')
    expect(exampleStatusLabel('grpc', 0)).toBe('OK (0)')
    expect(exampleStatusLabel('http', 404)).toBe('404')
    expect(exampleStatusLabel('graphql', 200)).toBe('200')
  })

  it('mentions the rest of the workspace only when it is synced', () => {
    expect(exampleDeleteDescription('200 OK', true)).toBe('"200 OK" will be deleted for everyone in the workspace.')
    expect(exampleDeleteDescription('200 OK', false)).toBe('"200 OK" will be deleted.')
  })
})
