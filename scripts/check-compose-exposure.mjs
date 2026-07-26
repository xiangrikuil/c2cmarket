#!/usr/bin/env node

import { spawnSync } from 'node:child_process'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const variants = [
  {
    name: 'development',
    args: ['--env-file', '.env.example', '-f', 'compose.yaml'],
    requirePrivatePostgres: false,
  },
  {
    name: 'production',
    args: [
      '--env-file',
      '.env.production.example',
      '-f',
      'compose.yaml',
      '-f',
      'compose.prod.yaml',
    ],
    requirePrivatePostgres: true,
  },
  {
    name: 'staging',
    args: [
      '--env-file',
      '.env.staging.example',
      '-f',
      'compose.yaml',
      '-f',
      'compose.prod.yaml',
    ],
    requirePrivatePostgres: true,
  },
]

const failures = []

for (const variant of variants) {
  const result = spawnSync(
    'docker',
    ['compose', '--profile', 'app', ...variant.args, 'config', '--format', 'json'],
    {
      cwd: repoRoot,
      encoding: 'utf8',
    },
  )

  if (result.error) {
    failures.push(`${variant.name}: failed to execute docker compose: ${result.error.message}`)
    continue
  }
  if (result.status !== 0) {
    failures.push(`${variant.name}: docker compose config failed: ${result.stderr.trim()}`)
    continue
  }

  let config
  try {
    config = JSON.parse(result.stdout)
  } catch (error) {
    failures.push(`${variant.name}: invalid JSON from docker compose: ${error.message}`)
    continue
  }

  const backendPorts = config.services?.backend?.ports ?? []
  if (backendPorts.length === 0) {
    failures.push(`${variant.name}: backend has no published port`)
  }
  for (const port of backendPorts) {
    if (port.host_ip !== '127.0.0.1') {
      failures.push(
        `${variant.name}: backend port ${port.published ?? port.target} binds ${port.host_ip || 'all interfaces'}`,
      )
    }
  }

  if (variant.requirePrivatePostgres) {
    const postgresPorts = config.services?.postgres?.ports ?? []
    if (postgresPorts.length > 0) {
      failures.push(`${variant.name}: PostgreSQL must not publish a host port`)
    }
  }
}

if (failures.length > 0) {
  console.error(failures.join('\n'))
  process.exit(1)
}

console.log('Compose exposure guard passed for development, production, and staging.')
