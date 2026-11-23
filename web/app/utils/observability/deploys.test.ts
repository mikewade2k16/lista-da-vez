import { describe, expect, it } from 'vitest'

import type { ObsDeployStage } from '~/types/observability'
import {
  formatDeployDuration,
  shortSha,
  slowestStageIndex,
  stageBarPercent,
  verdictBand,
} from './deploys'

const stage = (name: string, durationMs: number | null, depth = 0): ObsDeployStage => ({
  name,
  depth,
  status: 'ok',
  durationMs,
})

describe('deploys utils', () => {
  it('formata duracao em mm:ss', () => {
    expect(formatDeployDuration(221_533)).toBe('03:42')
    expect(formatDeployDuration(736_489)).toBe('12:16')
    expect(formatDeployDuration(0)).toBe('00:00')
    expect(formatDeployDuration(null)).toBe('-')
    expect(formatDeployDuration(-1)).toBe('-')
  })

  it('mapeia veredito para faixa de cor', () => {
    expect(verdictBand('ok')).toBe('good')
    expect(verdictBand('slow')).toBe('warn')
    expect(verdictBand('failed')).toBe('bad')
  })

  it('abrevia o commit', () => {
    expect(shortSha('0123456789abcdef')).toBe('0123456')
    expect(shortSha('')).toBe('-')
  })

  it('acha a etapa mais lenta ignorando filhos e sem duracao', () => {
    const stages = [stage('a', 1000), stage('filho', 9000, 1), stage('b', 5000), stage('c', null)]
    expect(slowestStageIndex(stages)).toBe(2)
    expect(slowestStageIndex([stage('x', null)])).toBe(-1)
  })

  it('barra proporcional a maior duracao', () => {
    const stages = [stage('a', 1000), stage('b', 4000), stage('c', null)]
    expect(stageBarPercent(stages[1]!, stages)).toBe(100)
    expect(stageBarPercent(stages[0]!, stages)).toBe(25)
    expect(stageBarPercent(stages[2]!, stages)).toBe(0)
  })
})
