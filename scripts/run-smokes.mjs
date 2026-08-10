import { spawn } from 'node:child_process'

const smokeScripts = [
  'auth-smoke.mjs',
  'official-price-smoke.mjs',
  'api-market-smoke.mjs',
  'carpool-smoke.mjs',
  'profile-smoke.mjs',
  'announcement-smoke.mjs',
  'favorites-smoke.mjs',
  'review-smoke.mjs',
  'reports-smoke.mjs',
  'notification-smoke.mjs',
  'search-smoke.mjs',
]

const baseURL = process.env.API_BASE_URL || 'http://127.0.0.1:8080'
const runId = process.env.SMOKE_RUN_ID || `${Date.now()}-${Math.random().toString(16).slice(2, 10)}`

function runSmoke(script) {
  return new Promise((resolve) => {
    const startedAt = Date.now()
    let settled = false
    const finish = (result) => {
      if (settled) return
      settled = true
      resolve({
        script,
        durationMs: Date.now() - startedAt,
        ...result,
      })
    }
    const child = spawn(process.execPath, [`scripts/${script}`], {
      stdio: 'inherit',
      env: {
        ...process.env,
        API_BASE_URL: baseURL,
        SMOKE_RUN_ID: runId,
      },
    })
    child.on('error', error => finish({ ok: false, exitCode: null, error: error.message }))
    child.on('exit', code => {
      finish({
        ok: code === 0,
        exitCode: code,
        ...(code === 0 ? {} : { error: `${script} failed with exit code ${code}` }),
      })
    })
  })
}

const results = []
for (const script of smokeScripts) {
  console.log(`\n=== ${script} ===`)
  results.push(await runSmoke(script))
}

const ok = results.every(result => result.ok)
console.log(JSON.stringify({
  ok,
  apiBaseUrl: baseURL,
  runId,
  results,
}, null, 2))

if (!ok) process.exitCode = 1
