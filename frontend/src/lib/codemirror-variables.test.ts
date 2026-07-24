import { describe, it, expect } from 'vitest'
import { extractVarName } from './codemirror-variables'

describe('extractVarName', () => {
  it('trims surrounding whitespace', () => {
    expect(extractVarName('{{ foo }}')).toBe('foo')
    expect(extractVarName('{{foo}}')).toBe('foo')
    expect(extractVarName('{{  bar  }}')).toBe('bar')
  })
})
