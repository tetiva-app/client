import { createRenderer, type App, type Component, type RendererOptions } from 'vue'
import type { Pinia } from 'pinia'

export interface El {
  parent: El | null
  children: El[]
  props: Record<string, unknown>
  text: string
}

const el = (text = ''): El => ({ parent: null, children: [], props: {}, text })

function detach(n: El) {
  if (!n.parent) return
  n.parent.children.splice(n.parent.children.indexOf(n), 1)
  n.parent = null
}

const ops: RendererOptions<El, El> = {
  createElement: () => el(),
  createText: text => el(text),
  createComment: () => el(),
  setText: (n, text) => { n.text = text },
  setElementText: (n, text) => { n.children = []; n.text = text },
  insert: (child, parent, anchor) => {
    detach(child)
    const at = anchor ? parent.children.indexOf(anchor) : -1
    if (at >= 0) parent.children.splice(at, 0, child)
    else parent.children.push(child)
    child.parent = parent
  },
  remove: detach,
  parentNode: n => n.parent,
  nextSibling: n => (n.parent ? n.parent.children[n.parent.children.indexOf(n) + 1] ?? null : null),
  patchProp: (n, key, _prev, next) => { n.props[key] = next },
}

export function mountTree(root: Component, pinia: Pinia, props?: Record<string, unknown>): { app: App; root: El } {
  const app = createRenderer(ops).createApp(root, props)
  app.use(pinia)
  const container = el()
  app.mount(container)
  return { app, root: container }
}

export function find(root: El, testid: string): El | null {
  if (root.props['data-testid'] === testid) return root
  for (const child of root.children) {
    const hit = find(child, testid)
    if (hit) return hit
  }
  return null
}

export function textOf(n: El): string {
  return n.text + n.children.map(textOf).join('')
}
