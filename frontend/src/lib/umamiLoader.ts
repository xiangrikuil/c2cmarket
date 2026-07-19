export type UmamiRuntimeConfig = {
  enabled?: unknown
  scriptUrl?: unknown
  websiteId?: unknown
  domains?: unknown
  hostUrl?: unknown
}

export type UmamiScriptConfig = {
  scriptUrl: string
  websiteId: string
  domains?: string
  hostUrl?: string
}

const scriptId = 'c2cmarket-umami-script'

const configString = (value: unknown) => {
  return typeof value === 'string' ? value.trim() : ''
}

const configBoolean = (value: unknown) => {
  if (typeof value === 'boolean') return value
  return configString(value) === 'true'
}

export const buildUmamiScriptConfig = (runtimeConfig: UmamiRuntimeConfig): UmamiScriptConfig | null => {
  if (!configBoolean(runtimeConfig.enabled)) return null

  const scriptUrl = configString(runtimeConfig.scriptUrl)
  const websiteId = configString(runtimeConfig.websiteId)
  if (!scriptUrl || !websiteId) return null

  const domains = configString(runtimeConfig.domains)
  const hostUrl = configString(runtimeConfig.hostUrl)
  return {
    scriptUrl,
    websiteId,
    ...(domains ? { domains } : {}),
    ...(hostUrl ? { hostUrl } : {}),
  }
}

export const installUmamiScript = (
  config: UmamiScriptConfig | null,
  doc: Document | null = typeof document === 'undefined' ? null : document,
) => {
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
