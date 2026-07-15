import type { Carpool, PricingTier } from '@/data/mock'

export const fullCapacityTooltip = '按全部名额坐满后平均分摊计算，仅作参考，最终以结算时有效人数为准。'

export type PricingDisplay = {
  modeLabel: string
  primaryLabel: string
  primaryPrice: number
  secondaryLabel?: string
  secondaryPrice?: number
  detailSecondaryLabel?: string
  note: string
  nextTierLabel?: string
  nextTierPrice?: number
  currentMembers: number
  maxMembers: number
  remainingSeats: number
}

function roundPrice(value: number) {
  return Math.round(value)
}

function sortTiers(tiers: PricingTier[] = []) {
  return [...tiers].sort((a, b) => a.memberCount - b.memberCount)
}

export function getRemainingSeats(carpool: Carpool) {
  return Math.max(carpool.maxMembers - carpool.currentConfirmedMembers, 0)
}

function getTierForMembers(carpool: Carpool, membersAfterJoin: number) {
  const tiers = sortTiers(carpool.pricingTiers)
  return tiers.filter(tier => tier.memberCount <= membersAfterJoin).at(-1) ?? tiers[0]
}

function getNextTier(carpool: Carpool, membersAfterJoin: number) {
  return sortTiers(carpool.pricingTiers).find(tier => tier.memberCount > membersAfterJoin)
}

export function getCurrentPayablePrice(carpool: Carpool) {
  if (carpool.pricingMode === 'fixed') {
    return carpool.fixedMonthlyPrice ?? 0
  }

  if (carpool.pricingMode === 'equal_share') {
    const cost = carpool.totalShareableCost ?? 0
    return roundPrice(cost / Math.max(carpool.currentConfirmedMembers + 1, 1))
  }

  const tier = getTierForMembers(carpool, carpool.currentConfirmedMembers + 1)
  return tier?.price ?? 0
}

export function getFullCapacityPrice(carpool: Carpool) {
  if (carpool.pricingMode !== 'equal_share') return null
  const cost = carpool.totalShareableCost ?? 0
  return roundPrice(cost / Math.max(carpool.maxMembers, 1))
}

export function getPricingDisplay(carpool: Carpool): PricingDisplay {
  const currentMembers = carpool.currentConfirmedMembers
  const maxMembers = carpool.maxMembers
  const remainingSeats = getRemainingSeats(carpool)

  if (carpool.pricingMode === 'fixed') {
    return {
      modeLabel: '固定月费',
      primaryLabel: '固定',
      primaryPrice: getCurrentPayablePrice(carpool),
      note: '申请后价格快照不变',
      currentMembers,
      maxMembers,
      remainingSeats,
    }
  }

  if (carpool.pricingMode === 'equal_share') {
    return {
      modeLabel: '按人数均摊',
      primaryLabel: '现在加入约',
      primaryPrice: getCurrentPayablePrice(carpool),
      secondaryLabel: '满员后约',
      secondaryPrice: getFullCapacityPrice(carpool) ?? undefined,
      detailSecondaryLabel: '满员均摊价',
      note: '结算前随人数变化',
      currentMembers,
      maxMembers,
      remainingSeats,
    }
  }

  const membersAfterJoin = currentMembers + 1
  const nextTier = getNextTier(carpool, membersAfterJoin)
  const needed = nextTier ? Math.max(nextTier.memberCount - membersAfterJoin, 1) : undefined

  return {
    modeLabel: '阶梯价格',
    primaryLabel: '当前约',
    primaryPrice: getCurrentPayablePrice(carpool),
    note: nextTier ? '按当前人数对应阶梯计算' : '已到当前最高人数阶梯',
    nextTierLabel: nextTier ? `再增加 ${needed} 人后约` : undefined,
    nextTierPrice: nextTier?.price,
    currentMembers,
    maxMembers,
    remainingSeats,
  }
}

export function isCurrentTradable(carpool: Carpool) {
  return getRemainingSeats(carpool) > 0
    && carpool.confirmedWithin48h
    && carpool.linuxdoBound
    && carpool.sourcePostAccessible
    && !carpool.hasInfoConflict
    && !carpool.hasUnresolvedDispute
    && carpool.status === '可上车'
}

export function compareByTradablePrice(a: Carpool, b: Carpool) {
  return getCurrentPayablePrice(a) - getCurrentPayablePrice(b)
}
