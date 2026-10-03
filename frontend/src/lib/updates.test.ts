import { describe, it, expect } from 'vitest'
import { updateManifestUrl } from './updates'
import { UPDATE_MANIFEST_URL } from '@/constants/updates'

describe('updateManifestUrl', () => {
  it('encodes whatever version string it gets', () => {
    expect(updateManifestUrl('1.2.0 beta&x=1', 'linux')).toBe(`${UPDATE_MANIFEST_URL}?v=1.2.0+beta%26x%3D1&os=linux`)
  })
})
