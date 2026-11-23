// Mapa rota -> modulo contratado, puro (sem runtime Nuxt) para ser usado pelo
// middleware module-enabled.global e pelo calculo da home no auth store.
// C11.2 — Mapa path → moduleId. Rota cuja path bate em um prefixo aqui exige
// que o modulo esteja habilitado em useCoreAccountStore().enabledModules.
// Modulos `core` e utilitarios nao entram aqui (sempre acessiveis).
//
// Espelha as tags moduleId do nav.config.ts. Se mover/renomear rota la, atualizar
// aqui — drift gera "menu esconde item mas rota direta ainda abre".
export interface ModulePathGuard {
  prefix: string
  moduleId?: string
  anyModuleIds?: readonly string[]
}

export const MODULE_PATH_GUARDS: ModulePathGuard[] = [
  { prefix: '/tasks', moduleId: 'tasks' },
  { prefix: '/calendario', moduleId: 'calendar' },
  { prefix: '/operacao-conteudo', moduleId: 'content_operations' },
  // omnichannel — inbox de atendimento. DEPENDE do modulo 'omnichannel' existir
  // no Go: o SyncCatalog do boot e quem cria a linha em core.modules e habilita
  // nas contas is_agency. Enquanto o shell Go nao existir, NENHUMA conta tem o
  // modulo em enabledModules e /omnichannel cai em /perfil por este guard.
  { prefix: '/omnichannel', moduleId: 'omnichannel' },
  // O workspace de clientes agrega dois modulos independentes. As rotas
  // especializadas exigem o owner correto; overview/perfil continuam acessiveis
  // com Customer Data mesmo quando a inteligencia opcional estiver desligada.
  { prefix: '/inteligencia-clientes/segmentos', moduleId: 'customer_data' },
  { prefix: '/inteligencia-clientes/fontes', moduleId: 'customer_intelligence' },
  { prefix: '/inteligencia-clientes/prompts', moduleId: 'customer_intelligence' },
  { prefix: '/inteligencia-clientes/atendimentos', moduleId: 'customer_intelligence' },
  { prefix: '/inteligencia-clientes/auditoria', moduleId: 'customer_intelligence' },
  { prefix: '/inteligencia-clientes/portfolio', moduleId: 'customer_intelligence' },
  {
    prefix: '/inteligencia-clientes',
    anyModuleIds: ['customer_data', 'customer_intelligence'],
  },
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
  { prefix: '/postagens', moduleId: 'social_publishing' },
  { prefix: '/finance', moduleId: 'finance' },
  { prefix: '/tools', moduleId: 'tools' },
  // queue — paginas de uso da Fila/operacao. Conta sem o modulo queue nao acessa
  // (espelha as tags moduleId:'queue' no nav.config.ts). /operacao cobre tambem
  // /operacao/usuarios.
  { prefix: '/operacao', moduleId: 'queue' },
  { prefix: '/transcricoes', moduleId: 'queue' },
  { prefix: '/comunicados', moduleId: 'queue' },
  { prefix: '/campanhas', moduleId: 'queue' },
  { prefix: '/consultor', moduleId: 'queue' },
  { prefix: '/ranking', moduleId: 'queue' },
  { prefix: '/dados', moduleId: 'queue' },
  { prefix: '/inteligencia', moduleId: 'queue' },
  { prefix: '/relatorios', moduleId: 'queue' },
  { prefix: '/multiloja', moduleId: 'queue' },
  { prefix: '/planejamento', moduleId: 'queue' },
  { prefix: '/configuracoes', moduleId: 'queue' },
  { prefix: '/alertas', moduleId: 'queue' },
  { prefix: '/suporte', moduleId: 'queue' },
  { prefix: '/feedback', moduleId: 'queue' },
  { prefix: '/meus-chamados', moduleId: 'queue' },
  { prefix: '/meus-feedbacks', moduleId: 'queue' },
  // core, notifications e banco nao dependem de modulo contratado. Superficies
  // internas como roadmap/themes/manage global sao tratadas separadamente pela
  // politica agencyOnly abaixo.
]

export function pathMatchesModulePrefix(path: string, prefix: string): boolean {
  const normalizedPath = String(path || '').replace(/\/+$/, '') || '/'
  const normalizedPrefix = String(prefix || '').replace(/\/+$/, '') || '/'
  return normalizedPath === normalizedPrefix || normalizedPath.startsWith(`${normalizedPrefix}/`)
}

export function findModulePathGuard(path: string): ModulePathGuard | undefined {
  return MODULE_PATH_GUARDS.find((guard) => pathMatchesModulePrefix(path, guard.prefix))
}

export function isModulePathGuardEnabled(
  guard: ModulePathGuard,
  enabledModules: ReadonlySet<string>,
): boolean {
  if (guard.moduleId) {
    return enabledModules.has(guard.moduleId)
  }
  return Boolean(guard.anyModuleIds?.some((moduleId) => enabledModules.has(moduleId)))
}
