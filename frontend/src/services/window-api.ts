import type { Result } from '@/types/common'

export interface SchemaContent {
  definition: string
  source: string
  language: string
  schemaJSON: string
}

export interface WindowServiceAPI {
  detachRequest(requestId: string, protocol: string, title: string): Promise<Result<boolean>>
  openSchemaViewer(definition: string, source: string, title: string, language?: string, schemaJSON?: string): Promise<Result<boolean>>
  isDetached(requestId: string): Promise<Result<boolean>>
  focusDetachedWindow(requestId: string): Promise<Result<boolean>>
  getSchemaContent(schemaId: string): Promise<Result<SchemaContent>>
}
