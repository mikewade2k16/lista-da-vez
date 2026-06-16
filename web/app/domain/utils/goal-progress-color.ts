export type GoalProgressTier = 'none' | 'low' | 'mid' | 'high' | 'hit'

// Faixa de cor do atingimento de meta, compartilhada pelo gauge individual do consultor e
// pela barra de % da loja. As cores em si ficam nos componentes (tokens do design system):
//   none -> muted/sem meta | low -> danger | mid -> accent-warning | high -> primary | hit -> success
export function goalProgressTier(progress: unknown, hasGoal = true): GoalProgressTier {
  if (!hasGoal) return 'none'
  const value = Number(progress)
  if (!Number.isFinite(value) || value <= 0) return 'low'
  if (value >= 100) return 'hit'
  if (value >= 80) return 'high'
  if (value >= 50) return 'mid'
  return 'low'
}
