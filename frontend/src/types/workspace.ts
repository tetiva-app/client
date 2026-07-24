export interface Workspace {
  id: string
  name: string
  isActive: boolean
  version: number
  remoteWorkspaceId: string | null
  createdAt: string
  updatedAt: string
}
