import assert from 'node:assert/strict'
import { test } from 'vitest'
import {
  ALL_LIVE_TOPIC,
  decodeRealtimeEventEnvelope,
  hasAllLiveTopic,
  isRealtimeEventEnvelope,
  tryDecodeRealtimeEventEnvelope,
} from '../realtimeEvents'

test('decodes the versioned all-live realtime envelope', () => {
  const envelope = decodeRealtimeEventEnvelope(JSON.stringify({
    schemaVersion: 1,
    topics: [ALL_LIVE_TOPIC],
  }))

  assert.deepEqual(envelope, {
    schemaVersion: 1,
    topics: ['all-live'],
  })
  assert.equal(hasAllLiveTopic(envelope), true)
})

test('rejects malformed or unsupported realtime envelopes', () => {
  assert.equal(isRealtimeEventEnvelope({ schemaVersion: 2, topics: ['all-live'] }), false)
  assert.equal(isRealtimeEventEnvelope({ schemaVersion: 1, topics: [] }), false)
  assert.equal(isRealtimeEventEnvelope({ schemaVersion: 1, topics: ['user-1'] }), false)
  assert.throws(() => decodeRealtimeEventEnvelope('{invalid'))
  assert.throws(() => decodeRealtimeEventEnvelope(JSON.stringify({ schemaVersion: 2, topics: ['all-live'] })))
  assert.equal(tryDecodeRealtimeEventEnvelope('{invalid'), null)
  assert.equal(tryDecodeRealtimeEventEnvelope(JSON.stringify({ schemaVersion: 2, topics: ['all-live'] })), null)
})
