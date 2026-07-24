export type { Result, ResultError } from './common'

export interface Collection {
  id: string
  workspaceId: string
  parentId: string | null
  name: string
  description: string
  authType: string
  authData: string
  preScript: string
  postScript: string
  sortOrder: number
  version: number
  createdAt: string
  updatedAt: string
}

export interface CollectionTreeNode extends Collection {
  children: CollectionTreeNode[]
}
