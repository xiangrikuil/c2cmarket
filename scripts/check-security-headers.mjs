#!/usr/bin/env node

import { readFile } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const headersPath = resolve(repoRoot, 'frontend/public/_headers')
const source = await readFile(headersPath, 'utf8')
const lines = source
  .split(/\r?\n/)
  .map(line => line.trim())
  .filter(Boolean)

const route = lines.shift()
const headers = new Map()
for (const line of lines) {
  const separator = line.indexOf(':')
  if (separator < 1) {
    throw new Error(`Invalid Cloudflare Pages header line: ${line}`)
  }
  headers.set(line.slice(0, separator).toLowerCase(), line.slice(separator + 1).trim())
}

const failures = []
if (route !== '/*') failures.push('Cloudflare Pages headers must apply to /*')

const expectedHeaders = new Map([
  ['permissions-policy', 'camera=(), geolocation=(), microphone=(), payment=(), usb=()'],
  ['referrer-policy', 'strict-origin-when-cross-origin'],
  ['strict-transport-security', 'max-age=31536000; includeSubDomains'],
  ['x-content-type-options', 'nosniff'],
  ['x-frame-options', 'DENY'],
])
for (const [name, value] of expectedHeaders) {
  if (headers.get(name) !== value) {
    failures.push(`${name} must equal ${value}`)
  }
}

const csp = headers.get('content-security-policy') ?? ''
const requiredDirectives = [
  "default-src 'self'",
  "base-uri 'self'",
  "object-src 'none'",
  "frame-ancestors 'none'",
  "form-action 'self'",
  "script-src 'self' https://challenges.cloudflare.com",
  "style-src 'self' 'unsafe-inline'",
  "connect-src 'self' https://api.c2cmarket.shop https://api-staging.c2cmarket.shop https://challenges.cloudflare.com https://o4511896757010432.ingest.us.sentry.io",
  'frame-src https://challenges.cloudflare.com',
  'upgrade-insecure-requests',
]
for (const directive of requiredDirectives) {
  if (!csp.split(';').map(value => value.trim()).includes(directive)) {
    failures.push(`content-security-policy is missing: ${directive}`)
  }
}
for (const forbidden of ["'unsafe-eval'", '*', 'http:']) {
  if (csp.split(/\s|;/).includes(forbidden)) {
    failures.push(`content-security-policy must not contain ${forbidden}`)
  }
}

if (failures.length > 0) {
  console.error(failures.join('\n'))
  process.exit(1)
}

console.log('Security header guard passed for Cloudflare Pages.')
