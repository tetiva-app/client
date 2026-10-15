import { describe, expect, it } from 'vitest'
import { grpcTarget, schemeInVariable, toggleGrpcTls } from './grpc-address'

describe('grpcTarget', () => {
  it.each([
    ['localhost:50051', 'localhost:50051', false],
    ['grpc://localhost:50051', 'localhost:50051', false],
    ['grpcs://api.example.com:8443', 'api.example.com:8443', true],
    ['grpcs://api.example.com', 'api.example.com:443', true],
    ['GRPCS://api.example.com', 'api.example.com:443', true],
    [' grpcs://api.example.com/ ', 'api.example.com:443', true],
    ['grpcs://[::1]', '[::1]:443', true],
    ['grpcs://[::1]:9443', '[::1]:9443', true],
    ['grpcs://{{host}}:{{port}}', '{{host}}:{{port}}', true],
    ['{{host}}:50051', '{{host}}:50051', false],
    ['grpcs://{{endpoint}}', '{{endpoint}}', true],
    ['https://api.example.com', 'api.example.com:443', true],
    ['HTTPS://api.example.com:8443/', 'api.example.com:8443', true],
    ['http://localhost:50051', 'localhost:50051', false],
  ])('%s dials %s', (address, target, tls) => {
    expect(grpcTarget(address)).toEqual({ target, tls })
  })
})

describe('toggleGrpcTls', () => {
  it('adds grpcs:// to a bare or grpc:// address', () => {
    expect(toggleGrpcTls('localhost:50051')).toBe('grpcs://localhost:50051')
    expect(toggleGrpcTls('grpc://localhost:50051')).toBe('grpcs://localhost:50051')
    expect(toggleGrpcTls('')).toBe('grpcs://')
  })

  it('drops the scheme to turn TLS off', () => {
    expect(toggleGrpcTls('grpcs://api.example.com:443')).toBe('api.example.com:443')
    expect(toggleGrpcTls('GRPCS://{{host}}')).toBe('{{host}}')
    expect(toggleGrpcTls('https://api.example.com')).toBe('api.example.com')
  })

  it('replaces another scheme instead of adding a second one', () => {
    expect(toggleGrpcTls('http://localhost:50051')).toBe('grpcs://localhost:50051')
    expect(toggleGrpcTls('dns://localhost:50051')).toBe('grpcs://localhost:50051')
  })
})

describe('schemeInVariable', () => {
  it('is true only when the whole address is one variable', () => {
    expect(schemeInVariable('{{baseUrl}}')).toBe(true)
    expect(schemeInVariable(' {{baseUrl}}/ ')).toBe(true)
    expect(schemeInVariable('{{host}}:50051')).toBe(false)
    expect(schemeInVariable('grpcs://{{host}}')).toBe(false)
    expect(schemeInVariable('localhost:50051')).toBe(false)
  })
})
