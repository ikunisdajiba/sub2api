/**
 * WorkBuddy 反代网关管理 API
 * 直接管理 wb-proxy 里的真实上游账号：列表 / 登录新增 / 签到 / 刷新积分 / 启停 / 删除 / 同步成 sub2 账号。
 */

import { apiClient } from './client'

export interface WBCheckin {
  today_checked_in?: boolean
  streak_days?: number
  daily_credit?: number
  today_credit?: number
  active?: boolean
}

export interface WBCredits {
  remain?: number
  used?: number
  size?: number
  used_percent?: string
  remain_percent?: string
  checkin?: WBCheckin
  [key: string]: unknown
}

export interface WBSub2Link {
  id: number
  name: string
  status?: string
  schedulable?: boolean
  concurrency?: number
  priority?: number
}

export interface WBAccount {
  uid: string
  nickname?: string
  realm?: string
  platform?: string
  product?: string
  domain?: string
  enabled?: boolean
  weight?: number
  note?: string
  expiresIn?: string
  lastError?: string
  credits?: WBCredits
  /** 该 wb 账号对应的独立 sub2 账号（未建号时缺省） */
  sub2_account?: WBSub2Link
  [key: string]: unknown
}

export interface WBAcountsPayload {
  accounts: WBAccount[]
  usable?: number
  realm?: string
  base_url?: string
  configured?: boolean
  upstream_key?: boolean
}

export interface WBLoginStart {
  state: string
  authUrl: string
  realm?: string
  platform?: string
}

export interface WBLoginPoll {
  status: string
  message?: string
  /** status=ok 时返回刚登录的账号，用于按账号维度建号 */
  account?: WBAccount
  [key: string]: unknown
}

export interface WBSyncOutcome {
  result: { created: string[]; updated: string[]; skipped: string[]; errors: string[] }
  accounts?: WBAccount[]
}

// 注意：apiClient 的响应拦截器已经把 { code, message, data } 解包成 data，
// 所以这里直接返回 response.data，不要再取一层。
export async function listWorkBuddyAccounts(realm = 'all'): Promise<WBAcountsPayload> {
  const res = await apiClient.get<WBAcountsPayload>('/admin/workbuddy/accounts', { params: { realm } })
  return res.data
}

export async function startWorkBuddyLogin(realm: string, platform: string): Promise<WBLoginStart> {
  const res = await apiClient.post<WBLoginStart>('/admin/workbuddy/login/start', { realm, platform })
  return res.data
}

export async function pollWorkBuddyLogin(state: string): Promise<WBLoginPoll> {
  const res = await apiClient.get<WBLoginPoll>('/admin/workbuddy/login/poll', { params: { state } })
  return res.data
}

export async function checkinWorkBuddy(uid?: string): Promise<WBAcountsPayload> {
  const res = await apiClient.post<WBAcountsPayload>('/admin/workbuddy/accounts/checkin', { uid: uid || '' })
  return res.data
}

export async function fetchWorkBuddyCredits(uid?: string): Promise<WBAcountsPayload> {
  const res = await apiClient.post<WBAcountsPayload>('/admin/workbuddy/accounts/credits', { uid: uid || '' })
  return res.data
}

export async function updateWorkBuddyAccount(payload: Record<string, unknown>): Promise<WBAcountsPayload> {
  const res = await apiClient.post<WBAcountsPayload>('/admin/workbuddy/accounts/update', payload)
  return res.data
}

export async function deleteWorkBuddyAccount(uid: string): Promise<WBAcountsPayload> {
  const res = await apiClient.post<WBAcountsPayload>('/admin/workbuddy/accounts/delete', { uid })
  return res.data
}

export async function syncWorkBuddyAccounts(
  uids?: string[],
  realm = 'all',
  groupIds?: number[],
  concurrency?: number
): Promise<WBSyncOutcome> {
  const res = await apiClient.post<WBSyncOutcome>('/admin/workbuddy/sync', {
    uids: uids || [],
    realm,
    group_ids: groupIds || [],
    concurrency: concurrency || 0
  })
  return res.data
}