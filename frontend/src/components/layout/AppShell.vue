<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import {
  Bell,
  ChevronDown,
  CircleHelp,
  Code2,
  Car,
  ExternalLink,
  Home,
  LogIn,
  LogOut,
  Megaphone,
  Menu,
  MessageSquarePlus,
  PackageSearch,
  Palette,
  PanelLeftClose,
  PanelLeftOpen,
  ScanSearch,
  Search,
  ShieldCheck,
  ShoppingBag,
  Siren,
  Star,
  Upload,
  UserCog,
  UserRound,
  UsersRound,
  X,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useFeedbackUnreadCount, useMerchantApiPurchaseIntents, useMerchantCarpoolApplications, useMyApiPurchaseIntents, useMyCarpoolApplications, useMyProfileQuery, useNotifications } from '@/queries/useMarketQueries'
import { useImportantAnnouncementUnreadCount } from '@/queries/useAnnouncementQueries'
import { appThemes, applyAppTheme, getInitialAppTheme, isAppTheme } from '@/theme/appThemes'
import { ACCOUNT_RECOVERY_PATH, isAccountRecoveryAllowedPath, isAccountRecoveryComplete } from '@/lib/accountRecovery'

const route = useRoute()
const router = useRouter()
const menuOpen = ref(false)
const sidebarCollapsed = ref(false)
const searchText = ref('')
const activeTheme = ref(getInitialAppTheme())
const { data: myApiOrders } = useMyApiPurchaseIntents({ sort: 'default_buyer' })
const { data: merchantApiOrders } = useMerchantApiPurchaseIntents({ sort: 'default_merchant' })
const { data: myCarpoolApplications } = useMyCarpoolApplications({ sort: 'default_buyer' })
const { data: merchantCarpoolApplications } = useMerchantCarpoolApplications({ sort: 'default_owner' })
const { data: myProfile } = useMyProfileQuery()
const { data: notifications } = useNotifications()
const { data: feedbackUnreadCount } = useFeedbackUnreadCount()
const { data: importantAnnouncementUnreadCount } = useImportantAnnouncementUnreadCount()

const buyerApiActionCount = computed(() => (myApiOrders.value ?? []).filter(item => ['open', 'contacted'].includes(item.status)).length)
const merchantApiActionCount = computed(() => (merchantApiOrders.value ?? []).filter(item => item.status === 'open').length)
const buyerCarpoolActionCount = computed(() => (myCarpoolApplications.value ?? []).filter(item => ['accepted_reserved', 'waiting_contact', 'contacted', 'pending_completion', 'disputed'].includes(item.status)).length)
const ownerCarpoolActionCount = computed(() => (merchantCarpoolApplications.value ?? []).filter(item => ['pending_owner', 'joined_pending_confirmation', 'pending_completion', 'disputed'].includes(item.status)).length)
const unreadBusinessCount = computed(() => (notifications.value ?? []).filter(item => item.unread).length)
const unreadCount = computed(() => unreadBusinessCount.value + (importantAnnouncementUnreadCount.value ?? 0))
const feedbackMenuUnreadCount = computed(() => feedbackUnreadCount.value ?? 0)
const currentUsername = computed(() => myProfile.value?.username ?? '')
const currentDisplayName = computed(() => myProfile.value?.displayName ?? myProfile.value?.username ?? '未登录')
const currentAvatarText = computed(() => currentDisplayName.value.slice(0, 1).toUpperCase())
const canViewAdminNav = computed(() => myProfile.value?.permissions.includes('admin') ?? false)
const announcementCenterTo = '/my/notifications?tab=announcements'
const accountRecoveryRequired = computed(() => myProfile.value ? !isAccountRecoveryComplete(myProfile.value) : false)

