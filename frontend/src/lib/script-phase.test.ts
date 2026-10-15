import { describe, it, expect } from 'vitest'
import { scriptBadge, scriptPhaseLabel } from './script-phase'

describe('scriptPhaseLabel', () => {
  it('uses the names the Scripts tab shows', () => {
    expect(scriptPhaseLabel('pre-script')).toBe('Pre-request')
    expect(scriptPhaseLabel('post-script')).toBe('Post-response')
    expect(scriptPhaseLabel('variable-persist')).toBe('Saving variables')
  })

  it('passes an unknown phase through', () => {
    expect(scriptPhaseLabel('something-new')).toBe('something-new')
  })
})

describe('scriptBadge', () => {
  it('counts only scripts with code in them', () => {
    expect(scriptBadge('pm.test()', 'console.log(1)')).toBe('2')
    expect(scriptBadge('\n  \n', 'console.log(1)')).toBe('1')
    expect(scriptBadge(' ', undefined)).toBe('')
  })
})
