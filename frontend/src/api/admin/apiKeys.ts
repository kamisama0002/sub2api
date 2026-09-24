/**
 * Admin API Keys API endpoints
 * Handles API key management for administrators
 */

import { apiClient } from '../client'
import type { ApiKey, PaginatedResponse, User } from '@/types'

export interface AdminApiKey extends ApiKey {
  user?: Pick<User, 'id' | 'username' | 'email'>
}

export async function listApiKeys(params: { page: number; page_size: number; search?: string; status?: string; user_id?: number }): Promise<PaginatedResponse<AdminApiKey>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminApiKey>>('/admin/api-keys', { params })
  return data
}

export async function revealApiKey(id: number, purpose: 'view' | 'copy'): Promise<{ id: number; key: string }> {
  const { data } = await apiClient.post<{ id: number; key: string }>(`/admin/api-keys/${id}/reveal`, { purpose })
  return data
}

export interface UpdateApiKeyGroupResult {
  api_key: ApiKey
  auto_granted_group_access: boolean
  granted_group_id?: number
  granted_group_name?: string
}

/**
 * Update an API key's group binding
 * @param id - API Key ID
 * @param groupId - Group ID (0 to unbind, positive to bind, null/undefined to skip)
 * @returns Updated API key with auto-grant info
 */
export async function updateApiKeyGroup(id: number, groupId: number | null): Promise<UpdateApiKeyGroupResult> {
  const { data } = await apiClient.put<UpdateApiKeyGroupResult>(`/admin/api-keys/${id}`, {
    group_id: groupId === null ? 0 : groupId
  })
  return data
}

export const apiKeysAPI = {
  listApiKeys,
  revealApiKey,
  updateApiKeyGroup
}

export default apiKeysAPI
