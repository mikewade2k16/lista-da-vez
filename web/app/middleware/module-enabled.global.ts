import { useCoreAccountStore } from '../../layers/core/stores/account'
import { findEditorModulePathGuard, isAgencyOnlyPath } from '~/utils/account-route-access'
import { findModulePathGuard, isModulePathGuardEnabled } from '~/utils/module-path-guards'

// Destino quando a conta ativa nao contratou o modulo da rota pedida (modo
// view-as do admin ou cliente real). Precisa ser uma rota SEGURA para TODO
// perfil: nunca-gated (fora de MODULE_PATH_GUARDS) e sem workspaceId
// (definePageMeta workspaceId: '') para que o auth.global tambem nao a barre
// por workspace. '/perfil' satisfaz ambos — evita qualquer loop com index.vue/
// auth.homePath, inclusive para o cliente nao-admin.
const MODULE_GATED_FALLBACK_PATH = '/perfil'

// C11.2 — mapa path -> moduleId e helpers vivem em ~/utils/module-path-guards
// (modulo puro, tambem usado pela home do auth store). Reexportados aqui.
export {
  MODULE_PATH_GUARDS,
  findModulePathGuard,
  isModulePathGuardEnabled,
  pathMatchesModulePrefix,
  type ModulePathGuard,
} from '~/utils/module-path-guards'

// Paths de admin-global (Manage da plataforma): so acessiveis na conta-agencia
// (activeAccount.isAgency). Em qualquer conta-cliente (view-as do admin) esses
// paths redirecionam para o fallback seguro. Espelha os itens agencyOnly do
// nav.config.ts — se mover/renomear rota la, atualizar aqui.
export default defineNuxtRouteMiddleware((to) => {
  // Rotas /auth/* nao precisam de account.
  if (to.path.startsWith('/auth')) return

  const account = useCoreAccountStore()

  // Modo super-admin/dev (platformView): libera TODAS as rotas, inclusive as de
  // modulos em desenvolvimento nao liberados nem para a conta-agencia. So o
  // platform_admin entra nesse modo (via switcher > "Plataforma (dev)").
  if (account.platformView) return

  // Gating de admin-global: rotas Manage da plataforma so abrem na conta-agencia.
  // Avaliado antes do gating de modulo (esses paths nao estao em
  // MODULE_PATH_GUARDS). Sem account ativa (hidrate) nao bloqueia — navegacao
  // subsequente respeita a regra. Em conta-cliente (isAgency falsy) → fallback
  // seguro (mesmo destino do gating de modulo).
  if (isAgencyOnlyPath(to.path)) {
    if (account.activeAccount && !account.activeAccount.isAgency) {
      return navigateTo(MODULE_GATED_FALLBACK_PATH, { replace: true })
    }
  }

  const guard = findEditorModulePathGuard(to.path) || findModulePathGuard(to.path)
  if (!guard) return
  // Sem account ativa (fail-closed espelhando o useDashboardNav):
  //  - Durante o hidrate inicial (accountsLoaded false) NAO bloqueia — auth.global
  //    ja redireciona se nao autenticado; quando a account aterrissar, navegacao
  //    subsequente respeita o guard.
  //  - Se o contexto ja resolveu (accountsLoaded true) e ainda nao ha account
  //    ativa (sem membership, lista vazia ou erro), rota de modulo nao deve abrir
  //    so pelo papel → manda pro fallback seguro.
  if (!account.activeAccount) {
    if (account.accountsLoaded) {
      return navigateTo(MODULE_GATED_FALLBACK_PATH, { replace: true })
    }
    return
  }

  // view-as: platform_admin TAMBEM e gated pela conta ativa do switcher. O
  // switcher e a ferramenta do admin para "ver como o cliente" — ao selecionar
  // uma conta, rotas refletem so os modulos que ela contratou. Manage/core nao
  // estao em MODULE_PATH_GUARDS, entao seguem sempre acessiveis (o admin nunca
  // perde o acesso administrativo). O gating de API (RequireModuleByPath) segue
  // isentando platform_admin de proposito; e o bloqueio de rota aqui que impede
  // o admin de ABRIR a pagina gated da conta que nao contratou o modulo.
  const enabledModules = new Set(account.enabledModules)
  if (!isModulePathGuardEnabled(guard, enabledModules)) {
    // Modulo nao habilitado para esta account → leva para um fallback SEGURO e
    // NUNCA-gated. Nao usamos '/' porque index.vue resolve para auth.homePath,
    // que para o admin e '/operacao' (gated por `queue`); se a conta ativa nao
    // tiver `queue`, esse guard barraria de novo e entraria em loop.
    return navigateTo(MODULE_GATED_FALLBACK_PATH, { replace: true })
  }
})
