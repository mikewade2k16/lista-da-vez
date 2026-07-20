import { computed } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { useCoreAccountStore } from '../../layers/core/stores/account'
import type { AuthUser, UserRole } from '~/types'

// COSTURA (F1) — adaptador temporario do modulo omnichannel portado do legado.
// Alvo de remocao: F14 (docs/LEGADO.md).
//
// O `useAdminSession` do legado devolve ~20 membros; o modulo portado consome 8
// (conferido call-site a call-site). Costura e adaptador, nao reimplementacao:
// aqui ficam so os 8. Ver docs/omnichannel/specs/OMNI-F1.md C4.
//
// Fonte unica: tudo deriva de useAuthStore (sessao/JWT) + useCoreAccountStore
// (conta ativa). Este composable NAO guarda estado proprio — se guardasse, seria
// uma segunda verdade de sessao.

// O front legado gateia por um papel de 4 valores; o Omni tem o seu vocabulario
// no banco (core.roles). Este mapa e a traducao — e o unico lugar onde ela existe.
const LEGACY_ROLE_BY_APP_ROLE: Record<string, UserRole> = {
  platform_admin: 'ADMIN',
  owner: 'ADMIN',
  director: 'SUPERVISOR',
  manager: 'SUPERVISOR',
  marketing: 'AGENT',
  consultant: 'AGENT',
  store_terminal: 'VIEWER',
}

export function useAdminSession() {
  const auth = useAuthStore()
  const account = useCoreAccountStore()

  // Papel desconhecido cai em VIEWER (menor privilegio), nunca em ADMIN.
  const legacyRole = computed<UserRole>(
    () => LEGACY_ROLE_BY_APP_ROLE[String(auth.role || '').trim()] ?? 'VIEWER',
  )

  // Exibicao apenas — NUNCA escopo. O escopo de conta viaja no X-Account-Id,
  // injetado pelo provider global (plugins/account-id-bridge.client.ts).
  const tenantSlug = computed(() => String(account.activeAccount?.slug || ''))

  const user = computed<AuthUser | null>(() => {
    const current = auth.user as Record<string, unknown> | null
    if (!current) {
      return null
    }

    return {
      id: String(current.id || ''),
      // "tenant" do legado == conta ativa do Omni (core.accounts). O modulo usa
      // isto so como chave de preferencia local e no otimista de mensagem.
      tenantId: String(account.activeAccountId || ''),
      tenantSlug: tenantSlug.value,
      email: String(current.email || ''),
      name: String(current.displayName || current.name || current.email || ''),
      nick: (current.nick as string | null | undefined) ?? null,
      profileImage: (current.avatarPath as string | null | undefined) ?? null,
      role: legacyRole.value,
    }
  })

  // O legado separava sessao legada (`user`/`token`) de sessao core
  // (`coreUser`/`coreToken`) porque conviviam dois backends. No Omni ha uma so
  // sessao — os dois pares apontam para ela. Os call-sites que fazem
  // `coreToken.value || token.value` continuam corretos.
  const coreUser = computed(() => user.value)
  const token = computed(() => String(auth.accessToken || ''))
  const coreToken = computed(() => token.value)

  async function logout() {
    await auth.logout()
  }

  // No legado, trocar de cliente devolvia um token novo do BFF e este metodo o
  // gravava na sessao. No Omni quem troca de conta e o switcher do shell
  // (useCoreAccountStore) — a rota /v1/omnichannel/session/context nao existe e
  // nao esta prevista em nenhuma fase. Portanto: NAO trocamos token aqui (aceitar
  // um token vindo de resposta e escrever na sessao seria inventar contrato de
  // auth). Reconciliamos o contexto a partir da fonte canonica.
  async function syncSessionFromToken(_payload?: {
    accessToken?: string
    sessionContext?: unknown
    redirectToLogin?: boolean
  }) {
    await auth.fetchContext()
  }

  return {
    user,
    coreUser,
    token,
    coreToken,
    legacyRole,
    tenantSlug,
    logout,
    syncSessionFromToken,
  }
}
