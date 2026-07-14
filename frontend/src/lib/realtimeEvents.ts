import { backendBaseURL } from '@/lib/backendClient'

export const REALTIME_EVENT_NAMES: ['ready', 'invalidate'] = ['ready', 'invalidate']
export const ALL_LIVE_TOPIC = 'all-live' as const

export type RealtimeEventName = typeof REALTIME_EVENT_NAMES[number]

export type RealtimeEventEnvelope = {
  schemaVersion: 1
  topics: Array<typeof ALL_LIVE_TOPIC>
}

export function realtimeEventsURL() {
  return `${backendBaseURL()}/api/v1/me/events`
}

export function decodeRealtimeEventEnvelope(payload?: string): RealtimeEventEnvelope {
  const value: unknown = JSON.parse(payload ?? '')
  if (!isRealtimeEventEnvelope(value)) {
    throw new Error('Invalid realtime event envelope.')
  }
  return value
}

export function tryDecodeRealtimeEventEnvelope(payload?: string): RealtimeEventEnvelope | null {
  try {
    return decodeRealtimeEventEnvelope(payload)
  } catch {
    return null
  }
}

export function isRealtimeEventEnvelope(value: unknown): value is RealtimeEventEnvelope {
  if (!value || typeof value !== 'object') return false
  const candidate = value as Record<string, unknown>
  return candidate.schemaVersion === 1
    && Array.isArray(candidate.topics)
    && candidate.topics.length > 0
    && candidate.topics.every(topic => topic === ALL_LIVE_TOPIC)
}

export function hasAllLiveTopic(envelope: RealtimeEventEnvelope) {
  return envelope.topics.includes(ALL_LIVE_TOPIC)
}
