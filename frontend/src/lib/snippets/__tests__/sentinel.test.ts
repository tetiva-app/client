import { describe, expect, it } from 'vitest'
import type { HarRequest } from '@/types/snippet'
import { restoreUnsafeNames, withSentinels } from '../sentinel'

function har(fields: Partial<HarRequest>): HarRequest {
  return {
    method: 'POST',
    url: 'https://api.example.com/a',
    httpVersion: 'HTTP/1.1',
    headers: [],
    queryString: [],
    cookies: [],
    headersSize: -1,
    bodySize: -1,
    ...fields,
  }
}

function strings(h: HarRequest): string[] {
  return [
    h.url,
    ...h.headers.flatMap((x) => [x.name, x.value]),
    ...h.queryString.flatMap((x) => [x.name, x.value]),
    h.postData?.text ?? '',
    ...(h.postData?.params ?? []).flatMap((p) => [p.name, p.value]),
  ]
}

describe('withSentinels', () => {
  it('replaces safe names in every field and restores them verbatim', () => {
    const input = har({
      url: 'https://api.example.com/users/{{id}}/v{{ver}}',
      headers: [{ name: 'X-{{hdr}}', value: 'Bearer {{token}}' }],
      queryString: [{ name: '{{qn}}', value: '{{qv}}' }],
      postData: {
        mimeType: 'application/x-www-form-urlencoded',
        text: '{"n":"{{name}}"}',
        params: [{ name: '{{pn}}', value: 'a{{pv}}b' }],
      },
    })
    const s = withSentinels(input)

    expect(JSON.stringify(s.har)).not.toContain('{{')
    expect(s.har.url).toBe('https://api.example.com/users/zqv0vqz/vzqv1vqz')
    expect(strings(s.har).map((x) => s.restore(x))).toEqual(strings(input))
    expect(s.warnings).toEqual([])
  })

  it('gives a repeated name one sentinel', () => {
    const s = withSentinels(har({
      url: 'https://api.example.com/{{id}}',
      headers: [{ name: 'X-Id', value: '{{id}}' }],
    }))
    expect(s.har.url).toBe('https://api.example.com/zqv0vqz')
    expect(s.har.headers[0].value).toBe('zqv0vqz')
  })

  it('shows unsafe names as VAR_<i> with a warning', () => {
    const s = withSentinels(har({
      url: "https://api.example.com/{{x'; printf injected; #}}",
      headers: [
        { name: 'Authorization', value: 'Bearer {{token}}' },
        { name: 'X-Home', value: '{{$HOME}}' },
      ],
    }))

    expect(s.restore(s.har.url)).toBe('https://api.example.com/VAR_0')
    expect(s.restore(s.har.headers[0].value)).toBe('Bearer {{token}}')
    expect(s.restore(s.har.headers[1].value)).toBe('VAR_1')
    expect(s.warnings).toEqual([
      `Variable "x'; printf injected; #" contains unsafe characters and is shown as VAR_0`,
      'Variable "$HOME" contains unsafe characters and is shown as VAR_1',
    ])
  })

  it('does not trim names with spaces', () => {
    const s = withSentinels(har({
      headers: [
        { name: 'A', value: '{{ token }}' },
        { name: 'B', value: '{{token}}' },
      ],
    }))

    expect(s.har.headers[0].value).not.toBe(s.har.headers[1].value)
    expect(s.restore(s.har.headers[0].value)).toBe('VAR_0')
    expect(s.restore(s.har.headers[1].value)).toBe('{{token}}')
    expect(s.warnings).toEqual(['Variable " token " contains unsafe characters and is shown as VAR_0'])
  })

  it('uses a numeric sentinel for a port in the authority', () => {
    const s = withSentinels(har({ url: 'https://{{host}}:{{port}}/a/{{port}}' }))

    expect(s.har.url).toMatch(/^https:\/\/zqv\d+vqz:59990\/a\/zqv\d+vqz$/)
    expect(new URL(s.har.url).port).toBe('59990')
    expect(s.restore(s.har.url)).toBe('https://{{host}}:{{port}}/a/{{port}}')
    expect(s.restore('https://zqv0vqz:59990/a/zqv1vqz?k=v')).toBe('https://{{host}}:{{port}}/a/{{port}}?k=v')
  })

  it('skips port numbers that already occur in the request', () => {
    const s = withSentinels(har({
      url: 'https://api.example.com:{{port}}/a',
      headers: [{ name: 'X-N', value: '59990' }],
    }))

    expect(s.har.url).toBe('https://api.example.com:59991/a')
    expect(s.restore(`${s.har.url} 59990`)).toBe('https://api.example.com:{{port}}/a 59990')
  })

  it('restores a port only right after a URL host', () => {
    const s = withSentinels(har({ url: 'https://api.example.com:{{port}}/a' }))
    expect(s.har.url).toBe('https://api.example.com:59990/a')

    const code = `url = "${s.har.url}"; n = 59990; t = "10:59990"; p = "https://api.example.com/a:59990"; x = 599901`
    expect(s.restore(code)).toBe(
      'url = "https://api.example.com:{{port}}/a"; n = 59990; t = "10:59990"; p = "https://api.example.com/a:59990"; x = 599901')
    expect(s.restore("'https://user:pw@api.example.com:59990'")).toBe("'https://user:pw@api.example.com:{{port}}'")
    expect(s.restore('"https://api.example.com:599901/a"')).toBe('"https://api.example.com:599901/a"')
  })

  it('turns a leading base variable into a parseable host', () => {
    const s = withSentinels(har({ url: '{{baseUrl}}/users/{{id}}' }))

    expect(s.har.url).toBe('https://zqv0vqz.invalid/users/zqv1vqz')
    expect(s.restore('https://zqv0vqz.invalid/users/zqv1vqz?x=1')).toBe('{{baseUrl}}/users/{{id}}?x=1')
    expect(s.restore('Host: zqv0vqz.invalid')).toBe('Host: {{baseUrl}}')
  })

  it('restores a lone base variable without the slash url.parse adds', () => {
    const lone = withSentinels(har({ url: '{{baseUrl}}' }))
    expect(lone.restore(`"${lone.har.url}"`)).toBe('"{{baseUrl}}"')
    expect(lone.restore('"https://zqv0vqz.invalid/"')).toBe('"{{baseUrl}}"')

    const glued = withSentinels(har({ url: '{{baseUrl}}users' }))
    expect(glued.restore(`'${glued.har.url}'`)).toBe(`'{{baseUrl}}users'`)
  })

  it('keeps a port after a leading base variable', () => {
    const s = withSentinels(har({ url: '{{host}}:{{port}}/a' }))

    expect(s.har.url).toBe('https://zqv0vqz.invalid:59990/a')
    expect(s.restore(s.har.url)).toBe('{{host}}:{{port}}/a')
  })

  it('picks a token prefix that occurs nowhere in the input', () => {
    const s = withSentinels(har({
      url: 'https://api.example.com/{{id}}',
      headers: [{ name: 'X-Literal', value: 'ZQV2VQZ' }],
      postData: { mimeType: 'text/plain', text: 'zqv0vqz zqvz1vqz {{token}}', params: [] },
    }))

    expect(s.har.url).toBe('https://api.example.com/zqvzz0vqz')
    expect(s.restore(s.har.postData!.text)).toBe('zqv0vqz zqvz1vqz {{token}}')
    expect(s.restore(s.har.headers[0].value)).toBe('ZQV2VQZ')
    expect(s.restore('zqv0vqz zqv2vqz')).toBe('zqv0vqz zqv2vqz')
  })

  it('grows the prefix past the longest z run after any zqv', () => {
    const s = withSentinels(har({
      url: 'https://api.example.com/{{id}}',
      postData: { mimeType: 'text/plain', text: 'zqvzqvzzzz pizza zzqvz', params: [] },
    }))
    expect(s.har.url).toBe('https://api.example.com/zqvzzzzz0vqz')
  })

  it('picks the prefix in linear time for a body that keeps extending it', () => {
    const run = 1024 * 1024
    const input = har({
      url: 'https://api.example.com/{{id}}',
      postData: { mimeType: 'text/plain', text: `zqv${'z'.repeat(run)}`, params: [] },
    })
    const start = performance.now()
    const s = withSentinels(input)
    const elapsed = performance.now() - start

    expect(s.har.url).toBe(`https://api.example.com/zqv${'z'.repeat(run + 1)}0vqz`)
    expect(elapsed).toBeLessThan(200)
    expect(s.restore(s.har.url)).toBe('https://api.example.com/{{id}}')
  }, 600_000)

  it('leaves the input untouched', () => {
    const input = har({ url: '{{baseUrl}}/a', headers: [{ name: 'A', value: '{{b}}' }] })
    const before = structuredClone(input)
    withSentinels(input)
    expect(input).toEqual(before)
  })

  it('does not touch file names or extensions', () => {
    const s = withSentinels(har({
      postData: {
        mimeType: 'multipart/form-data',
        text: '',
        params: [{ name: 'file', value: '', fileName: '{{f}}.pdf' }],
      },
      _tetiva: { binaryFile: '{{b}}.bin' },
    }))

    expect(s.har.postData?.params[0].fileName).toBe('{{f}}.pdf')
    expect(s.har._tetiva?.binaryFile).toBe('{{b}}.bin')
  })
})

describe('restoreUnsafeNames', () => {
  it('keeps safe names and shows the rest as VAR_<i> with a warning', () => {
    expect(restoreUnsafeNames("{{token}} {{$HOME}} {{x'; printf injected; #}} {{$HOME}} {{ token }}")).toEqual({
      text: '{{token}} VAR_0 VAR_1 VAR_0 VAR_2',
      warnings: [
        'Variable "$HOME" contains unsafe characters and is shown as VAR_0',
        `Variable "x'; printf injected; #" contains unsafe characters and is shown as VAR_1`,
        'Variable " token " contains unsafe characters and is shown as VAR_2',
      ],
    })
  })

  it('carries the numbering across texts and warns about a name once', () => {
    const seen = new Map<string, number>()
    expect(restoreUnsafeNames('a {{$HOME}}', seen).warnings).toHaveLength(1)
    expect(restoreUnsafeNames('{{a b}} {{$HOME}}', seen)).toEqual({
      text: 'VAR_1 VAR_0',
      warnings: ['Variable "a b" contains unsafe characters and is shown as VAR_1'],
    })
  })
})
