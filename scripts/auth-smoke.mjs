const baseURL = process.env.API_BASE_URL || 'http://127.0.0.1:8080'

function assert(condition, message) {
  if (!condition) throw new Error(message)
}

function cookieHeader(jar) {
  return Object.entries(jar).map(([name, value]) => `${name}=${value}`).join('; ')
}

function storeCookies(jar, response) {
  const values = typeof response.headers.getSetCookie === 'function'
    ? response.headers.getSetCookie()
    : (response.headers.get('set-cookie') ? [response.headers.get('set-cookie')] : [])
  for (const raw of values) {
    const first = raw.split(';')[0]
    const index = first.indexOf('=')
    if (index > 0) jar[first.slice(0, index)] = first.slice(index + 1)
  }
}

async function request(path, options = {}, jar = {}) {
  const headers = { ...(options.headers || {}) }
  const cookies = cookieHeader(jar)
  if (cookies) headers.Cookie = cookies
  const response = await fetch(`${baseURL}${path}`, {
    redirect: options.redirect || 'manual',
    ...options,
    headers,
  })
  storeCookies(jar, response)
  if (options.expectedStatus && response.status === options.expectedStatus) {
    const text = await response.text()
    return text ? JSON.parse(text) : null
  }
  if (!response.ok && response.status < 300 || response.status >= 400) {
    const text = await response.text()
    throw new Error(`${response.status} ${response.statusText}: ${text}`)
  }
  const text = await response.text()
  return text ? JSON.parse(text) : null
}

async function oauthLogin(code, jar) {
  const start = await request('/api/v1/auth/oauth/start?returnTo=/my', {}, jar)
  assert(start.authorizationUrl, 'start should return authorizationUrl')
  const url = new URL(start.authorizationUrl)
  url.searchParams.set('code', code)
  const callbackResponse = await fetch(url.toString(), {
    redirect: 'manual',
    headers: { Cookie: cookieHeader(jar) },
  })
  storeCookies(jar, callbackResponse)
  assert(callbackResponse.status === 302, 'callback should redirect after login')
  assert(callbackResponse.headers.get('location') === '/my', 'callback should redirect to returnTo')
  const session = await request('/api/v1/auth/session', {}, jar)
  assert(session.user.username === code.replace(/^fake-/, '').toLowerCase(), 'session username should match fake code')
  assert(session.user.linuxDoBinding?.bound === true, 'session should include linux.do binding')
  assert(session.csrfToken, 'session should include csrfToken')
  return session
}

async function main() {
  const userJar = {}
  const adminJar = {}

  const userSession = await oauthLogin(`fake-auth-user-${Date.now()}`, userJar)
  assert(!userSession.user.permissions.includes('admin'), 'regular OAuth user should not be admin')

  const denied = await request('/api/v1/admin/announcements', { expectedStatus: 403 }, userJar)
  assert(denied.code === 'PERMISSION_DENIED', 'regular user should be denied admin route')

  const adminSession = await oauthLogin(`fake-auth-admin-${Date.now()}`, adminJar)
  assert(adminSession.user.permissions.includes('admin'), 'admin fake OAuth user should include admin permission')

  const adminAnnouncements = await request('/api/v1/admin/announcements', {}, adminJar)
  assert(Array.isArray(adminAnnouncements.items), 'admin should read announcement list')

  await request('/api/v1/auth/logout', {
    method: 'POST',
    headers: { 'X-CSRF-Token': adminSession.csrfToken },
    expectedStatus: 204,
  }, adminJar)
  const afterLogout = await request('/api/v1/auth/session', { expectedStatus: 401 }, adminJar)
  assert(afterLogout.code === 'SESSION_REVOKED' || afterLogout.code === 'SESSION_EXPIRED', 'logout should invalidate session')

  console.log(JSON.stringify({
    ok: true,
    user: userSession.user.username,
    admin: adminSession.user.username,
    adminPermissions: adminSession.user.permissions,
  }, null, 2))
}

main().catch(error => {
  console.error(error)
  process.exit(1)
})
