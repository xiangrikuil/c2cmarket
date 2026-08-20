<script setup lang="ts">
import { useRouter } from 'vue-router'
import CarFront from 'lucide-vue-next/dist/esm/icons/car-front.js'
import ChevronRight from 'lucide-vue-next/dist/esm/icons/chevron-right.js'
import Code2 from 'lucide-vue-next/dist/esm/icons/code-xml.js'
import ShieldCheck from 'lucide-vue-next/dist/esm/icons/shield-check.js'
import type { ApiService, Carpool } from '@/data/mock'
import { formatModelSummary } from '@/components/api-service-detail/utils'
import { getApiMerchantDisplayName } from '@/lib/apiServicePresentation'
import { getApiServicePricePresentation } from '@/lib/apiServicePricingPresentation'
import { getCurrentPayablePrice, getRemainingSeats } from '@/lib/pricing'
import { getApiServiceProductIconSrc, getProductIconSrc } from '@/lib/productCategoryIcon'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'

const props = defineProps<{
  carpools: Carpool[]
  apiServices: ApiService[]
  carpoolCount: number
  apiServiceCount: number
  hasMarketData: boolean
  isLoading: boolean
  isError: boolean
  categoryIconByCode: Map<string, string>
}>()

const emit = defineEmits<{ retry: [] }>()
const router = useRouter()

const openMarketRecord = (event: MouseEvent | KeyboardEvent, to: string) => {
  if (event instanceof MouseEvent && (event.target as HTMLElement).closest('a,button')) return
  router.push(to)
}

const productIconSrc = (product: string) => getProductIconSrc(product, props.categoryIconByCode)
const apiServiceIconSrc = (service: ApiService) => getApiServiceProductIconSrc(service, props.categoryIconByCode)
const apiServicePrice = (service: ApiService) => getApiServicePricePresentation(service)
</script>

