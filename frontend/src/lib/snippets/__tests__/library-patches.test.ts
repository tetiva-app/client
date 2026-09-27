import { HTTPSnippet } from '@readme/httpsnippet'
import { describe, expect, it } from 'vitest'

// patches/@readme+httpsnippet+11.4.0.patch: each case pins one change against the patched package itself.

interface Req {
  method?: string
  url?: string
  query?: [string, string][]
  headers?: [string, string][]
  mimeType?: string
  text?: string
  params?: { name: string; value?: string; fileName?: string; contentType?: string }[]
}

type Target = 'go' | 'java' | 'csharp' | 'php'

function snippet(target: Target, client: string, req: Req): string {
  const har = {
    method: req.method ?? 'POST',
    url: req.url ?? 'https://api.example.com/x',
    httpVersion: 'HTTP/1.1',
    headers: (req.headers ?? []).map(([name, value]) => ({ name, value })),
    queryString: (req.query ?? []).map(([name, value]) => ({ name, value })),
    cookies: [],
    headersSize: -1,
    bodySize: -1,
    ...(req.mimeType ? { postData: { mimeType: req.mimeType, text: req.text ?? '', params: req.params ?? [] } } : {}),
  }
  const out = new HTTPSnippet(har as ConstructorParameters<typeof HTTPSnippet>[0]).convert(target, client)[0]
  if (typeof out !== 'string') throw new Error(`${target}/${client} printed nothing`)
  return out
}

const QUOTED_HEADER: Req = { headers: [["X\"'\\Z", 'v']] }

describe('header names are string literals', () => {
  it.each([
    ['go', 'native', 'req.Header.Add("X\\"\'\\\\Z", "v")'],
    ['java', 'nethttp', '.header("X\\"\'\\\\Z", "v")'],
    ['java', 'okhttp', '.addHeader("X\\"\'\\\\Z", "v")'],
    ['csharp', 'httpclient', '{ "X\\"\'\\\\Z", "v" },'],
    ['php', 'guzzle', "'X\"\\'\\\\Z' => 'v',"],
  ] as [Target, string, string][])('%s/%s', (target, client, line) => {
    expect(snippet(target, client, QUOTED_HEADER)).toContain(line)
  })
})

describe('methods are string literals', () => {
  it.each([
    ['go', 'native', 'http.NewRequest("X\\"Y\'Z", url, nil)'],
    ['java', 'nethttp', '.method("X\\"Y\'Z", HttpRequest.BodyPublishers.noBody())'],
    ['java', 'okhttp', '.method("X\\"Y\'Z", null)'],
    ['csharp', 'httpclient', 'Method = new HttpMethod("X\\"Y\'Z"),'],
    ['php', 'guzzle', "$client->request('X\"Y\\'Z', "],
  ] as [Target, string, string][])('%s/%s', (target, client, text) => {
    expect(snippet(target, client, { method: 'X"Y\'Z' })).toContain(text)
  })
})

describe('URLs are string literals', () => {
  // url.parse leaves a javascript: URL unescaped.
  const url = 'javascript:alert("x")//\'#"\\'
  it.each([
    ['go', 'native', String.raw`url := "javascript:alert(\"x\")//'#\"\\"`],
    ['java', 'nethttp', String.raw`.uri(URI.create("javascript:alert(\"x\")//'#\"\\"))`],
    ['java', 'okhttp', String.raw`.url("javascript:alert(\"x\")//'#\"\\")`],
    ['csharp', 'httpclient', String.raw`RequestUri = new Uri("javascript:alert(\"x\")//'#\"\\"),`],
    ['php', 'guzzle', String.raw`$client->request('GET', 'javascript:alert("x")//\'#"\\');`],
  ] as [Target, string, string][])('%s/%s', (target, client, line) => {
    expect(snippet(target, client, { method: 'GET', url })).toContain(line)
  })

  it('PHP escapes an apostrophe that url.format leaves in the userinfo', () => {
    expect(snippet('php', 'guzzle', { method: 'GET', url: "https://o'neil:p'w@api.example.com/" }))
      .toContain(String.raw`$client->request('GET', 'https://o\'neil:p\'w@api.example.com/');`)
  })
})

describe('names that are Object.prototype members', () => {
  const FIELDS: Req = {
    mimeType: 'application/x-www-form-urlencoded',
    params: [{ name: 'constructor', value: 'c' }, { name: '__proto__', value: 'p' }, { name: 'toString', value: 't' }],
  }

  it.each([
    ['go', 'native'],
    ['java', 'nethttp'],
    ['java', 'okhttp'],
  ] as [Target, string][])('%s/%s sends them as plain form fields', (target, client) => {
    expect(snippet(target, client, FIELDS)).toContain('"constructor=c&__proto__=p&toString=t"')
  })

  it('PHP prints them as plain form fields', () => {
    expect(snippet('php', 'guzzle', FIELDS)).toContain([
      "  'form_params' => [",
      "    'constructor' => 'c',",
      "    '__proto__' => 'p',",
      "    'toString' => 't'",
      '  ],',
    ].join('\n'))
  })

  it('keeps them in the query', () => {
    const query: [string, string][] = [['constructor', 'c'], ['__proto__', 'p'], ['toString', 't']]
    expect(snippet('go', 'native', { method: 'GET', query }))
      .toContain('url := "https://api.example.com/x?constructor=c&__proto__=p&toString=t"')
  })
})

