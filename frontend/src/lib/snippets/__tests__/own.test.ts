import { afterAll, describe, expect, it } from 'vitest'
import type { HarParam, HarRequest, SnippetInput } from '@/types/snippet'
import { generate } from '../dist/snippets.mjs'
import { CURL_REFUSED, FIXTURES, goldenFixtures, REFUSED } from '../__fixtures__/fixtures'
import { exec, expectedSent, HAS, HAS_BASH, OWN, type Own, ownRunner, type Sent } from './own-run'

const SYNTAX: Record<Own, [string, string[]]> = {
  curl: ['bash', ['-n']],
  'python-requests': ['python3', ['-c', 'import ast,sys; ast.parse(sys.stdin.read())']],
  'js-fetch': [process.execPath, ['--input-type=module', '--check']],
}

const runner = ownRunner()
afterAll(() => runner.dispose())

function code(input: SnippetInput, key: Own): string {
  const r = generate(input, key)
  expect(r.code, key).not.toBe('')
  return r.code
}

const run = (input: SnippetInput, key: Own): Sent => runner.run(code(input, key), key)
const expected = (har: HarRequest): Sent => expectedSent(har, true)

function withHar(name: keyof typeof FIXTURES, change: (har: HarRequest) => void): SnippetInput {
  const input = structuredClone(FIXTURES[name])
  change(input.har!)
  return input
}

const SENT_AS_IS = ['special_chars', 'special_json', 'get_query', 'form_urlencoded'] as const
const HTTP_GOLDEN = Object.entries(goldenFixtures()).filter(([name, i]) => i.har && !REFUSED.has(name))

describe.each(OWN)('%s', (key) => {
  it.skipIf(!HAS[key]).each(SENT_AS_IS)('sends %s exactly as in the HAR', (name) => {
    const input = FIXTURES[name]
    expect(run(input, key)).toEqual(expected(input.har!))
  })

  it.skipIf(!HAS[key]).each(HTTP_GOLDEN.filter(([name]) => key !== 'curl' || !CURL_REFUSED.has(name)))('prints valid syntax for %s', (_, input) => {
    const [cmd, args] = SYNTAX[key]
    exec(cmd, args, code(input, key))
  })
})

