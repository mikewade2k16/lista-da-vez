import type { RoadmapGroup } from "./types";

export const ROADMAP_GROUPS: RoadmapGroup[] = [
  {
    id: "multi-tenant",
    label: "Reestruturação Multi-Tenant",
    description: "Branch refactor/multi-tenant-core — schema core, RBAC, Module Registry, layers e módulos satélites."
  },
  {
    id: "tasks-backend",
    label: "Tasks Orquestrador — Backend",
    description: "Transformar o protótipo localStorage em produto multi-tenant real: schema tasks.*, API Go, realtime, RBAC, notificações e sistema de views."
  },
  {
    id: "crm-360",
    label: "CRM 360 — Fila + ERP",
    description: "Indicadores por consultor e loja cruzando dados de atendimento da fila com vendas do ERP: conversão, faturamento, PA, ticket médio, produto não encontrado."
  },
  {
    id: "automation",
    label: "Automação WhatsApp/IA",
    description: "Assistente proativa de WhatsApp (n8n + WAHA, persona Tony) trazida para dentro do Omni como módulo automation/. Integração por fases com CRM/catálogo/ERP via API Go."
  },
  {
    id: "bio",
    label: "Bio Links — Site/Bio",
    description: "CRUD multitenant das páginas de bio (link-in-bio) servidas pelo front Nuxt separado. Cliente edita só a própria bio; admin/agência gerencia todas com filtro por cliente. Plano: docs/bio/PLANO_MODULO_BIO.md."
  },
  {
    id: "cardapio",
    label: "Cardápio Online",
    description: "CRUD multitenant de cardápios online (restaurantes) servidos por um front Nuxt estático no host do cliente, com resolução de tenant por domínio, pedidos recalculados no servidor e tracking. Por enquanto na account da Crow. Plano: docs/cardapio/PLANO_MODULO_CARDAPIO.md."
  },
  {
    id: "infra-deploy",
    label: "Infra & Deploy",
    description: "Pipeline de deploy do Omni: imagens no GHCR buildadas no GitHub Actions (a VPS só faz pull, nunca compila) + ambiente de staging isolado e sob demanda para testar antes de promover pra produção. Plano: docs/deploy/REGISTRY_STAGING_DEPLOY_PLAN.md."
  },
  {
    id: "fila-operacao",
    label: "Fila — Página Operação",
    description: "Ajustes de operação da Fila: controle por loja individual para usuários multi-loja, limpeza do modal de encerrar, justificativa só ao avançar e métrica de pausas (motivo/horário/duração) persistida e em Relatórios. Plano: docs/operacao/AJUSTES_OPERACAO_PLAN.md."
  },
  {
    id: "menu-layout",
    label: "Organização do Menu (Header × Sidebar)",
    description: "Config global, editável pelo platform_admin, de como o menu se divide entre header e sidebar: posição por item (header/sidebar/ambos/oculto) + reordenar, persistida em core.platform_settings. Inclui fix responsivo do header (overflow 'Mais'). Plano: docs/MENU_LAYOUT_CONFIG.md."
  },
  {
    id: "comissao-v2",
    label: "Comissão v2 — cálculo no back (API-first)",
    description: "Recebimento por atingimento de meta calculado no backend como serviço de domínio único (pacote queue/commission), embutido em /v1/erp/crm. Consultor sobre a PRÓPRIA venda com trava de meta e penalidade PA/Ticket; gerente sobre o total da loja com faixas por tipo de loja (Shopping/Bairro). Inclui a auditoria das demais lógicas só-no-front (P1-P3). Plano: planos/vamos-fazer-altera-es-em-purrfect-pony.md."
  },
  {
    id: "calendario",
    label: "Calendário de Conteúdo",
    description: "Agenda de conteúdo por cliente da agência (/calendario): eventos (post/story/reels/reunião), notas por mês, responsáveis reais, feriados/datas comemorativas e config na página. Fonte 100% real (sem mock). Plano: docs/CALENDARIO_PLAN.md."
  },
  {
    id: "ui-liquid-glass",
    label: "UI Liquid Glass (design system)",
    description: "Converter a UI do painel para a estética liquid glass do /calendario: aurora ambiente global no shell (movida do calendário) + cards/painéis/controles em superfícies de vidro (backdrop-filter que refratam a aurora) + botões flutuantes translúcidos, aplicados página a página com tokens do design system (nunca hex hardcoded), validando contraste no tema claro/escuro e em mobile."
  },
  {
    id: "ac-fixes-2026-07",
    label: "Correções do Diagnóstico 2026-07",
    description: "Correções priorizadas a partir da auditoria de manutenção/segurança/performance registrada em docs/relatorios/2026-07: cache do resolvedor principal, runbooks de VPS/segredos, hardening de banco/backup, paginação de relatórios, recorte do front, limites/healthchecks no compose, Finance na API Go, testes e monitoração mínima. Specs por item em docs/specs/ac-fixes-2026-07/."
  },
  {
    id: "observabilidade-n8n",
    label: "Observabilidade & Alertas (n8n)",
    description: "Monitoramento ativo do Omni usando o n8n como orquestrador de alertas: check-vps.sh e /healthz disparam webhooks, fan-out para e-mail/Telegram/ntfy, monitor externo de uptime, detecção de anomalia (RAM/CPU/5xx/latência), painel de status interno e alertas de negócio. Nasce do AC-16 do diagnóstico docs/relatorios/2026-07."
  }
];
