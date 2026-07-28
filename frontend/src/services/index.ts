import type { CollectionServiceAPI } from './api'
import type { RequestServiceAPI } from './request-api'
import type { EnvironmentServiceAPI } from './environment-api'
import type { PortabilityServiceAPI } from './portability-api'
import type { WorkspaceServiceAPI } from './workspace-api'
import type { WindowServiceAPI } from './window-api'
import type { SyncServiceAPI } from './sync-api'
import type { SearchServiceAPI } from './search-api'
import type { CookieServiceAPI } from './cookie-api'
import type { HistoryServiceAPI } from './history-api'
import type { SettingsServiceAPI } from './settings-api'

export type { CollectionServiceAPI }
export type {
  CreateCollectionRequest,
  EditCollectionRequest,
  DeleteCollectionRequest,
  ReorderCollectionRequest,
  MoveCollectionReq,
} from './api'

export type { RequestServiceAPI }
export type {
  CreateRequestReq,
  EditRequestReq,
  DeleteRequestReq,
  ReorderRequestReq,
  MoveRequestReq,
} from './request-api'

export type { EnvironmentServiceAPI }
export type {
  CreateEnvironmentReq,
  EditEnvironmentReq,
  DeleteEnvironmentReq,
  AddVariableReq,
  EditVariableReq,
  DeleteVariableReq,
} from './environment-api'

export type { PortabilityServiceAPI }
export type {
  ImportCollectionResult,
  ImportEnvironmentResult,
  ExportResult,
} from './portability-api'

export type { WorkspaceServiceAPI }
export type {
  CreateWorkspaceRequest,
  EditWorkspaceRequest,
  DeleteWorkspaceRequest,
  SetActiveWorkspaceRequest,
} from './workspace-api'

export type { WindowServiceAPI }
export type { SchemaContent } from './window-api'

export type { SyncServiceAPI }
export type {
  AuthState,
  MeState,
  ConnectRequest,
  RegisterRequest,
  SyncStatus,
  RemoteWorkspace,
  LinkWorkspaceRequest,
  UnlinkWorkspaceRequest,
} from './sync-api'

export type { SearchServiceAPI, SearchHitDTO, SearchResponseDTO } from './search-api'

export type { CookieServiceAPI, CookieDTO } from './cookie-api'

export type {
  HistoryServiceAPI,
  DeleteHistoryRequest,
  ClearHistoryRequest,
  ReplayHistoryRequest,
} from './history-api'

export type { MCPSettings, SetMCPSettingsRequest, SettingsServiceAPI } from './settings-api'

export type { WebSocketServiceAPI } from './websocket-api'

function detectWailsEnvironment(): boolean {
  if (typeof window === 'undefined') return false
  // Wails v3: runtime sets window._wails
  if ('_wails' in window) return true
  // Wails v3 macOS: native WebKit bridge
  if ((window as any).webkit?.messageHandlers?.external) return true
  // Wails v3 Windows: WebView2 bridge exists before page scripts run, while
  // window._wails is only injected after navigationCompleted
  if ((window as any).chrome?.webview?.postMessage) return true
  // Wails v2 fallback
  if ('__wails_runtime__' in window || '__wails__' in window) return true
  return false
}

// Detect once at module load: /wails/runtime.js sets window._wails before app
// modules execute, while any later import('@wailsio/runtime') sets it as a side
// effect even in a plain browser — lazy detection flipped mock services to Wails.
const IS_WAILS_ENVIRONMENT = detectWailsEnvironment()

export function isWailsEnvironment(): boolean {
  return IS_WAILS_ENVIRONMENT
}

let collectionService: CollectionServiceAPI | null = null

export async function getCollectionService(): Promise<CollectionServiceAPI> {
  if (!collectionService) {
    if (isWailsEnvironment()) {
      const { WailsCollectionService } = await import('./wails-collection')
      collectionService = new WailsCollectionService()
    } else {
      const { MockCollectionService } = await import('./mock-collection')
      collectionService = new MockCollectionService()
      console.info('[Tetiva] Running in browser mode with mock services')
    }
  }
  return collectionService
}

let requestService: RequestServiceAPI | null = null

export async function getRequestService(): Promise<RequestServiceAPI> {
  if (!requestService) {
    if (isWailsEnvironment()) {
      const { WailsRequestService } = await import('./wails-request')
      requestService = new WailsRequestService()
    } else {
      const { MockRequestService } = await import('./mock-request')
      requestService = new MockRequestService()
    }
  }
  return requestService
}

let environmentService: EnvironmentServiceAPI | null = null

