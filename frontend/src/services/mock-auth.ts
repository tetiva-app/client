import type { Result } from '@/types/common'
import type {
  AuthConfigReq,
  AuthServiceAPI,
  FlowInfo,
  FlowStatus,
  ResolvedOwner,
  StartFlowReq,
  TokenStatus,
} from './auth-api'
import { emptyFlowInfo } from './auth-api'
import { makeError } from './makeError'
import { parseAuthData } from '@/lib/auth-data'
import type { Collection } from '@/types/collection'

const TOKEN_TTL_MS = 3600_000
const MOCK_FLOW_MS = 300
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
const LOOPBACK_V4_RE = /^127\.\d{1,3}\.\d{1,3}\.\d{1,3}$/

function isLoopbackHost(hostname: string): boolean {
  const host = hostname.replace(/^\[|\]$/g, '')
  return host.toLowerCase() === 'localhost' || host === '::1' || LOOPBACK_V4_RE.test(host)
}

// Same rules and the same wording as auth.ValidateEndpointURL, so a
// configuration the real backend refuses cannot look valid in browser mode.
function endpointError(raw: string): string {
  const trimmed = raw.trim()
  if (!trimmed) return 'URL is required'

  let url: URL
  try {
    url = new URL(trimmed)
  } catch {
    return 'URL must be absolute, like https://idp.example/oauth/token'
  }
  if (!url.host) return 'URL must be absolute, like https://idp.example/oauth/token'
  if (url.username || url.password) return 'URL must not carry userinfo'
  if (trimmed.includes('#')) return 'URL must not carry a fragment'

  switch (url.protocol) {
    case 'https:':
      return ''
    case 'http:':
      return isLoopbackHost(url.hostname) ? '' : 'http is allowed only for localhost; use https'
    default:
      return `unsupported URL scheme "${url.protocol.replace(/:$/, '')}"`
  }
}

// auth.ValidateRedirectPort: empty takes the Auth tab default, "0" asks for a
// free port.
function redirectPortError(raw: string): string {
  const trimmed = raw.trim()
  if (!trimmed) return ''
  if (!/^\d+$/.test(trimmed) || Number(trimmed) > 65535) {
    return 'must be a port number between 0 and 65535; 0 picks a free one'
  }
  return ''
}

interface MockFlow {
  status: FlowStatus
  timer: ReturnType<typeof setTimeout> | null
}

// Browser mode has no token store and no IdP: "Get token" mints an in-memory
// token so the Auth tab can be driven end to end without a backend.
export class MockAuthService implements AuthServiceAPI {
  private tokens = new Map<string, number>()
  private flows = new Map<string, MockFlow>()
  private flowHandlers = new Map<string, (s: FlowStatus) => void>()

  private key(req: AuthConfigReq): string {
    return `${req.ownerKind}:${req.ownerId}`
  }

  // Browser mode only: the e2e suite stretches the simulated flow so pressing
  // Cancel while it is pending is not a race against a 300 ms timer.
  private flowDelayMs(): number {
    try {
      const raw = Number(localStorage.getItem('tetiva.mockFlowMs'))
      if (Number.isFinite(raw) && raw >= 0) return raw
    } catch {
      // Storage disabled; the default is fine.
    }
    return MOCK_FLOW_MS
  }

  private status(key: string): TokenStatus {
    const expiresAt = this.tokens.get(key)
    if (!expiresAt) return { state: 'none', expiresAt: '' }
    const iso = new Date(expiresAt).toISOString()
    return { state: expiresAt > Date.now() ? 'valid' : 'expired-refreshable', expiresAt: iso }
  }

  async tokenStatus(req: AuthConfigReq): Promise<Result<TokenStatus>> {
    if (req.authType !== 'oauth2') return { data: { state: 'none', expiresAt: '' } }
    return { data: this.status(this.key(req)) }
  }

  async fetchToken(req: AuthConfigReq): Promise<Result<TokenStatus>> {
    if (req.authType !== 'oauth2') {
      return makeError<TokenStatus>('validation', 'tokens can only be fetched for OAuth 2.0')
    }
    const fields = parseAuthData(req.authData)
    const missing: Record<string, string> = {}
    if (!String(fields.tokenUrl ?? '').trim()) missing.tokenUrl = 'required'
    if (!String(fields.clientId ?? '').trim()) missing.clientId = 'required'
    if (Object.keys(missing).length > 0) {
      return makeError<TokenStatus>('validation', 'Token URL and Client ID are required', missing)
    }

    const key = this.key(req)
    this.tokens.set(key, Date.now() + TOKEN_TTL_MS)
    return { data: this.status(key) }
  }

  async clearToken(req: AuthConfigReq): Promise<Result<Record<string, never>>> {
    this.tokens.delete(this.key(req))
    return { data: {} }
  }

  async startAuthCodeFlow(req: StartFlowReq): Promise<Result<FlowInfo>> {
    return this.startFlow(req, 'authorization_code')
  }

  async startDeviceFlow(req: StartFlowReq): Promise<Result<FlowInfo>> {
    return this.startFlow(req, 'device_code')
  }

  async flowStatus(flowId: string): Promise<Result<FlowStatus>> {
    if (!UUID_RE.test(flowId)) {
      return makeError<FlowStatus>('validation', 'invalid UUID', { flowId: 'invalid UUID' })
    }
    const flow = this.flows.get(flowId)
    if (!flow) return { data: { state: '', error: '', info: emptyFlowInfo() } }
    return { data: { ...flow.status, info: { ...flow.status.info } } }
  }

