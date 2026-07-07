import assert from 'node:assert/strict'
import { test } from 'vitest'
import { buildUmamiScriptConfig } from '../umamiLoader'

test('buildUmamiScriptConfig requires explicit enablement and required public fields', () => {
  assert.equal(buildUmamiScriptConfig({}), null)
  assert.equal(buildUmamiScriptConfig({
    VITE_UMAMI_ENABLED: 'true',
    VITE_UMAMI_SCRIPT_URL: 'https://umami.example.test/script.js',
  }), null)
  assert.equal(buildUmamiScriptConfig({
    VITE_UMAMI_ENABLED: 'true',
    VITE_UMAMI_WEBSITE_ID: 'site-id',
  }), null)
})

test('buildUmamiScriptConfig returns only frontend-safe tracker attributes', () => {
  const config = buildUmamiScriptConfig({
    VITE_UMAMI_ENABLED: 'true',
    VITE_UMAMI_SCRIPT_URL: 'https://umami.example.test/script.js',
    VITE_UMAMI_WEBSITE_ID: 'site-id',
    VITE_UMAMI_DOMAINS: 'c2c.example.test,www.c2c.example.test',
    VITE_UMAMI_HOST_URL: 'https://umami.example.test',
    VITE_UMAMI_SHARE_URL: 'https://umami.example.test/share/secret',
    VITE_UMAMI_API_KEY: 'must-not-be-used',
  })

  assert.deepEqual(config, {
    scriptUrl: 'https://umami.example.test/script.js',
    websiteId: 'site-id',
    domains: 'c2c.example.test,www.c2c.example.test',
    hostUrl: 'https://umami.example.test',
  })
})
