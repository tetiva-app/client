import type { Result } from '@/types/common'
import type { WindowServiceAPI, SchemaContent } from './window-api'
import { unwrap } from './unwrap'

// Wails bindings are auto-generated at runtime — lazy import to avoid build errors
let _svc: any = null
async function svc() {
  if (!_svc) {
    const mod = await import('../../bindings/github.com/tetiva-app/client/internal/adapters/wails')
    _svc = mod.WindowService
  }
  return _svc
}

export class WailsWindowService implements WindowServiceAPI {
  async detachRequest(requestId: string, protocol: string, title: string): Promise<Result<boolean>> {
    return unwrap<boolean>(await (await svc()).DetachRequest(requestId, protocol, title))
  }

  async openSchemaViewer(definition: string, source: string, title: string, language = 'protobuf', schemaJSON = ''): Promise<Result<boolean>> {
    return unwrap<boolean>(await (await svc()).OpenSchemaViewer(definition, source, title, language, schemaJSON))
  }

  async isDetached(requestId: string): Promise<Result<boolean>> {
    return unwrap<boolean>(await (await svc()).IsDetached(requestId))
  }

  async focusDetachedWindow(requestId: string): Promise<Result<boolean>> {
    return unwrap<boolean>(await (await svc()).FocusDetachedWindow(requestId))
  }

  async getSchemaContent(schemaId: string): Promise<Result<SchemaContent>> {
    return unwrap<SchemaContent>(await (await svc()).GetSchemaContent(schemaId))
  }
}
