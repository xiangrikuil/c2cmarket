export const apiModes = ['real', 'mock'] as const

export type ApiMode = typeof apiModes[number]

export function requireApiMode(value: unknown): ApiMode {
  const mode = typeof value === 'string' ? value.trim() : ''
  if (mode === 'real' || mode === 'mock') return mode
  throw new Error('NUXT_PUBLIC_API_MODE must be explicitly set to "real" or "mock".')
}
