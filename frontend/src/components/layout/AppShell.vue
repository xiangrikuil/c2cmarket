<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch, type Component } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useQueryClient } from '@tanstack/vue-query'
import BadgeCheck from 'lucide-vue-next/dist/esm/icons/badge-check.js'
import Bell from 'lucide-vue-next/dist/esm/icons/bell.js'
import CarFront from 'lucide-vue-next/dist/esm/icons/car-front.js'
import Cable from 'lucide-vue-next/dist/esm/icons/cable.js'
import ChevronDown from 'lucide-vue-next/dist/esm/icons/chevron-down.js'
import CircleHelp from 'lucide-vue-next/dist/esm/icons/circle-question-mark.js'
import Code2 from 'lucide-vue-next/dist/esm/icons/code-xml.js'
import ExternalLink from 'lucide-vue-next/dist/esm/icons/external-link.js'
import FlaskConical from 'lucide-vue-next/dist/esm/icons/flask-conical.js'
import Home from 'lucide-vue-next/dist/esm/icons/house.js'
import LogIn from 'lucide-vue-next/dist/esm/icons/log-in.js'
import LogOut from 'lucide-vue-next/dist/esm/icons/log-out.js'
import Menu from 'lucide-vue-next/dist/esm/icons/menu.js'
import MessageSquarePlus from 'lucide-vue-next/dist/esm/icons/message-square-plus.js'
import PackageSearch from 'lucide-vue-next/dist/esm/icons/package-search.js'
import PanelLeftClose from 'lucide-vue-next/dist/esm/icons/panel-left-close.js'
import PanelLeftOpen from 'lucide-vue-next/dist/esm/icons/panel-left-open.js'
import Search from 'lucide-vue-next/dist/esm/icons/search.js'
import ShieldCheck from 'lucide-vue-next/dist/esm/icons/shield-check.js'
import ShoppingBag from 'lucide-vue-next/dist/esm/icons/shopping-bag.js'
import Star from 'lucide-vue-next/dist/esm/icons/star.js'
import Upload from 'lucide-vue-next/dist/esm/icons/upload.js'
import UserCog from 'lucide-vue-next/dist/esm/icons/user-cog.js'
import UserRound from 'lucide-vue-next/dist/esm/icons/user-round.js'
import X from 'lucide-vue-next/dist/esm/icons/x.js'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useMyContactMethodsQuery, useMyProfileQuery, useNotifications } from '@/queries/useAppShellQueries'
import { useNavigationBadges } from '@/queries/useRealtimeQueries'
import { useRealtimeSync } from '@/composables/useRealtimeSync'
import { ACCOUNT_RECOVERY_PATH, isAccountRecoveryComplete, shouldRedirectToAccountRecovery } from '@/lib/accountRecovery'
import { usePersistentSidebar } from '@/composables/usePersistentSidebar'
import { loginRoute } from '@/lib/authNavigation'
import { apiMarketViewFromQuery } from '@/lib/apiQuotaOfferUi'
import DevPersonaSwitcher from '@/components/layout/DevPersonaSwitcher.vue'
import { CAPABILITY, hasAnyCapability, hasCapability } from '@/lib/capabilities'
import { LIMITED_API_QUOTA_OFFERS_ENABLED } from '@/lib/featureFlags'
import { logoutCurrentSession } from '@/lib/sessionActions'
import type { WorkspaceNavKey } from '@/router'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()
const menuOpen = ref(false)
const logoutLoading = ref(false)
const { sidebarCollapsed } = usePersistentSidebar('c2c-user-sidebar-collapsed')
const searchText = ref('')
const { data: myProfile, isPending: profilePending } = useMyProfileQuery(import.meta.client)
const isAuthenticated = computed(() => Boolean(myProfile.value))
const authenticatedUserId = computed(() => myProfile.value?.id ?? '')
const contactMethodsQuery = useMyContactMethodsQuery(isAuthenticated, authenticatedUserId)
const wechatOnboardingOpen = ref(false)
const authResolved = computed(() => import.meta.client && !profilePending.value)
const showLoginAction = computed(() => authResolved.value && !isAuthenticated.value)
const { data: notifications } = useNotifications(isAuthenticated)
const { data: navigationBadges } = useNavigationBadges(computed(() => Boolean(myProfile.value)))
useRealtimeSync(computed(() => Boolean(myProfile.value)))