const navGroups = computed(() => {
  const browseGroup = {
    title: '浏览',
    items: [
      { label: '行情首页', to: '/', count: null, icon: Home },
      { label: '官网价格', to: '/official-prices', count: null, icon: ShieldCheck },
      { label: '订阅拼车', to: '/carpools', count: null, icon: UsersRound },
      { label: 'API 集市', to: '/api-market', count: null, icon: Code2 },
      { label: '找车源', to: '/demands', count: null, icon: ScanSearch },
    ],
  }
  const publishGroup = {
    title: '发布',
    items: [
      { label: '提交低价线索', to: '/official-prices/submit', count: null, icon: MessageSquarePlus },
      { label: '发布车源', to: '/carpools/new', count: null, icon: Car },
      { label: '发布 API 服务', to: '/api-market/new', count: null, icon: PackageSearch },
    ],
  }
  const userGroup = {
    title: '个人工作台',
    items: [
      { label: '我的中心', to: '/my', count: null, icon: UserRound },
    ],
  }
  const merchantGroup = {
    title: '商户工作台',
    items: [
      { label: '商户中心', to: '/merchant/api-orders', count: null, icon: PackageSearch },
      { label: '订单管理', to: '/merchant/carpool-applications', count: ownerCarpoolActionCount.value, icon: UserCog },
    ],
  }
  const adminGroup = {
    title: '管理',
    items: [
      { label: '管理台总览', to: '/admin', count: 12, icon: PackageSearch },
      { label: '套餐目录', to: '/admin/product-plans', count: null, icon: PackageSearch },
      { label: 'API 模型目录', to: '/admin/api-models', count: null, icon: Code2 },
      { label: '低价线索审核', to: '/admin/price-leads', count: 4, icon: ShieldCheck },
      { label: '车源治理', to: '/admin/carpools', count: 8, icon: Car },
      { label: 'API 服务审核', to: '/admin/api-services', count: 3, icon: Code2 },
      { label: '公告管理', to: '/admin/announcements', count: null, icon: Megaphone },
      { label: '上车申请管理', to: '/admin/carpool-applications', count: ownerCarpoolActionCount.value, icon: UsersRound },
      { label: '用户管理', to: '/admin/users', count: 6, icon: UserCog },
      { label: '问题反馈', to: '/admin/feedback', count: null, icon: CircleHelp },
      { label: '举报纠纷', to: '/admin/reports', count: 4, icon: Siren },
    ],
  }

  return canViewAdminNav.value
    ? [browseGroup, publishGroup, adminGroup, userGroup, merchantGroup]
    : [browseGroup, publishGroup, userGroup, merchantGroup]
})

const topNotifications = computed(() => (notifications.value ?? []).slice(0, 4))

const activeNavItem = computed(() => {
  return navGroups.value
    .flatMap(group => group.items)
    .filter(item => matchesRoute(item.to))
    .sort((a, b) => b.to.length - a.to.length)[0]
})

const currentTitle = computed(() => activeNavItem.value?.label ?? String(route.meta.title ?? 'C2CMarket'))

function isActive(to: string) {
  return activeNavItem.value?.to === to
}

function matchesRoute(to: string) {
  if (to === '/') return route.path === '/'
  return route.path === to || route.path.startsWith(`${to}/`)
}

watch(
  () => route.fullPath,
  () => {
    menuOpen.value = false
  },
)

