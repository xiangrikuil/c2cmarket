import * as Sentry from '@sentry/nuxt'
import { useRuntimeConfig } from '#imports'
import {
  parseSentryEnabled,
  parseSentrySampleRate,
  sanitizeSentryBreadcrumb,
  sanitizeSentryEvent,
} from './src/lib/sentryPrivacy'

const runtimeConfig = useRuntimeConfig()
const sentryConfig = runtimeConfig.public.sentry

Sentry.init({
  dsn: sentryConfig.dsn,
  enabled: parseSentryEnabled(sentryConfig.enabled) && Boolean(sentryConfig.dsn),
  environment: sentryConfig.environment,
  release: sentryConfig.release || undefined,
  sendDefaultPii: false,
  tracesSampleRate: parseSentrySampleRate(sentryConfig.tracesSampleRate, 0.05),
  tracePropagationTargets: [
    /^\/api(?:\/|$)/,
    /^https:\/\/api(?:-staging)?\.c2cmarket\.shop\/api(?:\/|$)/,
  ],
  beforeSend: sanitizeSentryEvent,
  beforeSendTransaction: sanitizeSentryEvent,
  beforeBreadcrumb: sanitizeSentryBreadcrumb,
})
