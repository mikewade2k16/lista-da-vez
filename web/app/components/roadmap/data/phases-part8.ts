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
];
