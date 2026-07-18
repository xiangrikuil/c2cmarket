import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

type WranglerConfig = {
  name?: string
  keep_vars?: boolean
  main?: string
  compatibility_flags?: string[]
  assets?: {
    directory?: string
  }
  vars?: Record<string, string>
}

describe('Cloudflare Worker deployment config', () => {
  const environments = [
    ['production', '../../../../wrangler.jsonc', 'c2cmarket'],
    ['staging', '../../../../wrangler.staging.jsonc', 'c2cmarket-staging'],
  ] as const

  it.each(environments)('serves %s Nuxt SSR through the Cloudflare Worker output', (environment, path, name) => {
    const source = readFileSync(new URL(path, import.meta.url), 'utf8')
    const config = JSON.parse(source) as WranglerConfig

    expect(config.name).toBe(name)
    expect(config.keep_vars).toBe(true)
    expect(config.main).toBe('./frontend/.output/server/index.mjs')
    expect(config.compatibility_flags).toContain('nodejs_compat')
    expect(config.assets).toEqual({
      directory: './frontend/.output/public',
    })
    expect(config.vars?.NUXT_PUBLIC_API_MODE).toBe('real')
    expect(config.vars?.NUXT_PUBLIC_SITE_URL).toBe(environment === 'production'
      ? 'https://c2cmarket.shop'
      : 'https://staging.c2cmarket.shop')
  })

  it('keeps production backends loopback-only behind the VPS Caddy origin', () => {
    const compose = readFileSync(new URL('../../../../compose.yaml', import.meta.url), 'utf8')
    const composeOverride = readFileSync(
      new URL('../../../../compose.prod.yaml', import.meta.url),
      'utf8',
    )
    const caddyfile = readFileSync(
      new URL('../../../../deploy/caddy/Caddyfile.example', import.meta.url),
      'utf8',
    )

    expect(compose).toContain('image: ${BACKEND_IMAGE:-c2cmarket-backend:local}')
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

  it('releases only tested staging and main pushes through the reusable backend workflow', () => {
    const ci = readFileSync(new URL('../../../../.github/workflows/ci.yml', import.meta.url), 'utf8')
    const release = readFileSync(
      new URL('../../../../.github/workflows/release-backend.yml', import.meta.url),
      'utf8',
    )

    expect(ci).toContain('branches: [staging, main]')
    expect(ci).toContain("if: github.event_name == 'push' && github.ref == 'refs/heads/staging'")
    expect(ci).toContain("if: github.event_name == 'push' && github.ref == 'refs/heads/main'")
    expect(ci).toContain('needs: [backend, frontend]')
    expect(ci).toContain('uses: ./.github/workflows/release-backend.yml')
    expect(release).toContain('workflow_call:')
    expect(release).toContain('ghcr.io/xiangrikuil/c2cmarket-backend')
    expect(release).toContain('${{ inputs.git_sha }}')
    expect(release).toContain('password: ${{ secrets.GITHUB_TOKEN }}')
    expect(release).toContain('name: ${{ inputs.deploy_environment }}')
    expect(release).toContain('-o IdentitiesOnly=yes')
    expect(release).not.toContain('StrictHostKeyChecking=no')
    expect(release).not.toContain('root@')
  })

  it('keeps production backup, migration, image pull, and current-link installation explicit', () => {
    const deploy = readFileSync(
      new URL('../../../../scripts/deploy-vps-backend.sh', import.meta.url),
      'utf8',
    )
    const install = readFileSync(
      new URL('../../../../scripts/install-vps-release.sh', import.meta.url),
      'utf8',
    )
    const runbook = readFileSync(
      new URL('../../../../docs/ops/cloudflare-workers-vps-backends.md', import.meta.url),
      'utf8',
    )

    const backupIndex = deploy.indexOf('backup-production-postgres.sh')
    const migrationIndex = deploy.indexOf('--profile migrate run --rm migrate')
    const startupIndex = deploy.indexOf('--profile app up -d --no-build backend')

    expect(backupIndex).toBeGreaterThan(-1)
    expect(migrationIndex).toBeGreaterThan(backupIndex)
    expect(startupIndex).toBeGreaterThan(migrationIndex)
    expect(deploy).toContain('ghcr\\.io/xiangrikuil/c2cmarket-backend:[0-9a-f]{40}')
    expect(deploy).toContain('export BACKEND_IMAGE BACKEND_PORT')
    expect(deploy).toContain('127.0.0.1:${BACKEND_PORT}/readyz')
    expect(install).toContain('/opt/c2cmarket')
    expect(install).toContain('releases/${DEPLOY_ENVIRONMENT}/${GIT_SHA}')
    expect(install).toContain('ln -sfn "${release_dir}" "${CURRENT_LINK}"')
    expect(runbook).toContain('VPS_SSH_PRIVATE_KEY')
    expect(runbook).toContain('VPS_SSH_KNOWN_HOSTS')
    expect(runbook).toContain('GHCR_READ_TOKEN')
    expect(runbook).toContain('required reviewer')
    expect(runbook).toContain('不得复用个人 `root` 私钥')
  })
})
