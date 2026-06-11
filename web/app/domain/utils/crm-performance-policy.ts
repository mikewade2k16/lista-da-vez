export type CrmListUsageTier = {
  id: string
  label: string
  minRate: number
}

export type CrmGoalPayoutMode = 'percent' | 'amount'

export type CrmGoalPayoutRule = {
  threshold: number
  value: number
  mode: CrmGoalPayoutMode
}

export type CrmGoalPayoutPolicy = {
  consultant: CrmGoalPayoutRule[]
  manager: CrmGoalPayoutRule[]
  support: CrmGoalPayoutRule[]
}

export type CrmGoalPayoutResult = {
  amountCents: number
  rule: CrmGoalPayoutRule | null
}

export const DEFAULT_CRM_LIST_USAGE_MIN_ORDERS_FOR_HIGHLIGHT = 5

export const DEFAULT_CRM_LIST_USAGE_TIERS: CrmListUsageTier[] = [
  { id: 'pessimo', label: 'Pessimo', minRate: 0 },
  { id: 'ruim', label: 'Ruim', minRate: 10 },
  { id: 'normal', label: 'Normal', minRate: 50 },
  { id: 'bom', label: 'Bom', minRate: 65 },
  { id: 'otimo', label: 'Otimo', minRate: 80 },
  { id: 'perfeito', label: 'Perfeito', minRate: 100 },
]

export const DEFAULT_CRM_GOAL_PAYOUT_POLICY: CrmGoalPayoutPolicy = {
  consultant: [
    { threshold: 80, value: 1, mode: 'percent' },
    { threshold: 90, value: 2, mode: 'percent' },
    { threshold: 100, value: 3, mode: 'percent' },
    { threshold: 120, value: 3.2, mode: 'percent' },
  ],
  manager: [
    { threshold: 80, value: 0.8, mode: 'percent' },
    { threshold: 90, value: 0.9, mode: 'percent' },
    { threshold: 100, value: 1, mode: 'percent' },
    { threshold: 120, value: 1.2, mode: 'percent' },
  ],
  support: [
    { threshold: 80, value: 80, mode: 'amount' },
    { threshold: 90, value: 90, mode: 'amount' },
    { threshold: 100, value: 100, mode: 'amount' },
    { threshold: 120, value: 120, mode: 'amount' },
  ],
}

function normalizeText(value: unknown) {
  return String(value || '').trim()
}

function positiveRate(value: unknown) {
  const numeric = Number(value || 0)
  if (!Number.isFinite(numeric)) return 0
  return Math.min(1000, Math.max(0, numeric))
}

function normalizeTier(tier: Partial<CrmListUsageTier>, index: number): CrmListUsageTier | null {
  const label = normalizeText(tier?.label)
  if (!label) return null
  const id = normalizeText(tier?.id) || label.toLowerCase().replace(/\s+/g, '-')
  return {
    id: id || `faixa-${index + 1}`,
    label,
    minRate: Math.min(100, positiveRate(tier?.minRate)),
  }
}

function normalizePayoutMode(value: unknown): CrmGoalPayoutMode {
  return normalizeText(value) === 'amount' ? 'amount' : 'percent'
}

function normalizePayoutRule(rule: Partial<CrmGoalPayoutRule>): CrmGoalPayoutRule | null {
  const threshold = positiveRate(rule?.threshold)
  const value = positiveRate(rule?.value)
  if (threshold <= 0 || value <= 0) return null
  return {
    threshold,
    value,
    mode: normalizePayoutMode(rule?.mode),
  }
}

function normalizePayoutRules(
  rules: Partial<CrmGoalPayoutRule>[] | undefined,
  fallback: CrmGoalPayoutRule[],
) {
  const normalized = (Array.isArray(rules) ? rules : [])
    .map(normalizePayoutRule)
    .filter((rule): rule is CrmGoalPayoutRule => Boolean(rule))
    .sort((left, right) => left.threshold - right.threshold)

  return normalized.length ? normalized : fallback.map((rule) => ({ ...rule }))
}

export function normalizeCrmListUsageTiers(value: unknown): CrmListUsageTier[] {
  const tiers = (Array.isArray(value) ? value : [])
    .map((tier, index) => normalizeTier(tier, index))
    .filter((tier): tier is CrmListUsageTier => Boolean(tier))
    .sort((left, right) => left.minRate - right.minRate)

  return tiers.length ? tiers : DEFAULT_CRM_LIST_USAGE_TIERS.map((tier) => ({ ...tier }))
}

export function normalizeCrmListUsageMinOrders(value: unknown) {
  const numeric = Math.trunc(Number(value || 0))
  if (!Number.isFinite(numeric) || numeric <= 0) {
    return DEFAULT_CRM_LIST_USAGE_MIN_ORDERS_FOR_HIGHLIGHT
  }
  return Math.min(999, numeric)
}

export function classifyCrmListUsageRate(rate: unknown, tiers: unknown) {
  const normalizedRate = Math.min(100, positiveRate(rate))
  const normalizedTiers = normalizeCrmListUsageTiers(tiers)
  let selected = normalizedTiers[0]

  for (const tier of normalizedTiers) {
    if (normalizedRate >= tier.minRate) {
      selected = tier
    }
  }

  return selected
}

export function crmListUsageNormalThreshold(tiers: unknown) {
  const normalizedTiers = normalizeCrmListUsageTiers(tiers)
  const normalTier = normalizedTiers.find(
    (tier) => tier.id === 'normal' || tier.label.toLowerCase() === 'normal',
  )
  if (normalTier) return normalTier.minRate

  const firstPositiveTier = normalizedTiers.find((tier) => tier.minRate >= 50)
  return firstPositiveTier?.minRate ?? 50
}

export function normalizeCrmGoalPayoutPolicy(value: unknown): CrmGoalPayoutPolicy {
  const source = value && typeof value === 'object' ? (value as Partial<CrmGoalPayoutPolicy>) : {}

  return {
    consultant: normalizePayoutRules(source.consultant, DEFAULT_CRM_GOAL_PAYOUT_POLICY.consultant),
    manager: normalizePayoutRules(source.manager, DEFAULT_CRM_GOAL_PAYOUT_POLICY.manager),
    support: normalizePayoutRules(source.support, DEFAULT_CRM_GOAL_PAYOUT_POLICY.support),
  }
}

export function resolveCrmGoalPayoutRule(goalProgress: unknown, rules: CrmGoalPayoutRule[] = []) {
  const progress = positiveRate(goalProgress)
  let selected: CrmGoalPayoutRule | null = null

  for (const rule of [...rules].sort((left, right) => left.threshold - right.threshold)) {
    if (progress >= rule.threshold) {
      selected = rule
    }
  }

  return selected
}

export function calculateCrmGoalPayout(
  salesCents: unknown,
  goalProgress: unknown,
  policy: unknown,
  role: keyof CrmGoalPayoutPolicy = 'consultant',
): CrmGoalPayoutResult {
  const normalizedPolicy = normalizeCrmGoalPayoutPolicy(policy)
  const rule = resolveCrmGoalPayoutRule(goalProgress, normalizedPolicy[role])
  if (!rule) return { amountCents: 0, rule: null }

  if (rule.mode === 'amount') {
    return { amountCents: Math.round(rule.value * 100), rule }
  }

  return {
    amountCents: Math.round(Math.max(0, Number(salesCents || 0) || 0) * (rule.value / 100)),
    rule,
  }
}
