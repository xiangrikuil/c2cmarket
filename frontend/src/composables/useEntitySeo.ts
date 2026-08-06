import { useHead, useRoute, useRuntimeConfig, useSeoMeta } from '#app'
import { computed, toValue, type MaybeRefOrGetter } from 'vue'

type JsonLd = Record<string, unknown>

export function useEntitySeo(options: {
  title: MaybeRefOrGetter<string>
  description: MaybeRefOrGetter<string>
  schema?: MaybeRefOrGetter<JsonLd | null>
  indexable?: MaybeRefOrGetter<boolean>
}) {
  const route = useRoute()
  const config = useRuntimeConfig()
  const siteUrl = computed(() => String(config.public.siteUrl || 'https://c2cmarket.shop'))
  const canonical = computed(() => new URL(route.path, siteUrl.value).toString())

  useSeoMeta({
    title: () => toValue(options.title),
    description: () => toValue(options.description),
    ogTitle: () => toValue(options.title),
    ogDescription: () => toValue(options.description),
    ogUrl: canonical,
    twitterCard: 'summary',
    twitterTitle: () => toValue(options.title),
    twitterDescription: () => toValue(options.description),
    robots: () => options.indexable === undefined || toValue(options.indexable)
      ? 'index, follow, max-image-preview:large'
      : 'noindex, nofollow',
  })

  useHead(() => {
    const schema = options.schema ? toValue(options.schema) : null
    if (!schema) return {}
    return {
      script: [{
        key: 'entity-structured-data',
        type: 'application/ld+json',
        textContent: JSON.stringify({ '@context': 'https://schema.org', ...schema }),
      }],
    }
  })
}
