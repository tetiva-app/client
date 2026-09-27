import { describe, it, expect, vi } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { targetMetaFor } from '@/lib/snippets/targets'
import type { Request } from '@/types/request'

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

import GraphQLUrlBar from './GraphQLUrlBar.vue'

async function render(request: Partial<Request>, loading: boolean): Promise<string[]> {
  const emitted: string[] = []
  const on = (name: string) => (...args: unknown[]) => { emitted.push([name, ...args].join(' ')) }
  await renderToString(createSSRApp(GraphQLUrlBar, {
    request: { url: 'https://api.test/graphql', ...request } as Request,
    loading,
    onExecute: on('execute'),
    onCancel: on('cancel'),
    onCopy: on('copy'),
    onGenerate: on('generate'),
  }))
  return emitted
}

describe('GraphQLUrlBar', () => {
  it('cancels a running query instead of asking to run it again', async () => {
    const emitted = await render({}, true)

    expect(split.attrs.loading).toBe(true)
    split.attrs.onCancel()

    expect(emitted).toEqual(['cancel'])
  })

  it('runs the query and hands Copy as and Generate code to the editor', async () => {
    const emitted = await render({}, false)

    expect(split.attrs.label).toBe('Query')
    expect(split.attrs.disabled).toBe(false)
    expect(split.attrs.targets).toEqual(targetMetaFor('graphql'))
    split.attrs.onRun()
    split.attrs.onCopy('python-requests')
    split.attrs.onGenerate()

    expect(emitted).toEqual(['execute', 'copy python-requests', 'generate'])
  })

  it('keeps Query disabled without a URL', async () => {
    await render({ url: '' }, false)

    expect(split.attrs.disabled).toBe(true)
  })
})
