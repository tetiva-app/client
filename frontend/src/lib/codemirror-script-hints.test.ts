import { describe, expect, it } from 'vitest'
import { CompletionContext } from '@codemirror/autocomplete'
import { EditorState } from '@codemirror/state'
import { scriptHintsSource } from './codemirror-script-hints'

function labels(doc: string) {
  const context = new CompletionContext(EditorState.create({ doc }), doc.length, false)
  return scriptHintsSource(context)?.options.map(o => o.label) ?? []
}

describe('scriptHintsSource', () => {
  it('completes pm.expect', () => {
    expect(labels('pm.exp')).toEqual(['pm.expect'])
  })

  it('completes the response assertions', () => {
    expect(labels('pm.response.to.')).toEqual([
      'pm.response.to.have.status',
      'pm.response.to.have.header',
      'pm.response.to.have.jsonBody',
      'pm.response.to.be.ok',
      'pm.response.to.be.success',
    ])
  })

  it('completes response headers and time', () => {
    expect(labels('pm.response.h')).toEqual(['pm.response.headers.get'])
    expect(labels('pm.response.resp')).toEqual(['pm.response.responseTime'])
  })
})