const buyerApiActionCount = computed(() => navigationBadges.value?.buyer.apiOrderActions ?? 0)
const merchantApiActionCount = computed(() => navigationBadges.value?.merchant.apiOrderActions ?? 0)
const buyerCarpoolActionCount = computed(() => navigationBadges.value?.buyer.carpoolActions ?? 0)
const ownerCarpoolActionCount = computed(() => navigationBadges.value?.merchant.carpoolActions ?? 0)
const unreadBusinessCount = computed(() => navigationBadges.value?.notificationUnread ?? 0)
const importantAnnouncementUnreadCount = computed(() => navigationBadges.value?.importantAnnouncementUnread ?? 0)
const messageCenterCount = computed(() => unreadBusinessCount.value + importantAnnouncementUnreadCount.value)
const supportActionCount = computed(() => navigationBadges.value?.supportActionCount ?? 0)
const currentUsername = computed(() => myProfile.value?.username ?? '')
const currentDisplayName = computed(() => myProfile.value?.displayName ?? myProfile.value?.username ?? '未登录')
const currentAvatarURL = computed(() => myProfile.value?.avatarUrl ?? '')
const currentAvatarText = computed(() => currentDisplayName.value.slice(0, 1).toUpperCase())
const canPublishCarpool = computed(() => hasCapability(myProfile.value, CAPABILITY.carpoolPublish))
const canPublishApiService = computed(() => hasCapability(myProfile.value, CAPABILITY.apiServicePublish))
const canManageApiProbe = computed(() => hasCapability(myProfile.value, CAPABILITY.apiProbeManage))
const canViewAdminNav = computed(() => hasCapability(myProfile.value, CAPABILITY.adminAccess))
const canPublishAnything = computed(() => canPublishCarpool.value || canPublishApiService.value)
const currentLoginTo = computed(() => loginRoute(route.fullPath))
const anonymousCarpoolPublishTo = loginRoute('/carpools/new')
const anonymousApiPublishTo = loginRoute('/api-market/new')
const accountRecoveryRequired = computed(() => myProfile.value ? !isAccountRecoveryComplete(myProfile.value) : false)

function wechatOnboardingStorageKey(userId: string) {
  return `c2cmarket.wechat-onboarding-dismissed.v1:${userId}`
}

function wechatOnboardingDismissed(userId: string) {
  try {
    return window.sessionStorage.getItem(wechatOnboardingStorageKey(userId)) === '1'
  } catch {
    return false
  }
}

function dismissWechatOnboarding() {
  const userId = myProfile.value?.id
  if (userId) {
    try {
      window.sessionStorage.setItem(wechatOnboardingStorageKey(userId), '1')
    } catch {
      // Browsing remains available when session storage is unavailable.
    }
  }
  wechatOnboardingOpen.value = false
}

function updateWechatOnboardingOpen(open: boolean) {
  if (!open) dismissWechatOnboarding()
}

async function configureWechat() {
  dismissWechatOnboarding()
  await router.push('/my/contacts')
}
const apiMarketNavItems = [
  ...(LIMITED_API_QUOTA_OFFERS_ENABLED ? [{ label: '限量额度包', view: 'limited' } as const] : []),
  { label: '短期流量包', view: 'packages' },
  { label: '自选额度', view: 'free' },
] as const
const canViewMerchantWorkspace = computed(() => hasAnyCapability(myProfile.value, [
  CAPABILITY.carpoolPublish,
  CAPABILITY.apiServicePublish,
  CAPABILITY.apiProbeManage,
]))

type NavigationGroup = {
  title: string
  items: Array<{ key: string, label: string, to: string, count: number | null, icon: Component, workspaceNavKey?: WorkspaceNavKey }>
}

