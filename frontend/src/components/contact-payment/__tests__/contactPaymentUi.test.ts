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
const transactionContactSelector = optionalSource('../TransactionContactSelector.vue')
const merchantContactMethodsSection = optionalSource('../../api-service-publish/MerchantContactMethodsSection.vue')
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

  it('微信和支付宝使用互斥单选，未选项仍保留已有资料', () => {
    expect(paymentSettingsEditor).toContain('<RadioGroup')
    expect(paymentSettingsEditor).toContain('selectPaymentMethod')
    expect(paymentMethodCard).toContain('v-if="!option.enabled"')
    expect(paymentMethodCard).toContain('<RadioGroupItem')
    expect(paymentMethodCard).toContain('选择')
    expect(paymentMethodCard).toContain('v-else')
    expect(paymentMethodCard).toContain('paymentQrCodeDataUrl')
    expect(paymentMethodCard).toContain('request-remove-qr')
    expect(paymentMethodCard).not.toContain('<Checkbox')
  })

  it('个人中心头像来源使用 shadcn-vue RadioGroup', () => {
    expect(myCenter).toContain("import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'")
    expect(myCenter).toContain('<RadioGroup v-model="profileForm.avatarMode"')
    expect(myCenter).toContain('id="avatar-mode-linuxdo" value="linuxdo"')
    expect(myCenter).toContain('id="avatar-mode-custom-url" value="custom_url"')
    expect(myCenter).not.toContain('type="radio"')
  })

  it('交易表单显式选择已验证邮箱或启用微信', () => {
    expect(transactionContactSelector).toContain('isTransactionContactEligible')
    expect(transactionContactSelector).toContain('<RadioGroup')
    expect(transactionContactSelector).toContain('使用已验证账号邮箱')
    expect(transactionContactSelector).toContain('账号安全通知邮箱不会自动公开')
    expect(merchantContactMethodsSection).toContain('<TransactionContactSelector')
    expect(merchantContactMethodsSection).toContain('v-model="form.ownerContactMethodId"')
    expect(publishPage).toContain('<MerchantContactMethodsSection')
    expect(quotaRushPublishPage).toContain('<MerchantContactMethodsSection')
    expect(myCenter).not.toContain('ContactUsageScopeSelector')
    expect(myCenter).not.toContain('usageScopes')
  })

  it('账号恢复邮箱与交易联系邮箱使用独立草稿和挑战状态', () => {
    expect(myCenter).toContain('const accountEmailForm = reactive')
    expect(myCenter).toContain('const contactEmailForm = reactive')
    expect(myCenter).toContain('accountEmailForm.email = currentProfile.email ||')
    expect(myCenter).toContain("contactEmailForm.email = contact?.displayValue ?? ''")
    expect(myCenter).toContain('contactEmailVerificationChallengeEmail')
    expect(myCenter).not.toContain('email?.displayValue || profile.value.email')
  })

  it('显示未保存状态并在离开页面前保护草稿', () => {
    expect(contactMethodCard).toContain('有未保存更改')
    expect(paymentMethodCard).toContain('有未保存更改')
    expect(paymentSettingsEditor).toContain("'dirty-change'")
    expect(paymentSettingsEditor).toContain('savedSnapshot')
    expect(paymentSettingsEditor).toContain('cloneApiPaymentAccountSettings')
    expect(myCenter).toContain('useUnsavedChangesGuard')
    expect(myCenter).toContain('currentSettingsDirty')
    expect(myCenter).toContain('apiPaymentEditorDirty')
    expect(myCenter).toContain('|| Boolean(contactEmailVerificationChallengeEmail.value)')
    expect(myCenter).toContain('当前账户设置尚未保存')
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
    expect(paymentSummary.match(/v-for="option in settings\.paymentOptions"/g)).toBeNull()
    expect(paymentSummary).toContain('enabledOption')
  })

  it('只保留一个完成度卡并通过弹窗预览已保存资料', () => {
    expect(configurationProgressCard).toContain('配置完成度')
    expect(configurationProgressCard).toContain('交易联系方式')
    expect(configurationProgressCard).not.toContain("label: '微信'")
    expect(configurationProgressCard).toContain('预览买家看到的信息')
    expect(configurationProgressCard).toContain('仅在有效联系窗口或订单中展示，不会出现在公开主页')
    expect(myCenter).toContain('交易时可选择已验证邮箱或微信，账号邮箱不会自动公开。')
    expect(myCenter).not.toContain('微信用于订单沟通，已绑定时会自动跳过。')
    expect(buyerPreviewDrawer).toContain('未保存更改不在此预览中')
    expect(buyerPreviewDrawer).toContain('有效联系窗口')
    expect(buyerPreviewDrawer).toContain('API 订单')
    expect(myCenter).not.toContain('资料安全提醒')
    expect(myCenter).not.toContain('参与方看到的资料卡')
  })
})
