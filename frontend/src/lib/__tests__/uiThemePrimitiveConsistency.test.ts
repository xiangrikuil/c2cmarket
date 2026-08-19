import { globSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

function source(path: string) {
  return readFileSync(new URL(path, import.meta.url), 'utf8')
}

const srcDirectory = fileURLToPath(new URL('../..', import.meta.url))
const productVueFiles = globSync(['pages/**/*.vue', 'components/**/*.vue'], { cwd: srcDirectory })
const minimalTheme = source('../../theme/minimal-modern.css')
const aquaTheme = source('../../theme/aqua-console.css')
const appThemes = source('../../theme/appThemes.ts')
const styles = source('../../styles.css')
const login = source('../../components/auth/LoginPanel.vue')
const logo = source('../../../public/c2cmarket-logo-mark.svg')

describe('深蓝紫主题与基础控件一致性', () => {
  it('从 CSS 默认值和 SSR 主题共同使用深蓝紫品牌 Token', () => {
    expect(minimalTheme).toMatch(/:root,\s*\nhtml\[data-theme="minimal-modern"\]/)
    expect(minimalTheme).toContain('--primary: #5b4fe9')
    expect(minimalTheme).toContain('--primary-hover: #493bcb')
    expect(minimalTheme).toContain('--accent: #f0eeff')
    expect(minimalTheme).toContain('--success: #15803d')
    expect(minimalTheme).toContain('--warning: #b45309')
    expect(minimalTheme).toContain('--destructive: #dc2626')
    expect(minimalTheme).toContain('--info: #0369a1')
    expect(appThemes).toContain("label: '深蓝紫'")
    expect(appThemes).toContain("swatch: '#5B4FE9'")
  })

  it('旧 Aqua 主题只在显式主题作用域下生效', () => {
    expect(aquaTheme).not.toMatch(/(^|\n):root\b/)
    expect(aquaTheme).not.toMatch(/(^|\n)\.dark\s*,/)
    expect(aquaTheme).toContain('html[data-theme="aqua-console"]')
    expect(appThemes).not.toContain("value: 'aqua-console'")
  })

  it('主按钮保持纯色且没有旧电蓝外发光', () => {
    expect(styles).toMatch(/\[data-slot="button"\]\[data-variant="default"\][\s\S]*?background-color: var\(--primary\)/)
    expect(styles).toMatch(/\[data-slot="button"\]\[data-variant="default"\][\s\S]*?background-image: none/)
    expect(styles).not.toContain('rgb(0 82 255')
  })

  it('业务页面与组件不再直接维护原生交互控件', () => {
    for (const file of productVueFiles) {
      const content = readFileSync(`${srcDirectory}/${file}`, 'utf8')
      expect(content, file).not.toMatch(/<button\b|<select\b|<option\b|type=["'](?:checkbox|radio)["']/)
    }
  })

  it('使用统一语义图标并把品牌 SVG 放到独立资产', () => {
    for (const file of productVueFiles) {
      const content = readFileSync(`${srcDirectory}/${file}`, 'utf8')
      expect(content, file).not.toMatch(/\b(?:AlertTriangle|CircleAlert)\b/)
    }
    expect(login).not.toContain('<svg')
    expect(login).toContain('src="/linuxdo-mark.svg"')
    expect(logo).toContain('#5B4FE9')
    expect(logo).toContain('#776CF0')
  })
})