export async function getEnvironmentService(): Promise<EnvironmentServiceAPI> {
  if (!environmentService) {
    if (isWailsEnvironment()) {
      const { WailsEnvironmentService } = await import('./wails-environment')
      environmentService = new WailsEnvironmentService()
    } else {
      const { MockEnvironmentService } = await import('./mock-environment')
      environmentService = new MockEnvironmentService()
    }
  }
  return environmentService
}

let portabilityService: PortabilityServiceAPI | null = null

export async function getPortabilityService(): Promise<PortabilityServiceAPI> {
  if (!portabilityService) {
    if (isWailsEnvironment()) {
      const { WailsPortabilityService } = await import('./wails-portability')
      portabilityService = new WailsPortabilityService()
    } else {
      const { MockPortabilityService } = await import('./mock-portability')
      portabilityService = new MockPortabilityService()
    }
  }
  return portabilityService
}

let workspaceService: WorkspaceServiceAPI | null = null

export async function getWorkspaceService(): Promise<WorkspaceServiceAPI> {
  if (!workspaceService) {
    if (isWailsEnvironment()) {
      const { WailsWorkspaceService } = await import('./wails-workspace')
      workspaceService = new WailsWorkspaceService()
    } else {
      const { MockWorkspaceService } = await import('./mock-workspace')
      workspaceService = new MockWorkspaceService()
    }
  }
  return workspaceService
}

let windowService: WindowServiceAPI | null = null

export async function getWindowService(): Promise<WindowServiceAPI | null> {
  if (!windowService) {
    if (isWailsEnvironment()) {
      const { WailsWindowService } = await import('./wails-window')
      windowService = new WailsWindowService()
    } else {
      // No window management in browser mode
      return null
    }
  }
  return windowService
}

let syncService: Promise<SyncServiceAPI | null> | null = null

// Memoize the promise, not the instance: concurrent callers would otherwise each
// build their own service and fork the mock's in-memory session.
export async function getSyncService(): Promise<SyncServiceAPI | null> {
  if (!syncService) {
    syncService = (async () => {
      if (isWailsEnvironment()) {
        const { WailsSyncService } = await import('./wails-sync')
        return new WailsSyncService()
      }
      const { MockSyncService } = await import('./mock-sync')
      return new MockSyncService()
    })()
  }
  return syncService
}

let searchService: SearchServiceAPI | null = null

export async function getSearchService(): Promise<SearchServiceAPI> {
  if (!searchService) {
    if (isWailsEnvironment()) {
      const { WailsSearchService } = await import('./wails-search')
      searchService = new WailsSearchService()
    } else {
      const { MockSearchService } = await import('./mock-search')
      searchService = new MockSearchService()
    }
  }
  return searchService
}

let cookieService: CookieServiceAPI | null = null

export async function getCookieService(): Promise<CookieServiceAPI> {
  if (!cookieService) {
    if (isWailsEnvironment()) {
      const { WailsCookieService } = await import('./wails-cookie')
      cookieService = new WailsCookieService()
    } else {
      const { MockCookieService } = await import('./mock-cookie')
      cookieService = new MockCookieService()
    }
  }
  return cookieService
}

let historyService: HistoryServiceAPI | null = null

export async function getHistoryService(): Promise<HistoryServiceAPI> {
  if (!historyService) {
    if (isWailsEnvironment()) {
      const { WailsHistoryService } = await import('./wails-history')
      historyService = new WailsHistoryService()
    } else {
      const { MockHistoryService } = await import('./mock-history')
      historyService = new MockHistoryService()
    }
  }
  return historyService
}

let settingsService: SettingsServiceAPI | null = null

export async function getSettingsService(): Promise<SettingsServiceAPI> {
  if (!settingsService) {
    if (isWailsEnvironment()) {
      const { WailsSettingsService } = await import('./wails-settings')
      settingsService = new WailsSettingsService()
    } else {
      const { MockSettingsService } = await import('./mock-settings')
      settingsService = new MockSettingsService()
    }
  }
  return settingsService
}

let websocketService: import('./websocket-api').WebSocketServiceAPI | null = null

export async function getWebSocketService(): Promise<import('./websocket-api').WebSocketServiceAPI> {
  if (!websocketService) {
    if (isWailsEnvironment()) {
      const { WailsWebSocketService } = await import('./wails-websocket')
      websocketService = new WailsWebSocketService()
    } else {
      const { MockWebSocketService } = await import('./mock-websocket')
      websocketService = new MockWebSocketService()
    }
  }
  return websocketService
}
