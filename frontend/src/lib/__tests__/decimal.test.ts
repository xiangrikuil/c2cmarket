import { describe, expect, it } from 'vitest'
import {
  compareDecimal,
  divideDecimal,
  formatDecimal,
  multiplyDecimal,
  normalizeDecimalTrimmed,
} from '@/lib/decimal'

describe('十进制金额与美元额度', () => {
  it('稳定计算 ¥10 / ¥0.80 = $12.50', () => {
    const allowance = divideDecimal('10.00', '0.8000', 6)
    expect(allowance).toBe('12.500000')
    expect(formatDecimal(allowance, 2, 6)).toBe('12.50')
  })

  it('提交值与后端两位人民币校验保持一致', () => {
    const allowance = normalizeDecimalTrimmed(divideDecimal('10.01', '0.80', 6), 6)
    expect(allowance).toBe('12.5125')
    expect(multiplyDecimal(allowance, '0.80', 2)).toBe('10.01')
  })

  it('比较和格式化不会经过二进制浮点数', () => {
    expect(compareDecimal('12.500000', '12.5')).toBe(0)
    expect(formatDecimal('12345.500000', 2, 6)).toBe('12,345.50')
  })
})
