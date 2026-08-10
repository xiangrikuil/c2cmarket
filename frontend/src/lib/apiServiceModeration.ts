const publicApiServiceStatuses = new Set(['在线'])
const exceptionApiServiceStatuses = new Set(['待处理', '待复核', '已下架', '已拒绝', '已移除'])
const adminActionApiServiceStatuses = new Set(['待处理', '已下架'])

export function isApiServicePublicStatus(status: string) {
  return publicApiServiceStatuses.has(status.trim())
}

export function isApiServiceExceptionStatus(status: string) {
  return exceptionApiServiceStatuses.has(status.trim())
}

export function isApiServiceAdminActionStatus(status: string) {
  return adminActionApiServiceStatuses.has(status.trim())
}
