import { spawn } from 'node:child_process'

const smokeScripts = [
  'auth-smoke.mjs',
  'official-price-smoke.mjs',
  'api-market-smoke.mjs',
  'carpool-smoke.mjs',
  'profile-smoke.mjs',
  'announcement-smoke.mjs',
  'demand-smoke.mjs',
  'favorites-smoke.mjs',
  'review-smoke.mjs',
  'reports-smoke.mjs',
  'notification-smoke.mjs',
  'search-smoke.mjs',
]

const baseURL = process.env.API_BASE_URL || 'http://127.0.0.1:8080'

function runSmoke(script) {
  return new Promise((resolve, reject) => {
    const child = spawn(process.execPath, [`scripts/${script}`], {
      stdio: 'inherit',
      env: {
        ...process.env,
        API_BASE_URL: baseURL,
      },
    })
    child.on('error', reject)
    child.on('exit', code => {
      if (code === 0) {
        resolve()
        return
      }
      reject(new Error(`${script} failed with exit code ${code}`))
    })
  })
}

for (const script of smokeScripts) {
  console.log(`\n=== ${script} ===`)
  await runSmoke(script)
}

console.log(JSON.stringify({
  ok: true,
  apiBaseUrl: baseURL,
  scripts: smokeScripts,
}, null, 2))
