<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useHead, useRuntimeConfig, useSeoMeta } from '#app'
import AppShell from '@/components/layout/AppShell.vue'
import AdminShell from '@/components/layout/AdminShell.vue'
import { Toaster } from '@/components/ui/sonner'
import { breadcrumbItems, resolveRouteSeo } from '@/seo/routeSeo'

const route = useRoute()
const config = useRuntimeConfig()
const standaloneLayout = computed(() => route.meta.standalone === true)
const adminLayout = computed(() => !standaloneLayout.value && route.path.startsWith('/admin'))
const seo = computed(() => resolveRouteSeo(route))
const siteUrl = computed(() => String(config.public.siteUrl || 'https://c2cmarket.shop'))
const canonical = computed(() => new URL(route.path, siteUrl.value).toString())

useSeoMeta({
  title: () => seo.value.title,
  description: () => seo.value.description,
  robots: () => seo.value.indexable ? 'index, follow, max-image-preview:large' : 'noindex, nofollow',
  ogSiteName: 'C2CMarket',
  ogType: 'website',
  ogTitle: () => seo.value.title,
  ogDescription: () => seo.value.description,
  ogUrl: canonical,
  twitterCard: 'summary',
})

useHead(() => ({
  htmlAttrs: { lang: 'zh-CN' },
  link: [{ rel: 'canonical', href: canonical.value }],
  script: seo.value.indexable
    ? [{
        key: 'route-structured-data',
        type: 'application/ld+json',
        textContent: JSON.stringify({
          '@context': 'https://schema.org',
          '@graph': [
            {
              '@type': 'WebSite',
              name: 'C2CMarket',
              url: siteUrl.value,
              potentialAction: {
                '@type': 'SearchAction',
                target: `${new URL('/search', siteUrl.value)}?q={search_term_string}`,
                'query-input': 'required name=search_term_string',
              },
            },
            {
              '@type': 'BreadcrumbList',
              itemListElement: breadcrumbItems(route, siteUrl.value),
            },
          ],
        }),
      }]
    : [],
}))
</script>

<template>
  <NuxtLoadingIndicator color="var(--primary)" />
  <NuxtPage v-if="standaloneLayout" />

  <AdminShell v-else-if="adminLayout">
    <NuxtPage />
  </AdminShell>

  <AppShell v-else>
    <NuxtPage />
  </AppShell>
  <Toaster position="top-right" rich-colors />
</template>
