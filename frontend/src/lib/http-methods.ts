// HTTP method colors following Swagger UI convention
export const methodColors: Record<string, string> = {
  GET: '#61AFFE',
  POST: '#49CC90',
  PUT: '#FCA130',
  PATCH: '#50E3C2',
  DELETE: '#F93E3E',
  OPTIONS: '#0D7CC1',
  HEAD: '#9012FE',
}

export const methods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS', 'HEAD'] as const

export type HttpMethod = (typeof methods)[number]

// Shortened labels for compact UI (sidebar badges)
export function methodLabel(method: string): string {
  if (method === 'DELETE') return 'DEL'
  if (method === 'OPTIONS') return 'OPT'
  return method
}

export const METHOD_COLOR_FALLBACK = '#8B949E'
