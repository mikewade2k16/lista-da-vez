import type { RoadmapPhase } from "./types";

export const ROADMAP_PHASES_PART8: RoadmapPhase[] = [
  {
    id: "social-publishing-f0",
    code: "SP-F0",
    title: "Agendamento de postagens — fundação isolada",
    goal:
      "Entregar um módulo independente para conectar Instagram profissional, agendar imagem, publicar por fila durável e consultar analytics, sem alterar o Calendário ou o Crow Assistant.",
    status: "in_progress",
    startedAt: "2026-07-23",
    estimateWeeks: "piloto técnico",
    group: "social-publishing",
    tasks: [
      {
        id: "sp-f0-plan",
        label:
          "[doc] Registrar fronteiras, contratos futuros e estratégia de retorno antes da implementação",
        done: true,
      },
      {
        id: "sp-f0-db",
        label:
          "[banco] Schema social_publishing com conexão cifrada, publicações, snapshots e outboxes separadas para publicação e analytics",
        done: true,
      },
      {
        id: "sp-f0-back",
        label:
          "[back] API Go em camadas, adapter Instagram real e worker com idempotência/retry",
        done: true,
      },
      {
        id: "sp-f0-front",
        label:
          "[frontend] Workspace /postagens com fila, compositor, analytics e conexão técnica",
        done: true,
      },
      {
        id: "sp-f0-gates",
        label:
          "[segurança] Module Registry, permissões e gates de conta/workspace no front e no back",
        done: true,
      },
      {
        id: "sp-f0-tests",
        label:
          "[qa] Testes Go/frontend, lint, build, cenários de migration e smoke visual desktop/mobile",
        done: true,
      },
      {
        id: "sp-f0-meta-homologation",
        label:
          "[homologação] Publicar uma imagem e coletar analytics com conta Meta profissional de teste controlada",
        done: false,
      },
    ],
    verifiable:
      "Em uma conta de teste, criar rascunho, agendar uma imagem HTTPS, observar a fila publicar no horário e consultar métricas; desabilitar o módulo remove o acesso sem afetar /calendario.",
    blockers: ["Credencial e conta Meta profissional de teste controladas"],
  },
  {
    id: "social-publishing-f1",
    code: "SP-F1",
    title: "Agendamento de postagens — integração com Calendário e Crow",
    goal:
      "Ligar eventos e arquivos do Calendário ao contrato idempotente do módulo, mostrar analytics resumidos no evento e liberar contexto controlado ao Crow Assistant.",
    status: "blocked",
    estimateWeeks: "após homologação do SP-F0",
    group: "social-publishing",
    tasks: [
      {
        id: "sp-f1-calendar",
        label:
          "[futuro] Criar publicação a partir do evento/arquivo do Calendário por source_ref idempotente",
        done: false,
      },
      {
        id: "sp-f1-calendar-analytics",
        label:
          "[futuro] Exibir status e analytics resumidos no evento depois da publicação",
        done: false,
      },
      {
        id: "sp-f1-crow",
        label:
          "[futuro] Autorizar o Crow Assistant a consultar agenda e analytics e propor ações auditáveis",
        done: false,
      },
    ],
    verifiable:
      "Evento do Calendário dispara exatamente uma publicação, recebe o resumo de analytics e o Crow responde usando os mesmos dados autoritativos.",
    blockers: ["Comando explícito do usuário após homologação do SP-F0"],
  },
  {
    id: "content-operations-f0",
    code: "CO-F0",
    title: "Operação de conteúdo — MVP de alertas",
    goal:
      "Cruzar os dados autoritativos de Tasks e Calendário para lembrar ativamente a equipe sobre captação, produção, aprovação e postagem, exibindo tudo pela central global de notificações.",
    status: "in_progress",
    startedAt: "2026-08-18",
    estimateWeeks: "pausado; retomada futura",
    group: "content-operations",
    tasks: [
      {
        id: "co-f0-back",
        label: "[back] Brief de leitura com regras por cliente, recorte de conta e fontes Tasks/Calendário",
        done: true,
      },
      {
        id: "co-f0-task-items",
        label: "[tasks] Estados e datas opcionais nos itens usados para identificar o estágio do conteúdo",
        done: true,
      },
      {
        id: "co-f0-page",
        label: "[frontend] Página /operacao-conteudo com prioridades agrupadas por cliente",
        done: true,
      },
      {
        id: "co-f0-global-notifications",
        label: "[frontend] Entregar os alertas na central global, com origem Operação de conteúdo",
        done: true,
      },
      {
        id: "co-f0-crow-readonly",
        label: "[integração] Crow consulta o resumo com o mesmo recorte, sem gerar nem disparar alertas",
        done: true,
      },
      {
        id: "co-f0-back-tests",
        label: "[qa pendente] Completar testes HTTP/repository, permissões, multi-tenant, clientes e limites de data/fuso",
        done: false,
        note: "Existem testes unitários iniciais do service; eles não encerram a validação do módulo.",
      },
      {
        id: "co-f0-front-tests",
        label: "[qa pendente] Testar composable, página, loading/erro/vazio, links de origem e fluxo E2E Tasks → alerta → Calendário",
        done: false,
        note: "A central global teve teste do agregador e smoke visual, mas a página e a integração completa ainda não foram homologadas.",
      },
      {
        id: "co-f0-real-data",
        label: "[homologação pendente] Validar cada regra com dados representativos e revisar falsos positivos/negativos",
        done: false,
      },
      {
        id: "co-f0-alert-lifecycle",
        label: "[produto pendente] Definir histórico, confirmar, adiar/dispensar, preferências e limites configuráveis",
        done: false,
      },
      {
        id: "co-f0-out-of-session",
        label: "[produto pendente] Implementar disparos fora da sessão por canais autorizados, se necessário",
        done: false,
        note: "Hoje os lembretes são consultados pela central enquanto o sistema está aberto; não há envio autônomo por e-mail ou WhatsApp.",
      },
    ],
    verifiable:
      "Com dados controlados, alterar tasks e eventos produz exatamente os alertas esperados por cliente, sem vazamento entre contas; a página e a central global atualizam corretamente no desktop/mobile e toda a matriz de testes fica verde.",
    blockers: ["Implementação pausada por decisão do dono em 2026-08-18"],
  },
];
