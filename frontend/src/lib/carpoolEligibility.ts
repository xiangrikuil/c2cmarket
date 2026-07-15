import type { Carpool, CarpoolApplicationEligibility, CarpoolSeatSummary } from '@/data/mock'

export function hasCredentialSharingLanguage(value: string) {
  const hasRiskyCredentialText = /(共享|共用|转交|借用).*(账号|密码|主账号|session|cookie|token|登录态)|主账号|主 key|主key|session|cookie|refresh token|api key/i.test(value)
  const statesProhibition = /(不得|不能|不可|禁止|不允许|拒绝|避免|不保存|不交付|不提供|不会保存|不会交付|不会提供).{0,16}(共享|共用|转交|借用|填写|粘贴|上传|提供|交换|索要|账号|密码|主账号|session|cookie|token|登录态|api key)/i.test(value)
  return hasRiskyCredentialText && !statesProhibition
}

export function evaluateCarpoolApplicationEligibility(
  carpool: Carpool,
  seatSummary?: Pick<CarpoolSeatSummary, 'availableSeats'> | null,
  hasOngoingApplication = false,
  currentUserId = '',
  hasActiveMembership = false,
): CarpoolApplicationEligibility {
  const note = carpool.accessArrangementNote?.trim() ?? ''
  if (carpool.accessArrangementMode === 'not_allowed' || hasCredentialSharingLanguage(note)) {
    return { code: 'credential_risk', canApply: false, reason: '访问安排包含共享凭据风险，当前不能申请。', resolutionAction: 'wait_for_owner_correction' }
  }
  if (!note || (/chatgpt|openai/i.test(carpool.product) && !carpool.riskAcknowledged) || carpool.hasUnresolvedDispute) {
    return { code: 'owner_action_required', canApply: false, reason: '车源资料或风险声明需要车主修正。', resolutionAction: 'wait_for_owner_correction' }
  }
  if (!['可上车', '候补'].includes(carpool.status) && carpool.status !== '已满') {
    return { code: 'paused', canApply: false, reason: '车源当前暂停或尚未公开。', resolutionAction: 'browse_other_listings' }
  }
  if (currentUserId && carpool.ownerUserId === currentUserId) {
    return { code: 'self_owned', canApply: false, reason: '不能申请自己的车源。', resolutionAction: 'manage_own_listing' }
  }
  if (hasActiveMembership) {
    return { code: 'already_member', canApply: false, reason: '你已是该车源的成员。', resolutionAction: 'view_membership' }
  }
  if (hasOngoingApplication) {
    return { code: 'already_applied', canApply: false, reason: '你已有该车源的进行中申请。', resolutionAction: 'view_application' }
  }
  const availableSeats = seatSummary?.availableSeats ?? Math.max(0, carpool.maxMembers - carpool.currentConfirmedMembers)
  if (availableSeats < 1 || carpool.status === '已满') {
    return { code: 'sold_out', canApply: false, reason: '当前车源没有可申请名额。', resolutionAction: 'browse_other_listings' }
  }
  return { code: 'eligible', canApply: true, reason: '当前可申请上车。', resolutionAction: 'apply' }
}
