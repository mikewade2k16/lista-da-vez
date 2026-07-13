import { useCoreAccountStore } from '../../layers/core/stores/account'

// Destino quando a conta ativa nao contratou o modulo da rota pedida (modo
// view-as do admin ou cliente real). Precisa ser uma rota SEGURA para TODO
// perfil: nunca-gated (fora de MODULE_PATH_GUARDS) e sem workspaceId
// (definePageMeta workspaceId: '') para que o auth.global tambem nao a barre
// por workspace. '/perfil' satisfaz ambos — evita qualquer loop com index.vue/
// auth.homePath, inclusive para o cliente nao-admin.
const MODULE_GATED_FALLBACK_PATH = '/perfil'

// C11.2 — Mapa path → moduleId. Rota cuja path bate em um prefixo aqui exige
// que o modulo esteja habilitado em useCoreAccountStore().enabledModules.
// Modulos `core` e utilitarios nao entram aqui (sempre acessiveis).
//
// Espelha as tags moduleId do nav.config.ts. Se mover/renomear rota la, atualizar
// aqui — drift gera "menu esconde item mas rota direta ainda abre".
const MODULE_PATH_GUARDS: Array<{ prefix: string; moduleId: string }> = [
  { prefix: '/tasks', moduleId: 'tasks' },
  { prefix: '/calendario', moduleId: 'calendar' },
  { prefix: '/crm', moduleId: 'crm' },
  { prefix: '/erp', moduleId: 'crm' },
  { prefix: '/site/leads', moduleId: 'site' },
  { prefix: '/site/produtos', moduleId: 'site' },
  { prefix: '/site/tracking', moduleId: 'site' },
  { prefix: '/site/bio', moduleId: 'bio' },
  { prefix: '/cardapio', moduleId: 'cardapio' },
  { prefix: '/manage/leads-web', moduleId: 'site' },
  { prefix: '/manage/produtos-web', moduleId: 'site' },
  { prefix: '/meta-ads', moduleId: 'meta_ads' },
  { prefix: '/finance', moduleId: 'finance' },
  { prefix: '/tools', moduleId: 'tools' },
  // queue — paginas de uso da Fila/operacao. Conta sem o modulo queue nao acessa
  // (espelha as tags moduleId:'queue' no nav.config.ts). /operacao cobre tambem
  // /operacao/usuarios e /operacao/clientes (gestao da Fila).
  { prefix: '/operacao', moduleId: 'queue' },
  { prefix: '/consultor', moduleId: 'queue' },
  { prefix: '/ranking', moduleId: 'queue' },
  { prefix: '/dados', moduleId: 'queue' },
  { prefix: '/inteligencia', moduleId: 'queue' },
  { prefix: '/relatorios', moduleId: 'queue' },
  { prefix: '/multiloja', moduleId: 'queue' },
  { prefix: '/configuracoes', moduleId: 'queue' },
  { prefix: '/alertas', moduleId: 'queue' },
  { prefix: '/feedback', moduleId: 'queue' },
  // core, notifications, roadmap, manage (admin de account), themes, banco:
  // sempre acessiveis (nao dependem de modulo contratado pela account).
]

// Paths de admin-global (Manage da plataforma): so acessiveis na conta-agencia
// (activeAccount.isAgency). Em qualquer conta-cliente (view-as do admin) esses
// paths redirecionam para o fallback seguro. Espelha os itens agencyOnly do
// nav.config.ts — se mover/renomear rota la, atualizar aqui.
const AGENCY_ONLY_PATHS = [
  '/manage/users',
  '/manage/organizations',
  '/manage/role-templates',
  '/manage/clientes-web',
]

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
  if (AGENCY_ONLY_PATHS.some((p) => to.path === p || to.path.startsWith(`${p}/`))) {
    if (account.activeAccount && !account.activeAccount.isAgency) {
      return navigateTo(MODULE_GATED_FALLBACK_PATH, { replace: true })
    }
  }

  const guard = MODULE_PATH_GUARDS.find((g) => to.path.startsWith(g.prefix))
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
  if (!enabledModules.has(guard.moduleId)) {
    // Modulo nao habilitado para esta account → leva para um fallback SEGURO e
    // NUNCA-gated. Nao usamos '/' porque index.vue resolve para auth.homePath,
    // que para o admin e '/operacao' (gated por `queue`); se a conta ativa nao
    // tiver `queue`, esse guard barraria de novo e entraria em loop.
    return navigateTo(MODULE_GATED_FALLBACK_PATH, { replace: true })
  }
})
