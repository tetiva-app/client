import { execFileSync } from 'node:child_process'
import { copyFileSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterAll, expect, it } from 'vitest'

const DIST = new URL('../dist/', import.meta.url)
const DTS = readFileSync(new URL('snippets.d.mts', DIST), 'utf8')
const tmp = mkdtempSync(join(tmpdir(), 'tetiva-snippets-dts-'))

afterAll(() => rmSync(tmp, { recursive: true, force: true }))

// The share page copies dist/ as is, so the declarations cannot point back into this repo.
const FUNCTIONS = [
  'generate', 'targetsFor', 'SNIPPET_TARGETS', 'effectiveAuth', 'harFromSnapshotRequest', 'snippetInputFromSnapshot', 'snapshotToPostman',
]
const TYPES = [
  'Snapshot', 'SnapshotCollection', 'SnapshotFolder', 'SnapshotItem', 'SnapshotRequest', 'SnapshotHeader', 'SnapshotBody',
  'SnapshotAuth', 'SnapshotScripts', 'SnapshotExample', 'SnapshotEnvironment', 'SnapshotVariable',
  'SnippetInput', 'HarRequest', 'GrpcSnippet', 'WsSnippet', 'SnippetTarget', 'SnippetResult', 'SnippetProtocol', 'SnippetLanguage',
]

it('imports nothing from this repo', () => {
  expect(DTS).not.toMatch(/from\s+['"]@\//)
  expect(DTS).not.toMatch(/from\s+['"]\./)
  expect(DTS).not.toMatch(/import\s*\(/)
})

it('type-checks in a consumer that has only dist/', () => {
  copyFileSync(new URL('snippets.mjs', DIST), join(tmp, 'snippets.mjs'))
  copyFileSync(new URL('snippets.d.mts', DIST), join(tmp, 'snippets.d.mts'))
  writeFileSync(join(tmp, 'package.json'), JSON.stringify({ type: 'module' }))
  writeFileSync(join(tmp, 'tsconfig.json'), JSON.stringify({
    compilerOptions: {
      strict: true, noEmit: true, skipLibCheck: false, noUnusedLocals: true,
      module: 'nodenext', moduleResolution: 'nodenext', target: 'es2022', lib: ['es2022'], types: [],
    },
    files: ['consumer.mts'],
  }))
  writeFileSync(join(tmp, 'consumer.mts'), [
    `import { ${FUNCTIONS.join(', ')} } from './snippets.mjs'`,
    `import type { ${TYPES.join(', ')} } from './snippets.mjs'`,
    `export const functions = [${FUNCTIONS.join(', ')}]`,
    ...TYPES.map((t) => `export type Use${t} = ${t}`),
    '',
  ].join('\n'))

  const tsc = createRequire(import.meta.url).resolve('typescript/bin/tsc')
  let output = ''
  try {
    execFileSync(process.execPath, [tsc, '-p', tmp], { encoding: 'utf8' })
  } catch (e) {
    output = (e as { stdout?: string }).stdout ?? String(e)
  }
  expect(output).toBe('')
}, 60_000)