watch(
  () => [route.fullPath, accountRecoveryRequired.value] as const,
  () => {
    if (!accountRecoveryRequired.value || isAccountRecoveryAllowedPath(route.path)) return
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

function openNotifications() {
  router.push('/my/notifications')
}

function logoutMock() {
  toast('已退出登录。')
  router.push('/login')
}

function setActiveTheme(theme: unknown) {
  if (typeof theme !== 'string' || !isAppTheme(theme)) return
  activeTheme.value = theme
  applyAppTheme(theme)
}

function closeMenu() {
  menuOpen.value = false
}
</script>

<template>
  <div
    class="min-h-screen bg-background lg:grid"
    :style="{ gridTemplateColumns: sidebarCollapsed ? '64px minmax(0, 1fr)' : '222px minmax(0, 1fr)' }"
  >
    <aside class="sticky top-0 hidden h-screen overflow-hidden border-r border-slate-200 bg-white/95 backdrop-blur transition-[width] duration-200 lg:flex lg:flex-col">
      <RouterLink
        to="/"
        class="flex h-[60px] items-center border-b border-slate-200 font-semibold tracking-tight"
        :class="sidebarCollapsed ? 'justify-center px-0' : 'gap-2.5 px-5'"
      >
        <img src="/c2cmarket-logo-mark.svg?v=20260704-logo2" alt="C2CMarket" class="h-7 w-7 shrink-0" />
        <span v-if="!sidebarCollapsed" class="min-w-0">
          <span class="block truncate text-[19px] font-bold leading-tight text-slate-900">C2CMarket</span>
        </span>
      </RouterLink>
      <nav class="c2c-sidebar-scroll min-h-0 flex-1 space-y-[26px] overflow-y-auto px-3 py-5">
        <section v-for="group in navGroups" :key="group.title">
          <h2
            class="flex h-5 items-center px-2 text-[12px] font-medium text-slate-500"
            :class="sidebarCollapsed ? 'justify-center px-0' : ''"
          >
            <span v-if="sidebarCollapsed" class="h-px w-6 bg-border"></span>
            <span v-else>{{ group.title }}</span>
          </h2>
          <div class="mt-2 grid gap-1">
            <RouterLink
              v-for="item in group.items"
              :key="item.to"
              :to="item.to"
              class="flex h-9 items-center rounded-md text-[14px] font-semibold text-slate-700 transition hover:bg-teal-50 hover:text-teal-700"
              :title="sidebarCollapsed ? item.label : undefined"
              :class="isActive(item.to) ? 'bg-teal-50 text-teal-700 shadow-sm' : ''"
            >
              <span
                class="flex min-w-0 items-center"
                :class="sidebarCollapsed ? 'w-full justify-center' : 'gap-3 px-3'"
              >
                <component :is="item.icon" class="h-4 w-4 shrink-0" />
                <span v-if="!sidebarCollapsed" class="truncate">{{ item.label }}</span>
              </span>
              <Badge v-if="item.count && !sidebarCollapsed" variant="secondary" class="mr-2 h-5 px-1.5 text-[11px]">{{ item.count }}</Badge>
            </RouterLink>
          </div>
        </section>
      </nav>
      <div class="border-t border-slate-200 p-2">
        <RouterLink
          v-if="!sidebarCollapsed"
          :to="announcementCenterTo"
          class="mb-3 flex items-center justify-between rounded-md border border-slate-200 bg-white px-3 py-3 text-xs leading-5 text-slate-600 shadow-sm transition hover:border-teal-200 hover:bg-teal-50/70 hover:text-teal-800"
        >
          <div>
            <div class="font-medium text-teal-700">平台公告</div>
            <div class="mt-1">查看公告与更新</div>
          </div>
          <ChevronDown class="h-4 w-4 -rotate-90 text-slate-400" />
        </RouterLink>
        <Button
          variant="ghost"
          size="sm"
          class="h-9 w-full justify-start gap-2 px-2 text-[13px] text-slate-500"
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
      class="fixed inset-0 z-40 bg-black/30 lg:hidden"
      @click="closeMenu"
    ></div>

    <div
      v-if="menuOpen"
      class="fixed inset-y-0 left-0 z-50 flex w-[min(336px,calc(100vw-48px))] flex-col border-r border-border bg-card shadow-xl lg:hidden"
      aria-label="移动端导航抽屉"
    >
      <div class="flex h-[60px] items-center justify-between border-b border-border px-4">
        <RouterLink to="/" class="flex min-w-0 items-center gap-3 font-semibold tracking-tight" @click="closeMenu">
          <img src="/c2cmarket-logo-mark.svg?v=20260704-logo2" alt="C2CMarket" class="h-8 w-8 shrink-0" />
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
            <RouterLink
              v-for="item in group.items"
              :key="item.to"
              :to="item.to"
              class="flex items-center justify-between rounded-md px-3 py-2.5 text-sm font-medium text-slate-600 transition hover:bg-teal-50 hover:text-teal-700"
              :class="isActive(item.to) ? 'bg-teal-50 text-teal-700' : ''"
              @click="closeMenu"
            >
              <span class="flex min-w-0 items-center gap-2">
                <component :is="item.icon" class="h-4 w-4 shrink-0" />
                <span class="truncate">{{ item.label }}</span>
              </span>
              <Badge v-if="item.count" variant="secondary">{{ item.count }}</Badge>
            </RouterLink>
          </div>
        </section>
      </nav>
      <RouterLink
        :to="announcementCenterTo"
        class="border-t border-border p-4 text-xs leading-5 text-muted-foreground transition hover:bg-teal-50 hover:text-teal-800"
        @click="closeMenu"
      >
        平台公告 · 查看公告与更新
      </RouterLink>
    </div>

    <div class="min-w-0">
      <header class="sticky top-0 z-50 border-b border-slate-200 bg-white/88 backdrop-blur">
        <div class="flex h-[60px] items-center gap-4 px-4 sm:px-5 lg:px-5">
          <Button variant="ghost" size="icon" class="lg:hidden" aria-label="打开导航菜单" @click="menuOpen = true">
            <Menu class="h-4 w-4" />
          </Button>
          <RouterLink to="/" class="flex items-center gap-2 font-semibold tracking-tight lg:hidden">
            <img src="/c2cmarket-logo-mark.svg?v=20260704-logo2" alt="C2CMarket" class="h-8 w-8" />
          </RouterLink>
          <div class="hidden min-w-0 shrink-0 md:block lg:w-[260px] 2xl:w-[338px]">
            <div class="truncate text-lg font-semibold text-slate-900">{{ currentTitle }}</div>
          </div>
          <div class="hidden w-[548px] max-w-[34vw] items-center md:flex 2xl:max-w-[40vw]">
            <div class="relative w-full">
              <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
              <Input
                v-model="searchText"
                name="global-search"
                class="h-9 rounded-md bg-white pl-9 pr-14 shadow-none"
                aria-label="搜索产品、车源、开通方式、地区或商户"
                placeholder="搜索产品、车源、API 服务"
                @keyup.enter="runSearch"
              />
              <button class="absolute right-2 top-1/2 -translate-y-1/2 rounded border border-slate-200 px-1.5 py-0.5 text-xs text-slate-400" type="button" @click="runSearch">⌘ K</button>
            </div>
          </div>
          <div class="flex-1" />
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" size="icon" aria-label="切换主题">
                <Palette class="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="w-44">
              <DropdownMenuLabel>主题</DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuRadioGroup :model-value="activeTheme" @update:model-value="setActiveTheme">
                <DropdownMenuRadioItem v-for="theme in appThemes" :key="theme.value" :value="theme.value">
                  <span class="h-3 w-3 rounded-full border border-border" :style="{ background: theme.swatch }"></span>
                  <span>{{ theme.label }}</span>
                </DropdownMenuRadioItem>
              </DropdownMenuRadioGroup>
            </DropdownMenuContent>
          </DropdownMenu>
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" size="icon" class="relative text-slate-600" @click="openNotifications">
                <Bell class="h-4 w-4" />
                <span v-if="unreadCount" class="absolute -right-0.5 -top-0.5 inline-flex h-4 min-w-[16px] items-center justify-center rounded-full bg-primary px-1 text-[10px] leading-none text-primary-foreground">{{ unreadCount }}</span>
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
            </DropdownMenuContent>
          </DropdownMenu>
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button size="sm" class="hidden md:inline-flex">
                发布
                <ChevronDown class="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="w-48">
              <DropdownMenuItem as-child>
                <RouterLink to="/official-prices/submit" class="flex items-center gap-2">
                  <ShieldCheck class="h-4 w-4" />提交低价线索
                </RouterLink>
              </DropdownMenuItem>
              <DropdownMenuItem as-child>
                <RouterLink to="/carpools/new" class="flex items-center gap-2">
                  <Upload class="h-4 w-4" />导入 / 发布车源
                </RouterLink>
              </DropdownMenuItem>
              <DropdownMenuItem as-child>
                <RouterLink to="/api-market/new" class="flex items-center gap-2">
                  <Code2 class="h-4 w-4" />发布 API 服务
                </RouterLink>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <RouterLink v-if="!myProfile" :to="{ path: '/login', query: { returnTo: route.fullPath } }" class="hidden md:inline-flex">
            <Button variant="outline" size="sm">
              <LogIn class="h-4 w-4" />
              登录
            </Button>
          </RouterLink>
          <DropdownMenu v-else>
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" size="sm" class="hidden gap-2 text-slate-700 md:inline-flex">
                <span class="grid h-7 w-7 place-items-center rounded-full bg-slate-200 text-[12px] text-slate-700">{{ currentAvatarText }}</span>
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
                <RouterLink to="/my/profile" class="flex items-center gap-2">
                  <UserRound class="h-4 w-4" />个人资料
                </RouterLink>
              </DropdownMenuItem>
              <DropdownMenuItem as-child>
                <RouterLink to="/my/contacts" class="flex items-center gap-2">
                  <MessageSquarePlus class="h-4 w-4" />联系方式
                </RouterLink>
              </DropdownMenuItem>
              <DropdownMenuItem as-child>
                <RouterLink to="/my/feedback" class="flex items-center justify-between gap-3">
                  <span class="flex min-w-0 items-center gap-2">
                    <CircleHelp class="h-4 w-4 shrink-0" />
                    <span>问题反馈</span>
                  </span>
                  <span
                    v-if="feedbackMenuUnreadCount"
                    class="inline-flex h-4 min-w-[16px] items-center justify-center rounded-full bg-primary px-1 text-[10px] leading-none text-primary-foreground"
                  >
                    {{ feedbackMenuUnreadCount }}
                  </span>
                </RouterLink>
              </DropdownMenuItem>
              <DropdownMenuItem as-child>
                <RouterLink to="/my/account" class="flex items-center gap-2">
                  <ShieldCheck class="h-4 w-4" />账号与认证
                </RouterLink>
              </DropdownMenuItem>
              <DropdownMenuItem as-child>
                <RouterLink :to="{ path: '/login', query: { returnTo: route.fullPath } }" class="flex items-center gap-2">
                  <LogIn class="h-4 w-4" />登录 / 绑定
                </RouterLink>
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem @click="logoutMock">
                <LogOut class="h-4 w-4" />退出登录
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

      </header>

      <main class="w-full px-4 py-5 sm:px-5 lg:px-5">
        <slot />
      </main>
    </div>
  </div>
</template>
