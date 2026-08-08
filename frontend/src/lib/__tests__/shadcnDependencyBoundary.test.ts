import { readdirSync, readFileSync } from 'node:fs'
import { extname, join, relative, sep } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const sourceRoot = fileURLToPath(new URL('../../', import.meta.url))
const productionExtensions = new Set(['.js', '.mjs', '.ts', '.tsx', '.vue'])
const directRekaReference = /\bfrom\s*['"]reka-ui(?:\/[^'"]*)?['"]|\b(?:import|require)\s*\(\s*['"]reka-ui(?:\/[^'"]*)?['"]\s*\)|^\s*import\s*['"]reka-ui(?:\/[^'"]*)?['"]/m
const appProviderImport = "import { ConfigProvider } from 'reka-ui'"

function productionSourceFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name)
    if (entry.isDirectory()) {
      return entry.name === '__tests__' ? [] : productionSourceFiles(path)
    }
    return productionExtensions.has(extname(entry.name)) ? [path] : []
  })
}

function sourceRelativePath(path: string) {
  return relative(sourceRoot, path).split(sep).join('/')
}

describe('shadcn-vue dependency boundary', () => {
  it('keeps direct Reka UI usage inside generated primitives and the app provider', () => {
    const appSource = readFileSync(join(sourceRoot, 'App.vue'), 'utf8')
    const violations = productionSourceFiles(sourceRoot)
      .map(path => ({ path: sourceRelativePath(path), source: readFileSync(path, 'utf8') }))
      .filter(file => file.path !== 'App.vue' && !file.path.startsWith('components/ui/'))
      .filter(file => directRekaReference.test(file.source))
      .map(file => file.path)

    expect(violations).toEqual([])
    expect(appSource).toContain(appProviderImport)
    expect(appSource.replace(appProviderImport, '')).not.toMatch(directRekaReference)
  })
})
