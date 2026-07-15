import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

type WranglerConfig = {
  name?: string
  keep_vars?: boolean
  assets?: {
    directory?: string
    not_found_handling?: string
  }
}

describe('Cloudflare Worker deployment config', () => {
  const environments = [
    ['production', '../../../../wrangler.jsonc', 'c2cmarket'],
    ['staging', '../../../../wrangler.staging.jsonc', 'c2cmarket-staging'],
  ] as const

  it.each(environments)('serves %s Vue navigation routes through the SPA fallback', (_, path, name) => {
    const source = readFileSync(new URL(path, import.meta.url), 'utf8')
    const config = JSON.parse(source) as WranglerConfig

    expect(config.name).toBe(name)
    expect(config.keep_vars).toBe(true)
    expect(config.assets).toEqual({
      directory: './frontend/dist',
      not_found_handling: 'single-page-application',
    })
  })
})
