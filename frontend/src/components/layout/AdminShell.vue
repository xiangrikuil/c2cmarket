<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { AlertTriangle, ArrowLeft, Bell, BookOpen, Boxes, Car, ClipboardList, Code2, FileText, Gauge, Menu, MessageSquareWarning, PackageSearch, PanelLeftClose, PanelLeftOpen, Search, Settings, ShieldCheck, UserCog, Users, X } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useMyProfileQuery } from '@/queries/useMarketQueries'
import { useNavigationBadges } from '@/queries/useRealtimeQueries'
import { usePersistentSidebar } from '@/composables/usePersistentSidebar'
import { useRealtimeSync } from '@/composables/useRealtimeSync'
import { ACCOUNT_RECOVERY_PATH, isAccountRecoveryAllowedPath, isAccountRecoveryComplete } from '@/lib/accountRecovery'

const route = useRoute()
const router = useRouter()
const menuOpen = ref(false)
const searchText = ref('')
const { sidebarCollapsed } = usePersistentSidebar('c2c-admin-sidebar-collapsed')
const { data: profile } = useMyProfileQuery()
const { data: badges } = useNavigationBadges(computed(() => Boolean(profile.value)))
useRealtimeSync(computed(() => Boolean(profile.value)))

const navGroups = computed(() => [
  { title: '概览', items: [{ label: '管理工作台', to: '/admin', icon: Gauge, count: badges.value?.admin?.total ?? null }] },
  { title: '待办与治理', items: [
    { label: '官网价格维护', to: '/admin/official-prices', icon: ShieldCheck, count: badges.value?.admin?.officialPrices ?? null },
    { label: '车源异常', to: '/admin/carpools', icon: Car, count: badges.value?.admin?.carpools ?? null },
    { label: 'API 服务审核', to: '/admin/api-services', icon: Code2, count: badges.value?.admin?.apiServices ?? null },
    { label: '问题反馈', to: '/admin/feedback', icon: ClipboardList, count: badges.value?.admin?.feedbackTickets ?? null },
    { label: '举报纠纷', to: '/admin/reports', icon: MessageSquareWarning, count: badges.value?.admin?.reports ?? null },
    { label: '申诉处理', to: '/admin/appeals', icon: AlertTriangle, count: null },
  ] },
  { title: '市场目录', items: [
    { label: '套餐目录', to: '/admin/product-plans', icon: Boxes, count: null },
    { label: 'API 模型目录', to: '/admin/api-models', icon: PackageSearch, count: null },
    { label: '模型审计', to: '/admin/model-audit', icon: BookOpen, count: null },
  ] },
  { title: '交易与用户', items: [
    { label: 'API 订单追踪', to: '/admin/trade-intents', icon: FileText, count: null },
    { label: '求车管理', to: '/admin/demands', icon: Search, count: null },
    { label: '用户目录', to: '/admin/users', icon: Users, count: null },
  ] },
  { title: '内容与系统', items: [
    { label: '公告管理', to: '/admin/announcements', icon: Bell, count: null },
    { label: '审计日志', to: '/admin/logs', icon: Settings, count: null },
  ] },
])

const activeItem = computed(() => navGroups.value.flatMap(group => group.items)
  .filter(item => item.to === '/admin' ? route.path === '/admin' : route.path === item.to || route.path.startsWith(`${item.to}/`))
  .sort((a, b) => b.to.length - a.to.length)[0])
const currentTitle = computed(() => activeItem.value?.label ?? '管理台')
const adminName = computed(() => profile.value?.displayName || profile.value?.username || '管理员')
const recoveryRequired = computed(() => profile.value ? !isAccountRecoveryComplete(profile.value) : false)

function formatCount(value: number | null) {
  if (!value) return ''
  return value > 99 ? '99+' : String(value)
}

function runSearch() {
  const keyword = searchText.value.trim()
  router.push({ path: '/admin/users', query: keyword ? { search: keyword } : {} })
  menuOpen.value = false
}

function closeMenu() { menuOpen.value = false }
function onKeydown(event: KeyboardEvent) { if (event.key === 'Escape') closeMenu() }

