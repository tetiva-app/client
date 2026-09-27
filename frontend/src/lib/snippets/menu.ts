import type { SnippetTargetMeta } from './targets'

export interface CopyMenuModel {
  top: SnippetTargetMeta[]
  submenu: SnippetTargetMeta[]
}

export function copyMenuModel(targets: SnippetTargetMeta[]): CopyMenuModel {
  if (targets.length <= 2) return { top: [...targets], submenu: [] }
  return { top: targets.slice(0, 1), submenu: targets.slice(1) }
}
