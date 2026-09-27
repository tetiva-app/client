import type { HarParam, HarRequest } from '@/types/snippet'
import { Unsupported } from '../types'
import { buildURL, formEncode, harBody, sentHeaders, shellQuote } from './encode'

// curl reads these as glob ranges and sets; the sentinel swap has already hidden {{placeholders}}.
const GLOB_CHARS = /[[\]{}]/

// Names -F would split, rewrite or drop: curl ends a name at =, percent-encodes " and line breaks
// in it, and sends an empty one as no name at all.
const BAD_FIELD_NAME = /^$|^[@<]|[=;"\p{Cc}]/u

export function renderCurl(har: HarRequest): string {
  const lines = sentHeaders(har).map((h) =>
    // curl drops a header given as "Name:"; the semicolon form sends it with an empty value.
    `-H ${shellQuote(h.value ? `${h.name}: ${h.value}` : `${h.name};`)}`)

  const body = harBody(har)
  // --data-raw and --form-string: with -d or -F a value starting with @ makes curl read a local file.
  if (body.kind === 'text') lines.push(`--data-raw ${shellQuote(body.text)}`)
  else if (body.kind === 'form') lines.push(`--data-raw ${shellQuote(formEncode(body.params))}`)
  else if (body.kind === 'file') lines.push(`--data-binary ${shellQuote(`@${body.name}`)}`)
  else if (body.kind === 'multipart') lines.push(...body.params.map(formPart))

  const url = buildURL(har)
  const globoff = GLOB_CHARS.test(url) ? ' --globoff' : ''
  return [`curl${globoff}${methodFlag(har.method, body.kind !== 'none')} ${shellQuote(url)}`, ...lines].join(' \\\n  ')
}

// Without ;filename= curl sends only the last path segment.
function formPart(p: HarParam): string {
  if (BAD_FIELD_NAME.test(p.name)) throw new Unsupported(`Field name ${JSON.stringify(p.name)} is not supported by curl -F`)
  if (!p.fileName) return `--form-string ${shellQuote(`${p.name}=${p.value}`)}`
  const file = formWord(p.fileName)
  return `-F ${shellQuote(`${p.name}=@${file};filename=${file}`)}`
}

// The quoting of curl's form parser: inside double quotes only \\ and \" are escapes.
function formWord(s: string): string {
  return `"${s.replace(/["\\]/g, '\\$&')}"`
}

// A body makes curl default to POST; -X HEAD would wait for a body that never comes.
function methodFlag(method: string, hasBody: boolean): string {
  if (method === 'GET' && !hasBody) return ''
  if (method === 'HEAD' && !hasBody) return ' -I'
  return ` -X ${/^[A-Za-z0-9_-]+$/.test(method) ? method : shellQuote(method)}`
}
