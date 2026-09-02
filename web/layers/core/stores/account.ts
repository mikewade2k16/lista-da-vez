import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { AUTH_TOKEN_COOKIE, createApiRequest } from '~/utils/api-client'

export interface AccountSummary {
  id: string
  name: string
  slug: string
  organizationId: string
  planCode: string
  modules: string[]
  // Contrato com /v2/me/accounts (backend multitenant): isAgency marca a conta
  // que e o workspace da agencia (ex.: Crow Visuals); organizationName e o nome
  // da org da conta ('' quando nenhuma). Ambos podem vir undefined ate o backend
  // + rebuild entrarem — normalizados com default em fetchAccounts.
  isAgency: boolean
  organizationName: string
}

export interface RoleSummary {
  id: string
  code: string
  label: string
  isLocked: boolean
}

export interface AccountContext {
  account: AccountSummary
  user: { id: string; name: string; email: string }
  roles: RoleSummary[]
  permissions: string[]
  org: { id: string; name: string; slug: string } | null
}

const ACTIVE_ACCOUNT_COOKIE = 'ldv_active_account_id'

export function isSelectableAccountId(accounts: AccountSummary[], accountId: string): boolean {
  const normalized = String(accountId || '').trim()
  return Boolean(normalized && accounts.some((account) => account.id === normalized))
}

