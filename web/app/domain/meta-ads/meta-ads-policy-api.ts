import type { ApiRequest } from '~/domain/calendar/calendar-api'

export interface MetaAdsActionPolicy {
  configured: boolean
  adAccountId: string
  currency: string
  maxDailyBudget: number | null
  maxLifetimeBudget: number | null
  allowCreate: boolean
  allowDuplicate: boolean
  allowResume: boolean
  updatedAt: string
}

export interface MetaAdsActionPolicyInput {
  maxDailyBudget: number | null
  maxLifetimeBudget: number | null
  allowCreate: boolean
  allowDuplicate: boolean
  allowResume: boolean
}

function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {}
}

function nullablePositiveNumber(value: unknown): number | null {
  if (value === null || value === undefined || value === '') return null
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null
}

export function normalizeMetaAdsActionPolicy(value: unknown): MetaAdsActionPolicy {
  const raw = record(value)
  return {
    configured: raw.configured === true,
    adAccountId: typeof raw.adAccountId === 'string' ? raw.adAccountId.trim() : '',
    currency: typeof raw.currency === 'string' ? raw.currency.trim().slice(0, 3).toUpperCase() : '',
    maxDailyBudget: nullablePositiveNumber(raw.maxDailyBudget),
    maxLifetimeBudget: nullablePositiveNumber(raw.maxLifetimeBudget),
    allowCreate: raw.allowCreate === true,
    allowDuplicate: raw.allowDuplicate === true,
    allowResume: raw.allowResume === true,
    updatedAt: typeof raw.updatedAt === 'string' ? raw.updatedAt.trim() : '',
  }
}

function policyPath(adAccountId: string): string {
  return `/v1/meta-ads/ad-accounts/${encodeURIComponent(adAccountId.trim())}/action-policy`
}

export async function getMetaAdsActionPolicy(
  api: ApiRequest,
  adAccountId: string,
  signal?: AbortSignal,
): Promise<MetaAdsActionPolicy> {
  const options = signal ? { signal } : undefined
  return normalizeMetaAdsActionPolicy(await api(policyPath(adAccountId), options))
}

export async function saveMetaAdsActionPolicy(
  api: ApiRequest,
  adAccountId: string,
  input: MetaAdsActionPolicyInput,
  signal?: AbortSignal,
): Promise<MetaAdsActionPolicy> {
  return normalizeMetaAdsActionPolicy(
    await api(policyPath(adAccountId), {
      method: 'PUT',
      body: input,
      ...(signal ? { signal } : {}),
    }),
  )
}
