import { backendCloseDemand, backendDemandById, backendDemands, backendMyDemands, backendSubmitDemand } from '@/lib/demandBackend'
import { shouldUseRealBackend } from '@/api/client'
import type { DemandRecord, SubmitDemandPayload } from './types'

const mockDelayMs = 120

async function waitForMock() {
  await new Promise(resolve => window.setTimeout(resolve, mockDelayMs))
}

async function demandMocks() {
  return import('@/mocks/demand')
}

export async function getDemands(): Promise<DemandRecord[]> {
  if (shouldUseRealBackend()) return backendDemands()
  await waitForMock()
  const { listMockDemands } = await demandMocks()
  return listMockDemands()
}

export async function getMyDemands(): Promise<DemandRecord[]> {
  if (shouldUseRealBackend()) return backendMyDemands()
  await waitForMock()
  const { listMockDemands } = await demandMocks()
  return listMockDemands()
}

export async function getDemandById(id: string): Promise<DemandRecord | null> {
  if (shouldUseRealBackend()) return backendDemandById(id)
  await waitForMock()
  const { getMockDemandById } = await demandMocks()
  return getMockDemandById(id)
}

export async function submitDemand(payload: SubmitDemandPayload): Promise<DemandRecord> {
  if (shouldUseRealBackend()) return backendSubmitDemand(payload)
  await waitForMock()
  const { createMockDemand } = await demandMocks()
  return createMockDemand(payload)
}

export async function closeDemand(id: string): Promise<DemandRecord> {
  if (shouldUseRealBackend()) return backendCloseDemand(id)
  await waitForMock()
  const { toggleMockDemandClosed } = await demandMocks()
  return toggleMockDemandClosed(id)
}

export type { DemandRecord, DemandStatus, SubmitDemandPayload } from './types'
