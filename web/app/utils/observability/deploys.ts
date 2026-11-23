import type { ObsDeploy, ObsDeployStage, ObsDeployVerdict } from '~/types/observability'
import type { Band } from './format'

// Logica pura da aba "Deploys" do painel de observabilidade.

// "mm:ss" (ex.: 03:42; 12:16). Sem duracao vira "-".
export function formatDeployDuration(ms: number | null | undefined): string {
  if (ms === null || ms === undefined || !Number.isFinite(ms) || ms < 0) return '-'
  const total = Math.round(ms / 1000)
  const minutes = Math.floor(total / 60)
  const seconds = total % 60
  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
}

export const VERDICT_LABEL: Record<ObsDeployVerdict, string> = {
  ok: 'OK',
  slow: 'Lento',
  failed: 'Falhou',
}

export function verdictBand(verdict: ObsDeployVerdict): Band {
  if (verdict === 'ok') return 'good'
  if (verdict === 'slow') return 'warn'
  return 'bad'
}

export const STATUS_TEXT: Record<ObsDeploy['status'], string> = {
  success: 'Sucesso',
  failed: 'Falhou',
  interrupted: 'Interrompido',
  unknown: 'Desconhecido',
}

export function shortSha(sha: string | null | undefined): string {
  return sha ? sha.slice(0, 7) : '-'
}

// Indice da etapa mais lenta (so as de nivel 0, para pai e filhos nao se
// sobreporem na comparacao). -1 quando nenhuma tem duracao.
export function slowestStageIndex(stages: ObsDeployStage[]): number {
  let best = -1
  let bestMs = 0
  stages.forEach((stage, index) => {
    if (stage.depth !== 0 || stage.durationMs === null) return
    if (stage.durationMs > bestMs) {
      best = index
      bestMs = stage.durationMs
    }
  })
  return best
}

// Largura da barra (0-100) proporcional a duracao da etapa mais longa do deploy.
export function stageBarPercent(stage: ObsDeployStage, stages: ObsDeployStage[]): number {
  const max = Math.max(0, ...stages.map((item) => item.durationMs ?? 0))
  if (max <= 0 || stage.durationMs === null) return 0
  return Math.max(1, Math.round((stage.durationMs / max) * 100))
}
