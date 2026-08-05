import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

describe('API promotion architecture', () => {
  it('wires administrator preflight facts into the create dialog', () => {
    const backend = readFileSync(new URL('../apiMarketBackend.ts', import.meta.url), 'utf8')
    const queries = readFileSync(new URL('../../queries/useMarketQueries.ts', import.meta.url), 'utf8')
    const page = readFileSync(new URL('../../pages/AdminApiPromotionsPage.vue', import.meta.url), 'utf8')

    expect(backend).toContain('/api/v1/admin/api-service-promotions/availability?')
    expect(queries).toContain('useAdminApiPromotionAvailability')
    expect(page).toContain('availability.eligibility.configurable')
    expect(page).toContain('availability.eligibility.displayable')
    expect(page).toContain('availability.eligibility.warningReasons')
    expect(page).toContain('availability.eligibility.suppressionReasons')
    expect(page).toContain('availability.overlappingCampaigns')
    expect(page).toContain('availability.remainingCapacity')
    expect(page).toContain('availability.sameServiceOverlap')
    expect(page).toContain('apiPromotionAvailabilityBlockReasons')
  })

  it('maps the generated public DTO without a double assertion', () => {
    const backend = readFileSync(new URL('../apiMarketBackend.ts', import.meta.url), 'utf8')

    expect(backend).toContain('function mapBackendPublicAPIService(service: PublicApiService)')
    expect(backend).not.toContain('item.service as unknown as BackendAPIService')
  })

  it('injects promotion only into the free and fixed-package grids', () => {
    const page = readFileSync(new URL('../../pages/ApiMarketPage.vue', import.meta.url), 'utf8')

    expect(page).not.toContain('ApiPromotionSection')
    expect(page).toContain('freeServiceDisplayRows')
    expect(page).toContain('packageDisplayRows')
    expect(page).toContain('placePromotions')
    expect(page).toContain('@activate="trackPromotedCardClick(entry.promotion, entry.promotionPosition)"')
    expect(page).not.toContain('@click.capture="trackPromotedCardClick(entry.promotion)"')
    expect(page).not.toMatch(/quotaRows[\s\S]{0,500}placePromotions/)
  })

  it('keeps a compact promotion label without the long disclaimer', () => {
    const freeCard = readFileSync(new URL('../../components/api-market/ApiFreeServiceCard.vue', import.meta.url), 'utf8')
    const packageCard = readFileSync(new URL('../../components/api-market/ApiPackageCard.vue', import.meta.url), 'utf8')
    const styles = readFileSync(new URL('../../styles.css', import.meta.url), 'utf8')

    expect(freeCard).toContain('<Badge v-if="promoted" variant="status"><Megaphone class="h-3 w-3" />推广</Badge>')
    expect(packageCard).toContain('<Badge v-if="promoted" variant="status"><Megaphone class="h-3 w-3" />推广</Badge>')
    expect(freeCard).not.toContain('商业推广，不代表平台质量认证或信誉背书')
    expect(packageCard).not.toContain('商业推广，不代表平台质量认证或信誉背书')
    expect(freeCard).toContain('api-product-card')
    expect(packageCard).toContain('api-product-card')
    expect(styles).toMatch(/\.api-product-card \{[\s\S]*height: 100%;[\s\S]*min-height: 0;/)
    expect(freeCard).not.toContain('api-free-service-card__promotion-note')
    expect(packageCard).not.toContain('api-package-card__promotion-note')
  })
})