watch(() => route.fullPath, closeMenu)
watch(
  () => [profile.value, route.fullPath] as const,
  ([currentProfile]) => {
    if (!currentProfile) return
    if (!currentProfile.permissions.includes('admin')) {
      router.replace('/')
      return
    }
    if (recoveryRequired.value && !isAccountRecoveryAllowedPath(route.path)) {
      router.replace({ path: ACCOUNT_RECOVERY_PATH, query: { returnTo: route.fullPath } })
    }
  },
  { immediate: true },
)
onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="min-h-screen bg-background lg:grid" :style="{ gridTemplateColumns: sidebarCollapsed ? '64px minmax(0,1fr)' : '208px minmax(0,1fr)' }">
    <aside class="sticky top-0 hidden h-screen border-r border-sidebar-border bg-sidebar text-sidebar-foreground lg:flex lg:flex-col">
      <RouterLink to="/admin" class="flex h-14 items-center border-b border-sidebar-border" :class="sidebarCollapsed ? 'justify-center' : 'gap-2 px-4'">
        <img src="/c2cmarket-logo-mark.svg?v=20260708-electric-blue" alt="C2CMarket" class="h-7 w-7" />
        <span v-if="!sidebarCollapsed" class="font-semibold">管理台</span>
      </RouterLink>
      <nav class="min-h-0 flex-1 space-y-5 overflow-y-auto px-2 py-4" aria-label="管理端导航">
        <section v-for="group in navGroups" :key="group.title">
          <h2 class="px-2 text-[11px] font-medium text-sidebar-foreground/55"><span v-if="!sidebarCollapsed">{{ group.title }}</span><span v-else class="mx-auto block h-px w-6 bg-sidebar-border" /></h2>
          <div class="mt-1.5 grid gap-1">
            <RouterLink v-for="item in group.items" :key="item.to" :to="item.to" class="flex h-9 items-center rounded-md text-sm font-medium transition hover:bg-sidebar-accent" :class="[activeItem?.to === item.to ? 'bg-sidebar-accent text-sidebar-accent-foreground' : 'text-sidebar-foreground/78', sidebarCollapsed ? 'justify-center' : 'gap-2.5 px-2.5']" :title="sidebarCollapsed ? item.label : undefined">
              <component :is="item.icon" class="h-4 w-4 shrink-0" /><span v-if="!sidebarCollapsed" class="min-w-0 flex-1 truncate">{{ item.label }}</span><Badge v-if="item.count && !sidebarCollapsed" variant="secondary">{{ formatCount(item.count) }}</Badge>
            </RouterLink>
          </div>
        </section>
      </nav>
      <div class="border-t border-sidebar-border p-2">
        <RouterLink to="/" class="mb-1 flex h-9 items-center rounded-md text-sm text-sidebar-foreground/75 hover:bg-sidebar-accent" :class="sidebarCollapsed ? 'justify-center' : 'gap-2 px-2'"><ArrowLeft class="h-4 w-4" /><span v-if="!sidebarCollapsed">返回用户端</span></RouterLink>
        <Button variant="ghost" size="sm" class="w-full" :class="sidebarCollapsed ? 'justify-center px-0' : 'justify-start'" :aria-label="sidebarCollapsed ? '展开管理侧栏' : '收起管理侧栏'" @click="sidebarCollapsed = !sidebarCollapsed"><PanelLeftOpen v-if="sidebarCollapsed" class="h-4 w-4" /><PanelLeftClose v-else class="h-4 w-4" /><span v-if="!sidebarCollapsed" class="ml-2">收起侧栏</span></Button>
      </div>
    </aside>

    <div v-if="menuOpen" class="fixed inset-0 z-40 bg-foreground/35 lg:hidden" @click="closeMenu" />
    <aside v-if="menuOpen" class="fixed inset-y-0 left-0 z-50 flex w-[min(336px,calc(100vw-40px))] flex-col border-r border-border bg-card shadow-xl lg:hidden" role="dialog" aria-modal="true" aria-label="管理端移动导航">
      <div class="flex h-14 items-center justify-between border-b border-border px-4"><span class="font-semibold">C2CMarket 管理台</span><Button variant="ghost" size="icon" aria-label="关闭管理导航" @click="closeMenu"><X class="h-4 w-4" /></Button></div>
      <nav class="flex-1 space-y-5 overflow-y-auto p-4">
        <section v-for="group in navGroups" :key="group.title"><h2 class="text-xs text-muted-foreground">{{ group.title }}</h2><div class="mt-2 grid gap-1"><RouterLink v-for="item in group.items" :key="item.to" :to="item.to" class="flex items-center gap-2 rounded-md px-3 py-2.5 text-sm" :class="activeItem?.to === item.to ? 'bg-accent font-medium' : ''"><component :is="item.icon" class="h-4 w-4" />{{ item.label }}<Badge v-if="item.count" variant="secondary" class="ml-auto">{{ formatCount(item.count) }}</Badge></RouterLink></div></section>
      </nav>
      <RouterLink to="/" class="flex items-center gap-2 border-t border-border p-4 text-sm"><ArrowLeft class="h-4 w-4" />返回用户端</RouterLink>
    </aside>

    <div class="min-w-0">
      <header class="sticky top-0 z-30 border-b border-border bg-card/90 backdrop-blur">
        <div class="flex h-14 items-center gap-3 px-4 sm:px-5 lg:px-6">
          <Button variant="ghost" size="icon" class="lg:hidden" aria-label="打开管理导航" @click="menuOpen = true"><Menu class="h-4 w-4" /></Button>
          <h1 class="hidden min-w-[150px] text-lg font-semibold md:block">{{ currentTitle }}</h1>
          <div class="relative hidden w-full max-w-xl md:block"><Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" /><Input v-model="searchText" class="h-9 pl-9" aria-label="后台全局搜索" placeholder="搜索用户或管理对象" @keyup.enter="runSearch" /></div>
          <div class="flex-1" />
          <Badge v-if="badges?.admin?.total" variant="secondary">我的待办 {{ formatCount(badges.admin.total) }}</Badge>
          <div class="hidden items-center gap-2 text-sm sm:flex"><span class="grid h-7 w-7 place-items-center rounded-full bg-primary/10 text-primary"><UserCog class="h-4 w-4" /></span><span>{{ adminName }}</span></div>
          <RouterLink to="/"><Button size="sm" variant="outline"><ArrowLeft class="h-4 w-4" /><span class="hidden sm:inline">返回用户端</span></Button></RouterLink>
        </div>
      </header>
      <main class="w-full px-4 py-5 sm:px-5 lg:px-6"><slot /></main>
    </div>
  </div>
</template>