describe('curl', () => {
  it('sends a body starting with @ as data, not as a file to read', () => {
    const snippet = code(withHar('special_chars', (h) => { h.postData!.text = '@marker.txt' }), 'curl')
    expect(snippet).toContain("--data-raw '@marker.txt'")
    expect(snippet).not.toContain('-d ')
  })

  it('attaches files with -F, text fields with --form-string and lets curl set the boundary', () => {
    const input = withHar('multipart_file', (h) => {
      h.headers.push({ name: 'Content-Type', value: 'multipart/form-data; boundary=x' })
    })
    const snippet = code(input, 'curl')
    expect(snippet).not.toContain("-H 'Content-Type")
    expect(snippet).toContain(`--form-string 'title=Отчёт "Q3"'`)
    expect(snippet).toContain(`-F 'file=@"report \\"q3\\".pdf";filename="report \\"q3\\".pdf"'`)
    expect(snippet).toContain("--form-string 'поле=x'")
  })

  const multipart = (params: HarParam[]) => withHar('multipart_file', (h) => { h.postData!.params = params })
  const refusal = (name: string) => ({ code: '', warnings: [`Field name ${JSON.stringify(name)} is not supported by curl -F`] })

  it('does not let a field name pick the file curl reads', () => {
    const input = multipart([{ name: 'x=@/dev/null;filename=', value: '', fileName: 'does-not-exist.txt' }])
    expect(generate(input, 'curl')).toEqual(refusal('x=@/dev/null;filename='))
  })

  it('refuses field names the curl -F grammar cannot carry', () => {
    const names = ['a=b', 'a;b', 'a"b', 'a\nb', 'a\rb', 'a\tb', 'a\u0000b', 'a\u001fb', 'a\u007fb', 'a\u0085b', '@a', '<a', '']
    for (const name of names) {
      expect(generate(multipart([{ name, value: 'v' }]), 'curl'), JSON.stringify(name)).toEqual(refusal(name))
      expect(generate(multipart([{ name, value: '', fileName: 'a.txt' }]), 'curl'), JSON.stringify(name)).toEqual(refusal(name))
    }
  })

  it('checks field names after the placeholder swap', () => {
    const input = multipart([{ name: 'x-{{name}}', value: 'v' }, { name: '{{a=b}}', value: 'w' }])
    const r = generate(input, 'curl')
    expect(r.code).toContain("--form-string 'x-{{name}}=v'")
    expect(r.code).toContain("--form-string 'VAR_0=w'")
    expect(generate(multipart([{ name: 'a={{b}}', value: 'v' }]), 'curl')).toEqual(refusal('a={{b}}'))
  })

  it.skipIf(!HAS_BASH)('sends other field names as they are and the file name exactly as in the HAR', () => {
    const input = multipart([
      { name: `a b'c\\d a@b <c Привет $(id)`, value: 'v' },
      { name: 'quoted', value: '', fileName: 'a";filename="x.txt' },
      { name: 'params', value: '', fileName: 'x;type=text/html;filename=y\\' },
      { name: 'spaces', value: '', fileName: ' lead and trail ' },
    ])
    expect(run(input, 'curl')).toEqual(expected(input.har!))
  })

  const withURL = (url: string, method = 'GET') => withHar('get_query', (h) => {
    h.url = url
    h.method = method
    h.queryString = []
  })

  it('turns URL globbing off when the URL has brackets or braces', () => {
    for (const path of ['/a[0]', '/a]', '/{id}', '/a}']) expect(code(withURL(`https://api.example.com${path}`), 'curl'), path).toMatch(/^curl --globoff '/)
    expect(code(withURL('https://[::1]:8080/a'), 'curl')).toMatch(/^curl --globoff '/)
    expect(code(withURL('https://api.example.com/{id}', 'POST'), 'curl')).toMatch(/^curl --globoff -X POST '/)
    expect(code(withURL('https://api.example.com/a'), 'curl')).toMatch(/^curl '/)
  })

  it('leaves globbing on for a query it percent-encodes and for placeholders', () => {
    expect(code(withHar('get_query', (h) => { h.queryString = [{ name: 'ids[]', value: '{1}' }] }), 'curl')).not.toContain('--globoff')
    expect(code(withURL('{{baseUrl}}/users/{{id}}'), 'curl')).not.toContain('--globoff')
  })

  it.skipIf(!HAS_BASH)('sends a URL with brackets and braces as it is', () => {
    const input = withURL('https://api.example.com/items[1-2]/{a,b}')
    expect(run(input, 'curl')).toEqual(expected(input.har!))
  })

  it('reads a binary body from the named file', () => {
    expect(code(FIXTURES.binary, 'curl')).toContain("--data-binary '@blob.bin'")
  })

  it('names the method only when curl would not pick it itself', () => {
    expect(code(FIXTURES.get_query, 'curl')).not.toContain('-X')
    expect(code(FIXTURES.post_json, 'curl')).toContain('curl -X POST ')
    if (HAS_BASH) expect(run(withHar('post_json', (h) => { h.method = 'GET' }), 'curl').method).toBe('GET')
    expect(code(withHar('get_query', (h) => { h.method = 'HEAD' }), 'curl')).toMatch(/^curl -I '/)
    expect(code(withHar('get_query', (h) => { h.method = "PURGE'; id; '" }), 'curl')).toContain(`-X 'PURGE'\\''; id; '\\'''`)
  })

  it('keeps a header with an empty value', () => {
    expect(code(withHar('get_query', (h) => { h.headers = [{ name: 'X-Empty', value: '' }] }), 'curl')).toContain("-H 'X-Empty;'")
  })
})

describe('python', () => {
  it('opens the binary body file', () => {
    expect(code(FIXTURES.binary, 'python-requests')).toContain('data = open("blob.bin", "rb")')
  })

  it('uploads files with their name and type and leaves the multipart Content-Type to requests', () => {
    const input = withHar('multipart_file', (h) => {
      h.headers.push({ name: 'Content-Type', value: 'multipart/form-data; boundary=x' })
    })
    const snippet = code(input, 'python-requests')
    expect(snippet).not.toContain('"Content-Type"')
    expect(snippet).toContain('("title", "Отчёт \\"Q3\\"")')
    expect(snippet).toContain('("file", ("report \\"q3\\".pdf", open("report \\"q3\\".pdf", "rb"), "application/pdf"))')
  })

  it('still sends multipart when there are no file parts', () => {
    const input = withHar('multipart_file', (h) => { h.postData!.params = h.postData!.params.filter((p) => !p.fileName) })
    const snippet = code(input, 'python-requests')
    expect(snippet).toContain('("title", (None, "Отчёт \\"Q3\\""))')
    expect(snippet).toContain('files=files')
  })

  it('escapes control characters', () => {
    const snippet = code(withHar('special_chars', (h) => { h.postData!.text = 'a\u0000b\u001bc\u2028d\re\tf' }), 'python-requests')
    expect(snippet).toContain('data = "a\\u0000b\\u001bc\\u2028d\\re\\tf"')
  })
})

describe('fetch', () => {
  it('appends each form field and leaves the multipart Content-Type to fetch', () => {
    const input = withHar('multipart_file', (h) => {
      h.headers.push({ name: 'Content-Type', value: 'multipart/form-data; boundary=x' })
    })
    const snippet = code(input, 'js-fetch')
    expect(snippet).not.toContain('"Content-Type"')
    expect(snippet).toContain('const body = new FormData();')
    expect(snippet).toContain('body.append("title", "Отчёт \\"Q3\\"");')
    expect(snippet).toContain('body.append("поле", "x");')
  })

  const REQUEST_STUB = `globalThis.fetch = async (url, options) => {
  const request = new Request(url, options)
  console.log(JSON.stringify({ method: request.method, body: await request.text() }))
  return { text: async () => '' }
};
`
  const sendWithRequest = (snippet: string) => JSON.parse(exec(process.execPath, ['--input-type=module'], REQUEST_STUB + snippet))

  it.each(['GET', 'HEAD'])('leaves out a %s body, which a real Request refuses', (method) => {
    for (const name of ['post_json', 'form_urlencoded', 'multipart_file', 'binary'] as const) {
      const r = generate(withHar(name, (h) => { h.method = method }), 'js-fetch')
      const note = `JavaScript cannot send a body with ${method}; the body is left out`
      expect(r.warnings, name).toEqual([note])
      expect(r.code.split('\n')[0], name).toBe(`// ${note}`)
      expect(r.code.split('\n').slice(1).join('\n'), name).not.toMatch(/attach file|contents of file|body/)
      expect(sendWithRequest(r.code), name).toEqual({ method, body: '' })
    }
  })

  it('keeps the body of other methods and of a bodiless GET', () => {
    expect(sendWithRequest(code(FIXTURES.post_json, 'js-fetch')).body).toBe(FIXTURES.post_json.har!.postData!.text)
    const get = generate(FIXTURES.get_query, 'js-fetch')
    expect(get.warnings).toEqual([])
    expect(sendWithRequest(get.code)).toEqual({ method: 'GET', body: '' })
  })
})
