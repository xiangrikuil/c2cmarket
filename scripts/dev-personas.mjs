#!/usr/bin/env node

import { constants } from 'node:fs'
import { chmod, lstat, mkdir, open } from 'node:fs/promises'
import { resolve } from 'node:path'

const personas = ['buyer', 'seller', 'admin']
const defaultBaseURL = 'http://127.0.0.1:8080'
const defaultOutputDirectory = 'output/dev-sessions'

function usage() {
  return `Prepare fixed C2CMarket development personas.

Usage:
  node scripts/dev-personas.mjs [all|buyer|seller|admin] [options]

Options:
  --base-url <url>     Backend base URL (default: ${defaultBaseURL})
  --output-dir <path>  Protected session output directory (default: ${defaultOutputDirectory})
  --help               Show this help

Stable users:
  buyer   dev-buyer
  seller  dev-seller
  admin   dev-admin

The backend must run with development authentication enabled. Session cookies
and CSRF tokens are written only to mode-0600 JSON files and are never printed.`
}

function parseArguments(argv) {
  let selection = 'all'
  let selectionSeen = false
  let baseURL = defaultBaseURL
  let outputDirectory = defaultOutputDirectory

  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index]
    if (argument === '--help') return { help: true }
    if (argument === '--base-url' || argument === '--output-dir') {
      const value = argv[index + 1]
      if (!value || value.startsWith('--')) throw new Error(`${argument} requires a value`)
      if (argument === '--base-url') baseURL = value
      else outputDirectory = value
      index += 1
      continue
    }
    if (argument.startsWith('--')) throw new Error(`unknown option: ${argument}`)
    if (selectionSeen) throw new Error(`unexpected argument: ${argument}`)
    selection = argument.toLowerCase()
    selectionSeen = true
  }

  if (selection !== 'all' && !personas.includes(selection)) {
    throw new Error('persona must be all, buyer, seller, or admin')
  }

  let parsedBaseURL
  try {
    parsedBaseURL = new URL(baseURL)
  } catch {
    throw new Error('invalid --base-url')
  }
  if (!['http:', 'https:'].includes(parsedBaseURL.protocol)) {
    throw new Error('--base-url must use http or https')
  }
  if (parsedBaseURL.username || parsedBaseURL.password) {
    throw new Error('--base-url must not include credentials')
  }
  if (parsedBaseURL.search || parsedBaseURL.hash) {
    throw new Error('--base-url must not include a query string or fragment')
  }

  return {
    help: false,
    selectedPersonas: selection === 'all' ? personas : [selection],
    baseURL: parsedBaseURL.toString().replace(/\/$/, ''),
    outputDirectory: resolve(outputDirectory),
  }
}

function responseCookies(response) {
  const values = typeof response.headers.getSetCookie === 'function'
    ? response.headers.getSetCookie()
    : (response.headers.get('set-cookie') ? [response.headers.get('set-cookie')] : [])
  return values.filter(Boolean).map(value => value.split(';', 1)[0])
}

async function decodeResponse(response) {
  const text = await response.text()
  let body = null
  if (text) {
    try {
      body = JSON.parse(text)
    } catch {
      body = { detail: text }
    }
  }
  if (!response.ok) {
    const detail = body?.detail || body?.title || response.statusText || 'request failed'
    throw new Error(`${response.status} ${detail}`)
  }
  return body
}

async function preparePersona(baseURL, persona) {
  let response
  try {
    response = await fetch(`${baseURL}/api/v1/auth/dev-persona-session`, {
      method: 'POST',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ persona }),
    })
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    throw new Error(`cannot reach ${baseURL}: ${detail}`)
  }

  const session = await decodeResponse(response)
  const cookies = responseCookies(response)
  if (!session?.csrfToken || cookies.length === 0) {
    throw new Error(`${persona} response did not include a normal session cookie and CSRF token`)
  }
  return { session, cookies }
}

async function writeProtectedSession(outputDirectory, persona, prepared, baseURL) {
  await mkdir(outputDirectory, { recursive: true, mode: 0o700 })
  const directoryStat = await lstat(outputDirectory)
  if (!directoryStat.isDirectory() || directoryStat.isSymbolicLink()) {
    throw new Error(`session output directory must be a real directory: ${outputDirectory}`)
  }
  await chmod(outputDirectory, 0o700)
  const outputPath = resolve(outputDirectory, `${persona}.json`)
  const payload = {
    persona,
    baseUrl: baseURL,
    username: prepared.session.user.username,
    cookieHeader: prepared.cookies.join('; '),
    session: prepared.session,
  }
  const outputFile = await open(
    outputPath,
    constants.O_CREAT | constants.O_WRONLY | constants.O_TRUNC | constants.O_NOFOLLOW,
    0o600,
  )
  try {
    await outputFile.chmod(0o600)
    await outputFile.writeFile(`${JSON.stringify(payload, null, 2)}\n`)
  } finally {
    await outputFile.close()
  }
  return outputPath
}

async function main() {
  const options = parseArguments(process.argv.slice(2))
  if (options.help) {
    process.stdout.write(`${usage()}\n`)
    return
  }

  const prepared = []
  for (const persona of options.selectedPersonas) {
    const session = await preparePersona(options.baseURL, persona)
    const outputPath = await writeProtectedSession(options.outputDirectory, persona, session, options.baseURL)
    prepared.push({
      persona,
      username: session.session.user.username,
      outputPath,
    })
  }

  process.stdout.write(`${JSON.stringify({ ok: true, prepared }, null, 2)}\n`)
}

main().catch((error) => {
  const detail = error instanceof Error ? error.message : String(error)
  process.stderr.write(`dev persona setup failed: ${detail}\n`)
  process.exitCode = 1
})
