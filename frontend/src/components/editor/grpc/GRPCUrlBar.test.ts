import { describe, it, expect, vi } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { targetMetaFor } from '@/lib/snippets/targets'

const split = vi.hoisted(() => ({ attrs: {} as Record<string, any> }))

vi.mock('@/components/editor/RunSplitButton.vue', async () => {
  const { defineComponent } = await import('vue')
  return {
    default: defineComponent({
      inheritAttrs: false,
      setup(_, { attrs }) {
        split.attrs = attrs
        return () => null
      },
    }),
  }
})

import GRPCUrlBar from './GRPCUrlBar.vue'

let html = ''

function lockButton(): string {
  const at = html.indexOf('aria-label="TLS"')
  expect(at).toBeGreaterThanOrEqual(0)
  return html.slice(html.lastIndexOf('<button', at), html.indexOf('>', at) + 1)
}

async function render(props: { host?: string; service?: string; method?: string; loading: boolean }): Promise<string[]> {
  const emitted: string[] = []
  const on = (name: string) => (...args: unknown[]) => { emitted.push([name, ...args].join(' ')) }
  html = await renderToString(createSSRApp(GRPCUrlBar, {
    host: 'localhost:50051',
    service: 'example.v1.UserService',
    method: 'GetUser',
    ...props,
    onInvoke: on('invoke'),
    onCancel: on('cancel'),
    onCopy: on('copy'),
    onGenerate: on('generate'),
  }))
  return emitted
}

describe('GRPCUrlBar', () => {
  it('cancels a running call instead of asking to invoke it again', async () => {
    const emitted = await render({ loading: true })

    expect(split.attrs.loading).toBe(true)
    split.attrs.onCancel()

    expect(emitted).toEqual(['cancel'])
  })

  it('invokes the method and hands Copy as and Generate code to the editor', async () => {
    const emitted = await render({ loading: false })

    expect(split.attrs.label).toBe('Invoke')
    expect(split.attrs.disabled).toBe(false)
    expect(split.attrs.targets).toEqual(targetMetaFor('grpc'))
    split.attrs.onRun()
    split.attrs.onCopy('grpcurl')
    split.attrs.onGenerate()

    expect(emitted).toEqual(['invoke', 'copy grpcurl', 'generate'])
  })

  it('keeps Invoke disabled until a method is picked', async () => {
    await render({ method: '', loading: false })

    expect(split.attrs.disabled).toBe(true)
  })

  it('shows a closed lock for a grpcs:// address', async () => {
    await render({ host: 'grpcs://api.example.com', loading: false })

    expect(lockButton()).toContain('aria-pressed="true"')
    expect(lockButton()).toContain('title="TLS on (click to turn off)"')
  })

  it('leaves TLS to the variable when the whole address is one', async () => {
    await render({ host: '{{baseUrl}}', loading: false })

    expect(lockButton()).toMatch(/ disabled[ >]/)
    expect(lockButton()).toContain('title="TLS is set by the variable"')
  })

  it('keeps the lock usable when a variable holds only the host', async () => {
    await render({ host: '{{host}}:50051', loading: false })

    expect(lockButton()).not.toMatch(/ disabled[ >]/)
    expect(lockButton()).toContain('title="TLS off (click to turn on)"')
  })

  it('shows an open lock for a plaintext address and hints both forms', async () => {
    await render({ host: 'localhost:50051', loading: false })

    expect(lockButton()).toContain('aria-pressed="false"')
    expect(lockButton()).toContain('title="TLS off (click to turn on)"')
    expect(html).toContain('placeholder="host:port or grpcs://host"')
  })
})
