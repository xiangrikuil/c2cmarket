import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

function source(path: string) {
  return readFileSync(new URL(path, import.meta.url), 'utf8')
}

describe('交易评价交互', () => {
  const dialog = source('../../components/review/ReviewDialog.vue')
  const ratingInput = source('../../components/review/StarRatingInput.vue')
  const reviewCenter = source('../../pages/MyReviewsPage.vue')
  const carpoolDetail = source('../../pages/CarpoolApplicationDetailPage.vue')
  const apiOrderDetail = source('../../pages/ApiPurchaseOrderDetailPage.vue')
  const publicUser = source('../../pages/PublicUserPage.vue')
  const reputationCard = source('../../components/reputation/ReputationSummaryCard.vue')
  const reputationProgress = source('../../components/reputation/ReputationProgressList.vue')

  it('新评价默认不选星且标签或说明至少填写一种', () => {
    expect(dialog).toContain("rating: null as number | null")
    expect(dialog).toContain("form.rating !== null && (form.tags.length > 0 || form.note.trim().length > 0)")
    expect(dialog).toContain('v-for="tag in row.allowedTags"')
    expect(dialog).toContain('失败')
  })

  it('五星输入支持键盘和无障碍语义', () => {
    expect(ratingInput).toContain('role="radiogroup"')
    expect(ratingInput).toContain('role="radio"')
    expect(ratingInput).toContain("event.key === 'ArrowRight'")
    expect(ratingInput).toContain("event.key === 'Home'")
    expect(ratingInput).toContain(':aria-label="`${value} 分`"')
  })

  it('详情页使用 review 查询参数恢复弹框并保留其他查询参数', () => {
    for (const page of [carpoolDetail, apiOrderDetail]) {
      expect(page).toContain("route.query.review === 'open'")
      expect(page).toContain("query: { ...route.query, review: 'open' }")
      expect(page).toContain('delete query.review')
      expect(page).toContain('<ReviewDialog')
      expect(page).not.toContain("path: '/my/reviews'")
    }
    expect(reviewCenter).toContain('<ReviewDialog')
    expect(reviewCenter).toContain('const selectedRow = computed')
    expect(reviewCenter).toContain('router.push({ query:')
    expect(reviewCenter).toContain('reviewDirection: row.direction')
    expect(reviewCenter).toContain("item.direction === 'pending'")
    expect(reviewCenter).not.toContain('双盲评价')
  })

  it('公开页展示原始评价摘要与逐条评分，信誉卡不展示修正评分', () => {
    expect(publicUser).toContain('reviewSummary.average.toFixed(1)')
    expect(publicUser).toContain('reviewSummary.distribution')
    expect(publicUser).toContain('<StarRatingDisplay')
    expect(publicUser).toContain('来自平台内已完成交易')
    expect(publicUser).not.toContain('已验证交易')
    expect(reputationCard).not.toContain('修正评分')
    expect(reputationCard).not.toContain('weightedRating')
    expect(reputationProgress).not.toContain('已验证交易')
  })
})