const navGroups = computed(() => {
  const browseGroup = {
    title: '发现市场',
    items: [
      { key: 'home', label: '首页', to: '/', count: null, icon: Home },
      { key: 'carpools', label: '订阅拼车', to: '/carpools', count: null, icon: CarFront },
      { key: 'api-market', label: 'API 市场', to: '/api-market', count: null, icon: Code2 },
      { key: 'official-prices', label: '官网价格', to: '/official-prices', count: null, icon: ShieldCheck },
    ],
  }
  const publishGroup = {
    title: '发布入口',
    items: [
      ...(canPublishCarpool.value ? [{ key: 'publish-carpool', label: '发布车源', to: '/carpools/new', count: null, icon: CarFront }] : []),
      ...(canPublishApiService.value ? [{ key: 'publish-api-service', label: '发布 API 服务', to: '/api-market/new', count: null, icon: PackageSearch }] : []),
    ],
  }
  const userGroup = {
    title: '我的交易',
    items: [
      { key: 'my-rides', label: '我的上车', to: '/my/rides', count: buyerCarpoolActionCount.value, icon: CarFront },
      { key: 'my-api-orders', label: 'API 购买订单', to: '/my/api-orders', count: buyerApiActionCount.value, icon: ShoppingBag },
      { key: 'favorites', label: '收藏', to: '/my/favorites', count: null, icon: Star },
      { key: 'message-center', label: '消息中心', to: '/my/notifications', count: messageCenterCount.value, icon: Bell, workspaceNavKey: 'message-center' as const },
    ],
  }
  const merchantGroup = {
    title: '经营中心',
    items: [
      ...(canPublishCarpool.value ? [
        { key: 'carpool-management', label: '拼车管理', to: '/my/carpools', count: ownerCarpoolActionCount.value, icon: CarFront },
      ] : []),
      ...(canPublishApiService.value ? [
        { key: 'my-api-services', label: '我的 API 服务', to: '/my/api-services', count: null, icon: Code2 },
        { key: 'merchant-api-orders', label: 'API 销售订单', to: '/merchant/api-orders', count: merchantApiActionCount.value, icon: PackageSearch },
      ] : []),
      ...(canManageApiProbe.value ? [{ key: 'api-probe-connections', label: '探针连接', to: '/my/api-probe-connections', count: null, icon: Cable }] : []),
    ],
  }
  const toolsGroup = {
    title: '工具',
    items: [
      { key: 'api-model-tester', label: 'API 模型测试', to: '/tools/api-model-tester', count: null, icon: FlaskConical },
    ],
  }
  const accountGroup = {
    title: '账户',
    items: [
      { key: 'personal-center', label: '个人中心', to: '/my', count: null, icon: UserRound, workspaceNavKey: 'personal-center' as const },
      { key: 'account-settings', label: '账户设置', to: '/my/profile', count: null, icon: MessageSquarePlus, workspaceNavKey: 'account-settings' as const },
      { key: 'reputation-rights', label: '信誉与权益', to: '/my/reputation', count: null, icon: BadgeCheck, workspaceNavKey: 'reputation-rights' as const },
      { key: 'support-center', label: '支持中心', to: '/my/reports', count: supportActionCount.value, icon: CircleHelp, workspaceNavKey: 'support-center' as const },
    ],
  }
  const adminEntryGroup = {
    title: '管理',
    items: [{ key: 'admin', label: '进入管理台', to: '/admin', count: navigationBadges.value?.admin?.total ?? null, icon: UserCog }],
  }

  if (!isAuthenticated.value) return [browseGroup]

  const groups: NavigationGroup[] = [browseGroup]
  if (publishGroup.items.length > 0) groups.push(publishGroup)
  groups.push(userGroup)
  if (canViewMerchantWorkspace.value) groups.push(merchantGroup)
  groups.push(toolsGroup)
  groups.push(accountGroup)
  if (canViewAdminNav.value) groups.push(adminEntryGroup)
  return groups
})

const topNotifications = computed(() => (notifications.value ?? []).slice(0, 4))

const activeNavItem = computed(() => {
  return navGroups.value
    .flatMap(group => group.items)
    .filter(item => matchesRoute(item))
    .sort((a, b) => b.to.length - a.to.length)[0]
})

const currentTitle = computed(() => {
  if (route.path === '/api-market') {
    const currentView = apiMarketViewFromQuery(route.query.view)
    return `API 市场 / ${apiMarketNavItems.find(item => item.view === currentView)?.label ?? '自选额度'}`
  }
  return activeNavItem.value?.label ?? String(route.meta.title ?? 'C2CMarket')
})

function isActive(key: string) {
  return activeNavItem.value?.key === key
}

function matchesRoute(item: NavigationGroup['items'][number]) {
  const { to, workspaceNavKey } = item
  if (workspaceNavKey) return route.meta.workspaceNavKey === workspaceNavKey
  if (to === '/') return route.path === '/'
  return route.path === to || route.path.startsWith(`${to}/`)
}

