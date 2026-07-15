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

  it('keeps the local Cloudflare Tunnel on a persistent HTTP/2 launch service', () => {
    const tunnelConfig = readFileSync(
      new URL('../../../../deploy/cloudflared/config.yml.example', import.meta.url),
      'utf8',
    )
    const launchAgent = readFileSync(
      new URL('../../../../deploy/launchd/com.cloudflare.cloudflared.plist.example', import.meta.url),
      'utf8',
    )

    expect(tunnelConfig).toMatch(/^protocol: http2$/m)
    expect(launchAgent).toContain('<string>com.cloudflare.cloudflared</string>')
    expect(launchAgent).toContain('<string>/Users/CHANGE_ME/.cloudflared/config.yml</string>')
    expect(launchAgent).toMatch(/<key>KeepAlive<\/key>[\s\S]*?<key>SuccessfulExit<\/key>\s*<false\/>/)
    expect(launchAgent).toMatch(/<key>RunAtLoad<\/key>\s*<true\/>/)
  })
})
