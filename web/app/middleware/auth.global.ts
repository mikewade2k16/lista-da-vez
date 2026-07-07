import { useAuthStore } from '~/stores/auth'
import { useCoreAccountStore } from '../../layers/core/stores/account'
import { AUTH_TOKEN_COOKIE } from '~/utils/api-client'

export default defineNuxtRouteMiddleware(async (to) => {
  // Fallback publico e estatico do PWA. Sem este bypass, a pagina offline
  // seria redirecionada para o login durante a hidratacao.
  if (to.path === '/offline') {
    return
  }

  const auth = useAuthStore()
  const isAuthRoute = to.path.startsWith('/auth')
  const accessToken = useCookie(AUTH_TOKEN_COOKIE)
  const hasAccessToken = Boolean(String(accessToken.value || '').trim())

  if (!hasAccessToken) {
    if (isAuthRoute) {
      return
    }

    return navigateTo(
      {
        path: '/auth/login',
        query: to.fullPath && to.fullPath !== '/' ? { redirect: to.fullPath } : undefined,
      },
      { replace: true },
    )
  }

  // Bootstrap de sessao e concern do CLIENTE: a sessao vive no cookie e os plugins
  // de auth (auth-bridge/account-id-bridge) sao client-only. Rodar ensureSession/
  // fetchAccounts no SERVIDOR e fragil — numa rota SSR (ex.: /cardapio), uma falha
  // transitoria de hidratacao chamava clearSession (deslogava no hard reload) ou
  // estourava ao ler o store no servidor (accounts undefined). Com o token
  // presente, deixamos o SSR renderizar e o cliente resolve sessao + gating no
  // hidrate. As rotas /auth/* ja sao tratadas no fluxo abaixo.
  if (import.meta.server) {
    return
  }

  // Em rotas /auth/* (login/reset/invite), pular o ensureSession evita travar
  // a tela quando o token local esta valido mas o contexto remoto demora.
  // Se o usuario ja esta autenticado, ainda redirecionamos abaixo via
  // auth.isAuthenticated (que reflete o estado local em memoria).
  if (!isAuthRoute) {
    await auth.ensureSession()
    // C11.3 — Apos auth confirmada, hidrata accounts do user via API real
    // /v2/me/accounts (popula useCoreAccountStore.accounts para o AccountSwitcher
    // e useCoreAccountStore.enabledModules para o useDashboardNav).
    // No-op se ja hidratado (fetchAccounts e idempotente — sobrescreve a lista).
    if (auth.isAuthenticated) {
      const accountStore = useCoreAccountStore()
      if (accountStore.accounts.length === 0) {
        await accountStore.fetchAccounts()
      }
    }
  }

  if (isAuthRoute) {
    if (auth.isAuthenticated && to.path === '/auth/login') {
      return navigateTo(auth.mustChangePassword ? '/perfil' : auth.homePath, { replace: true })
    }

    return
  }

  if (!auth.isAuthenticated) {
    return navigateTo(
      {
        path: '/auth/login',
        query: to.fullPath && to.fullPath !== '/' ? { redirect: to.fullPath } : undefined,
      },
      { replace: true },
    )
  }

  if (auth.mustChangePassword && to.path !== '/perfil') {
    return navigateTo('/perfil', { replace: true })
  }

  const workspaceId = String(to.meta.workspaceId || '').trim()
  if (workspaceId && !auth.allowedWorkspaces.includes(workspaceId)) {
    return navigateTo(auth.homePath, { replace: true })
  }
})
