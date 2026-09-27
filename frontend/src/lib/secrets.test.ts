import { describe, it, expect } from 'vitest'
import { isSensitiveHeader, isSensitiveQueryParam, redactHeaders, redactURL, redactValue } from './secrets'
import type { HeaderItem } from '@/types/request'

// The cases mirror internal/domain/secrets/*_test.go; a change there belongs here too.
describe('secrets (browser-mode port of the Go package)', () => {
  it('isSensitiveHeader', () => {
    const sensitive = [
      'Authorization', 'proxy-authorization', 'Cookie', 'Set-Cookie', 'X-API-Key', 'Api-Key',
      'X-Auth-Token', 'X-Access-Token', 'X-CSRF-Token', 'X-Session-Token', 'X-Amz-Security-Token',
      'X-Amz-Content-Sha256', 'WWW-Authenticate',
      'apikey', 'Private-Token', 'X-Vault-Token', 'X-Goog-Api-Key', 'Ocp-Apim-Subscription-Key',
      'X-RapidAPI-Key', 'X-Api-Token', 'X-Shopify-Access-Token',
      'X-My-Secret', 'X-Client-Key', 'X-Session-Id', 'X-Signature', 'X-Password', 'X-Passwd',
      'X-Credential', 'X-Custom-Cookie', 'X-Private-Thing', 'X-Auth-User', ' authorization ',
      '{{headerName}}', 'X-{{env}}-Id', ' {{h}} ',
      'Idempotency-Key', 'X_Api_Key', 'x.api.key', 'XApiKey', 'APIKey', 'X-Tokens', 'X-Credentials',
      'X-Original-Authorization', 'X-Apikey', 'X-Accesstoken', 'X-Refreshtoken', 'X-Clientsecret',
      'X-Jsessionid', 'X-XSRF', 'X-Csrftoken', 'sessionID', 'X-Authtoken', 'X-Privatekey',
      'X-{{env}}-Access-Control',
      'X-Apitoken', 'X-Accesskey', 'X-Authkey', 'X-Privkey', 'X-Sessiontoken', 'X-Idtoken', 'X-Secrettoken',
      'X-Userpassword', 'X-Clienttoken', 'X-Securitytoken', 'X-Hubsignature', 'X-Sessioncookie',
      'X-Api-Key2', 'Set-Cookie2', 'Proxy-Authenticate', 'X-Bearertoken',
    ]
    const plain = [
      '', 'Accept', 'X-Request-Id', 'Content-Type', 'X-Amz-Date', 'User-Agent', 'X-Trace',
      'Keyword', 'X-Author', 'X-Authored-By', 'X-Monkey', 'X-Keyboard-Layout', 'X-Tokenizer',
      'Access-Control-Allow-Credentials', 'Access-Control-Allow-Headers', 'access-control-expose-headers',
      'X-Passage', 'X-Sessions-Total', 'X-Api-Version2',
    ]
    expect(sensitive.filter(n => !isSensitiveHeader(n))).toEqual([])
    expect(plain.filter(n => isSensitiveHeader(n))).toEqual([])
  })

  it('isSensitiveQueryParam', () => {
    const sensitive = [
      'token', 'access_token', 'refresh_token', 'api_key', 'apikey', 'key', 'secret', 'password',
      'sig', 'signature', 'code', 'code_verifier', 'client_secret', 'assertion', 'id_token',
      'x-amz-signature', 'x-amz-credential', 'x-amz-security-token',
      'ACCESS_TOKEN', 'access%5Ftoken', '%70assword', ' Token ',
      'my_session', 'private_data', 'user_passwd', 'X-Amz-Credential',
      'apiKey', 'accessToken', 'refreshToken', 'clientSecret', 'jsessionid', '_csrf', 'csrfmiddlewaretoken',
      'xsrf_token', 'oauth_token', 'idToken', 'session.id',
      'apitoken', 'accesskey', 'authkey', 'sessiontoken', 'idtoken', 'newpassword', 'oauth_verifier',
      'token2', 'key1', 'pagetoken',
    ]
    const plain = [
      '', 'page', 'sort', 'state', 'flag', 'X-Amz-Date', '100%off', 'q', 'limit',
      'keyword', 'keywords', 'author', 'author_id', 'authorName', 'oauth_state', 'monkey', 'turkey', 'hockey',
      'passage', 'tokenizer', 'page2', 'v1', 'donkey2',
    ]
    expect(sensitive.filter(n => !isSensitiveQueryParam(n))).toEqual([])
    expect(plain.filter(n => isSensitiveQueryParam(n))).toEqual([])
  })

  it.each([
    ['', ''],
    ['Bearer abc', 'Bearer <redacted>'],
    ['Bearer {{t}}', 'Bearer {{t}}'],
    ['bearer abc', 'bearer <redacted>'],
    ['Basic dXNlcjpwYXNz', 'Basic <redacted>'],
    ['Token abc', 'Token <redacted>'],
    ['Digest username="u", nonce="n"', 'Digest <redacted>'],
    ['Bearer   abc', 'Bearer   <redacted>'],
    ['Bearer ', 'Bearer '],
    ['Bearer', 'Bearer'],
    ['Bearer{{t}}', 'Bearer{{t}}'],
    ['Bearer abc{{t}}', 'Bearer <redacted>{{t}}'],
    ['{{a}}-xyz-{{b}}', '{{a}}<redacted>{{b}}'],
    ['{{token}}', '{{token}}'],
    [' {{session}} ', ' {{session}} '],
    ['{{a}} {{b}} x', '{{a}} {{b}}<redacted>'],
    ['abc', '<redacted>'],
    ['session=abc; theme={{t}}', '<redacted>{{t}}'],
    ['Bearer {{a}}.{{b}}', 'Bearer {{a}}<redacted>{{b}}'],
  ])('redactValue(%j)', (input, want) => {
    expect(redactValue(input)).toBe(want)
  })

  it.each([
    ['https://api.example.com/u?page=2', 'https://api.example.com/u?page=2'],
    ['/relative/path', '/relative/path'],
    ['', ''],
    [
      'https://bucket.s3.amazonaws.com/r.csv?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKIAIOSFODNN7EXAMPLE%2F20260925&X-Amz-Signature=deadbeef',
      'https://bucket.s3.amazonaws.com/r.csv?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=<redacted>&X-Amz-Signature=<redacted>',
    ],
    ['https://app.example.com/cb?code=SPLxlOBeZQQ&state=xyz', 'https://app.example.com/cb?code=<redacted>&state=xyz'],
    [
      'https://app.example.com/cb#access_token=eyJhbGciOi.payload.sig&token_type=bearer&state=xyz',
      'https://app.example.com/cb#access_token=<redacted>&token_type=<redacted>&state=xyz',
    ],
    ['https://app.example.com/docs#section-2', 'https://app.example.com/docs#section-2'],
    ['https://user:hunter2@git.example.com/repo.git', 'https://<redacted>@git.example.com/repo.git'],
    ['https://ghp_abc@github.com/o/r', 'https://<redacted>@github.com/o/r'],
    ['https://{{user}}:{{pass}}@host/p', 'https://{{user}}:{{pass}}@host/p'],
    ['https://host/p?next=a@b', 'https://host/p?next=a@b'],
    ['/cb?code=abc#id_token=x', '/cb?code=<redacted>#id_token=<redacted>'],
    ['https://host/p?token={{t}}#access_token={{t}}', 'https://host/p?token={{t}}#access_token={{t}}'],
    ['https://api.example.com/u?key=', 'https://api.example.com/u?key='],
    ['https://api.example.com/u?access%5Ftoken=a', 'https://api.example.com/u?access%5Ftoken=<redacted>'],
  ])('redactURL(%j)', (input, want) => {
    expect(redactURL(input)).toBe(want)
  })

  it('redactHeaders masks credentials and keeps references, without touching its input', () => {
    const input: HeaderItem[] = [
      { key: 'Authorization', value: 'Bearer abc', enabled: true },
      { key: 'X-Token', value: 'Bearer {{token}}', enabled: true },
      { key: '{{h}}', value: 'literal-secret', enabled: true },
      { key: '{{h}}', value: '{{v}}', enabled: true },
      { key: 'Set-Cookie', value: 'id=1; Path=/', enabled: false },
      { key: 'Content-Type', value: 'application/json', enabled: true },
      { key: 'X-Request-Id', value: '42', enabled: false },
    ]
    const before = input.map(h => ({ ...h }))

    const out = redactHeaders(input)

    expect(out).toEqual([
      { key: 'Authorization', value: 'Bearer <redacted>', enabled: true },
      { key: 'X-Token', value: 'Bearer {{token}}', enabled: true },
      { key: '{{h}}', value: '<redacted>', enabled: true },
      { key: '{{h}}', value: '{{v}}', enabled: true },
      { key: 'Set-Cookie', value: '<redacted>', enabled: false },
      { key: 'Content-Type', value: 'application/json', enabled: true },
      { key: 'X-Request-Id', value: '42', enabled: false },
    ])
    expect(input).toEqual(before)
  })

  it('redactHeaders masks URLs carried by redirect and link headers', () => {
    const input: HeaderItem[] = [
      { key: 'Location', value: 'https://bucket.s3.example/r.csv?X-Amz-Credential=AKIA&X-Amz-Signature=sig', enabled: true },
      { key: 'location', value: 'https://app.example.com/cb#access_token=eyJ.a.b', enabled: true },
      { key: 'Content-Location', value: 'https://u:p@api.example.com/r?page=1', enabled: true },
      { key: 'Link', value: '<https://api.example.com/r?page=2&access_token=abc>; rel="next", <https://api.example.com/r?page=9>; rel="last"', enabled: true },
      { key: 'Refresh', value: '5; url=https://app.example.com/cb?code=abc', enabled: true },
      { key: 'Refresh', value: "0;URL='https://app.example.com/?sig=abc'", enabled: true },
      { key: 'Location', value: '/users/42', enabled: true },
      { key: 'X-Next', value: 'https://api.example.com/r?token=abc', enabled: true },
    ]

    expect(redactHeaders(input).map(h => h.value)).toEqual([
      'https://bucket.s3.example/r.csv?X-Amz-Credential=<redacted>&X-Amz-Signature=<redacted>',
      'https://app.example.com/cb#access_token=<redacted>',
      'https://<redacted>@api.example.com/r?page=1',
      '<https://api.example.com/r?page=2&access_token=<redacted>>; rel="next", <https://api.example.com/r?page=9>; rel="last"',
      '5; url=https://app.example.com/cb?code=<redacted>',
      "0;URL='https://app.example.com/?sig=<redacted>'",
      '/users/42',
      'https://api.example.com/r?token=abc',
    ])
  })

  it('redactHeaders is idempotent', () => {
    const input: HeaderItem[] = [
      { key: 'Authorization', value: 'Bearer abc', enabled: true },
      { key: 'Location', value: 'https://u:p@h/cb?code=a#access_token=b', enabled: true },
      { key: 'Link', value: '<https://h/r?token=a>; rel="next"', enabled: true },
      { key: 'Refresh', value: '1; url=https://h/?sig=a', enabled: true },
    ]
    const once = redactHeaders(input)
    expect(redactHeaders(once)).toEqual(once)
  })
})
