import { createSSRApp, h } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it } from 'vitest'
import WorkspaceSectionTabs from '../WorkspaceSectionTabs.vue'

const EmptyPage = { render: () => h('div') }
const routes = [
  '/my/reputation',
  '/my/promotion-benefits',
  '/my/reports',
  '/my/reports/:kind/:id',
  '/my/feedback',
  '/my/feedback/:id',
].map(path => ({ path, component: EmptyPage }))

async function renderTabs(path: string, section: 'reputation-rights' | 'support-center', promotionEnabled?: boolean) {
  const router = createRouter({ history: createMemoryHistory(), routes })
  await router.push(path)
  await router.isReady()
  const app = createSSRApp({ render: () => h(WorkspaceSectionTabs, { section, promotionEnabled }) })
  app.use(router)
  return renderToString(app)
}

describe('WorkspaceSectionTabs', () => {
  it('keeps reputation and promotion as native deep links', async () => {
    const html = await renderTabs('/my/promotion-benefits', 'reputation-rights')

    expect(html).toContain('href="/my/reputation"')
    expect(html).toContain('href="/my/promotion-benefits"')
    expect(html).toContain('推广权益')
    expect((html.match(/data-state="active"/g) ?? [])).toHaveLength(1)
  })

  it('keeps support detail routes under the report tab', async () => {
    const html = await renderTabs('/my/reports/report/report-1', 'support-center')

    expect(html).toContain('href="/my/reports"')
    expect(html).toContain('href="/my/feedback"')
    expect(html).toContain('举报与申诉')
    expect((html.match(/data-state="active"/g) ?? [])).toHaveLength(1)
  })

  it('hides promotion benefits when the program is disabled', async () => {
    const html = await renderTabs('/my/reputation', 'reputation-rights', false)

    expect(html).toContain('href="/my/reputation"')
    expect(html).not.toContain('href="/my/promotion-benefits"')
    expect(html).not.toContain('推广权益')
    expect((html.match(/data-state="active"/g) ?? [])).toHaveLength(1)
  })
})
