import { readFileSync } from 'node:fs'
import { expect, it } from 'vitest'

const INSTALL_SCRIPTS = ['preinstall', 'install', 'postinstall']

function read(name: string) {
  return JSON.parse(readFileSync(new URL(`../${name}`, import.meta.url), 'utf8'))
}

// npm rewrites a lock whose root entry disagrees with package.json, so every install would dirty the tree.
it('keeps the root entry of package-lock.json in sync with package.json', () => {
  const pkg = read('package.json')
  const root = read('package-lock.json').packages['']
  expect(root).toMatchObject({
    name: pkg.name,
    version: pkg.version,
    dependencies: pkg.dependencies,
    devDependencies: pkg.devDependencies,
  })
  expect(root.hasInstallScript ?? false).toBe(INSTALL_SCRIPTS.some((s) => s in pkg.scripts))
})
