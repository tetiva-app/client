import type { AuthType } from '@/types/request'

// Mirrors entities.ValidAuthTypes(); constants/auth.test.ts holds both lists
// against the Go fixture so a new scheme cannot land on one side only.
export const AUTH_TYPES: AuthType[] = [
  'none',
  'basic',
  'bearer',
  'api_key',
  'inherit',
  'oauth2',
  'jwt',
  'digest',
  'aws_sigv4',
]

// A collection is the top of the inherit chain, so it never inherits itself.
export const COLLECTION_AUTH_TYPES: AuthType[] = AUTH_TYPES.filter(t => t !== 'inherit')

export const AUTH_TYPE_LABELS: Record<AuthType, string> = {
  inherit: 'Inherit',
  none: 'None',
  basic: 'Basic Auth',
  bearer: 'Bearer Token',
  api_key: 'API Key',
  oauth2: 'OAuth 2.0',
  jwt: 'JWT Bearer',
  digest: 'Digest',
  aws_sigv4: 'AWS Signature',
}

// Collections speak about themselves, not about a request.
const COLLECTION_LABEL_OVERRIDES: Partial<Record<AuthType, string>> = {
  none: 'No Auth',
}

export const AUTH_BADGE_LABELS: Partial<Record<AuthType, string>> = {
  basic: 'Basic',
  bearer: 'Bearer',
  api_key: 'API Key',
  oauth2: 'OAuth2',
  jwt: 'JWT',
  digest: 'Digest',
  aws_sigv4: 'AWS',
}

export interface AuthTypeOption {
  value: AuthType
  label: string
}

// Selector order: the new schemes are appended so the keyboard walk the auth
// e2e relies on (Inherit → None → Basic) keeps its positions.
const SELECTOR_ORDER: AuthType[] = [
  'inherit',
  'none',
  'basic',
  'bearer',
  'api_key',
  'oauth2',
  'jwt',
  'digest',
  'aws_sigv4',
]

export const REQUEST_AUTH_OPTIONS: AuthTypeOption[] = SELECTOR_ORDER.map(value => ({
  value,
  label: AUTH_TYPE_LABELS[value],
}))

export function collectionAuthOptions(): AuthTypeOption[] {
  return SELECTOR_ORDER
    .filter(value => COLLECTION_AUTH_TYPES.includes(value))
    .map(value => ({ value, label: COLLECTION_LABEL_OVERRIDES[value] ?? AUTH_TYPE_LABELS[value] }))
}

export function authBadgeLabel(type: string | undefined): string {
  if (!type) return ''
  return AUTH_BADGE_LABELS[type as AuthType] ?? ''
}
