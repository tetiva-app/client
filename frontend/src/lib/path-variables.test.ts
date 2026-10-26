import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import type { PathVariable } from '@/types/request'
import { replaceVars } from '@/lib/snippets/vars'
import {
  applyPathVariables,
  isPathVariableName,
  paramsCount,
  pathVariableKeys,
  pathVariableSpans,
  renamePathVariable,
  resolveURL,
  syncPathVariables,
} from './path-variables'

interface Cases {
  keys: { url: string; keys: string[] }[]
  apply: { url: string; vars: PathVariable[]; want: string }[]
  resolve: { url: string; vars: PathVariable[]; env: Record<string, string>; want: string }[]
}

const cases: Cases = JSON.parse(readFileSync(new URL('../../../testdata/path-variables.json', import.meta.url), 'utf8'))

const pv = (...kv: string[]): PathVariable[] => {
  const out: PathVariable[] = []
  for (let i = 0; i < kv.length; i += 2) out.push({ key: kv[i], value: kv[i + 1] })
  return out
}

describe('pathVariableKeys', () => {
  it.each(cases.keys)('$url', ({ url, keys }) => {
    expect(pathVariableKeys(url)).toEqual(keys)
  })
})

describe('applyPathVariables', () => {
  it.each(cases.apply)('$url', ({ url, vars, want }) => {
    expect(applyPathVariables(url, vars)).toBe(want)
  })
})

describe('resolveURL', () => {
  it.each(cases.resolve)('$url', ({ url, vars, env, want }) => {
    const resolve = (s: string) => replaceVars(s, (name, match) => env[name] ?? match)
    expect(resolveURL(url, vars, resolve)).toBe(want)
  })
})

describe('syncPathVariables', () => {
  it('follows the URL order and drops keys the URL lost', () => {
    expect(syncPathVariables('/orgs/:org/users/:id', pv('id', '42', 'gone', 'x'))).toEqual(pv('org', '', 'id', '42'))
  })

  it('takes a remembered value for a key that comes back', () => {
    const remembered = new Map([['id', '42']])
    expect(syncPathVariables('/users/:id', [], remembered)).toEqual(pv('id', '42'))
    expect(syncPathVariables('/users/:id', pv('id', '7'), remembered)).toEqual(pv('id', '7'))
  })
})

describe('renamePathVariable', () => {
  it('renames every segment of the key and nothing else', () => {
    expect(renamePathVariable('http://h:8080/a/:id/b/:id/:idx?id=:id', 'id', 'userId'))
      .toBe('http://h:8080/a/:userId/b/:userId/:idx?id=:id')
  })
})

describe('isPathVariableName', () => {
  it.each([['id', true], ['user-id', true], ['', false], ['a/b', false], ['a?b', false], ['a#b', false]])(
    '%s → %s', (name, ok) => {
      expect(isPathVariableName(name as string)).toBe(ok)
    },
  )
})

describe('paramsCount', () => {
  it('counts query parameters and path variables', () => {
    expect(paramsCount('https://a.io/users/:id/:tab?x=1&y=2')).toBe(4)
    expect(paramsCount('{{base}}/users/:id')).toBe(1)
    expect(paramsCount('')).toBe(0)
  })
})

describe('pathVariableSpans', () => {
  it('marks each :name segment of the path only', () => {
    const url = 'http://h:8080/users/:id/x/:tab?q=:no'
    expect(pathVariableSpans(url).map(([from, to]) => url.slice(from, to))).toEqual([':id', ':tab'])
  })
})
