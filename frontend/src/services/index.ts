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
import type { AuthServiceAPI } from './auth-api'

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
  ServerCapabilities,
  SignInIntent,
  StartBrowserSignInRequest,
  BrowserSignInInfo,
  BrowserSignInState,
  BrowserSignInStatus,
  BrowserSignInEvent,
} from './sync-api'
export { emptyBrowserSignInInfo } from './sync-api'

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

export type { AuthServiceAPI }
export type {
  AuthConfigReq,
  AuthOwnerKind,
  ResolvedOwner,
  TokenState,
  TokenStatus,
} from './auth-api'

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
  // Server build (-tags server): nothing is injected into the HTML, the bundled
  // runtime sets window._wails only after this module ran, so opt in via URL
  if (new URLSearchParams(window.location?.search ?? '').get('wails') === '1') return true
  return false
}

// Detect once at module load: /wails/runtime.js sets window._wails before app
// modules execute, while any later import('@wailsio/runtime') sets it as a side
// effect even in a plain browser — lazy detection flipped mock services to Wails.
const IS_WAILS_ENVIRONMENT = detectWailsEnvironment()

export function isWailsEnvironment(): boolean {
  return IS_WAILS_ENVIRONMENT
}

// Every getter memoizes the promise, not the instance: two callers racing the dynamic
// import would each build a service and fork the mock's session. Rejections aren't cached.
function memoize<T>(load: () => Promise<T>): () => Promise<T> {
  let cached: Promise<T> | null = null
  return () => {
    if (!cached) {
      const pending = load()
      cached = pending
      pending.catch(() => {
        if (cached === pending) cached = null
      })
    }
    return cached
  }
}

export const getCollectionService = memoize<CollectionServiceAPI>(async () => {
  if (isWailsEnvironment()) {
    const { WailsCollectionService } = await import('./wails-collection')
    return new WailsCollectionService()
  }
  const { MockCollectionService } = await import('./mock-collection')
  console.info('[Tetiva] Running in browser mode with mock services')
  return new MockCollectionService()
})

export const getRequestService = memoize<RequestServiceAPI>(async () => {
  if (isWailsEnvironment()) {
    const { WailsRequestService } = await import('./wails-request')
    return new WailsRequestService()
  }
  const { MockRequestService } = await import('./mock-request')
  return new MockRequestService()
})

export const getEnvironmentService = memoize<EnvironmentServiceAPI>(async () => {
  if (isWailsEnvironment()) {
    const { WailsEnvironmentService } = await import('./wails-environment')
    return new WailsEnvironmentService()
  }
  const { MockEnvironmentService } = await import('./mock-environment')
  return new MockEnvironmentService()
})

export const getPortabilityService = memoize<PortabilityServiceAPI>(async () => {
  if (isWailsEnvironment()) {
    const { WailsPortabilityService } = await import('./wails-portability')
    return new WailsPortabilityService()
  }
  const { MockPortabilityService } = await import('./mock-portability')
  return new MockPortabilityService()
})

export const getWorkspaceService = memoize<WorkspaceServiceAPI>(async () => {
  if (isWailsEnvironment()) {
    const { WailsWorkspaceService } = await import('./wails-workspace')
    return new WailsWorkspaceService()
  }
  const { MockWorkspaceService } = await import('./mock-workspace')
  return new MockWorkspaceService()
})

export const getWindowService = memoize<WindowServiceAPI | null>(async () => {
  // No window management in browser mode
  if (!isWailsEnvironment()) return null
  const { WailsWindowService } = await import('./wails-window')
  return new WailsWindowService()
})

export const getSyncService = memoize<SyncServiceAPI | null>(async () => {
  if (isWailsEnvironment()) {
    const { WailsSyncService } = await import('./wails-sync')
    return new WailsSyncService()
  }
  const { MockSyncService } = await import('./mock-sync')
  return new MockSyncService()
})

export const getSearchService = memoize<SearchServiceAPI>(async () => {
  if (isWailsEnvironment()) {
    const { WailsSearchService } = await import('./wails-search')
    return new WailsSearchService()
  }
  const { MockSearchService } = await import('./mock-search')
  return new MockSearchService()
})

export const getCookieService = memoize<CookieServiceAPI>(async () => {
  if (isWailsEnvironment()) {
    const { WailsCookieService } = await import('./wails-cookie')
    return new WailsCookieService()
  }
  const { MockCookieService } = await import('./mock-cookie')
  return new MockCookieService()
})

export const getHistoryService = memoize<HistoryServiceAPI>(async () => {
  if (isWailsEnvironment()) {
    const { WailsHistoryService } = await import('./wails-history')
    return new WailsHistoryService()
  }
  const { MockHistoryService } = await import('./mock-history')
  return new MockHistoryService()
})

export const getSettingsService = memoize<SettingsServiceAPI>(async () => {
  if (isWailsEnvironment()) {
    const { WailsSettingsService } = await import('./wails-settings')
    return new WailsSettingsService()
  }
  const { MockSettingsService } = await import('./mock-settings')
  return new MockSettingsService()
})

export const getWebSocketService = memoize<import('./websocket-api').WebSocketServiceAPI>(async () => {
  if (isWailsEnvironment()) {
    const { WailsWebSocketService } = await import('./wails-websocket')
    return new WailsWebSocketService()
  }
  const { MockWebSocketService } = await import('./mock-websocket')
  return new MockWebSocketService()
})

export const getAuthService = memoize<AuthServiceAPI>(async () => {
  if (isWailsEnvironment()) {
    const { WailsAuthService } = await import('./wails-auth')
    return new WailsAuthService()
  }
  const { MockAuthService } = await import('./mock-auth')
  return new MockAuthService()
})
