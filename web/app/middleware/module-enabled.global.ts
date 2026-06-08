import { useAuthStore } from '~/stores/auth'
import { useCoreAccountStore } from '../../layers/core/stores/account'

// C11.2 — Mapa path → moduleId. Rota cuja path bate em um prefixo aqui exige
// que o modulo esteja habilitado em useCoreAccountStore().enabledModules.
// Modulos `core` e utilitarios nao entram aqui (sempre acessiveis).
//
// Espelha as tags moduleId do nav.config.ts. Se mover/renomear rota la, atualizar
// aqui — drift gera "menu esconde item mas rota direta ainda abre".
const MODULE_PATH_GUARDS: Array<{ prefix: string; moduleId: string }> = [
  { prefix: '/tasks', moduleId: 'tasks' },
  { prefix: '/crm', moduleId: 'crm' },
  { prefix: '/erp', moduleId: 'crm' },
  { prefix: '/site/leads', moduleId: 'site' },
  { prefix: '/site/produtos', moduleId: 'site' },
  { prefix: '/site/tracking', moduleId: 'site' },
  { prefix: '/manage/leads-web', moduleId: 'site' },
  { prefix: '/manage/produtos-web', moduleId: 'site' },
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

export default defineNuxtRouteMiddleware((to) => {
  // Rotas /auth/* nao precisam de account.
  if (to.path.startsWith('/auth')) return

  const guard = MODULE_PATH_GUARDS.find((g) => to.path.startsWith(g.prefix))
  if (!guard) return

  // platform_admin gerencia TODAS as accounts e nao esta vinculado aos modulos
  // de uma account ativa especifica — nunca e barrado pelo gating de modulos.
  // Sem isto, se a account ativa do admin estiver sem o modulo (ex.: conta recem
  // criada que ele administra), o redirect para '/' entra em loop com index.vue
  // e o painel fica em branco.
  const auth = useAuthStore()
  if (auth.role === 'platform_admin') return

  const account = useCoreAccountStore()
  // Sem account ativa (ex.: durante hidrate inicial), nao bloqueia — auth.global
  // ja redireciona se nao autenticado. Quando account aterrissar, navegacao
  // subsequente respeita o guard.
  if (!account.activeAccount) return

  const enabledModules = new Set(account.enabledModules)
  if (!enabledModules.has(guard.moduleId)) {
    // Modulo nao habilitado para esta account → leva pra home segura.
    return navigateTo('/', { replace: true })
  }
})
