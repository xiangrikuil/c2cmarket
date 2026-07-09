export const APP_THEME_STORAGE_KEY = 'c2cmarket-theme'

export const appThemes = [
  {
    value: 'minimal-modern',
    label: '极致电蓝',
    swatch: '#0052FF',
  },
] as const

export type AppTheme = typeof appThemes[number]['value']

export const DEFAULT_APP_THEME: AppTheme = 'minimal-modern'

export function isAppTheme(value: string | null): value is AppTheme {
  return appThemes.some(theme => theme.value === value)
}

export function getInitialAppTheme(): AppTheme {
  const storedTheme = window.localStorage.getItem(APP_THEME_STORAGE_KEY)
  return isAppTheme(storedTheme) ? storedTheme : DEFAULT_APP_THEME
}

export function applyAppTheme(theme: AppTheme) {
  document.documentElement.dataset.theme = theme
  window.localStorage.setItem(APP_THEME_STORAGE_KEY, theme)
}

export function initializeAppTheme() {
  applyAppTheme(getInitialAppTheme())
}
