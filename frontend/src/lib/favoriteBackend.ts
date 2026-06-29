import type { FavoriteListItem, FavoriteTargetType } from '@/lib/api'
import { backendMutation, backendRequest, ensureBackendSession } from '@/lib/backendClient'

type ListResponse<T> = { items: T[] }

type BackendFavoriteTargetType = 'carpool' | 'api_service'

type BackendFavorite = {
  id: string
  targetType: BackendFavoriteTargetType
  targetId: string
  title: string
  subtitle: string
  status: string
  to: string
  createdAt: string
}

type BackendFavoriteStatus = {
  favorited: boolean
  favorite?: BackendFavorite
}

function toBackendTargetType(value: FavoriteTargetType): BackendFavoriteTargetType {
  return value === 'api-service' ? 'api_service' : 'carpool'
}

function toFrontendTargetType(value: BackendFavoriteTargetType): FavoriteTargetType {
  return value === 'api_service' ? 'api-service' : 'carpool'
}

function mapFavorite(item: BackendFavorite): FavoriteListItem {
  return {
    id: item.id,
    targetType: toFrontendTargetType(item.targetType),
    targetId: item.targetId,
    title: item.title,
    subtitle: item.subtitle,
    status: item.status,
    to: item.to,
    createdAt: item.createdAt,
  }
}

export async function backendFavorites() {
  await ensureBackendSession('buyer', false)
  const response = await backendRequest<ListResponse<BackendFavorite>>('/api/v1/me/favorites')
  return response.items.map(mapFavorite)
}

export async function backendFavoriteStatus(targetType: FavoriteTargetType, targetId: string) {
  await ensureBackendSession('buyer', false)
  const response = await backendRequest<BackendFavoriteStatus>(`/api/v1/me/favorites/${toBackendTargetType(targetType)}/${encodeURIComponent(targetId)}`)
  return response.favorited
}

export async function backendToggleFavorite(targetType: FavoriteTargetType, targetId: string) {
  await ensureBackendSession('buyer', false)
  const backendType = toBackendTargetType(targetType)
  const encodedTargetID = encodeURIComponent(targetId)
  const current = await backendRequest<BackendFavoriteStatus>(`/api/v1/me/favorites/${backendType}/${encodedTargetID}`)
  const response = current.favorited
    ? await backendMutation<BackendFavoriteStatus>(`/api/v1/me/favorites/${backendType}/${encodedTargetID}`, {}, { method: 'DELETE' })
    : await backendMutation<BackendFavoriteStatus>(`/api/v1/me/favorites/${backendType}/${encodedTargetID}`, {}, {
      method: 'PUT',
      idempotencyPrefix: 'favorite-put',
    })
  return {
    favorited: response.favorited,
    favorite: response.favorite ? mapFavorite(response.favorite) : undefined,
  }
}