function isApiMarketViewActive(view: typeof apiMarketNavItems[number]['view']) {
  if (route.path !== '/api-market') return false
  const currentView = apiMarketViewFromQuery(route.query.view)
  return currentView === view
}

watch(
  () => route.fullPath,
  () => {
    menuOpen.value = false
  },
)

watch(
  [isAuthenticated, contactMethodsQuery.isSuccess, contactMethodsQuery.data],
  ([authenticated, contactsResolved, methods]) => {
    const userId = myProfile.value?.id
    if (!authenticated || !contactsResolved || !userId) {
      wechatOnboardingOpen.value = false
      return
    }
    const hasWechat = (methods ?? []).some(method => method.enabled && method.type === 'wechat')
    wechatOnboardingOpen.value = !hasWechat && !wechatOnboardingDismissed(userId)
  },
  { immediate: true },
)

watch(
  () => [route.fullPath, accountRecoveryRequired.value] as const,
  () => {
    if (!accountRecoveryRequired.value || !shouldRedirectToAccountRecovery(route.path, route.meta.auth)) return
    router.replace({
      path: ACCOUNT_RECOVERY_PATH,
      query: { returnTo: route.fullPath },
    })
  },
  { immediate: true },
)

function runSearch() {
  const keyword = searchText.value.trim()
  router.push(keyword ? { path: '/search', query: { q: keyword } } : { path: '/search' })
  menuOpen.value = false
}

function formatBadgeCount(value: number | null | undefined) {
  const count = value ?? 0
  return count > 99 ? '99+' : count
}

async function logout() {
  if (logoutLoading.value) return
  logoutLoading.value = true
  try {
    await logoutCurrentSession(queryClient)
    toast.success('已退出登录。')
    await router.replace('/login')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '退出登录失败')
  } finally {
    logoutLoading.value = false
  }
}

function closeMenu() {
  menuOpen.value = false
}

function onNavigationKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') closeMenu()
}

onMounted(() => window.addEventListener('keydown', onNavigationKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onNavigationKeydown))
</script>

