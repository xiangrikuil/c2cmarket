type PublicEnv = Record<string, unknown>

export type UmamiScriptConfig = {
  scriptUrl: string
  websiteId: string
  domains?: string
  hostUrl?: string
}

const scriptId = 'c2cmarket-umami-script'

const envString = (env: PublicEnv, key: string) => {
  const value = env[key]
  return typeof value === 'string' ? value.trim() : ''
}

export const buildUmamiScriptConfig = (env: PublicEnv = import.meta.env): UmamiScriptConfig | null => {
  if (envString(env, 'VITE_UMAMI_ENABLED') !== 'true') return null

  const scriptUrl = envString(env, 'VITE_UMAMI_SCRIPT_URL')
  const websiteId = envString(env, 'VITE_UMAMI_WEBSITE_ID')
  if (!scriptUrl || !websiteId) return null

  const domains = envString(env, 'VITE_UMAMI_DOMAINS')
  const hostUrl = envString(env, 'VITE_UMAMI_HOST_URL')
  return {
    scriptUrl,
    websiteId,
    ...(domains ? { domains } : {}),
    ...(hostUrl ? { hostUrl } : {}),
  }
}

export const installUmamiScript = (doc: Document | null = typeof document === 'undefined' ? null : document) => {
  const config = buildUmamiScriptConfig()
  if (!config || !doc?.head) return false
  if (doc.getElementById(scriptId)) return true

  const script = doc.createElement('script')
  script.id = scriptId
  script.defer = true
  script.src = config.scriptUrl
  script.setAttribute('data-website-id', config.websiteId)
  if (config.domains) script.setAttribute('data-domains', config.domains)
  if (config.hostUrl) script.setAttribute('data-host-url', config.hostUrl)
  doc.head.appendChild(script)
  return true
}
