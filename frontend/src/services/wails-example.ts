import type { Example } from '@/types/example'
import type { Result } from '@/types/common'
import type {
  ExampleServiceAPI,
  CreateExampleReq,
  EditExampleReq,
  DeleteExampleReq,
  ScanExampleReq,
} from './example-api'
import { unwrap } from './unwrap'

// Wails bindings are auto-generated at runtime — lazy import to avoid build errors
let _svc: any = null
async function svc() {
  if (!_svc) {
    const mod = await import('../../bindings/github.com/tetiva-app/client/internal/adapters/wails')
    _svc = mod.ExampleService
  }
  return _svc
}

export class WailsExampleService implements ExampleServiceAPI {
  async list(requestId: string): Promise<Result<Example[]>> {
    return unwrap<Example[]>(await (await svc()).List(requestId))
  }

  async create(req: CreateExampleReq): Promise<Result<Example>> {
    return unwrap<Example>(await (await svc()).Create(req))
  }

  async edit(req: EditExampleReq): Promise<Result<Example>> {
    return unwrap<Example>(await (await svc()).Edit(req))
  }

  async delete(req: DeleteExampleReq): Promise<Result<boolean>> {
    return unwrap<boolean>(await (await svc()).Delete(req))
  }

  async scanSecrets(req: ScanExampleReq): Promise<Result<string[]>> {
    return unwrap<string[]>(await (await svc()).ScanSecrets(req))
  }
}
