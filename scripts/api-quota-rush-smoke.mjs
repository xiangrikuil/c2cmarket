#!/usr/bin/env node

import { readFile } from 'node:fs/promises'

function usage() {
  console.log(`Usage:
  node scripts/api-quota-rush-smoke.mjs \\
    --offer-id <uuid> \\
    --round-id <uuid> \\
    --sessions-file <jsonl> \\
    [--base-url http://127.0.0.1:8080] \\
    [--requests 1500] \\
    [--concurrency 150] \\
    [--expect-success 1000] \\
    [--expect-sold-out 500]

Each JSONL row must contain cookie, csrfToken, and buyerContactMethodId.
Optional row fields: selectedAccessMode, paymentMethod, buyerNote, ip.
The tool never prints session cookies or CSRF tokens.`)
}

function parsePositiveInteger(value, flag) {
  const parsed = Number(value)
  if (!Number.isSafeInteger(parsed) || parsed <= 0) throw new Error(`${flag} must be a positive integer`)
  return parsed
}

function parseNonNegativeInteger(value, flag) {
  const parsed = Number(value)
  if (!Number.isSafeInteger(parsed) || parsed < 0) throw new Error(`${flag} must be a non-negative integer`)
  return parsed
}

function parseArgs(argv) {
  const options = {
    baseURL: process.env.API_BASE_URL ?? 'http://127.0.0.1:8080',
    concurrency: 150,
  }
  for (let index = 0; index < argv.length; index += 1) {
    const flag = argv[index]
    if (flag === '--help' || flag === '-h') return { help: true }
    const value = argv[index + 1]
    if (!value || value.startsWith('--')) throw new Error(`missing value for ${flag}`)
    index += 1
    if (flag === '--offer-id') options.offerId = value
    else if (flag === '--round-id') options.roundId = value
    else if (flag === '--sessions-file') options.sessionsFile = value
    else if (flag === '--base-url') options.baseURL = value.replace(/\/$/, '')
    else if (flag === '--requests') options.requests = parsePositiveInteger(value, flag)
    else if (flag === '--concurrency') options.concurrency = parsePositiveInteger(value, flag)
    else if (flag === '--expect-success') options.expectSuccess = parseNonNegativeInteger(value, flag)
    else if (flag === '--expect-sold-out') options.expectSoldOut = parseNonNegativeInteger(value, flag)
    else throw new Error(`unknown option ${flag}`)
  }
  if (!options.offerId) throw new Error('--offer-id is required')
  if (!options.sessionsFile) throw new Error('--sessions-file is required')
  return options
}

async function readSessions(path) {
  const source = await readFile(path, 'utf8')
  return source.split(/\r?\n/).flatMap((line, index) => {
    const trimmed = line.trim()
    if (!trimmed) return []
    let row
    try {
      row = JSON.parse(trimmed)
    } catch (error) {
      throw new Error(`invalid JSON on sessions line ${index + 1}: ${error.message}`)
    }
    for (const field of ['cookie', 'csrfToken', 'buyerContactMethodId']) {
      if (typeof row[field] !== 'string' || !row[field].trim()) throw new Error(`sessions line ${index + 1} is missing ${field}`)
    }
    return [row]
  })
}

function percentile(sorted, percent) {
  if (sorted.length === 0) return 0
  return sorted[Math.floor((sorted.length - 1) * percent / 100)]
}

async function createOrder(options, session, index) {
  const startedAt = performance.now()
  try {
    const response = await fetch(`${options.baseURL}/api/v1/api-quota-offers/${options.offerId}/orders`, {
      method: 'POST',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
        Cookie: session.cookie,
        'X-CSRF-Token': session.csrfToken,
        'Idempotency-Key': `quota-rush-${Date.now()}-${index}`,
        ...(session.ip ? { 'X-Forwarded-For': session.ip } : {}),
      },
      body: JSON.stringify({
        saleRoundId: options.roundId ?? '',
        buyerContactMethodId: session.buyerContactMethodId,
        selectedAccessMode: session.selectedAccessMode ?? 'buyer_dedicated_sub_key',
        paymentMethod: session.paymentMethod ?? 'wechat',
        buyerNote: session.buyerNote ?? '额度包并发 smoke',
      }),
    })
    const text = await response.text()
    let body = null
    if (text) {
      try {
        body = JSON.parse(text)
      } catch {
        body = null
      }
    }
    return {
      status: response.status,
      code: typeof body?.code === 'string' ? body.code : '',
      orderId: response.ok && typeof body?.id === 'string' ? body.id : '',
      latencyMs: performance.now() - startedAt,
    }
  } catch {
    return { status: 0, code: 'NETWORK_ERROR', orderId: '', latencyMs: performance.now() - startedAt }
  }
}

async function main() {
  const options = parseArgs(process.argv.slice(2))
  if (options.help) {
    usage()
    return
  }
  const sessions = await readSessions(options.sessionsFile)
  const requestCount = options.requests ?? sessions.length
  if (requestCount > sessions.length) throw new Error(`requested ${requestCount} buyers but sessions file only contains ${sessions.length}`)

  const results = new Array(requestCount)
  let nextIndex = 0
  const startedAt = performance.now()
  async function worker() {
    while (true) {
      const index = nextIndex
      nextIndex += 1
      if (index >= requestCount) return
      results[index] = await createOrder(options, sessions[index], index)
    }
  }
  await Promise.all(Array.from({ length: Math.min(options.concurrency, requestCount) }, worker))
  const elapsedMs = performance.now() - startedAt

  const distribution = {}
  const orderIDs = new Set()
  const latencies = []
  let successes = 0
  let soldOut = 0
  let serverErrors = 0
  for (const result of results) {
    const key = `${result.status}${result.code ? `:${result.code}` : ''}`
    distribution[key] = (distribution[key] ?? 0) + 1
    latencies.push(result.latencyMs)
    if (result.status >= 200 && result.status < 300) {
      successes += 1
      if (result.orderId) orderIDs.add(result.orderId)
    }
    if (result.code === 'API_QUOTA_SOLD_OUT') soldOut += 1
    if (result.status === 0 || result.status >= 500) serverErrors += 1
  }
  latencies.sort((left, right) => left - right)
  const report = {
    requests: requestCount,
    concurrency: Math.min(options.concurrency, requestCount),
    successes,
    soldOut,
    distinctOrderIds: orderIDs.size,
    serverErrors,
    throughputRequestsPerSecond: Number((requestCount / (elapsedMs / 1000)).toFixed(1)),
    latencyMs: {
      p50: Number(percentile(latencies, 50).toFixed(1)),
      p95: Number(percentile(latencies, 95).toFixed(1)),
      p99: Number(percentile(latencies, 99).toFixed(1)),
    },
    statusDistribution: distribution,
  }
  console.log(JSON.stringify(report, null, 2))

  if (serverErrors > 0) throw new Error(`rush returned ${serverErrors} network or 5xx errors`)
  if (orderIDs.size !== successes) throw new Error(`successful responses contained duplicate or missing order ids: ${orderIDs.size}/${successes}`)
  if (options.expectSuccess !== undefined && successes !== options.expectSuccess) throw new Error(`expected ${options.expectSuccess} successes, got ${successes}`)
  if (options.expectSoldOut !== undefined && soldOut !== options.expectSoldOut) throw new Error(`expected ${options.expectSoldOut} sold-out responses, got ${soldOut}`)
}

main().catch(error => {
  console.error(error.message)
  process.exitCode = 1
})
