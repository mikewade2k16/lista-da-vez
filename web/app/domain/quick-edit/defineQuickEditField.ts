/**
 * Motor PLUGÁVEL de aviso acionável + edição inline (quick-edit).
 *
 * Um QuickEditFieldDescriptor é um objeto PURO que declara, para um dado que pode
 * faltar e cuja ausência altera o resultado exibido:
 *  - quando mostrar o aviso (isMissing) e qual texto (warning);
 *  - quem pode editar (canEdit, espelhando o gate do back);
 *  - o valor atual (current) e como gravá-lo pela API canônica (save);
 *  - como re-hidratar do back depois de salvar (afterSave).
 *
 * Adicionar um caso novo (qualquer dado faltante editável, em qualquer tela) =
 * escrever 1 descriptor + soltar <InlineFieldGuard :descriptor :context />.
 * Zero lógica bespoke por tela, zero fork de componente.
 */

// Tipo do input que o popover renderiza para o campo.
export type QuickEditFieldType = 'number' | 'percent' | 'currency'

// Permissões disponíveis para o gate de edição (espelha o back).
export interface QuickEditPermission {
  canManageGoalTargets: boolean
}

/**
 * Contexto entregue ao guard/descriptor. Carrega o escopo (tenant/store/consultant/
 * month), os flags de gap vindos do payload e os "serviços" (store de metas +
 * refresh do CRM) para o descriptor gravar pela API canônica e re-hidratar do back.
 *
 * `TContext` é genérico para cada superfície montar o shape que precisa; o motor só
 * exige que `permission` exista. Os descriptors tipam o que consomem.
 */
export interface QuickEditContextBase {
  permission: QuickEditPermission
}

export interface QuickEditFieldDescriptor<TContext extends QuickEditContextBase> {
  // Identificador estável do campo (ex.: 'storeTicketGoal').
  id: string
  // Rótulo curto exibido no popover.
  label: string
  // Tipo do input.
  type: QuickEditFieldType
  // Texto auxiliar opcional sob o input (unidade, dica).
  hint?: string
  // Quando mostrar o aviso (lê os flags do payload no contexto).
  isMissing: (ctx: TContext) => boolean
  // Texto do aviso acionável.
  warning: (ctx: TContext) => string
  // Gate de edição, espelhando o back.
  canEdit: (permission: QuickEditPermission) => boolean
  // Valor atual do campo (do payload). null = ainda sem valor.
  current: (ctx: TContext) => number | null
  // Grava via API canônica do recurso. Lança em caso de erro.
  save: (value: number, ctx: TContext) => Promise<void>
  // Re-hidrata do back após salvar (regra "Nada hardcoded").
  afterSave: (ctx: TContext) => Promise<void> | void
}

/**
 * Helper de identidade que apenas fixa o tipo do descriptor. Mantém os descriptors
 * declarativos e tipados sem boilerplate.
 */
export function defineQuickEditField<TContext extends QuickEditContextBase>(
  descriptor: QuickEditFieldDescriptor<TContext>,
): QuickEditFieldDescriptor<TContext> {
  return descriptor
}
