import { existsSync, readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

function optionalSource(path: string) {
  const url = new URL(path, import.meta.url)
  return existsSync(url) ? readFileSync(url, 'utf8') : ''
}

const myCenter = optionalSource('../../../pages/MyCenterPage.vue')
const publishPage = optionalSource('../../../pages/ApiServicePublishPage.vue')
const quotaRushPublishPage = optionalSource('../../../pages/ApiQuotaRushPublishPage.vue')
const contactMethodCard = optionalSource('../ContactMethodCard.vue')
const paymentMethodCard = optionalSource('../PaymentMethodCard.vue')
const paymentSettingsEditor = optionalSource('../ApiPaymentSettingsEditor.vue')
const paymentSettingsDialog = optionalSource('../ApiPaymentSettingsDialog.vue')
const paymentSummary = optionalSource('../../api-service-publish/AccountPaymentSummarySection.vue')
const configurationProgressCard = optionalSource('../ConfigurationProgressCard.vue')
const buyerPreviewDrawer = optionalSource('../BuyerPreviewDrawer.vue')
const marketQueries = optionalSource('../../../queries/useMarketQueries.ts')

describe('联系方式与收款设置 UI', () => {
  it('个人中心复用共享收款编辑器并保留页面级未保存保护', () => {
    for (const component of [
      'ContactMethodCard',
      'ApiPaymentSettingsEditor',
      'ConfigurationProgressCard',
      'BuyerPreviewDrawer',
    ]) {
      expect(myCenter).toContain(`<${component}`)
    }

    expect(contactMethodCard).toContain('<Card')
    expect(contactMethodCard).toContain('<slot name="icon"')
    expect(paymentSettingsEditor).toContain('<PaymentMethodCard')
    expect(paymentMethodCard).toContain("defineEmits")
    expect(paymentMethodCard).not.toContain('useMutation')
    expect(configurationProgressCard).toContain("defineEmits")
    expect(buyerPreviewDrawer).toContain('<Dialog')
    expect(myCenter).not.toContain('function saveApiPaymentSettings')
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
    expect(paymentSettingsEditor).toContain("'dirty-change'")
    expect(paymentSettingsEditor).toContain('savedSnapshot')
    expect(paymentSettingsEditor).toContain('cloneApiPaymentAccountSettings')
    expect(myCenter).toContain('useUnsavedChangesGuard')
    expect(myCenter).toContain('contactSettingsDirty')
    expect(myCenter).toContain('apiPaymentEditorDirty')
    expect(myCenter).toContain('联系方式与收款设置尚未保存')
  })

  it('删除收款码经过确认且确认后只修改草稿', () => {
    expect(paymentSettingsEditor).toContain('pendingQrRemoval')
    expect(paymentSettingsEditor).toContain('confirmQrRemoval')
    expect(paymentSettingsEditor).toContain('删除收款码？')
    expect(paymentSettingsEditor).toContain('删除后需保存 API 收款设置才会生效')
    expect(paymentSettingsEditor).toContain("['image/png', 'image/jpeg', 'image/webp']")
    expect(paymentSettingsEditor).toContain('512 * 1024')
    expect(paymentSettingsEditor).toContain('FileReader')
  })

  it('共享编辑器集中校验并通过既有 mutation 保存', () => {
    expect(paymentSettingsEditor).toContain('isApiPaymentAccountSettingsComplete')
    expect(paymentSettingsEditor).toContain('containsSensitiveContent')
    expect(paymentSettingsEditor).toContain('useUpdateApiPaymentAccountSettingsMutation')
    expect(paymentSettingsEditor).toContain('onSuccess: settings =>')
    expect(paymentSettingsEditor).toContain("emit('saved', settings)")
    expect(paymentSettingsEditor).toContain('onError: error => toast.error')
    expect(marketQueries).toContain('queryClient.setQueryData(apiPaymentAccountSettingsQueryKey(), data)')
    expect(publishPage).toMatch(/watch\(accountPaymentSettingsValue,[\s\S]*?form\.paymentOptions = settings\.paymentOptions\.map/)
  })

  it('发布弹窗每次打开新草稿并统一确认未保存关闭', () => {
    expect(paymentSettingsDialog).toContain(':key="editorSession"')
    expect(paymentSettingsDialog).toContain('v-if="open"')
    expect(paymentSettingsDialog).toContain('function requestClose()')
    expect(paymentSettingsDialog).toContain('discardConfirmationOpen.value = true')
    expect(paymentSettingsDialog).toContain('放弃未保存的修改？')
    expect(paymentSettingsDialog).toContain('放弃修改')
    expect(paymentSettingsDialog).toMatch(/function handleSaved[\s\S]*?emit\('update:open', false\)/)
    expect(paymentSettingsDialog).toContain('max-h-[calc(100dvh-1rem)]')
    expect(paymentSettingsDialog).toContain('overflow-y-auto')
  })

  it('发布页从摘要直接打开唯一弹窗且不离开当前路由', () => {
    expect(paymentSummary).toContain('defineEmits')
    expect(paymentSummary).toContain("emit('edit')")
    expect(paymentSummary).not.toContain('RouterLink')
    expect(paymentSummary).not.toContain('/my/contacts')
    expect(paymentSummary).not.toContain('使用个人中心设置')
    expect(publishPage).not.toContain('先到个人中心配置 API 收款设置')
    expect(quotaRushPublishPage).not.toContain('先到个人中心完成 API 收款设置')
    expect(publishPage.match(/<ApiPaymentSettingsDialog/g)).toHaveLength(1)
    expect(publishPage).toContain('v-model:open="paymentSettingsDialogOpen"')
    expect(publishPage).toContain('@edit="paymentSettingsDialogOpen = true"')
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
