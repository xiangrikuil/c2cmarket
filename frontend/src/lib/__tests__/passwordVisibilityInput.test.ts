import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const passwordInputSource = readFileSync(new URL('../../components/personal-center/PasswordVisibilityInput.vue', import.meta.url), 'utf8')
const myCenterSource = readFileSync(new URL('../../pages/MyCenterPage.vue', import.meta.url), 'utf8')

describe('备用密码显隐控件', () => {
  it('每个组件实例使用独立状态切换密码字段', () => {
    expect(passwordInputSource).toContain('const passwordVisible = ref(false)')
    expect(passwordInputSource).toContain(":type=\"passwordVisible ? 'text' : 'password'\"")
    expect(passwordInputSource).toContain('@click="passwordVisible = !passwordVisible"')
    expect(passwordInputSource).toContain('type="button"')
    expect(passwordInputSource).toContain(':aria-label="passwordVisible ? `隐藏${label}` : `显示${label}`"')
    expect(passwordInputSource).toContain(':aria-pressed="passwordVisible"')
    expect(passwordInputSource).toContain('<EyeOff v-if="passwordVisible"')
    expect(passwordInputSource).toContain('<Eye v-else')
  })

  it('传递错误描述关联并覆盖暗色错误背景', () => {
    expect(passwordInputSource).toContain(":aria-invalid=\"invalid ? 'true' : undefined\"")
    expect(passwordInputSource).toContain(':aria-describedby="describedBy"')
    expect(passwordInputSource).toContain('dark:bg-destructive/10')
  })

  it('为备用密码流程的三个密码字段统一启用显隐控件', () => {
    expect(myCenterSource.match(/<PasswordVisibilityInput/g)).toHaveLength(3)
    expect(myCenterSource).toContain('id="account-current-password"')
    expect(myCenterSource).toContain('id="account-new-password"')
    expect(myCenterSource).toContain('id="account-confirm-password"')
    expect(myCenterSource).toContain(":described-by=\"confirmPasswordMismatch ? 'account-confirm-password-error' : undefined\"")
    expect(myCenterSource).toContain('id="account-confirm-password-error"')
  })
})
