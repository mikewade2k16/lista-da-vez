import { setApiAccountIdProvider } from '~/utils/api-client'
import { useAuthStore } from '~/stores/auth'
import { useCoreAccountStore } from '../../layers/core/stores/account'

// Plugin client-only que liga a account ativa do Core V2 ao api-client. A partir
// dele, todo request multi-tenant (queue/crm/tasks/...) carrega automaticamente
// o header X-Account-Id que o backend exige (RequireModuleByPath). Isolar a
// ligacao em plugin evita import direto dos stores dentro do api-client e a
// dependencia circular que isso causaria (stores usam o api-client no setup do
// pinia). O provider le os stores a cada request, entao acompanha a troca de
// account no CoreAccountSwitcher de forma reativa.
export default defineNuxtPlugin(() => {
  const auth = useAuthStore()
  const account = useCoreAccountStore()

  setApiAccountIdProvider(() => {
    // TODO remover quando activeAccountId for sempre populado no boot.
    return account.activeAccountId || auth.activeTenantId || ''
  })
})