  async cancelFlow(flowId: string): Promise<Result<Record<string, never>>> {
    if (!UUID_RE.test(flowId)) {
      return makeError<Record<string, never>>('validation', 'invalid UUID', { flowId: 'invalid UUID' })
    }
    const flow = this.flows.get(flowId)
    // Idempotent: an unknown or already finished flow is a success, as in Go.
    if (!flow || flow.status.state !== 'pending') return { data: {} }
    if (flow.timer) clearTimeout(flow.timer)
    flow.timer = null
    flow.status = { ...flow.status, state: 'cancelled', error: '' }
    this.emitFlow(flowId, 'cancelled', '')
    return { data: {} }
  }

  async subscribeFlow(flowId: string, onState: (s: FlowStatus) => void): Promise<() => void> {
    this.flowHandlers.set(flowId, onState)
    return () => this.flowHandlers.delete(flowId)
  }

  // The event carries state and error only, exactly like the Wails one.
  private emitFlow(flowId: string, state: FlowStatus['state'], error: string): void {
    this.flowHandlers.get(flowId)?.({ state, error, info: emptyFlowInfo() })
  }

  private startFlow(req: StartFlowReq, grant: 'authorization_code' | 'device_code'): Result<FlowInfo> {
    if (req.authType !== 'oauth2') {
      return makeError<FlowInfo>('validation', 'browser flows only exist for OAuth 2.0', {
        authType: 'browser flows only exist for OAuth 2.0',
      })
    }
    if (!UUID_RE.test(req.flowId)) {
      return makeError<FlowInfo>('validation', 'invalid UUID', { flowId: 'invalid UUID' })
    }

    const fields = parseAuthData(req.authData)
    const str = (key: string) => String(fields[key] ?? '').trim()
    const invalid: Record<string, string> = {}
    if ((str('grant') || 'client_credentials') !== grant) {
      invalid.grant = `the configuration uses another grant, not "${grant}"`
    }
    const tokenUrl = endpointError(str('tokenUrl'))
    if (tokenUrl) invalid.tokenUrl = tokenUrl
    if (!str('clientId')) invalid.clientId = 'clientId is required'
    if (grant === 'authorization_code') {
      const authUrl = endpointError(str('authUrl'))
      if (authUrl) invalid.authUrl = authUrl
    }
    if (grant === 'device_code') {
      const deviceAuthUrl = endpointError(str('deviceAuthUrl'))
      if (deviceAuthUrl) invalid.deviceAuthUrl = deviceAuthUrl
    }
    if (!['', 'basic', 'body', 'none'].includes(str('clientAuth'))) {
      invalid.clientAuth = `unsupported client authentication "${str('clientAuth')}"`
    }
    if (Object.keys(invalid).length > 0) {
      return makeError<FlowInfo>('validation', 'the OAuth 2.0 configuration is incomplete', invalid)
    }
    // Go validates the port only once the configuration itself passed.
    const port = grant === 'authorization_code' ? redirectPortError(str('redirectPort')) : ''
    if (port) {
      return makeError<FlowInfo>('validation', 'the redirect port is invalid', { redirectPort: port })
    }

    const info: FlowInfo = {
      ...emptyFlowInfo(),
      flowId: req.flowId,
      expiresAt: new Date(Date.now() + 300_000).toISOString(),
    }
    if (grant === 'authorization_code') {
      info.authorizeUrl = `https://idp.mock/authorize?client_id=${encodeURIComponent(str('clientId'))}`
        + `&response_type=code&code_challenge_method=S256&state=${req.flowId}`
    } else {
      info.userCode = 'MOCK-CODE'
      info.verificationUri = 'https://idp.mock/device'
      info.verificationUriComplete = 'https://idp.mock/device?user_code=MOCK-CODE'
      info.intervalSec = 5
    }

    const ownerKey = this.key(req)
    const flow: MockFlow = { status: { state: 'pending', error: '', info }, timer: null }
    this.flows.set(req.flowId, flow)
    setTimeout(() => this.emitFlow(req.flowId, 'pending', ''), 0)
    flow.timer = setTimeout(() => {
      flow.timer = null
      this.tokens.set(ownerKey, Date.now() + TOKEN_TTL_MS)
      flow.status = { ...flow.status, state: 'done', error: '' }
      this.emitFlow(req.flowId, 'done', '')
    }, this.flowDelayMs())

    return { data: info }
  }

  // Same walk as the Go resolver: the request's own auth wins, otherwise the
  // first ancestor collection that configures one; none keeps walking up.
  async resolveOwner(requestId: string): Promise<Result<ResolvedOwner>> {
    const { getRequestService, getCollectionService } = await import('./index')
    const requests = await getRequestService()
    const reqResult = await requests.getById(requestId)
    if (reqResult.error) return makeError<ResolvedOwner>(reqResult.error.code, reqResult.error.message)

    const request = reqResult.data
    if (request.authType !== 'inherit') {
      return {
        data: {
          ownerKind: 'request',
          ownerId: request.id,
          authType: request.authType,
          authData: request.authData,
        },
      }
    }

    const collections = await getCollectionService()
    let id: string | null = request.collectionId
    const seen = new Set<string>()
    while (id && !seen.has(id)) {
      seen.add(id)
      const res: Result<Collection> = await collections.getById(id)
      if (res.error) break
      const collection: Collection = res.data
      if (collection.authType !== 'inherit' && collection.authType !== 'none') {
        return {
          data: {
            ownerKind: 'collection',
            ownerId: collection.id,
            authType: collection.authType,
            authData: collection.authData,
          },
        }
      }
      id = collection.parentId
    }

    return { data: { ownerKind: '', ownerId: '', authType: 'none', authData: '' } }
  }
}
