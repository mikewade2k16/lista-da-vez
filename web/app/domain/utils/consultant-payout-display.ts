/**
 * Helpers de DISPLAY do payout (recebimento por atingimento de meta).
 *
 * O cálculo do valor vive 100% no BACK (pacote Go queue/commission, embutido em
 * GET /v1/erp/crm). O front é só display: estes helpers leem o payout pré-calculado
 * e escolhem o rótulo, sem nunca recalcular nada. mapRoleToPayoutGroup decide se um
 * membro da loja recebe pelo grupo de gerente ou de caixa/suporte.
 */

import {
  mapRoleToPayoutGroup,
  type CrmPayoutRoleGroup,
} from '~/domain/utils/crm-performance-policy'
import type { ErpPayout, ErpStorePayout } from '~/domain/utils/consultant-integrated-view'

// Rótulo do payout do CONSULTOR (% da PRÓPRIA venda). Usa o ruleLabel do back
// quando vier; caso contrário deriva do ratePercent. "Sem faixa" quando ausente.
export function consultantPayoutLabel(payout: ErpPayout | null): string {
  if (!payout) return 'Sem faixa'
  if (payout.ruleLabel) return payout.ruleLabel
  if (payout.ratePercent > 0) {
    return `${payout.ratePercent.toLocaleString('pt-BR')}% da própria venda`
  }
  return 'Sem faixa'
}

// Rótulo do payout de quem recebe pela LOJA (gerente/caixa): % da loja.
export function storeRolePayoutLabel(payout: ErpPayout | null): string {
  if (!payout) return 'Sem faixa'
  if (payout.ruleLabel) return payout.ruleLabel
  if (payout.ratePercent > 0) {
    return `${payout.ratePercent.toLocaleString('pt-BR')}% da loja`
  }
  return 'Sem faixa'
}

// Escolhe managerPayout vs supportPayout da loja conforme o papel. Consultor não
// recebe pela loja (recebe pela própria venda), então retorna null nesse caso.
export function storePayoutForRole(store: ErpStorePayout | null, role: unknown): ErpPayout | null {
  if (!store) return null
  const group: CrmPayoutRoleGroup = mapRoleToPayoutGroup(role)
  if (group === 'manager') return store.managerPayout
  if (group === 'support') return store.supportPayout
  return null
}
