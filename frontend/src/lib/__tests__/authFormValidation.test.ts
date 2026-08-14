import { describe, expect, it, vi } from 'vitest'
import { BackendProblemError } from '@/lib/backendClient'
import { backendFieldErrors, focusFirstInvalidField } from '@/lib/authFormValidation'

describe('auth form validation', () => {
  it('focuses the first invalid field after Vue renders errors', async () => {
    const focus = vi.fn()
    const getElementById = vi.fn(() => ({ focus }))
    vi.stubGlobal('document', { getElementById })

    await focusFirstInvalidField(
      { email: '请输入有效的邮箱地址。', password: '请输入密码。' },
      [['email', 'reset-email'], ['password', 'reset-password']],
    )

    expect(getElementById).toHaveBeenCalledWith('reset-email')
    expect(focus).toHaveBeenCalledOnce()
    vi.unstubAllGlobals()
  })

  it('separates supported field errors from unknown backend fields', () => {
    const error = new BackendProblemError({
      type: 'about:blank',
      title: 'Validation failed',
      status: 422,
      code: 'VALIDATION_FAILED',
      detail: '请求字段不正确。',
      instance: '/api/v1/auth/password-reset/confirm',
      requestId: 'req-validation',
      errors: [
        { field: 'username', code: 'invalid', message: '用户名不正确。' },
        { field: 'futureField', code: 'invalid', message: '未知字段不正确。' },
      ],
    }, 422)

    expect(backendFieldErrors(error, { username: 'identifier' }, ['identifier'])).toEqual({
      fields: { identifier: '用户名不正确。' },
      hasUnmapped: true,
    })
  })
})
