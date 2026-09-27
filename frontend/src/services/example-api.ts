import type { Example, ExampleProtocol } from '@/types/example'
import type { HeaderItem } from '@/types/request'
import type { Result } from '@/types/common'

export interface CreateExampleReq {
  requestId: string
  name: string
  statusCode: number
  statusText: string
  headers: HeaderItem[]
  body: string
  contentType: string
  protocol: ExampleProtocol
}

export interface EditExampleReq {
  id: string
  name: string
  statusCode: number
  statusText: string
  headers: HeaderItem[]
  body: string
  contentType: string
  version: number
}

export interface DeleteExampleReq {
  id: string
  version: number
}

export interface ScanExampleReq {
  headers: HeaderItem[]
  body: string
}

export interface ExampleServiceAPI {
  list(requestId: string): Promise<Result<Example[]>>
  create(req: CreateExampleReq): Promise<Result<Example>>
  edit(req: EditExampleReq): Promise<Result<Example>>
  delete(req: DeleteExampleReq): Promise<Result<boolean>>
  // Names what looks like a credential and would survive masking, e.g. ["JWT"].
  scanSecrets(req: ScanExampleReq): Promise<Result<string[]>>
}
