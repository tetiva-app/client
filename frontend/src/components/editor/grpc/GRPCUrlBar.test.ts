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

async function render(props: { host?: string; service?: string; method?: string; loading: boolean }): Promise<string[]> {
  const emitted: string[] = []
  const on = (name: string) => (...args: unknown[]) => { emitted.push([name, ...args].join(' ')) }
  await renderToString(createSSRApp(GRPCUrlBar, {
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
})
