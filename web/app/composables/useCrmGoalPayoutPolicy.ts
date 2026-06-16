import { computed } from 'vue'

import {
  normalizeCrmGoalPayoutPolicy,
  type CrmGoalPayoutPolicy,
} from '~/domain/utils/crm-performance-policy'
import { useAppRuntimeStore } from '~/stores/app-runtime'

/**
 * Le a politica de recebimento por atingimento de meta (crmGoalPayoutPolicy) a partir das
 * settings de operacao no runtime — mesma fonte usada pela pagina Configuracoes > Metas CRM.
 * Espelha o padrao de useGamificationConfig.
 */
export function useCrmGoalPayoutPolicy() {
  const runtime = useAppRuntimeStore()
  const policy = computed<CrmGoalPayoutPolicy>(() =>
    normalizeCrmGoalPayoutPolicy(runtime.state?.settings?.crmGoalPayoutPolicy),
  )

  return { policy }
}
