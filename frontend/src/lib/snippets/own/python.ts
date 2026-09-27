import type { HarRequest } from '@/types/snippet'
import { harBody, sentHeaders } from './encode'

const SHORT_ESCAPES: Record<string, string> = { '\\': '\\\\', '"': '\\"', '\n': '\\n', '\r': '\\r', '\t': '\\t' }

function pyString(s: string): string {
  const escaped = s.replace(/[\\"\u0000-\u001f\u007f-\u009f\u2028\u2029]/g, (c) =>
    SHORT_ESCAPES[c] ?? `\\u${c.charCodeAt(0).toString(16).padStart(4, '0')}`)
  return `"${escaped}"`
}

function pyPair(name: string, value: string): string {
  return `(${pyString(name)}, ${value})`
}

function pyBlock(open: string, items: string[], close: string): string {
  return [open, ...items.map((i) => `    ${i},`), close].join('\n')
}

export function renderPython(har: HarRequest): string {
  const vars = [`url = ${pyString(har.url)}`]
  const args = [pyString(har.method), 'url']
  const assign = (name: string, value: string) => {
    vars.push(`${name} = ${value}`)
    args.push(`${name}=${name}`)
  }

  if (har.queryString.length) assign('params', pyBlock('[', har.queryString.map((q) => pyPair(q.name, pyString(q.value))), ']'))
  const headers = sentHeaders(har)
  if (headers.length) assign('headers', pyBlock('{', headers.map((h) => `${pyString(h.name)}: ${pyString(h.value)}`), '}'))

  const body = harBody(har)
  if (body.kind === 'text') assign('data', pyString(body.text))
  else if (body.kind === 'file') assign('data', `open(${pyString(body.name)}, "rb")`)
  else if (body.kind === 'form') assign('data', pyBlock('[', body.params.map((p) => pyPair(p.name, pyString(p.value))), ']'))
  else if (body.kind === 'multipart') {
    const fields = body.params.filter((p) => !p.fileName)
    const files = body.params.filter((p) => p.fileName).map((p) => {
      const file = [pyString(p.fileName!), `open(${pyString(p.fileName!)}, "rb")`]
      if (p.contentType) file.push(pyString(p.contentType))
      return pyPair(p.name, `(${file.join(', ')})`)
    })
    // data alone would go out urlencoded; (None, value) parts keep a file-less form multipart.
    if (!files.length) assign('files', pyBlock('[', fields.map((p) => pyPair(p.name, `(None, ${pyString(p.value)})`)), ']'))
    else {
      if (fields.length) assign('data', pyBlock('[', fields.map((p) => pyPair(p.name, pyString(p.value))), ']'))
      assign('files', pyBlock('[', files, ']'))
    }
  }

  return [
    'import requests',
    '',
    ...vars,
    '',
    `response = requests.request(${args.join(', ')})`,
    'print(response.text)',
  ].join('\n')
}
