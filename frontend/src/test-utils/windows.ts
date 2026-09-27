import { createRenderer, type App, type Component, type RendererOptions } from 'vue'
import { createPinia } from 'pinia'

const handlers = new Map<string, Set<(evt: unknown) => void>>()

export const wailsBus = {
  handlers,
  On(name: string, cb: (evt: unknown) => void) {
    if (!handlers.has(name)) handlers.set(name, new Set())
    handlers.get(name)!.add(cb)
    return () => { handlers.get(name)?.delete(cb) }
  },
  async Emit(name: string, data?: unknown) {
    for (const cb of [...(handlers.get(name) ?? [])]) cb({ name, data })
  },
  listeners(name: string): number {
    return handlers.get(name)?.size ?? 0
  },
}

interface Node { parent: Node | null; children: Node[] }

const node = (): Node => ({ parent: null, children: [] })

function detach(n: Node) {
  if (!n.parent) return
  n.parent.children.splice(n.parent.children.indexOf(n), 1)
  n.parent = null
}

const nodeOps: RendererOptions<Node, Node> = {
  createElement: node,
  createText: node,
  createComment: node,
  setText: () => {},
  setElementText: () => {},
  insert: (child, parent) => { detach(child); parent.children.push(child); child.parent = parent },
  remove: detach,
  parentNode: n => n.parent,
  nextSibling: n => (n.parent ? n.parent.children[n.parent.children.indexOf(n) + 1] ?? null : null),
  patchProp: () => {},
}

export function mountWindow(root: Component, props?: Record<string, unknown>, seed?: (app: App) => void): App {
  const app = createRenderer(nodeOps).createApp(root, props)
  app.use(createPinia())
  seed?.(app)
  app.mount(node())
  return app
}
