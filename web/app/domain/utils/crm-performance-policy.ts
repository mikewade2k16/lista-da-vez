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

// Base de cálculo do consultor: 'self' = sobre a PRÓPRIA venda; 'store' = sobre o total da loja.
export type CrmConsultantPayoutBase = 'self' | 'store'

// Regras específicas do consultor (editáveis na tela). O cálculo em si vive no BACK
// (pacote Go queue/commission, embutido em GET /v1/erp/crm) — aqui só ficam os tipos
// e o normalize que o editor de Metas CRM usa.
export type CrmConsultantRules = {
  base: CrmConsultantPayoutBase
  qualityPenaltyPercent: number // perda POR métrica (P.A./Ticket) não atingida
  // Gate da LOJA sobre o recebimento do consultor:
  storeFloorPercent: number // loja < isso -> consultor recebe 0
  storeFullPercent: number // loja >= isso -> faixa própria normal (consultant[])
  reducedRate: number // % na faixa reduzida da loja [floor, full)
  reducedRequiresOwnPercent: number // na faixa reduzida, meta própria mínima p/ receber
}

export type CrmGoalPayoutPolicy = {
  consultant: CrmGoalPayoutRule[]
  managerShopping: CrmGoalPayoutRule[]
  managerBairro: CrmGoalPayoutRule[]
  support: CrmGoalPayoutRule[]
  consultantRules: CrmConsultantRules
}

// Grupos que possuem faixas (arrays). consultantRules é config, não faixa.
export type CrmGoalPayoutTierGroup = 'consultant' | 'managerShopping' | 'managerBairro' | 'support'

// Grupo de PAPEL (não distingue Shopping/Bairro — isso é decidido pelo store_type no back).
// Usado APENAS para display: escolher qual payout pré-calculado do back mostrar no card.
// O cálculo do valor é 100% no back (queue/commission).
export type CrmPayoutRoleGroup = 'consultant' | 'manager' | 'support'

const MANAGER_ROLE_TOKENS = ['manager', 'gerente', 'gerencia', 'subgerente', 'lider', 'leader']
const SUPPORT_ROLE_TOKENS = [
  'support',
  'caixa',
  'cashier',
  'auxiliar',
  'assistant',
  'estoquista',
  'estoque',
  'financeiro',
  'recepcao',
]

export function mapRoleToPayoutGroup(role: unknown): CrmPayoutRoleGroup {
  const normalized = normalizeText(role).toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '')
  if (!normalized) return 'consultant'
  if (MANAGER_ROLE_TOKENS.some((token) => normalized.includes(token))) return 'manager'
  if (SUPPORT_ROLE_TOKENS.some((token) => normalized.includes(token))) return 'support'
  return 'consultant'
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

export const DEFAULT_CRM_CONSULTANT_RULES: CrmConsultantRules = {
  base: 'self',
  qualityPenaltyPercent: 0.1,
  storeFloorPercent: 50,
  storeFullPercent: 80,
  reducedRate: 1.5,
  reducedRequiresOwnPercent: 100,
}