<template>
  <section class="home-latest-market" aria-labelledby="home-market-snapshot-title" :aria-busy="isLoading">
    <header class="home-latest-header">
      <div>
        <h2 id="home-market-snapshot-title">市场新上架</h2>
        <p>先比较价格、库存与当前可交易状态</p>
      </div>
      <span v-if="hasMarketData" class="home-market-snapshot-summary"><strong>{{ carpoolCount + apiServiceCount }}</strong> 个当前可交易</span>
    </header>

    <ErrorState v-if="isError" class="home-market-snapshot-state" title="首页市场内容加载失败" @retry="emit('retry')" />
    <div v-else class="home-market-snapshot-grid">
      <section class="home-market-snapshot-column is-carpool" aria-labelledby="home-carpool-snapshot-title">
        <header class="home-market-snapshot-column-header">
          <span class="home-market-snapshot-section-icon" aria-hidden="true"><CarFront /></span>
          <div>
            <h3 id="home-carpool-snapshot-title">可申请车源</h3>
            <p v-if="hasMarketData">{{ carpoolCount }} 个车源开放申请</p>
            <span v-else class="home-market-count-skeleton" />
          </div>
          <RouterLink to="/carpools" class="home-latest-all">查看全部<ChevronRight aria-hidden="true" /></RouterLink>
        </header>

        <div v-if="isLoading" class="home-market-snapshot-list" aria-label="正在加载车源">
          <div v-for="row in 3" :key="row" class="home-market-snapshot-row is-skeleton"><span /><span><i /><i /></span><span><i /><i /></span></div>
        </div>
        <EmptyState v-else-if="carpools.length === 0" class="home-market-snapshot-empty" title="暂无可申请车源" description="当前没有符合申请条件的公开车源，可以稍后再查看。">
          <template #action><RouterLink to="/carpools" class="home-empty-link">浏览全部车源</RouterLink></template>
        </EmptyState>
        <div v-else class="home-market-snapshot-list">
          <div v-for="item in carpools" :key="item.id" class="home-market-snapshot-row" role="link" tabindex="0" :aria-label="`查看 ${item.product} 车源`" @click="openMarketRecord($event, `/carpools/${item.id}`)" @keydown.enter="openMarketRecord($event, `/carpools/${item.id}`)">
            <span class="home-record-icon"><img v-if="productIconSrc(item.product)" :src="productIconSrc(item.product)!" alt="" /><CarFront v-else aria-hidden="true" /></span>
            <span class="home-market-snapshot-main">
              <span class="home-market-snapshot-title-line"><strong>{{ item.product }}</strong><span :class="['home-market-snapshot-badge', { 'is-limited': getRemainingSeats(item) === 1 }]">{{ getRemainingSeats(item) === 1 ? '仅 1 位' : '可申请' }}</span></span>
              <span class="home-market-snapshot-meta"><span>{{ item.region }}</span><span>{{ item.owner }}</span><span>{{ item.trustLevel === null ? '信任暂无数据' : `信任等级 ${item.trustLevel}` }}</span></span>
            </span>
            <span class="home-market-snapshot-aside"><strong>¥{{ getCurrentPayablePrice(item) }}/月</strong><small>剩余 {{ getRemainingSeats(item) }} 位</small></span>
          </div>
        </div>
      </section>

      <section class="home-market-snapshot-column is-api" aria-labelledby="home-api-snapshot-title">
        <header class="home-market-snapshot-column-header">
          <span class="home-market-snapshot-section-icon" aria-hidden="true"><Code2 /></span>
          <div>
            <h3 id="home-api-snapshot-title">可购买 API 服务</h3>
            <p v-if="hasMarketData">{{ apiServiceCount }} 个服务支持创建订单</p>
            <span v-else class="home-market-count-skeleton" />
          </div>
          <RouterLink to="/api-market" class="home-latest-all">查看全部<ChevronRight aria-hidden="true" /></RouterLink>
        </header>

        <div v-if="isLoading" class="home-market-snapshot-list" aria-label="正在加载 API 服务">
          <div v-for="row in 3" :key="row" class="home-market-snapshot-row is-skeleton"><span /><span><i /><i /></span><span><i /><i /></span></div>
        </div>
        <EmptyState v-else-if="apiServices.length === 0" class="home-market-snapshot-empty" title="暂无可购买 API 服务" description="当前没有符合公开下单条件的 API 服务。">
          <template #action><RouterLink to="/api-market" class="home-empty-link">前往 API 市场</RouterLink></template>
        </EmptyState>
        <div v-else class="home-market-snapshot-list">
          <div v-for="item in apiServices" :key="item.id" class="home-market-snapshot-row" role="link" tabindex="0" :aria-label="`查看 ${item.title}`" @click="openMarketRecord($event, `/api-market/${item.id}`)" @keydown.enter="openMarketRecord($event, `/api-market/${item.id}`)">
            <span class="home-record-icon is-api"><img v-if="apiServiceIconSrc(item)" :src="apiServiceIconSrc(item)!" alt="" /><Code2 v-else aria-hidden="true" /></span>
            <span class="home-market-snapshot-main">
              <span class="home-market-snapshot-title-line"><strong>{{ item.title }}</strong><span class="home-market-snapshot-badge">可购买</span></span>
              <span class="home-market-snapshot-meta"><span>{{ getApiMerchantDisplayName(item) }}</span><span>{{ formatModelSummary(item.models) }}</span><span>{{ item.defaultMultiplier.toFixed(2) }}x 倍率</span></span>
            </span>
            <span class="home-market-snapshot-aside"><strong>{{ apiServicePrice(item).value }}</strong><small>{{ apiServicePrice(item).secondary }}</small></span>
          </div>
        </div>
      </section>
    </div>

    <footer class="home-market-boundary"><ShieldCheck aria-hidden="true" /><span><strong>平台提示：</strong>请先核对卖家资料、计费方式与可用窗口，再进行线下交易。</span></footer>
  </section>
</template>
