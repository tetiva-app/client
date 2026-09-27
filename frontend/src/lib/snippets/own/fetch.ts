import type { HarRequest } from '@/types/snippet'
import { buildURL, harBody, sentHeaders } from './encode'

const str = (s: string): string => JSON.stringify(s)

export function renderFetch(har: HarRequest): string {
  const lines = [`const url = ${str(buildURL(har))};`]
  const options = [`  method: ${str(har.method)},`]

  const headers = sentHeaders(har)
  if (headers.length) options.push('  headers: {', ...headers.map((h) => `    ${str(h.name)}: ${str(h.value)},`), '  },')

  const body = harBody(har)
  if (body.kind === 'text') options.push(`  body: ${str(body.text)},`)
  else if (body.kind === 'form' || body.kind === 'multipart') {
    lines.push(`const body = new ${body.kind === 'form' ? 'URLSearchParams' : 'FormData'}();`)
    for (const p of body.params) lines.push(`body.append(${str(p.name)}, ${str(p.value)});`)
    options.push('  body,')
  }

  return [
    ...lines,
    'const options = {',
    ...options,
    '};',
    '',
    'const response = await fetch(url, options);',
    'console.log(await response.text());',
  ].join('\n')
}
