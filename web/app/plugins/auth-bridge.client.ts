import { setApiUnauthorizedHandler } from '~/utils/api-client'
import { useAuthStore } from '~/stores/auth'

// Plugin client-only que trata sessao expirada/invalida em DOIS momentos, sem o
// api-client importar store/router (evita dep circular, mesmo padrao do
// account-id-bridge/loading-bridge):
//  - REATIVO: qualquer 401 da API desloga e manda pro login na hora.
//  - PROATIVO: ao chegar o expiresAt do token, redireciona sozinho — sem esperar
//    o usuario clicar em algo e tomar um erro confuso de "Autenticacao obrigatoria".
export default defineNuxtPlugin(() => {
  const auth = useAuthStore()
  const router = useRouter()
  let redirecting = false

  function redirectToLogin() {
    const route = router.currentRoute.value
    // Ja na area de auth (login/reset/convite): nao faz nada (evita loop).
    if (String(route.path || '').startsWith('/auth')) {
      return
    }
    if (redirecting) {
      return
    }
    redirecting = true

    auth.clearSession()

    const fullPath = String(route.fullPath || '')
    const query: Record<string, string> = { expired: '1' }
    if (fullPath && fullPath !== '/') {
      query.redirect = fullPath
    }

    Promise.resolve(navigateTo({ path: '/auth/login', query }, { replace: true })).finally(() => {
      redirecting = false
    })
  }

  setApiUnauthorizedHandler(redirectToLogin)

  // Proativo: arma um timer para o instante de expiracao do token. Re-arma quando a
  // sessao muda (login/refresh do contexto). expiresAt vem do principal (/me/context).
  let expiryTimer: ReturnType<typeof setTimeout> | null = null

  watch(
    () => auth.principal?.expiresAt,
    (expiresAt) => {
      if (expiryTimer) {
        clearTimeout(expiryTimer)
        expiryTimer = null
      }

      const expiryMs = expiresAt ? Date.parse(String(expiresAt)) : Number.NaN
      if (!Number.isFinite(expiryMs)) {
        return
      }

      const delay = expiryMs - Date.now()
      if (delay <= 0) {
        redirectToLogin()
        return
      }

      // setTimeout suporta ate ~24.8 dias; o TTL do token (12h) cabe folgado.
      expiryTimer = setTimeout(redirectToLogin, delay)
    },
    { immediate: true },
  )
})
