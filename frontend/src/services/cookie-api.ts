import type { Result } from '@/types/common'

export interface CookieDTO {
  id: string
  domain: string
  hostOnly: boolean
  path: string
  name: string
  value: string
  expiresAt: number | null
  httpOnly: boolean
  secure: boolean
  sameSite: '' | 'Lax' | 'Strict' | 'None'
}

export interface AddCookieRequest {
  workspaceId: string
  domain: string
  hostOnly: boolean
  path: string
  name: string
  value: string
  expiresAt: number | null
  httpOnly: boolean
  secure: boolean
  sameSite: string
}

export interface EditCookieRequest {
  id: string
  domain: string
  hostOnly: boolean
  path: string
  name: string
  value: string
  expiresAt: number | null
  httpOnly: boolean
  secure: boolean
  sameSite: string
}

export interface CookieServiceAPI {
  list(workspaceID: string): Promise<Result<CookieDTO[]>>
  getForURL(workspaceID: string, url: string): Promise<Result<CookieDTO[]>>
  add(req: AddCookieRequest): Promise<Result<CookieDTO>>
  edit(req: EditCookieRequest): Promise<Result<CookieDTO>>
  delete(id: string): Promise<Result<Record<string, never>>>
  deleteByDomain(workspaceID: string, domain: string): Promise<Result<number>>
  clear(workspaceID: string): Promise<Result<number>>
}
