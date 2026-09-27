import { mkdtempSync, readdirSync, readFileSync, rmSync, statSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { afterAll, expect, it } from 'vitest'

const SRC_DIR = fileURLToPath(new URL('../../../', import.meta.url))
const BUNDLE = new URL('../dist/snippets.mjs', import.meta.url)
const DECLARATIONS = new URL('../dist/snippets.d.mts', import.meta.url)
const BUILD_SCRIPT = new URL('../../../../scripts/build-snippets.mjs', import.meta.url).href
const tmp = mkdtempSync(join(tmpdir(), 'tetiva-snippets-'))

afterAll(() => rmSync(tmp, { recursive: true, force: true }))

function tree(dir: string): string[] {
  return readdirSync(dir, { recursive: true, encoding: 'utf8' }).sort().map((rel) => {
    const st = statSync(join(dir, rel))
    return `${rel} ${st.size} ${st.mtimeMs}`
  })
}

it('rebuilds byte for byte into the committed bundle without writing to src', async () => {
  const before = tree(SRC_DIR)
  const { build } = await import(/* @vite-ignore */ BUILD_SCRIPT) as { build: (outfile?: string) => Promise<void> }
  const outfile = join(tmp, 'snippets.mjs')

  await build(outfile)

  expect(readFileSync(outfile).equals(readFileSync(BUNDLE))).toBe(true)
  expect(readFileSync(join(tmp, 'snippets.d.mts')).equals(readFileSync(DECLARATIONS))).toBe(true)
  expect(tree(SRC_DIR)).toEqual(before)
}, 60_000)
