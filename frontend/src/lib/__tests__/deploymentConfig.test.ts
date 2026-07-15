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
  it('serves Vue navigation routes through the SPA fallback', () => {
    const source = readFileSync(new URL('../../../../wrangler.jsonc', import.meta.url), 'utf8')
    const config = JSON.parse(source) as WranglerConfig

    expect(config.name).toBe('c2cmarket')
    expect(config.keep_vars).toBe(true)
    expect(config.assets).toEqual({
      directory: './frontend/dist',
      not_found_handling: 'single-page-application',
    })
  })
})
