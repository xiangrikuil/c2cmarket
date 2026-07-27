import { existsSync, readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

function optionalSource(path: string) {
  const url = new URL(path, import.meta.url)
  return existsSync(url) ? readFileSync(url, 'utf8') : ''
}

const myCenter = optionalSource('../../../pages/MyCenterPage.vue')
const contactMethodCard = optionalSource('../ContactMethodCard.vue')
const paymentMethodCard = optionalSource('../PaymentMethodCard.vue')
const configurationProgressCard = optionalSource('../ConfigurationProgressCard.vue')
const buyerPreviewDrawer = optionalSource('../BuyerPreviewDrawer.vue')

describe('联系方式与收款设置 UI', () => {
  it('把页面拆成四个展示型业务组件并保留页面状态所有权', () => {
    for (const component of [
      'ContactMethodCard',
      'PaymentMethodCard',
      'ConfigurationProgressCard',
      'BuyerPreviewDrawer',
    ]) {
      expect(myCenter).toContain(`<${component}`)
    }

    expect(contactMethodCard).toContain('<Card')
    expect(contactMethodCard).toContain('<slot name="icon"')
    expect(paymentMethodCard).toContain("defineEmits")
    expect(paymentMethodCard).not.toContain('useMutation')
    expect(configurationProgressCard).toContain("defineEmits")
    expect(buyerPreviewDrawer).toContain('<Dialog')
  })

  it('未启用的支付方式保持单行，启用后才展示编辑内容', () => {
    expect(paymentMethodCard).toContain('v-if="!option.enabled"')
    expect(paymentMethodCard).toContain('启用配置')
    expect(paymentMethodCard).toContain('v-else')
    expect(paymentMethodCard).toContain('paymentQrCodeDataUrl')
    expect(paymentMethodCard).toContain('request-remove-qr')
  })

  it('显示未保存状态并在离开页面前保护草稿', () => {
    expect(contactMethodCard).toContain('有未保存更改')
    expect(paymentMethodCard).toContain('有未保存更改')
    expect(myCenter).toContain('useUnsavedChangesGuard')
    expect(myCenter).toContain('contactSettingsDirty')
    expect(myCenter).toContain('联系方式与收款设置尚未保存')
  })

  it('删除收款码经过确认且确认后只修改草稿', () => {
    expect(myCenter).toContain('requestApiPaymentQrRemoval')
    expect(myCenter).toContain('confirmApiPaymentQrRemoval')
    expect(myCenter).toContain('删除收款码？')
    expect(myCenter).toContain('删除后需保存 API 收款设置才会生效')
  })

  it('只保留一个完成度卡并通过弹窗预览已保存资料', () => {
    expect(configurationProgressCard).toContain('配置完成度')
    expect(configurationProgressCard).toContain('预览买家看到的信息')
    expect(configurationProgressCard).toContain('仅在有效联系窗口或订单中展示，不会出现在公开主页')
    expect(buyerPreviewDrawer).toContain('未保存更改不在此预览中')
    expect(buyerPreviewDrawer).toContain('有效联系窗口')
    expect(buyerPreviewDrawer).toContain('API 订单')
    expect(myCenter).not.toContain('资料安全提醒')
    expect(myCenter).not.toContain('参与方看到的资料卡')
  })
})
