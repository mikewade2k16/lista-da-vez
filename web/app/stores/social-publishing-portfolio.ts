import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { useCoreAccountStore } from '../../layers/core/stores/account'
import * as publishingApi from '~/domain/social-publishing/social-publishing-api'
import type {
  SocialPublishingPortfolio,
  SocialPublishingScope,
} from '~/domain/social-publishing/model'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

interface RequestContext {
  activeAccountId: string
  scopeHostId: string
  generation: number
}

export const useSocialPublishingPortfolioStore = defineStore('social-publishing-portfolio', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const accountStore = useCoreAccountStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const scope = ref<SocialPublishingScope | null>(null)
  const portfolio = ref<SocialPublishingPortfolio | null>(null)
  const selectedClientId = ref('')
  const scopeResolved = ref(false)
  const loadingScope = ref(false)
  const loadingPortfolio = ref(false)
  const switching = ref(false)
  const error = ref('')

  let generation = 0
  let scopeRequestVersion = 0
  let portfolioRequestVersion = 0

  const canSelect = computed(() => scope.value?.canSelect === true)
  const portfolioMode = computed(() => canSelect.value && selectedClientId.value === '')
  const canReadPortfolio = computed(
    () =>
      auth.role === 'platform_admin' ||
      auth.effectivePermissionKeys.includes('social_publishing.analytics'),
  )

  function currentAccountId(): string {
    return String(accountStore.activeAccountId || '').trim()
  }

  function resolveScopeHostId(): string {
    const active = accountStore.activeAccount
    const activeId = currentAccountId()
    if (active?.isAgency && active.modules.includes('social_publishing')) return active.id

    const agencies = accountStore.accounts.filter(
      (account) => account.isAgency && account.modules.includes('social_publishing'),
    )
    const sameOrganization = active?.organizationId
      ? agencies.find((account) => account.organizationId === active.organizationId)
      : null
    if (sameOrganization) return sameOrganization.id
    return agencies.length === 1 ? agencies[0]?.id || activeId : activeId
  }

  function captureContext(): RequestContext {
    return {
      activeAccountId: currentAccountId(),
      scopeHostId: resolveScopeHostId(),
      generation,
    }
  }

  function isCurrent(context: RequestContext): boolean {
    return (
      context.generation === generation &&
      Boolean(context.activeAccountId) &&
      context.activeAccountId === currentAccountId() &&
      context.scopeHostId === resolveScopeHostId()
    )
  }

  function invalidateRequests(): void {
    generation += 1
    scopeRequestVersion += 1
    portfolioRequestVersion += 1
    loadingScope.value = false
    loadingPortfolio.value = false
  }

  async function loadScope(): Promise<SocialPublishingScope | null> {
    await auth.ensureSession()
    const context = captureContext()
    const requestVersion = ++scopeRequestVersion
    scopeResolved.value = false
    loadingScope.value = true
    error.value = ''
    portfolio.value = null
    if (!auth.isAuthenticated || !context.activeAccountId || !context.scopeHostId) {
      scope.value = null
      scopeResolved.value = true
      loadingScope.value = false
      error.value = 'Não foi possível confirmar a conta ativa.'
      return null
    }

    try {
      const next = await publishingApi.fetchPublishingScope(apiRequest, context.scopeHostId)
      if (!isCurrent(context) || requestVersion !== scopeRequestVersion) return null

      if (
        !next.canSelect &&
        (!next.lockedClientId || !next.clients.some((client) => client.id === next.lockedClientId))
      ) {
        scope.value = null
        selectedClientId.value = ''
        error.value = 'Não foi possível confirmar o cliente autorizado para esta conta.'
        return null
      }

      scope.value = next
      const activeAccount = accountStore.activeAccount
      const activeClientVisible = next.clients.some(
        (client) => client.id === context.activeAccountId,
      )
      if (next.canSelect && !activeAccount?.isAgency && !activeClientVisible) {
        scope.value = null
        selectedClientId.value = ''
        error.value = 'A conta ativa não pertence ao escopo autorizado de postagens.'
        return null
      }
      if (!next.canSelect && next.lockedClientId !== context.activeAccountId) {
        scope.value = null
        selectedClientId.value = ''
        error.value = 'O cliente autorizado não corresponde à conta ativa.'
        return null
      }

      selectedClientId.value = next.canSelect
        ? activeAccount?.isAgency
          ? ''
          : context.activeAccountId
        : next.lockedClientId
      return next
    } catch (caught) {
      if (isCurrent(context) && requestVersion === scopeRequestVersion) {
        scope.value = null
        selectedClientId.value = ''
        error.value = getApiErrorMessage(
          caught,
          'Não foi possível carregar os clientes de postagens.',
        )
      }
      return null
    } finally {
      if (requestVersion === scopeRequestVersion) {
        loadingScope.value = false
        if (isCurrent(context)) scopeResolved.value = true
      }
    }
  }

  async function loadPortfolio(): Promise<boolean> {
    const context = captureContext()
    const requestVersion = ++portfolioRequestVersion
    if (
      !auth.isAuthenticated ||
      !context.activeAccountId ||
      !context.scopeHostId ||
      !scope.value?.canSelect ||
      selectedClientId.value ||
      !canReadPortfolio.value
    ) {
      return false
    }

    loadingPortfolio.value = true
    error.value = ''
    try {
      const next = await publishingApi.fetchPublishingPortfolio(apiRequest, context.scopeHostId)
      if (!isCurrent(context) || requestVersion !== portfolioRequestVersion) return false

      const authorizedIds = new Set(scope.value.clients.map((client) => client.id))
      const internallyConsistent =
        next.clientCount === next.clients.length &&
        next.connectedClients <= next.clientCount &&
        next.clients.every((client) => authorizedIds.has(client.accountId))
      if (!internallyConsistent) {
        portfolio.value = null
        error.value = 'O consolidado retornou um escopo de clientes inválido.'
        return false
      }

      portfolio.value = next
      return true
    } catch (caught) {
      if (isCurrent(context) && requestVersion === portfolioRequestVersion) {
        portfolio.value = null
        error.value = getApiErrorMessage(
          caught,
          'Não foi possível carregar o portfólio de postagens.',
        )
      }
      return false
    } finally {
      if (isCurrent(context) && requestVersion === portfolioRequestVersion) {
        loadingPortfolio.value = false
      }
    }
  }

  function selectClient(clientId: string): boolean {
    if (!scope.value?.canSelect) return false
    const normalized = String(clientId || '').trim()
    if (normalized && !scope.value.clients.some((client) => client.id === normalized)) {
      return false
    }
    if (normalized === selectedClientId.value) return false
    portfolioRequestVersion += 1
    loadingPortfolio.value = false
    portfolio.value = null
    error.value = ''
    selectedClientId.value = normalized
    return true
  }

  function prepareAccountSwitch(): void {
    invalidateRequests()
    portfolio.value = null
    error.value = ''
  }

  function setSwitching(value: boolean): void {
    switching.value = value
  }

  function setError(message: string): void {
    error.value = String(message || '').trim()
  }

  function reset(): void {
    invalidateRequests()
    scope.value = null
    portfolio.value = null
    selectedClientId.value = ''
    scopeResolved.value = false
    switching.value = false
    error.value = ''
  }

  return {
    scope,
    portfolio,
    selectedClientId,
    scopeResolved,
    loadingScope,
    loadingPortfolio,
    switching,
    error,
    scopeHostId: computed(resolveScopeHostId),
    canSelect,
    portfolioMode,
    canReadPortfolio,
    loadScope,
    loadPortfolio,
    selectClient,
    prepareAccountSwitch,
    setSwitching,
    setError,
    reset,
  }
})
