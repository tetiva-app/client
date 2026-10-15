import { presentableDiff } from '@codemirror/merge'

export interface Fragment {
  text: string
  changed: boolean
}

export function changedFragments(a: string, b: string): { a: Fragment[]; b: Fragment[] } {
  const out = { a: [] as Fragment[], b: [] as Fragment[] }
  const push = (list: Fragment[], text: string, changed: boolean) => {
    if (text) list.push({ text, changed })
  }
  let posA = 0
  let posB = 0
  for (const change of presentableDiff(a, b, { scanLimit: 500 })) {
    push(out.a, a.slice(posA, change.fromA), false)
    push(out.a, a.slice(change.fromA, change.toA), true)
    push(out.b, b.slice(posB, change.fromB), false)
    push(out.b, b.slice(change.fromB, change.toB), true)
    posA = change.toA
    posB = change.toB
  }
  push(out.a, a.slice(posA), false)
  push(out.b, b.slice(posB), false)
  return out
}
