export interface Environment {
  id: string
  name: string
  isActive: boolean
  version: number
  createdAt: string
  updatedAt: string
}

export interface Variable {
  id: string
  environmentId: string
  key: string
  value: string
  isSecret: boolean
  enabled: boolean
  sortOrder: number
  version: number
  createdAt: string
  updatedAt: string
}
