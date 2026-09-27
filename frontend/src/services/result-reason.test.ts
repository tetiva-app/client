import { describe, it, expect } from 'vitest'
import goResult from '../../../internal/adapters/wails/testdata/result_reason.json'
import { guarded } from '@/lib/service-call'
import type { Result } from '@/types/common'
import { unwrap } from './unwrap'

describe('server error reason', () => {
  it('survives unwrap', () => {
    const r = unwrap<Record<string, never>>(goResult)
    expect(r.error?.reason).toBe('PUBLISH_QUOTA_EXCEEDED')
    expect(r.error).toEqual(goResult.error)
  })

  it('survives guarded', async () => {
    const r: Result<Record<string, never>> = await guarded(Promise.resolve(unwrap(goResult)))
    expect(r.error?.reason).toBe('PUBLISH_QUOTA_EXCEEDED')
    expect(r.error).toEqual(goResult.error)
  })
})
