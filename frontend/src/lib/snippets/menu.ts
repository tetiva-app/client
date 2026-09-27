import type { SnippetTargetMeta } from './targets'

export interface CopyMenuModel {
  top: SnippetTargetMeta[]
  submenu: SnippetTargetMeta[]
}

// A single leftover language stays top-level: a one-item submenu only adds a hover.
export function copyMenuModel(targets: SnippetTargetMeta[]): CopyMenuModel {
  if (targets.length <= 2) return { top: [...targets], submenu: [] }
  return { top: targets.slice(0, 1), submenu: targets.slice(1) }
}