<template>
  <div
    class="min-h-screen bg-background lg:grid"
    :style="{ gridTemplateColumns: sidebarCollapsed ? '64px minmax(0, 1fr)' : '208px minmax(0, 1fr)' }"
  >
    <aside class="sticky top-[var(--global-announcement-height,0rem)] hidden h-[calc(100vh-var(--global-announcement-height,0rem))] overflow-hidden border-r border-sidebar-border bg-sidebar/95 text-sidebar-foreground backdrop-blur transition-[width] duration-200 lg:flex lg:flex-col">
      <RouterLink
        to="/"
        class="flex h-[60px] items-center border-b border-sidebar-border font-semibold tracking-tight"
        :class="sidebarCollapsed ? 'justify-center px-0' : 'gap-2.5 px-5'"
      >
        <img src="/c2cmarket-logo-mark.svg?v=20260806-deep-violet" alt="C2CMarket" class="h-7 w-7 shrink-0" />
        <span v-if="!sidebarCollapsed" class="min-w-0">
          <span class="block truncate text-[19px] font-bold leading-tight text-sidebar-foreground">C2CMarket</span>
        </span>
      </RouterLink>
      <nav class="c2c-sidebar-scroll min-h-0 flex-1 space-y-[26px] overflow-y-auto px-3 py-5">
        <section v-for="group in navGroups" :key="group.title">
          <h2
            class="flex h-5 items-center px-2 text-[12px] font-medium text-sidebar-foreground/60"
            :class="sidebarCollapsed ? 'justify-center px-0' : ''"
          >
            <span v-if="sidebarCollapsed" class="h-px w-6 bg-border"></span>
            <span v-else>{{ group.title }}</span>
          </h2>
          <div class="mt-2 grid gap-1">
            <div v-for="item in group.items" :key="item.key">
              <RouterLink
                :to="item.to"
                class="flex h-9 items-center rounded-md text-[14px] font-semibold text-sidebar-foreground/80 transition hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
                :title="sidebarCollapsed ? item.label : undefined"
                :class="isActive(item.key) ? 'bg-sidebar-accent text-sidebar-accent-foreground shadow-sm' : ''"
              >
                <span
                  class="flex min-w-0 items-center"
                  :class="sidebarCollapsed ? 'w-full justify-center' : 'gap-3 px-3'"
                >
                  <component :is="item.icon" class="h-[18px] w-[18px] shrink-0" />
                  <span v-if="!sidebarCollapsed" class="truncate">{{ item.label }}</span>
                </span>
                <Badge v-if="item.count && !sidebarCollapsed" variant="secondary" class="mr-2 h-5 px-1.5 text-[11px]">{{ formatBadgeCount(item.count) }}</Badge>
              </RouterLink>
              <div
                v-if="item.to === '/api-market' && matchesRoute(item) && !sidebarCollapsed"
                class="ml-5 mt-1 grid gap-0.5 border-l border-sidebar-border pl-3"
              >
                <RouterLink
                  v-for="child in apiMarketNavItems"
                  :key="child.view"
                  :to="{ path: '/api-market', query: { view: child.view } }"
                  class="flex h-8 items-center rounded-md px-2 text-[13px] font-medium text-sidebar-foreground/65 transition hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
                  :class="isApiMarketViewActive(child.view) ? 'bg-sidebar-accent text-sidebar-accent-foreground' : ''"
                >
                  {{ child.label }}
                </RouterLink>
              </div>
            </div>
          </div>
        </section>
      </nav>
      <div class="border-t border-sidebar-border p-2">
        <Button
          variant="ghost"
          size="sm"
          class="h-9 w-full justify-start gap-2 px-2 text-[13px] text-sidebar-foreground/65"
          :class="sidebarCollapsed ? 'justify-center px-0' : ''"
          :title="sidebarCollapsed ? '展开侧栏' : '收起侧栏'"
          @click="sidebarCollapsed = !sidebarCollapsed"
        >
          <PanelLeftOpen v-if="sidebarCollapsed" class="h-4 w-4" />
          <PanelLeftClose v-else class="h-4 w-4" />
          <span v-if="!sidebarCollapsed">收起侧栏</span>
        </Button>
      </div>
    </aside>

    <div
      v-if="menuOpen"
      class="fixed inset-0 z-40 bg-foreground/35 lg:hidden"
      @click="closeMenu"
    ></div>

    <div
      v-if="menuOpen"
      class="fixed inset-y-0 left-0 z-50 flex w-[min(336px,calc(100vw-48px))] flex-col border-r border-border bg-card shadow-xl lg:hidden"
      role="dialog"
      aria-modal="true"
      aria-label="移动端导航抽屉"
    >
      <div class="flex h-[60px] items-center justify-between border-b border-border px-4">
        <RouterLink to="/" class="flex min-w-0 items-center gap-3 font-semibold tracking-tight" @click="closeMenu">
          <img src="/c2cmarket-logo-mark.svg?v=20260806-deep-violet" alt="C2CMarket" class="h-8 w-8 shrink-0" />
          <span class="min-w-0">
            <span class="block truncate text-[18px] font-bold leading-tight">C2CMarket</span>
          </span>
        </RouterLink>
        <Button variant="ghost" size="icon" aria-label="关闭导航菜单" @click="closeMenu">
          <X class="h-4 w-4" />
        </Button>
      </div>
      <div class="border-b border-border px-4 py-3">
        <div class="flex gap-2">
          <Input v-model="searchText" name="mobile-global-search" aria-label="搜索产品、车主或线索" placeholder="搜索产品 / 车主 / 线索" @keyup.enter="runSearch" />
          <Button variant="outline" size="icon" aria-label="搜索" @click="runSearch"><Search class="h-4 w-4" /></Button>
        </div>
      </div>
      <nav class="flex-1 space-y-4 overflow-y-auto px-4 py-4">
        <section v-for="group in navGroups" :key="group.title">
          <h2 class="px-2 text-xs font-medium text-muted-foreground">{{ group.title }}</h2>
          <div class="mt-2 grid gap-1">
            <div v-for="item in group.items" :key="item.key">
              <RouterLink
                :to="item.to"
                class="flex items-center justify-between rounded-md px-3 py-2.5 text-sm font-medium text-sidebar-foreground/80 transition hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
                :class="isActive(item.key) ? 'bg-sidebar-accent text-sidebar-accent-foreground' : ''"
                @click="closeMenu"
              >
                <span class="flex min-w-0 items-center gap-2">
                  <component :is="item.icon" class="h-[18px] w-[18px] shrink-0" />
                  <span class="truncate">{{ item.label }}</span>
                </span>
                <Badge v-if="item.count" variant="secondary">{{ formatBadgeCount(item.count) }}</Badge>
              </RouterLink>
              <div v-if="item.to === '/api-market'" class="ml-6 mt-1 grid gap-0.5 border-l border-border pl-3">
                <RouterLink
                  v-for="child in apiMarketNavItems"
                  :key="child.view"
                  :to="{ path: '/api-market', query: { view: child.view } }"
                  class="rounded-md px-3 py-2 text-[13px] font-medium text-sidebar-foreground/65 transition hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
                  :class="isApiMarketViewActive(child.view) ? 'bg-sidebar-accent text-sidebar-accent-foreground' : ''"
                  @click="closeMenu"
                >
                  {{ child.label }}
                </RouterLink>
              </div>
            </div>
          </div>
        </section>
      </nav>
      <div v-if="showLoginAction" class="grid gap-2 border-t border-border p-4">
        <Button as-child class="w-full">
          <RouterLink :to="currentLoginTo" @click="closeMenu">
            <LogIn class="h-4 w-4" />登录
          </RouterLink>
        </Button>
        <Button as-child variant="outline" class="w-full">
          <RouterLink :to="anonymousCarpoolPublishTo" @click="closeMenu">
            <CarFront class="h-4 w-4" />登录后发布车源
          </RouterLink>
        </Button>
        <Button as-child variant="outline" class="w-full">
          <RouterLink :to="anonymousApiPublishTo" @click="closeMenu">
            <Code2 class="h-4 w-4" />登录后发布 API 服务
          </RouterLink>
        </Button>
      </div>
    </div>

    <div class="min-w-0">
      <header class="sticky top-[var(--global-announcement-height,0rem)] z-50 border-b border-border bg-card/88 backdrop-blur">
        <div class="flex h-[60px] items-center gap-4 px-4 sm:px-5 lg:px-5">
          <Button variant="ghost" size="icon" class="lg:hidden" aria-label="打开导航菜单" @click="menuOpen = true">
            <Menu class="h-4 w-4" />
          </Button>
          <RouterLink to="/" class="flex items-center gap-2 font-semibold tracking-tight lg:hidden">
            <img src="/c2cmarket-logo-mark.svg?v=20260806-deep-violet" alt="C2CMarket" class="h-8 w-8" />
          </RouterLink>
          <div class="hidden min-w-0 shrink-0 md:block lg:w-[260px] 2xl:w-[338px]">
            <div class="truncate text-lg font-semibold text-foreground">{{ currentTitle }}</div>
          </div>
          <div class="hidden w-[548px] max-w-[34vw] items-center md:flex 2xl:max-w-[40vw]">
            <div class="relative w-full">
              <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                v-model="searchText"
                name="global-search"
                class="h-9 rounded-md bg-background pl-9 pr-14 shadow-none"
                aria-label="搜索产品、车源、开通方式、地区或商户"
                placeholder="搜索产品、车源、API 服务"
                @keyup.enter="runSearch"
              />
              <Button class="absolute right-2 top-1/2 h-6 -translate-y-1/2 px-1.5 text-xs text-muted-foreground" type="button" size="sm" variant="outline" @click="runSearch">⌘ K</Button>
            </div>
          </div>
          <div class="flex-1" />
          <DevPersonaSwitcher :current-username="currentUsername" />
          <DropdownMenu v-if="isAuthenticated">
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" size="icon" class="relative text-muted-foreground" aria-label="打开通知">
                <Bell class="h-4 w-4" />
                <span v-if="unreadBusinessCount" class="absolute -right-0.5 -top-0.5 inline-flex h-4 min-w-[16px] items-center justify-center rounded-full bg-primary px-1 text-[10px] leading-none text-primary-foreground">{{ formatBadgeCount(unreadBusinessCount) }}</span>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="w-80">
              <DropdownMenuLabel>通知</DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem v-for="item in topNotifications" :key="item.id" as-child>
                <RouterLink :to="item.to" class="grid gap-1 whitespace-normal">
                  <span class="font-medium">{{ item.title }}</span>
                  <span class="text-xs text-muted-foreground">{{ item.detail }}</span>
                </RouterLink>
              </DropdownMenuItem>
              <DropdownMenuItem v-if="topNotifications.length === 0" class="text-xs text-muted-foreground">
                暂无通知
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem as-child>
                <RouterLink to="/my/notifications" class="justify-center font-medium">查看全部通知</RouterLink>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <DropdownMenu v-if="canPublishAnything">
            <DropdownMenuTrigger as-child>
              <Button size="sm" class="hidden md:inline-flex">
                发布
                <ChevronDown class="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="w-48">
              <DropdownMenuItem v-if="canPublishCarpool" as-child>
                <RouterLink to="/carpools/new" class="flex items-center gap-2">
                  <Upload class="h-4 w-4" />导入 / 发布车源
                </RouterLink>
              </DropdownMenuItem>
              <DropdownMenuItem v-if="canPublishApiService" as-child>
                <RouterLink to="/api-market/new" class="flex items-center gap-2">
                  <Code2 class="h-4 w-4" />发布 API 服务
                </RouterLink>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <DropdownMenu v-else-if="showLoginAction">
            <DropdownMenuTrigger as-child>
              <Button size="sm" class="hidden md:inline-flex">
                登录后发布
                <ChevronDown class="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="w-52">
              <DropdownMenuItem as-child>
                <RouterLink :to="anonymousCarpoolPublishTo" class="flex items-center gap-2">
                  <Upload class="h-4 w-4" />登录后发布车源
                </RouterLink>
              </DropdownMenuItem>
              <DropdownMenuItem as-child>
                <RouterLink :to="anonymousApiPublishTo" class="flex items-center gap-2">
                  <Code2 class="h-4 w-4" />登录后发布 API 服务
                </RouterLink>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <RouterLink v-if="showLoginAction" :to="currentLoginTo" class="hidden md:inline-flex">
            <Button variant="outline" size="sm">
              <LogIn class="h-4 w-4" />
              登录
            </Button>
          </RouterLink>
          <DropdownMenu v-else-if="isAuthenticated">
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" size="sm" class="hidden gap-2 text-foreground md:inline-flex">
                <span class="grid h-7 w-7 place-items-center overflow-hidden rounded-full bg-secondary text-[12px] text-secondary-foreground">
                  <img v-if="currentAvatarURL" :src="currentAvatarURL" alt="" class="h-full w-full object-cover" />
                  <span v-else>{{ currentAvatarText }}</span>
                </span>
                <span class="font-semibold">{{ currentDisplayName }}</span>
                <ChevronDown class="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="w-56">
              <DropdownMenuLabel>{{ currentDisplayName }}</DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem v-if="currentUsername" as-child>
                <RouterLink :to="`/u/${currentUsername}`" class="flex items-center gap-2">
                  <ExternalLink class="h-4 w-4" />查看公开主页
                </RouterLink>
              </DropdownMenuItem>
              <DropdownMenuItem as-child>
                <RouterLink to="/my" class="flex items-center gap-2">
                  <UserRound class="h-4 w-4" />个人中心
                </RouterLink>
              </DropdownMenuItem>
              <DropdownMenuItem as-child>
                <RouterLink to="/my/profile" class="flex items-center gap-2">
                  <ShieldCheck class="h-4 w-4" />账户设置
                </RouterLink>
              </DropdownMenuItem>
              <DropdownMenuItem as-child>
                <RouterLink :to="{ path: '/login', query: { returnTo: route.fullPath } }" class="flex items-center gap-2">
                  <LogIn class="h-4 w-4" />登录 / 绑定
                </RouterLink>
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem :disabled="logoutLoading" @click="logout">
                <LogOut class="h-4 w-4" />{{ logoutLoading ? '正在退出…' : '退出登录' }}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

      </header>

      <main
        class="w-full px-4 py-5 sm:px-5 lg:px-5"
        :class="route.path === '/api-market' ? 'bg-white' : ''"
      >
        <slot />
      </main>
    </div>
  </div>

  <Dialog :open="wechatOnboardingOpen" @update:open="updateWechatOnboardingOpen">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>配置微信联系方式</DialogTitle>
        <DialogDescription>拼车和 API 额度交易使用微信联系。配置后会自动用于所有交易角色，但不代表平台已验证该微信号。</DialogDescription>
      </DialogHeader>
      <DialogFooter class="gap-2 sm:gap-0">
        <Button type="button" variant="outline" @click="dismissWechatOnboarding">稍后填写</Button>
        <Button type="button" @click="configureWechat">
          <MessageSquarePlus class="h-4 w-4" />去配置微信
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
