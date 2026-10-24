import { describe, it, expect } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import type { ScriptResult } from '@/types/execute'
import { tagWith } from '@/test-utils/markup'
import TestsBadge from './TestsBadge.vue'

function result(over: Partial<ScriptResult>): ScriptResult {
  return { preConsole: [], postConsole: [], tests: [], errors: [], ...over }
}

const passed = { name: 'status is 200', passed: true }
const failed = { name: 'has users', passed: false, error: 'expected 0 to be above 0' }
const postError = { phase: 'post-script', message: "ReferenceError: 'jsonn' is not defined" }

function render(r: ScriptResult | null): Promise<string> {
  return renderToString(createSSRApp(TestsBadge, { result: r }))
}

function text(html: string): string {
  return html.replace(/<[^>]*>/g, '').replace(/\s+/g, '')
}

describe('tests badge', () => {
  it('colours the count by tests and marks a failed script separately', async () => {
    const html = await render(result({ tests: [passed], errors: [postError] }))

    expect(tagWith(html, '(1/1)')).toContain('text-[var(--gc-success)]')
    const dot = tagWith(html, 'aria-label="Script failed"')
    expect(dot).toContain('role="img"')
    expect(dot).toContain('title="Script failed"')
    expect(dot).toContain('text-[var(--gc-error)]')
    expect(text(html)).toBe('(1/1)●')
  })

  it('turns the count red when a test fails', async () => {
    const html = await render(result({ tests: [passed, failed] }))

    expect(tagWith(html, '(1/2)')).toContain('text-[var(--gc-error)]')
    expect(html).not.toContain('Script failed')
  })

  it('shows a green count alone when tests pass and scripts ran clean', async () => {
    const html = await render(result({ tests: [passed], postConsole: ['ok'] }))

    expect(tagWith(html, '(1/1)')).toContain('text-[var(--gc-success)]')
    expect(text(html)).toBe('(1/1)')
  })

  it('shows a red dot when a script failed without tests', async () => {
    const html = await render(result({ errors: [postError], postConsole: ['ok'] }))

    const dot = tagWith(html, 'aria-label="Script failed"')
    expect(dot).toContain('text-[var(--gc-error)]')
    expect(text(html)).toBe('(●)')
  })

  it('shows a muted dot for console output alone', async () => {
    const html = await render(result({ preConsole: ['hello'] }))

    expect(tagWith(html, '(●)')).toContain('text-muted-foreground')
    expect(html).not.toContain('Script failed')
  })

  it('renders nothing without tests, errors or output', async () => {
    expect(text(await render(null))).toBe('')
    expect(text(await render(result({})))).toBe('')
  })
})