it('okhttp quotes the media type', () => {
  expect(snippet('java', 'okhttp', { mimeType: 'text/plain; x="y"', text: 'a' }))
    .toContain('MediaType mediaType = MediaType.parse("text/plain; x=\\"y\\"");')
})

describe('C#', () => {
  it('sends urlencoded pairs as escaped literals and keeps repeated keys', () => {
    const code = snippet('csharp', 'httpclient', {
      mimeType: 'application/x-www-form-urlencoded',
      params: [
        { name: 'q"', value: '" + System.Environment.MachineName + "' },
        { name: 'path', value: 'C:\\a\\b' },
        { name: 'role', value: 'a' },
        { name: 'role', value: 'b' },
      ],
    })
    expect(code).toContain([
      '    Content = new FormUrlEncodedContent(new List<KeyValuePair<string, string>>',
      '    {',
      '        new("q\\"", "\\" + System.Environment.MachineName + \\""),',
      '        new("path", "C:\\\\a\\\\b"),',
      '        new("role", "a"),',
      '        new("role", "b"),',
      '    }),',
    ].join('\n'))
  })

  it('escapes multipart names, file names and part types', () => {
    const code = snippet('csharp', 'httpclient', {
      mimeType: 'multipart/form-data',
      params: [
        { name: 'a"b', value: 'v' },
        { name: 'f', fileName: 'x"y\\.pdf', contentType: 'text/plain; charset="utf-8"' },
      ],
    })
    expect(code).toContain('Name = "a\\"b",')
    expect(code).toContain('FileName = "x\\"y\\\\.pdf",')
    expect(code).toContain('ContentType = MediaTypeHeaderValue.Parse("text/plain; charset=\\"utf-8\\""),')
  })

  it('parses a content type with parameters', () => {
    expect(snippet('csharp', 'httpclient', { mimeType: 'text/plain; x="y"', text: 'a' }))
      .toContain('ContentType = MediaTypeHeaderValue.Parse("text/plain; x=\\"y\\"")')
  })

  it('escapes the characters C# treats as line ends', () => {
    const code = snippet('csharp', 'httpclient', {
      headers: [['X-A', 'a\u2028b']],
      mimeType: 'text/plain',
      text: 'c\u0085d\u2029e',
    })
    expect(code).toContain('{ "X-A", "a\\u2028b" },')
    expect(code).toContain('new StringContent("c\\u0085d\\u2029e")')
  })
})

describe('PHP', () => {
  it('keeps control characters in double-quoted strings and leaves the rest single-quoted', () => {
    const code = snippet('php', 'guzzle', {
      headers: [['X-Tab', 'a\tb']],
      mimeType: 'text/plain',
      text: 'tab\tnul\u0000esc\u001bdel\u007fcr\r$HOME {$x} "q" \\ end',
    })
    expect(code).toContain(`'body' => "tab\\tnul\\x00esc\\edel\\x7fcr\\r\\$HOME {\\$x} \\"q\\" \\\\ end",`)
    expect(code).toContain(`'X-Tab' => "a\\tb",`)
    expect(snippet('php', 'guzzle', { mimeType: 'text/plain', text: "it's\nfine $x" }))
      .toContain(`'body' => 'it\\'s\nfine $x',`)
  })

  it('keeps a multipart field with an empty value', () => {
    const code = snippet('php', 'guzzle', { mimeType: 'multipart/form-data', params: [{ name: 'a', value: 'b' }, { name: 'empty', value: '' }] })
    expect(code).toMatch(/'name' => 'empty',\n\s+'contents' => ''\n/)
  })

  it('separates multipart from the headers that follow it', () => {
    const code = snippet('php', 'guzzle', {
      headers: [['Authorization', 'Bearer t']],
      mimeType: 'multipart/form-data',
      params: [{ name: 'a', value: 'b' }],
    })
    expect(code).toMatch(/\n {2}\],\n {2}'headers' => \[/)
  })
})

describe('multipart field names in the generated body', () => {
  it.each([
    ['go', 'native'],
    ['java', 'nethttp'],
    ['java', 'okhttp'],
  ] as [Target, string][])('%s/%s keeps non-ASCII and percent-encodes only quotes and line breaks', (target, client) => {
    const code = snippet(target, client, {
      mimeType: 'multipart/form-data',
      params: [{ name: 'поле "a"\nb', value: 'v' }, { name: 'f', fileName: 'x"y.pdf', value: '' }],
    })
    expect(code).toContain('name=\\"поле %22a%22%0D%0Ab\\"')
    expect(code).toContain('filename=\\"x%22y.pdf\\"')
    expect(code).not.toContain('%u')
  })
})
