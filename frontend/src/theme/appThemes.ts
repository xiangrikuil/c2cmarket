export const APP_THEME_STORAGE_KEY = 'c2cmarket-theme'

export const appThemes = [
  {
    value: 'aqua-console',
    label: '水色控制台',
    swatch: 'oklch(0.704 0.123 182.5)',
  },
  {
    value: 'neumorphic-cool',
    label: '柔灰浮雕',
    swatch: '#E0E5EC',
  },
  {
    value: 'minimal-modern',
    label: '极简电蓝',
    swatch: '#0052FF',
  },
] as const

export type AppTheme = typeof appThemes[number]['value']

export const DEFAULT_APP_THEME: AppTheme = 'aqua-console'

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
