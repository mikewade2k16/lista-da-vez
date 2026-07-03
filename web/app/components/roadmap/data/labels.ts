import type { ModuleStatus, ModulePriority, RuleCategory } from "./types";

export const ROADMAP_TITLE = "Reestruturação Multi-Tenant";
export const ROADMAP_SUBTITLE =
  "Acompanhamento das fases da branch refactor/multi-tenant-core. Cada fase é um deploy reversível; produção atual segue intocada em main/migracao/nuxt.";

// Aviso de estado real — 2026-05-28
//
// Auditoria de 2026-05-28 mostrou que várias Fases marcadas como `done` neste
// arquivo estão na verdade `in_progress` ou parciais quando confrontadas com o
// runtime real (ver docs/ROADMAP.md → "Estado real em 2026-05-28"):
//
//  - Fase 2: AccountModulesGuard é instanciado e descartado (`_ =` em app.go).
//  - Fase 3: core.user_tenant_roles / core.account_modules estão com 0 linhas.
//  - Fase 5: menu não consulta core.account_modules — filtra por role hardcoded.
//  - Fases 13/15/16: páginas mock (clientes-web/leads-web/produtos-web,
//    site/*Workspace) consomem um BFF Nitro em web/server/ com seed in-memory,
//    sem tocar Postgres.
//
// As notas das tarefas afetadas foram atualizadas. A fase nova
// "multitenant-completion" abaixo é a fonte de verdade da próxima branch
// (refactor/multi-tenant-complete) — só depois dela o lote 13/14/15/16+ retoma.

export const ROADMAP_MODULE_STATUS_LABEL: Record<ModuleStatus, string> = {
  pending: "Pendente",
  in_progress: "Em andamento",
  beta: "Beta",
  done: "Concluido"
};

export const ROADMAP_PRIORITY_LABEL: Record<ModulePriority, string> = {
  P0: "P0 - Critica",
  P1: "P1 - Alta",
  P2: "P2 - Media",
  P3: "P3 - Baixa"
};

export const ROADMAP_RULE_CATEGORY_LABEL: Record<RuleCategory, string> = {
  frontend: "Frontend",
  backend: "Backend",
  banco: "Banco",
  linguagens: "Linguagens",
  deploy: "Deploy",
  "padroes-gerais": "Padroes Gerais"
};
