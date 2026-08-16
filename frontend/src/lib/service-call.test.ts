import { describe, it, expect } from 'vitest'
import { guarded, TRANSPORT_ERROR_CODE, TRANSPORT_ERROR_MESSAGE } from './service-call'

describe('guarded', () => {
  it('passes a resolved result through untouched', async () => {
    const ok = { data: 42 }
    expect(await guarded(Promise.resolve(ok))).toBe(ok)
  })

  it('keeps a Result error as the service reported it', async () => {
    const failed = { data: false, error: { code: 'not_connected', message: 'nope' } }
    expect(await guarded(Promise.resolve(failed))).toBe(failed)
  })

  it('turns a rejected call into a transport error result', async () => {
    const result = await guarded<number>(Promise.reject(new Error('binding is gone')))

    expect(result.error).toEqual({ code: TRANSPORT_ERROR_CODE, message: TRANSPORT_ERROR_MESSAGE })
    expect(result.data).toBeNull()
  })
})
