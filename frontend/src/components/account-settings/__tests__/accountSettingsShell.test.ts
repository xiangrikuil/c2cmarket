import { createSSRApp, h } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it } from 'vitest'
import AccountSettingsShell from '../AccountSettingsShell.vue'

const settingPaths = ['/my/profile', '/my/contacts', '/my/account', '/my/privacy'] as const
const EmptyPage = { render: () => h('div') }

async function renderShell(path: typeof settingPaths[number], locked = false) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: settingPaths.map(settingPath => ({ path: settingPath, component: EmptyPage })),
  })
  await router.push(path)
  await router.isReady()

  const app = createSSRApp({
    render: () => h(AccountSettingsShell, {
      contactLabel: '联系方式',
      locked,
    }, { default: () => h('p', 'settings-content') }),
  })
  app.use(router)
  return renderToString(app)
}

describe('AccountSettingsShell', () => {
  for (const path of settingPaths) {
    it(`renders four native links and marks only ${path} active`, async () => {
      const html = await renderShell(path)
      const hrefs = [...html.matchAll(/<a[^>]*href="([^"]+)"/g)].map(match => match[1])
      const activeLink = html.match(/<a[^>]*aria-current="page"[^>]*>|<a[^>]*href="[^"]+"[^>]*aria-current="page"[^>]*>/)?.[0]

      expect(hrefs).toEqual(settingPaths)
      expect(activeLink).toContain(`href="${path}"`)
      expect((html.match(/aria-current="page"/g) ?? [])).toHaveLength(1)
    })
  }

  it('marks locked settings links while leaving account recovery reachable', async () => {
    const html = await renderShell('/my/account', true)
    const privacyLink = html.match(/<a[^>]*href="\/my\/privacy"[^>]*>/)?.[0]
    const accountLink = html.match(/<a[^>]*href="\/my\/account"[^>]*>/)?.[0]

    expect(privacyLink).toContain('aria-disabled="true"')
    expect(accountLink).not.toContain('aria-disabled')
  })
})