export const useCoreAccountStore = defineStore('core/account', () => {
  const runtimeConfig = useRuntimeConfig()
  const tokenCookie = useCookie(AUTH_TOKEN_COOKIE)
  const activeAccountCookie = useCookie(ACTIVE_ACCOUNT_COOKIE)

  const accounts = ref<AccountSummary[]>([])
  const activeAccountId = ref<string>('')
  const context = ref<AccountContext | null>(null)
  const loading = ref(false)
  const error = ref<string>('')
  // accountsLoaded distingue "ainda hidratando" de "resolveu" (com ou sem conta
  // ativa). Vira true no finally do fetchAccounts (sucesso OU erro): assim o
  // gating de modulo pode FECHAR (fail-closed) quando o contexto ja resolveu sem
  // conta ativa, sem causar flash durante o hidrate inicial (quando ainda false).
  const accountsLoaded = ref(false)
  // platformView: contexto SUPER-ADMIN/DEV (so platform_admin). Quando ativo, o
  // menu revela itens em desenvolvimento/`hidden` que nao foram liberados nem
  // para a conta-agencia (Crow Visuals). Escopo de API usa a conta-agencia (tem
  // todos os modulos). Selecionar qualquer org/cliente desliga este modo.
  const platformView = ref(false)

  const activeAccount = computed(
    () => accounts.value.find((a) => a.id === activeAccountId.value) ?? null,
  )

  const permissions = computed(() => context.value?.permissions ?? [])
  const enabledModules = computed(() => activeAccount.value?.modules ?? [])

  const api = createApiRequest(runtimeConfig, () => tokenCookie.value ?? '')

  // Dedupe de chamadas concorrentes a fetchAccounts. No login com defer de runtime,
  // o syncRuntimeAccess (background) e o auth.global.ts podem disparar fetchAccounts
  // ao mesmo tempo na 1a navegacao — sem isto, duas requests a /v2/me/accounts em
  // paralelo. Memoiza a promise em voo (mesmo padrao do ensureSession no auth store).
  let fetchAccountsPromise: Promise<void> | null = null

  async function fetchAccounts() {
    if (fetchAccountsPromise) {
      return fetchAccountsPromise
    }
    fetchAccountsPromise = runFetchAccounts()
    try {
      await fetchAccountsPromise
    } finally {
      fetchAccountsPromise = null
    }
  }

  async function runFetchAccounts() {
    loading.value = true
    error.value = ''
    try {
      // Fase 7C: paraleliza /v2/me/accounts com /v2/me/context speculativo
      // usando o accountId do cookie. Se o cookie estiver correto (caso
      // comum), os dois requests rodam em paralelo e cortamos uma round-trip
      // do bootstrap. Se o cookie nao bater com a lista retornada, refaz o
      // fetchContext com o accountId correto.
      const savedId = String(activeAccountCookie.value ?? '').trim()
      const accountsPromise = api('/v2/me/accounts') as Promise<any>
      const speculativeContextPromise = savedId
        ? (api(`/v2/me/context?accountId=${savedId}`) as Promise<any>).catch(() => null)
        : Promise.resolve(null)

      const [accountsData, speculativeContext] = await Promise.all([
        accountsPromise,
        speculativeContextPromise,
      ])

      // Normaliza os campos novos do contrato (isAgency/organizationName) com
      // defaults defensivos: ate o backend + rebuild entrarem eles vem undefined.
      const rawAccounts: Partial<AccountSummary>[] = Array.isArray(accountsData.accounts)
        ? accountsData.accounts
        : []
      accounts.value = rawAccounts.map(
        (a): AccountSummary => ({
          ...(a as AccountSummary),
          isAgency: Boolean(a?.isAgency),
          organizationName: a?.organizationName ?? '',
        }),
      )

      const found = accounts.value.find((a) => a.id === savedId)
      activeAccountId.value =
        found?.id ?? accountsData.defaultAccountId ?? accounts.value[0]?.id ?? ''

      // Cookie stale de sessao anterior (outro usuario/account no mesmo browser):
      // se o id salvo nao pertence a este usuario, corrige o cookie para a account
      // resolvida. Sem isso, o X-Account-Id global (account-id-bridge le este
      // store) poderia carregar a account de outra sessao e gerar 403/module
      // disabled nas rotas de queue/crm.
      if (savedId && !found) {
        activeAccountCookie.value = activeAccountId.value || null
      }

      const speculativeAccountId = speculativeContext?.context?.account?.id ?? ''
      if (
        speculativeContext &&
        activeAccountId.value &&
        speculativeAccountId === activeAccountId.value
      ) {
        context.value = speculativeContext.context ?? null
      } else if (activeAccountId.value) {
        await fetchContext(activeAccountId.value)
      }
    } catch (e: any) {
      error.value = e?.data?.error?.message ?? e?.message ?? 'Erro ao carregar accounts.'
    } finally {
      loading.value = false
      // Marca que a tentativa terminou (sucesso ou erro). A partir daqui, gating
      // de modulo sem conta ativa pode fechar (fail-closed) — antes disso o
      // contexto ainda nao resolveu e o filtro nao deve esconder itens.
      accountsLoaded.value = true
    }
  }

  async function fetchContext(accountId: string) {
    try {
      const data = (await api(`/v2/me/context?accountId=${accountId}`)) as any
      context.value = data.context ?? null
    } catch {
      context.value = null
    }
  }

  async function switchAccount(accountId: string) {
    const normalizedAccountId = String(accountId || '').trim()
    if (!isSelectableAccountId(accounts.value, normalizedAccountId)) {
      throw new Error('A conta selecionada não pertence ao contexto autorizado.')
    }
    // Sair do modo dev ao escolher uma conta real (org ou cliente).
    platformView.value = false
    // Fecha o contexto anterior antes de trocar o X-Account-Id reativo. Os
    // consumidores do Omnichannel observam activeAccountId com flush sync e
    // limpam projeções/caches antes de qualquer bootstrap da conta nova.
    context.value = null
    activeAccountId.value = normalizedAccountId
    activeAccountCookie.value = normalizedAccountId
    await fetchContext(normalizedAccountId)
  }

  // enterPlatformView ativa o contexto super-admin/dev: escopa na conta-agencia
  // (X-Account-Id valido, todos os modulos) e liga o flag que revela os itens em
  // desenvolvimento/`hidden`. So faz sentido para platform_admin (o switcher ja
  // e gateado a esse papel).
  async function enterPlatformView() {
    const agency = accounts.value.find((a) => a.isAgency)
    if (agency && agency.id !== activeAccountId.value) {
      await switchAccount(agency.id)
    }
    platformView.value = true
  }

  function hasPermission(key: string): boolean {
    return permissions.value.includes(key)
  }

  function reset() {
    accounts.value = []
    activeAccountId.value = ''
    context.value = null
    error.value = ''
    platformView.value = false
    accountsLoaded.value = false
  }

  return {
    accounts,
    activeAccountId,
    activeAccount,
    context,
    loading,
    error,
    permissions,
    enabledModules,
    accountsLoaded,
    platformView,
    fetchAccounts,
    switchAccount,
    enterPlatformView,
    hasPermission,
    reset,
  }
})
