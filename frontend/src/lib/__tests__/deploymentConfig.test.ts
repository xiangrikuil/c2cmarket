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

  it('keeps production backends loopback-only behind the VPS Caddy origin', () => {
    const composeOverride = readFileSync(
      new URL('../../../../compose.prod.yaml', import.meta.url),
      'utf8',
    )
    const caddyfile = readFileSync(
      new URL('../../../../deploy/caddy/Caddyfile.example', import.meta.url),
      'utf8',
    )

    expect(composeOverride).toContain('ports: !override')
    expect(composeOverride).toContain('127.0.0.1:${BACKEND_PORT:-8080}:${BACKEND_PORT:-8080}')
    expect(caddyfile).toContain('client_ip_headers CF-Connecting-IP X-Forwarded-For')
    expect(caddyfile).not.toContain('tls /etc/caddy/certs/')
    expect(caddyfile).toContain('reverse_proxy 127.0.0.1:8080')
    expect(caddyfile).toContain('reverse_proxy 127.0.0.1:8081')
  })

  it('schedules Linux production backups with the isolated production project', () => {
    const service = readFileSync(
      new URL('../../../../deploy/systemd/c2cmarket-postgres-backup.service.example', import.meta.url),
      'utf8',
    )
    const timer = readFileSync(
      new URL('../../../../deploy/systemd/c2cmarket-postgres-backup.timer.example', import.meta.url),
      'utf8',
    )

    expect(service).toContain('Environment=COMPOSE_PROJECT=c2c-prod')
    expect(service).toContain('Environment=ENV_FILE=/opt/c2cmarket/shared/.env.production')
    expect(service).toContain('Environment=BACKUP_DIR=/var/lib/c2cmarket/backups/production')
    expect(service).toContain('ExecStart=/bin/bash /opt/c2cmarket/current/scripts/backup-production-postgres.sh')
    expect(timer).toContain('OnCalendar=*-*-* 03:30:00 Asia/Shanghai')
    expect(timer).toContain('Persistent=true')
  })
})
