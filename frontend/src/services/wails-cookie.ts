import type { Result } from '@/types/common'
import type {
  AddCookieRequest,
  CookieDTO,
  CookieServiceAPI,
  EditCookieRequest,
} from './cookie-api'
import { unwrap } from './unwrap'

let _svc: any = null
async function svc() {
  if (!_svc) {
    const mod = await import('../../bindings/github.com/tetiva-app/client/internal/adapters/wails')
    _svc = mod.CookieService
  }
  return _svc
}

export class WailsCookieService implements CookieServiceAPI {
  async list(workspaceID: string): Promise<Result<CookieDTO[]>> {
    return unwrap<CookieDTO[]>(await (await svc()).List(workspaceID))
  }

  async getForURL(workspaceID: string, url: string): Promise<Result<CookieDTO[]>> {
    return unwrap<CookieDTO[]>(await (await svc()).GetForURL(workspaceID, url))
  }

  async add(req: AddCookieRequest): Promise<Result<CookieDTO>> {
    return unwrap<CookieDTO>(await (await svc()).Add(req))
  }

  async edit(req: EditCookieRequest): Promise<Result<CookieDTO>> {
    return unwrap<CookieDTO>(await (await svc()).Edit(req))
  }

  async delete(id: string): Promise<Result<Record<string, never>>> {
    return unwrap<Record<string, never>>(await (await svc()).Delete(id))
  }

  async deleteByDomain(workspaceID: string, domain: string): Promise<Result<number>> {
    return unwrap<number>(await (await svc()).DeleteByDomain(workspaceID, domain))
  }

  async clear(workspaceID: string): Promise<Result<number>> {
    return unwrap<number>(await (await svc()).Clear(workspaceID))
  }
}
