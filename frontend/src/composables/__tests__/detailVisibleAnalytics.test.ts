import assert from 'node:assert/strict'
import { test } from 'vitest'
import { createVisibleDurationTracker } from '../useDetailVisibleAnalytics'

test('visible duration tracker ignores short views and sends once', () => {
  let now = 1_000
  const sent: number[] = []
  const tracker = createVisibleDurationTracker({
    now: () => now,
    emit: seconds => sent.push(seconds),
  })

  tracker.start()
  now += 2_000
  tracker.flush()
  assert.deepEqual(sent, [])

  tracker.start()
  now += 8_000
  tracker.flush()
  tracker.flush()
  assert.deepEqual(sent, [8])
})

test('visible duration tracker caps long views', () => {
  let now = 1_000
  const sent: number[] = []
  const tracker = createVisibleDurationTracker({
    now: () => now,
    emit: seconds => sent.push(seconds),
  })

  tracker.start()
  now += 9_000_000
  tracker.flush()

  assert.deepEqual(sent, [1800])
})

test('visible duration tracker pauses hidden time before final flush', () => {
  let now = 1_000
  const sent: number[] = []
  const tracker = createVisibleDurationTracker({
    now: () => now,
    emit: seconds => sent.push(seconds),
  })

  tracker.start()
  now += 2_000
  tracker.pause()
  now += 60_000
  tracker.start()
  now += 4_000
  tracker.flush()

  assert.deepEqual(sent, [6])
})
