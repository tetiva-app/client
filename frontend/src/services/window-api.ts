import type { Result } from '@/types/common'
import type { ThemePreference } from '@/lib/settings-storage'

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
  closeDetached(requestId: string): Promise<Result<boolean>>
  getSchemaContent(schemaId: string): Promise<Result<SchemaContent>>
  childWindowCount(): Promise<Result<number>>
  closeChildWindows(): Promise<Result<void>>
  setTheme(preference: ThemePreference, effective: 'light' | 'dark'): Promise<Result<void>>
}
