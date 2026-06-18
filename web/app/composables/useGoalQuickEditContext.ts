import { computed } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { useConsultantsStore } from '~/stores/consultants'
import { useOperationGoalsStore } from '~/stores/operation-goals'
import { canManageGoalTargets } from '~/domain/utils/permissions'
import type { ErpStorePayout } from '~/domain/utils/consultant-integrated-view'
import type {
  ConsultantGoalFlags,
  GoalQuickEditContext,
  StoreGoalFlags,
} from '~/domain/quick-edit/fields/goalContext'

interface ConsultantFlagsInput {
  goalSource?: ConsultantGoalFlags['goalSource']
  missingMonthlyGoal?: boolean
  missingTicketGoal?: boolean
  missingPaGoal?: boolean
  monthlyGoal?: number
}

function normalizeText(value: unknown): string {
  return String(value ?? '').trim()
}

function currentMonthKey(): string {
  const now = new Date()
  return `${now.getUTCFullYear()}-${String(now.getUTCMonth() + 1).padStart(2, '0')}`
}

// Mês-alvo da meta = o PERÍODO que o /consultor está mostrando (integratedDateFrom),
// não o mês corrente fixo. Assim editar olhando "mês anterior" grava no mês certo.
function resolveMonthKey(periodFrom: unknown): string {
  const match = String(periodFrom ?? '')
    .trim()
    .match(/^(\d{4})-(\d{2})/)
  return match ? `${match[1]}-${match[2]}` : currentMonthKey()
}

/**
 * Centraliza a montagem do GoalQuickEditContext (auth + store de metas + refresh do
 * CRM) usado pelos descriptors de meta. Mantém os componentes "burros": eles só
 * informam escopo + flags e recebem o contexto pronto para o InlineFieldGuard.
 */
export function useGoalQuickEditContext() {
  const auth = useAuthStore()
  const goalsStore = useOperationGoalsStore()
  const consultantsStore = useConsultantsStore()

  const permission = computed(() => ({
    canManageGoalTargets: canManageGoalTargets(
      auth.role,
      auth.permissionKeys,
      auth.permissionsResolved,
    ),
  }))

  // Valores efetivos (ticket/PA) com herança da loja vêm das stats do card; o store
  // payout não os carrega. O chamador informa via `currentTicketGoal`/`currentPaGoal`.
  function buildStoreFlags(
    store: ErpStorePayout | null,
    currentTicketGoal = 0,
    currentPaGoal = 0,
  ): StoreGoalFlags {
    return {
      storeGoalSource: store?.storeGoalSource ?? 'none',
      missingStoreGoal: Boolean(store?.missingStoreGoal),
      missingTicketGoal: Boolean(store?.missingTicketGoal),
      missingPaGoal: Boolean(store?.missingPaGoal),
      splitConsultantCount: Math.max(0, Number(store?.splitConsultantCount || 0) || 0),
      avgTicketGoal: Math.max(0, Number(currentTicketGoal || 0) || 0),
      paGoal: Math.max(0, Number(currentPaGoal || 0) || 0),
      storeGoal: Math.max(0, Number(store?.storeGoal || 0) || 0),
    }
  }

  function buildContext(params: {
    storeId: unknown
    consultantId?: unknown
    store: ErpStorePayout | null
    currentTicketGoal?: number
    currentPaGoal?: number
    storeFlags?: StoreGoalFlags
    consultant?: ConsultantFlagsInput
  }): GoalQuickEditContext {
    const storeFlags =
      params.storeFlags ??
      buildStoreFlags(params.store, params.currentTicketGoal, params.currentPaGoal)
    return {
      permission: permission.value,
      tenantId: normalizeText(auth.activeTenantId || auth.tenantContext?.[0]?.id),
      storeId: normalizeText(params.storeId),
      consultantId: normalizeText(params.consultantId),
      month: resolveMonthKey(consultantsStore.integratedDateFrom),
      store: storeFlags,
      consultant: {
        goalSource: params.consultant?.goalSource ?? 'none',
        missingMonthlyGoal: Boolean(params.consultant?.missingMonthlyGoal),
        missingTicketGoal: Boolean(params.consultant?.missingTicketGoal),
        missingPaGoal: Boolean(params.consultant?.missingPaGoal),
        monthlyGoal: Math.max(0, Number(params.consultant?.monthlyGoal || 0) || 0),
      },
      goals: {
        loadGoals: (filters) => goalsStore.loadGoals(filters),
        createGoal: (payload) => goalsStore.createGoal(payload),
        updateGoal: (goalId, payload, options) => goalsStore.updateGoal(goalId, payload, options),
      },
      refreshCrm: async () => {
        await consultantsStore.refreshIntegratedView()
      },
    }
  }

  return { permission, buildContext }
}
