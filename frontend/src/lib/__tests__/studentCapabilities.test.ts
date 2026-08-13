import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { afterEach, describe, test, vi } from 'vitest'
import { routes } from '@/router'
import {
  CAPABILITY,
  capabilityValues,
  hasCapability,
  normalizeCapabilities,
  projectCapabilities,
} from '@/lib/capabilities'
import { getMockIdentity, setMockPersona } from '@/lib/mockAuth'

const appShellSource = readFileSync(new URL('../../components/layout/AppShell.vue', import.meta.url), 'utf8')
const myCenterSource = readFileSync(new URL('../../pages/MyCenterPage.vue', import.meta.url), 'utf8')
const carpoolDetailSource = readFileSync(new URL('../../pages/CarpoolDetailPage.vue', import.meta.url), 'utf8')
const apiServiceDetailSource = readFileSync(new URL('../../pages/ApiServiceDetailPage.vue', import.meta.url), 'utf8')
const promotionBenefitsSource = readFileSync(new URL('../../pages/MyPromotionBenefitsPage.vue', import.meta.url), 'utf8')
const legacyApiIntentRedirectSource = readFileSync(new URL('../../pages/LegacyApiIntentRedirectPage.vue', import.meta.url), 'utf8')
const sessionActionsSource = readFileSync(new URL('../sessionActions.ts', import.meta.url), 'utf8')

afterEach(() => {
  setMockPersona('linuxdo')
  vi.restoreAllMocks()
})

describe('学生买家能力投影', () => {
  test('generated capability vocabulary and projector stay exact and sorted', () => {
    assert.deepEqual(capabilityValues, [
      'admin.access',
      'api_order.create',
      'api_probe.manage',
      'api_quota.publish',
      'api_service.publish',
      'carpool.apply',
      'carpool.publish',
    ])
    assert.deepEqual(projectCapabilities({ linuxDoBound: false, studentClaim: false, admin: false }), [])
    assert.deepEqual(projectCapabilities({ linuxDoBound: false, studentClaim: true, admin: false }), [CAPABILITY.apiOrderCreate])
    assert.deepEqual(projectCapabilities({ linuxDoBound: true, studentClaim: false, admin: false }), [
      CAPABILITY.apiOrderCreate,
      CAPABILITY.apiProbeManage,
      CAPABILITY.apiQuotaPublish,
      CAPABILITY.apiServicePublish,
      CAPABILITY.carpoolApply,
      CAPABILITY.carpoolPublish,
    ])
    assert.deepEqual(projectCapabilities({ linuxDoBound: true, studentClaim: true, admin: true }), capabilityValues)
    assert.deepEqual(normalizeCapabilities([
      CAPABILITY.carpoolPublish,
      'unknown.capability',
      CAPABILITY.adminAccess,
      CAPABILITY.carpoolPublish,
    ]), [CAPABILITY.adminAccess, CAPABILITY.carpoolPublish])
  })

  test('mock personas explicitly match anonymous, student, linux.do, and admin facts', () => {
    setMockPersona('anonymous')
    assert.equal(getMockIdentity(), null)

    setMockPersona('student')
    const student = getMockIdentity()
    assert.ok(student)
    assert.equal(student.linuxDoBinding.bound, false)
    assert.deepEqual(student.studentClaim, {
      institutionDomain: 'example.edu',
      institutionName: '示例大学',
      claimedAt: '2026-08-12T00:00:00Z',
    })
    assert.deepEqual(student.capabilities, [CAPABILITY.apiOrderCreate])
    assert.equal(hasCapability(student, CAPABILITY.carpoolApply), false)
    assert.equal(hasCapability(student, CAPABILITY.apiServicePublish), false)

    setMockPersona('linuxdo')
    const linuxdo = getMockIdentity()
    assert.ok(linuxdo)
    assert.equal(linuxdo.linuxDoBinding.bound, true)
    assert.equal(linuxdo.studentClaim, null)
    assert.equal(hasCapability(linuxdo, CAPABILITY.apiProbeManage), true)
    assert.equal(hasCapability(linuxdo, CAPABILITY.adminAccess), false)

    setMockPersona('admin')
    const admin = getMockIdentity()
    assert.ok(admin)
    assert.equal(hasCapability(admin, CAPABILITY.adminAccess), true)
  })

  test('routes declare exact seller, probe, and admin capabilities while buyer after-sales stays participant-driven', () => {
    const routeByPath = new Map(routes.map(route => [route.path, route]))
    const expected = new Map([
      ['/carpools/new', CAPABILITY.carpoolPublish],
      ['/api-market/new', CAPABILITY.apiServicePublish],
      ['/api-market/quota/new', CAPABILITY.apiQuotaPublish],
      ['/my/carpools', CAPABILITY.carpoolPublish],
      ['/my/api-services', CAPABILITY.apiServicePublish],
      ['/my/api-probe-connections', CAPABILITY.apiProbeManage],
      ['/merchant/carpool-applications', CAPABILITY.carpoolPublish],
      ['/merchant/api-orders', CAPABILITY.apiServicePublish],
      ['/admin', CAPABILITY.adminAccess],
      ['/admin/student-registration', CAPABILITY.adminAccess],
      ['/admin/logs', CAPABILITY.adminAccess],
    ])
    for (const [path, capability] of expected) {
      assert.equal(routeByPath.get(path)?.meta?.capability, capability, path)
    }
    for (const path of ['/my/rides', '/my/api-orders', '/tools/api-model-tester']) {
      assert.equal(routeByPath.get(path)?.meta?.capability, undefined, path)
    }
  })
})

