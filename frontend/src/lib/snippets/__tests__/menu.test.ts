import { describe, expect, it } from 'vitest'
import { copyMenuModel } from '../menu'
import { targetMetaFor } from '../targets'

const keys = (list: { key: string }[]) => list.map((t) => t.key)

describe('copyMenuModel', () => {
  it('keeps cURL on top and moves the other HTTP languages into the submenu', () => {
    const model = copyMenuModel(targetMetaFor('http'))
    expect(keys(model.top)).toEqual(['curl'])
    expect(keys(model.submenu)).toEqual([
      'python-requests', 'js-fetch', 'go', 'java-httpclient', 'java-okhttp', 'csharp-httpclient', 'php-guzzle',
    ])
  })

  it('offers GraphQL the same languages as HTTP', () => {
    expect(copyMenuModel(targetMetaFor('graphql'))).toEqual(copyMenuModel(targetMetaFor('http')))
  })

  it('has no submenu for gRPC', () => {
    const model = copyMenuModel(targetMetaFor('grpc'))
    expect(keys(model.top)).toEqual(['grpcurl'])
    expect(model.submenu).toEqual([])
  })

  it('keeps a single leftover language top-level instead of a one-item submenu', () => {
    const model = copyMenuModel(targetMetaFor('websocket'))
    expect(keys(model.top)).toEqual(['websocat', 'js-websocket'])
    expect(model.submenu).toEqual([])
  })

  it('is empty without targets', () => {
    expect(copyMenuModel([])).toEqual({ top: [], submenu: [] })
  })
})
