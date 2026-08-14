import * as Sentry from '@sentry/nuxt'
import {
  parseSentryEnabled,
  parseSentrySampleRate,
  sanitizeSentryBreadcrumb,
  sanitizeSentryEvent,
} from './src/lib/sentryPrivacy'

const enabled = parseSentryEnabled(process.env.NUXT_PUBLIC_SENTRY_ENABLED)
const dsn = process.env.NUXT_PUBLIC_SENTRY_DSN?.trim() ?? ''

Sentry.init({
  dsn,
  enabled: enabled && Boolean(dsn),
  environment: process.env.NUXT_PUBLIC_SENTRY_ENVIRONMENT?.trim() || 'production',
  release: process.env.NUXT_PUBLIC_SENTRY_RELEASE?.trim()
    || process.env.SENTRY_RELEASE?.trim()
    || process.env.GITHUB_SHA?.trim()
    || undefined,
  sendDefaultPii: false,
  tracesSampleRate: parseSentrySampleRate(process.env.NUXT_PUBLIC_SENTRY_TRACES_SAMPLE_RATE, 0.05),
  tracePropagationTargets: [
    'https://api.c2cmarket.shop/api',
    'https://api-staging.c2cmarket.shop/api',
  ],
  beforeSend: sanitizeSentryEvent,
  beforeSendTransaction: sanitizeSentryEvent,
  beforeBreadcrumb: sanitizeSentryBreadcrumb,
})
