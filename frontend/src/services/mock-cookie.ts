import type { Result } from '@/types/common'
import type {
  AddCookieRequest,
  CookieDTO,
  CookieServiceAPI,
  EditCookieRequest,
} from './cookie-api'
import { makeError } from './makeError'

const STORAGE_KEY = 'gc-mock-cookies'

function load(): CookieDTO[] {
  try { return JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]') } catch { return [] }
}
function save(cs: CookieDTO[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(cs))
}

export class MockCookieService implements CookieServiceAPI {
  async list(_workspaceID: string): Promise<Result<CookieDTO[]>> {
    return { data: load() }
  }

  async getForURL(_workspaceID: string, url: string): Promise<Result<CookieDTO[]>> {
    try {
      const u = new URL(url)
      const data = load().filter((c) =>
        u.hostname === c.domain
        || (!c.hostOnly && u.hostname.endsWith('.' + c.domain))
      )
      return { data }
    } catch {
      return { data: [] }
    }
  }

  async add(req: AddCookieRequest): Promise<Result<CookieDTO>> {
    const created: CookieDTO = {
      id: crypto.randomUUID(),
      domain: req.domain, hostOnly: req.hostOnly,
      path: req.path || '/', name: req.name, value: req.value,
      expiresAt: req.expiresAt, httpOnly: req.httpOnly, secure: req.secure,
      sameSite: req.sameSite as CookieDTO['sameSite'],
    }
    const all = load().filter((c) => !(
      c.domain === created.domain && c.path === created.path && c.name === created.name
    ))
    all.push(created)
    save(all)
    return { data: created }
  }

  async edit(req: EditCookieRequest): Promise<Result<CookieDTO>> {
    const all = load()
    const idx = all.findIndex((c) => c.id === req.id)
    if (idx < 0) {
      return makeError<CookieDTO>('not_found', 'cookie not found')
    }
    all[idx] = {
      ...all[idx],
      ...req,
      sameSite: req.sameSite as CookieDTO['sameSite'],
    }
    save(all)
    return { data: all[idx] }
  }

  async delete(id: string): Promise<Result<Record<string, never>>> {
    save(load().filter((c) => c.id !== id))
    return { data: {} as Record<string, never> }
  }

  async deleteByDomain(_workspaceID: string, domain: string): Promise<Result<number>> {
    const all = load()
    const before = all.length
    save(all.filter((c) => c.domain !== domain))
    return { data: before - load().length }
  }

  async clear(_workspaceID: string): Promise<Result<number>> {
    const before = load().length
    save([])
    return { data: before }
  }
}
