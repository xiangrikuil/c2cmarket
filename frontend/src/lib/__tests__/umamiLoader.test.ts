import assert from 'node:assert/strict'
import { test } from 'vitest'
import { buildUmamiScriptConfig } from '../umamiLoader'

test('buildUmamiScriptConfig requires explicit enablement and required public fields', () => {
  assert.equal(buildUmamiScriptConfig({}), null)
  assert.equal(buildUmamiScriptConfig({
    enabled: true,
    scriptUrl: 'https://umami.example.test/script.js',
  }), null)
  assert.equal(buildUmamiScriptConfig({
    enabled: 'true',
    websiteId: 'site-id',
  }), null)
})

test('buildUmamiScriptConfig returns only frontend-safe tracker attributes', () => {
  const config = buildUmamiScriptConfig({
    enabled: 'true',
    scriptUrl: 'https://umami.example.test/script.js',
    websiteId: 'site-id',
    domains: 'c2c.example.test,www.c2c.example.test',
    hostUrl: 'https://umami.example.test',
  })

  assert.deepEqual(config, {
    scriptUrl: 'https://umami.example.test/script.js',
    websiteId: 'site-id',
    domains: 'c2c.example.test,www.c2c.example.test',
    hostUrl: 'https://umami.example.test',
  })
})
