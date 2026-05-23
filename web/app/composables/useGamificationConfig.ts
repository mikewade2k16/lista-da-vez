import { computed } from 'vue'

import { DEFAULT_SCORE_WEIGHTS as DEFAULT_SCORE_WEIGHT_SETTINGS } from '~/domain/data/operation-templates'
import { useAppRuntimeStore } from '~/stores/app-runtime'

export type BadgeRuleId =
  | 'goal-hit'
  | 'top-rank'
  | 'conversion-above-store'
  | 'ticket-above-goal'
  | 'pa-above-goal'

export interface BadgeRule {
  id: BadgeRuleId
  label: string
  icon: string
  description: string
  enabled: boolean
  threshold?: number
}

export interface ScoreWeights {
  conversion: number
  soldValue: number
  quality: number
  pa: number
  queueDiscipline: number
}

export interface GamificationConfig {
  badges: BadgeRule[]
  scoreWeights: ScoreWeights
}

const DEFAULT_BADGES: BadgeRule[] = [
  {
    id: 'goal-hit',
    label: 'Meta batida',
    icon: '🏆',
    description: 'Atingiu 100% ou mais da meta mensal.',
    enabled: true,
  },
  {
    id: 'top-rank',
    label: 'Top {threshold} do mês',
    icon: '⭐',
    description: 'Está entre os melhores do ranking mensal.',
    enabled: true,
    threshold: 3,
  },
  {
    id: 'conversion-above-store',
    label: 'Conversão acima da loja',
    icon: '⚡',
    description: 'Taxa de conversão acima da média da loja.',
    enabled: true,
  },
  {
    id: 'ticket-above-goal',
    label: 'Ticket acima da meta',
    icon: '🎯',
    description: 'Ticket médio acima da meta cadastrada.',
    enabled: true,
  },
  {
    id: 'pa-above-goal',
    label: 'P.A. acima da meta',
    icon: '📦',
    description: 'Peças por atendimento acima da meta cadastrada.',
    enabled: true,
  },
]

const DEFAULT_SCORE_WEIGHTS: ScoreWeights = {
  conversion: Number(DEFAULT_SCORE_WEIGHT_SETTINGS.scoreWeightConversion || 35),
  soldValue: Number(DEFAULT_SCORE_WEIGHT_SETTINGS.scoreWeightSoldValue || 25),
  quality: Number(DEFAULT_SCORE_WEIGHT_SETTINGS.scoreWeightQuality || 20),
  pa: Number(DEFAULT_SCORE_WEIGHT_SETTINGS.scoreWeightPa || 15),
  queueDiscipline: Number(DEFAULT_SCORE_WEIGHT_SETTINGS.scoreWeightQueueDiscipline || 5),
}

function normalizeWeight(value: unknown, fallback: number) {
  const numericValue = Number(value)
  if (!Number.isFinite(numericValue) || numericValue < 0) {
    return fallback
  }
  return numericValue
}

function resolveScoreWeights(settings: Record<string, unknown> = {}): ScoreWeights {
  return {
    conversion: normalizeWeight(
      settings.scoreWeightConversion,
      DEFAULT_SCORE_WEIGHTS.conversion,
    ),
    soldValue: normalizeWeight(settings.scoreWeightSoldValue, DEFAULT_SCORE_WEIGHTS.soldValue),
    quality: normalizeWeight(settings.scoreWeightQuality, DEFAULT_SCORE_WEIGHTS.quality),
    pa: normalizeWeight(settings.scoreWeightPa, DEFAULT_SCORE_WEIGHTS.pa),
    queueDiscipline: normalizeWeight(
      settings.scoreWeightQueueDiscipline,
      DEFAULT_SCORE_WEIGHTS.queueDiscipline,
    ),
  }
}

export function useGamificationConfig() {
  const runtime = useAppRuntimeStore()
  const config = computed<GamificationConfig>(() => ({
    badges: DEFAULT_BADGES,
    scoreWeights: resolveScoreWeights(runtime.state?.settings || {}),
  }))

  const enabledBadges = computed(() => config.value.badges.filter((badge) => badge.enabled))

  const scoreWeights = computed(() => config.value.scoreWeights)

  return {
    config,
    enabledBadges,
    scoreWeights,
  }
}

export function computeScore360(
  row: {
    conversionRate: number
    soldValue: number
    qualityScore: number
    paScore: number
    queueJumpServices: number
    attendances: number
  },
  context: { maxSold: number; maxPa: number; weights: ScoreWeights },
) {
  const { weights, maxSold, maxPa } = context
  const safeMaxSold = Math.max(maxSold, 1)
  const safeMaxPa = Math.max(maxPa, 0.01)
  const queueShare =
    row.attendances > 0 ? Math.min(1, row.queueJumpServices / row.attendances) : 0

  return (
    (row.conversionRate / 100) * weights.conversion +
    (row.soldValue / safeMaxSold) * weights.soldValue +
    (row.qualityScore / 100) * weights.quality +
    (row.paScore / safeMaxPa) * weights.pa +
    (1 - queueShare) * weights.queueDiscipline
  )
}

export function scoreBreakdown(
  row: {
    conversionRate: number
    soldValue: number
    qualityScore: number
    paScore: number
    queueJumpServices: number
    attendances: number
  },
  context: { maxSold: number; maxPa: number; weights: ScoreWeights },
) {
  const { weights, maxSold, maxPa } = context
  const safeMaxSold = Math.max(maxSold, 1)
  const safeMaxPa = Math.max(maxPa, 0.01)
  const queueShare =
    row.attendances > 0 ? Math.min(1, row.queueJumpServices / row.attendances) : 0

  return [
    {
      key: 'conversion',
      label: 'Conversão',
      weight: weights.conversion,
      contribution: (row.conversionRate / 100) * weights.conversion,
    },
    {
      key: 'soldValue',
      label: 'Valor vendido',
      weight: weights.soldValue,
      contribution: (row.soldValue / safeMaxSold) * weights.soldValue,
    },
    {
      key: 'quality',
      label: 'Qualidade',
      weight: weights.quality,
      contribution: (row.qualityScore / 100) * weights.quality,
    },
    {
      key: 'pa',
      label: 'P.A.',
      weight: weights.pa,
      contribution: (row.paScore / safeMaxPa) * weights.pa,
    },
    {
      key: 'queueDiscipline',
      label: 'Disciplina de fila',
      weight: weights.queueDiscipline,
      contribution: (1 - queueShare) * weights.queueDiscipline,
    },
  ]
}