test('menus, actions, and seller queries use capability facts without resource-count authority', () => {
  assert.match(appShellSource, /canManageApiProbe[\s\S]*CAPABILITY\.apiProbeManage/)
  assert.match(appShellSource, /canViewMerchantWorkspace[\s\S]*CAPABILITY\.carpoolPublish[\s\S]*CAPABILITY\.apiServicePublish[\s\S]*CAPABILITY\.apiProbeManage/)
  assert.match(appShellSource, /canViewAdminNav[\s\S]*CAPABILITY\.adminAccess/)
  assert.match(appShellSource, /canManageApiProbe\.value \? \[\{ label: '探针连接'/)
  assert.doesNotMatch(appShellSource, /hasMerchantWorkspace|ownedCarpools|ownedApiServices/)

  assert.match(myCenterSource, /useMyCarpools\(canPublishCarpool\)/)
  assert.match(myCenterSource, /useMyApiServices\('all', canPublishApiService\)/)
  assert.match(myCenterSource, /useApiPaymentAccountSettingsQuery\(canPublishApiService\)/)
  assert.match(myCenterSource, /useMerchantCarpoolApplications\([^\n]+, canPublishCarpool\)/)
  assert.match(myCenterSource, /useMerchantApiOrders\([^\n]+, canPublishApiService\)/)
  assert.match(carpoolDetailSource, /canApplyToCarpool[\s\S]*CAPABILITY\.carpoolApply/)
  assert.match(apiServiceDetailSource, /canCreateApiOrder[\s\S]*CAPABILITY\.apiOrderCreate/)
  assert.match(promotionBenefitsSource, /useMyApiServices\('all', canLoadOwnerServices\)/)
  assert.match(promotionBenefitsSource, /coupon\.status === 'available' && canPublishApiService/)
  assert.match(legacyApiIntentRedirectSource, /canReadMerchantOrders \? getMerchantApiOrders\(\) : Promise\.resolve\(\[\]\)/)
  assert.match(myCenterSource, /canPublishApiService\.value && apiServicesQuery\.isPending\.value/)
})

test('authenticated students keep notifications while publish actions stay capability-gated', () => {
  const notificationMenuStart = appShellSource.indexOf('<DropdownMenu v-if="isAuthenticated">')
  const publishMenuStart = appShellSource.indexOf('<DropdownMenu v-if="canPublishAnything">', notificationMenuStart)
  assert.notEqual(notificationMenuStart, -1)
  assert.notEqual(publishMenuStart, -1)

  const notificationMenu = appShellSource.slice(notificationMenuStart, publishMenuStart)
  assert.match(notificationMenu, /aria-label="打开通知"/)
  assert.match(notificationMenu, /<DropdownMenuItem as-child>\s*<RouterLink to="\/my\/notifications"/)
  assert.doesNotMatch(notificationMenu, /canPublishCarpool|canPublishApiService|canPublishAnything/)

  const anonymousPublishStart = appShellSource.indexOf('<DropdownMenu v-else-if="showLoginAction">', publishMenuStart)
  assert.notEqual(anonymousPublishStart, -1)
  const publishMenu = appShellSource.slice(publishMenuStart, anonymousPublishStart)
  assert.match(publishMenu, /v-if="canPublishCarpool"[\s\S]*to="\/carpools\/new"/)
  assert.match(publishMenu, /v-if="canPublishApiService"[\s\S]*to="\/api-market\/new"/)
})

test('shared logout clears session and authenticated query state', () => {
  assert.match(sessionActionsSource, /logoutBackendSession\(\)/)
  assert.match(sessionActionsSource, /queryClient\.cancelQueries\(\)/)
  assert.match(sessionActionsSource, /queryClient\.getMutationCache\(\)\.clear\(\)/)
  assert.match(sessionActionsSource, /queryClient\.clear\(\)/)
})
