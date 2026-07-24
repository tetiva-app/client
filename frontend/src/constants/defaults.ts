import type { Protocol, BodyType, AuthType, HTTPMethod } from '@/types/request'

export const DEFAULT_PROTOCOL: Protocol = 'http'
export const DEFAULT_METHOD: HTTPMethod = 'GET'
export const DEFAULT_BODY_TYPE: BodyType = 'none'
export const DEFAULT_AUTH_TYPE: AuthType = 'inherit'
export const DEFAULT_AUTH_DATA = '{}'

// Brand accent color (6C5CE7) as rgba for CodeMirror selections
export const BRAND_ACCENT_SELECTION = 'rgba(108, 92, 231, 0.3)'
