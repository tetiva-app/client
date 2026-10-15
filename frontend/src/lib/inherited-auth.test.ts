import { describe, it, expect } from 'vitest'
import type { Collection } from '@/types/collection'
import { chainSignature } from './inherited-auth'

function col(id: string, parentId: string | null, authType = 'none', version = 1): Collection {
  return {
    id, workspaceId: 'w', parentId, name: id, description: '', authType, authData: '{}',
    preScript: '', postScript: '', sortOrder: 0, version, createdAt: '', updatedAt: '',
  }
}

const map = (...cs: Collection[]) => new Map(cs.map(c => [c.id, c]))

describe('chainSignature', () => {
  it('changes when an ancestor changes its scheme', () => {
    expect(chainSignature(map(col('a', null, 'bearer'), col('b', 'a')), 'b'))
      .not.toBe(chainSignature(map(col('a', null, 'basic'), col('b', 'a')), 'b'))
  })

  it('ignores a version bump that changed nothing the resolver reads', () => {
    expect(chainSignature(map(col('a', null, 'bearer', 1), col('b', 'a')), 'b'))
      .toBe(chainSignature(map(col('a', null, 'bearer', 7), col('b', 'a')), 'b'))
  })

  it('tells two identically configured siblings apart', () => {
    const m = map(col('p', null), col('x', 'p', 'basic'), col('y', 'p', 'basic'))
    expect(chainSignature(m, 'x')).not.toBe(chainSignature(m, 'y'))
  })

  it('stops on a parentId cycle', () => {
    expect(chainSignature(map(col('a', 'b'), col('b', 'a')), 'a').split('\n')).toHaveLength(2)
  })

  it('is empty without a start', () => {
    expect(chainSignature(map(), undefined)).toBe('')
  })
})
