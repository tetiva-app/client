import { describe, it, expect, vi, afterEach } from 'vitest'
import { defineComponent, h, nextTick, ref, type App, type Component } from 'vue'
import type { EditorState, Transaction, TransactionSpec } from '@codemirror/state'
import { mountWindow } from '@/test-utils/windows'

interface HeadlessView {
  state: EditorState
  dispatch(...input: Array<Transaction | TransactionSpec>): void
}

const views = vi.hoisted(() => [] as HeadlessView[])

vi.mock('@codemirror/view', async (importOriginal) => {
  const cm = await importOriginal<typeof import('@codemirror/view')>()
  const { Transaction } = await import('@codemirror/state')
  function EditorView(config: { state: EditorState }) {
    const view = {
      state: config.state,
      dispatch(...input: Array<Transaction | TransactionSpec>) {
        const tr = input[0] instanceof Transaction ? input[0] : view.state.update(...(input as TransactionSpec[]))
        view.state = tr.state
        const update = {
          view, state: tr.state, startState: tr.startState, transactions: [tr],
          docChanged: tr.docChanged, selectionSet: tr.selection !== undefined,
        }
        for (const listener of tr.state.facet(cm.EditorView.updateListener)) listener(update as never)
      },
      destroy() {},
    }
    views.push(view)
    return view
  }
  Object.setPrototypeOf(EditorView, cm.EditorView)
  return { ...cm, EditorView }
})

vi.mock('./MarkdownToolbar.vue', () => ({ default: { render: () => null } }))

import MarkdownEditor from './MarkdownEditor.vue'
import CodeEditor from './CodeEditor.vue'
import GraphQLQueryEditor from './graphql/GraphQLQueryEditor.vue'
import VariableInput from '@/components/ui/VariableInput.vue'

interface EditorCase {
  name: string
  component: Component
  event: string
  props: (value: string) => Record<string, unknown>
  view: number
}

const cases: EditorCase[] = [
  { name: 'MarkdownEditor', component: MarkdownEditor, event: 'onUpdate:modelValue', props: v => ({ modelValue: v }), view: 0 },
  { name: 'CodeEditor', component: CodeEditor, event: 'onUpdate:content', props: v => ({ content: v, language: 'json' }), view: 0 },
  { name: 'VariableInput', component: VariableInput, event: 'onUpdate:modelValue', props: v => ({ modelValue: v }), view: 0 },
  {
    name: 'GraphQLQueryEditor query', component: GraphQLQueryEditor, event: 'onUpdate:query',
    props: v => ({ request: { graphqlQuery: v, graphqlVariables: '' }, schema: null }), view: 0,
  },
  {
    name: 'GraphQLQueryEditor variables', component: GraphQLQueryEditor, event: 'onUpdate:variables',
    props: v => ({ request: { graphqlQuery: '', graphqlVariables: v }, schema: null }), view: 1,
  },
]

let app: App | null = null

afterEach(() => {
  app?.unmount()
  app = null
  views.length = 0
})

function mount(c: EditorCase) {
  const value = ref('initial')
  const emitted: string[] = []
  app = mountWindow(defineComponent({
    setup: () => () => h(c.component, {
      ...c.props(value.value),
      [c.event]: (next: string) => { emitted.push(next); value.value = next },
    }),
  }))
  return { value, emitted, view: views[c.view] }
}

describe.each(cases)('$name', (c) => {
  it('emits a local edit', async () => {
    const { emitted, view } = mount(c)

    view.dispatch({ changes: { from: 0, insert: 'my ' } })
    await nextTick()

    expect(emitted).toEqual(['my initial'])
    expect(view.state.doc.toString()).toBe('my initial')
  })

  it('takes external values without emitting them back', async () => {
    const { value, emitted, view } = mount(c)

    value.value = 'first external'
    await nextTick()
    expect(view.state.doc.toString()).toBe('first external')

    value.value = 'second external'
    await nextTick()
    expect(view.state.doc.toString()).toBe('second external')

    expect(emitted).toEqual([])
  })

  it('takes an external value after a local edit', async () => {
    const { value, emitted, view } = mount(c)

    view.dispatch({ changes: { from: 0, insert: 'my ' } })
    await nextTick()
    value.value = 'from sync'
    await nextTick()

    expect(view.state.doc.toString()).toBe('from sync')
    expect(emitted).toEqual(['my initial'])
  })
})

describe('VariableInput on one line', () => {
  it('hands back an external value it had to flatten, so the store sends what the field shows', async () => {
    const { value, emitted, view } = mount(cases.find(c => c.name === 'VariableInput')!)

    value.value = 'https://api.example.com/x\n'
    await nextTick()
    value.value = 'https://api.example.com/y'
    await nextTick()

    expect(view.state.doc.toString()).toBe('https://api.example.com/y')
    expect(emitted).toEqual(['https://api.example.com/x'])
  })
})
