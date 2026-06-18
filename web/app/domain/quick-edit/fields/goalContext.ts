/**
 * Contexto e helpers compartilhados pelos descriptors de meta (loja/consultor).
 *
 * Os descriptors são objetos PUROS: toda a infra (store de metas + refresh do CRM)
 * chega via este contexto, montado uma vez na superfície (ex.: ConsultantPlayerCard).
 * O helper `upsertGoalScope` resolve a linha de meta existente do escopo (loja =
 * consultantId vazio; consultor = consultantId preenchido) e faz PATCH se existir,
 * senão POST — usando exclusivamente a API canônica `/v1/operations/goals`.
 */

import type { OperationGoalTarget } from '~/stores/operation-goals'
import type { QuickEditContextBase } from '~/domain/quick-edit/defineQuickEditField'

// Flags de gap da LOJA vindos do payload /v1/erp/crm (contrato congelado).
export interface StoreGoalFlags {
  storeGoalSource: 'own' | 'consultant-sum' | 'none'
  missingStoreGoal: boolean
  missingTicketGoal: boolean
  missingPaGoal: boolean
  splitConsultantCount: number
  // Valores atuais (efetivos) para o popover semear o input.
  avgTicketGoal: number
  paGoal: number
  storeGoal: number
}

// Flags de gap do CONSULTOR vindos do payload /v1/erp/crm (contrato congelado).
export interface ConsultantGoalFlags {
  goalSource: 'own' | 'store-split' | 'none'
  missingMonthlyGoal: boolean
  missingTicketGoal: boolean
  missingPaGoal: boolean
  monthlyGoal: number
}

// Mínimo que o helper de upsert precisa da store de metas (evita acoplar ao tipo
// completo do Pinia e mantém o descriptor testável).
export interface GoalsStoreLike {
  loadGoals: (filters?: Record<string, unknown>) => Promise<OperationGoalTarget[]>
  createGoal: (
    payload: Record<string, unknown>,
  ) => Promise<{ ok: boolean; message?: string; goal?: OperationGoalTarget }>
  updateGoal: (
    goalId: string,
    payload: Record<string, unknown>,
    options?: { reload?: boolean; skipLoadingIndicator?: boolean },
  ) => Promise<{ ok: boolean; message?: string; goal?: OperationGoalTarget }>
}

export interface GoalQuickEditContext extends QuickEditContextBase {
  tenantId: string
  storeId: string
  consultantId: string
  month: string
  store: StoreGoalFlags
  consultant: ConsultantGoalFlags
  goals: GoalsStoreLike
  // Re-hidrata o /v1/erp/crm (store consultants) após salvar.
  refreshCrm: () => Promise<void>
}

function normalizeText(value: unknown): string {
  return String(value ?? '').trim()
}

/**
 * Garante que as metas do escopo (tenant/store/month) estejam carregadas e devolve a
 * linha existente do escopo pedido (store: consultantId vazio; consultant: igual ao id).
 */
async function findExistingGoal(
  ctx: GoalQuickEditContext,
  scope: 'store' | 'consultant',
): Promise<OperationGoalTarget | null> {
  const rows = await ctx.goals.loadGoals({
    tenantId: ctx.tenantId,
    storeId: ctx.storeId,
    month: ctx.month,
  })
  const targetConsultantId = scope === 'consultant' ? normalizeText(ctx.consultantId) : ''
  return (
    (Array.isArray(rows) ? rows : []).find((row) => {
      if (normalizeText(row.storeId) !== normalizeText(ctx.storeId)) return false
      if (row.scope !== scope) return false
      return normalizeText(row.consultantId) === targetConsultantId
    }) ?? null
  )
}

/**
 * Upsert de meta no escopo informado: PATCH na linha existente (preservando os demais
 * campos da meta) ou POST de uma nova. `patch` traz só o(s) campo(s) editado(s).
 */
export async function upsertGoalScope(
  ctx: GoalQuickEditContext,
  scope: 'store' | 'consultant',
  patch: Partial<
    Pick<OperationGoalTarget, 'monthlyGoal' | 'avgTicketGoal' | 'paGoal' | 'conversionGoal'>
  >,
): Promise<void> {
  const existing = await findExistingGoal(ctx, scope)

  if (existing) {
    const result = await ctx.goals.updateGoal(existing.id, {
      monthlyGoal: existing.monthlyGoal,
      avgTicketGoal: existing.avgTicketGoal,
      conversionGoal: existing.conversionGoal,
      paGoal: existing.paGoal,
      ...patch,
    })
    if (!result.ok) {
      throw new Error(result.message || 'Nao foi possivel atualizar a meta.')
    }
    return
  }

  const result = await ctx.goals.createGoal({
    storeId: ctx.storeId,
    consultantId: scope === 'consultant' ? ctx.consultantId : '',
    month: ctx.month,
    monthlyGoal: 0,
    avgTicketGoal: 0,
    conversionGoal: 0,
    paGoal: 0,
    ...patch,
  })
  if (!result.ok) {
    throw new Error(result.message || 'Nao foi possivel cadastrar a meta.')
  }
}