export const DEFAULT_CRM_GOAL_PAYOUT_POLICY: CrmGoalPayoutPolicy = {
  // Faixa pela PRÓPRIA meta do consultor -> % sobre a própria venda (vale quando a
  // loja >= storeFullPercent). O gate da loja vive em consultantRules.
  consultant: [
    { threshold: 80, value: 1, mode: 'percent' },
    { threshold: 90, value: 2, mode: 'percent' },
    { threshold: 100, value: 3, mode: 'percent' },
    { threshold: 120, value: 3.2, mode: 'percent' },
  ],
  managerShopping: [
    { threshold: 80, value: 0.8, mode: 'percent' },
    { threshold: 90, value: 0.9, mode: 'percent' },
    { threshold: 100, value: 1, mode: 'percent' },
    { threshold: 120, value: 1.2, mode: 'percent' },
  ],
  managerBairro: [
    { threshold: 80, value: 1, mode: 'percent' },
    { threshold: 100, value: 1.7, mode: 'percent' },
    { threshold: 120, value: 2, mode: 'percent' },
  ],
  support: [
    { threshold: 80, value: 80, mode: 'amount' },
    { threshold: 90, value: 90, mode: 'amount' },
    { threshold: 100, value: 100, mode: 'amount' },
    { threshold: 120, value: 120, mode: 'amount' },
  ],
  consultantRules: { ...DEFAULT_CRM_CONSULTANT_RULES },
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

function normalizePayoutRule(
  rule: Partial<CrmGoalPayoutRule> | null | undefined,
): CrmGoalPayoutRule | null {
  if (!rule || typeof rule !== 'object') return null
  return {
    threshold: positiveRate(rule.threshold),
    value: positiveRate(rule.value),
    mode: normalizePayoutMode(rule.mode),
  }
}

// Preserva array vazio EXPLICITO (usuario removeu todas as faixas do grupo) — so cai no
// fallback de defaults quando o campo vem AUSENTE/nao-array. Sem isso, "remover ate zero" na
// pagina de Metas CRM voltava sempre aos defaults.
function normalizePayoutRules(
  rules: Partial<CrmGoalPayoutRule>[] | undefined,
  fallback: CrmGoalPayoutRule[],
) {
  if (!Array.isArray(rules)) {
    return fallback.map((rule) => ({ ...rule }))
  }

  return rules
    .map(normalizePayoutRule)
    .filter((rule): rule is CrmGoalPayoutRule => Boolean(rule))
    .sort((left, right) => left.threshold - right.threshold)
}

function normalizeConsultantBase(value: unknown): CrmConsultantPayoutBase {
  return normalizeText(value) === 'store' ? 'store' : 'self'
}

function normalizeConsultantRules(value: unknown): CrmConsultantRules {
  const source = value && typeof value === 'object' ? (value as Partial<CrmConsultantRules>) : {}
  const pick = (input: unknown, fallback: number) =>
    input === undefined || input === null ? fallback : positiveRate(input)
  return {
    base: normalizeConsultantBase(source.base),
    qualityPenaltyPercent: pick(
      source.qualityPenaltyPercent,
      DEFAULT_CRM_CONSULTANT_RULES.qualityPenaltyPercent,
    ),
    storeFloorPercent: pick(
      source.storeFloorPercent,
      DEFAULT_CRM_CONSULTANT_RULES.storeFloorPercent,
    ),
    storeFullPercent: pick(source.storeFullPercent, DEFAULT_CRM_CONSULTANT_RULES.storeFullPercent),
    reducedRate: pick(source.reducedRate, DEFAULT_CRM_CONSULTANT_RULES.reducedRate),
    reducedRequiresOwnPercent: pick(
      source.reducedRequiresOwnPercent,
      DEFAULT_CRM_CONSULTANT_RULES.reducedRequiresOwnPercent,
    ),
  }
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

// Normaliza a política v2. Retrocompatível: linhas antigas têm apenas
// `manager` (sem distinção de loja) — semeamos managerShopping E managerBairro a
// partir dela; faixas explicitamente vazias são preservadas; campos ausentes caem
// no default. O CÁLCULO em si é feito no back (queue/commission) — aqui só o shape.
export function normalizeCrmGoalPayoutPolicy(value: unknown): CrmGoalPayoutPolicy {
  const source =
    value && typeof value === 'object'
      ? (value as Partial<CrmGoalPayoutPolicy> & { manager?: Partial<CrmGoalPayoutRule>[] })
      : {}

  const legacyManager = source.manager

  return {
    consultant: normalizePayoutRules(source.consultant, DEFAULT_CRM_GOAL_PAYOUT_POLICY.consultant),
    managerShopping: normalizePayoutRules(
      Array.isArray(source.managerShopping) ? source.managerShopping : legacyManager,
      DEFAULT_CRM_GOAL_PAYOUT_POLICY.managerShopping,
    ),
    managerBairro: normalizePayoutRules(
      Array.isArray(source.managerBairro) ? source.managerBairro : legacyManager,
      DEFAULT_CRM_GOAL_PAYOUT_POLICY.managerBairro,
    ),
    support: normalizePayoutRules(source.support, DEFAULT_CRM_GOAL_PAYOUT_POLICY.support),
    consultantRules: normalizeConsultantRules(source.consultantRules),
  }
}
