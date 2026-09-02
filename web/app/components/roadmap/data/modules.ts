import type { RoadmapModule } from "./types";

export const ROADMAP_MODULES: RoadmapModule[] = [
  {
    id: "meta_ads",
    label: "Meta Ads",
    route: "/meta-ads",
    status: "beta",
    priority: "P0",
    category: "operacao-comercial",
    description:
      "Gestão e relatórios de tráfego pago de Meta (Facebook/Instagram) no painel, com Assistente 360 compartilhado. Go/PostgreSQL é a fonte autoritativa, resolve conta/cliente/RBAC e mantém writes desligados por padrão até a homologação do executor durável.",
    scope: [
      "Local: Facebook Login seguro, token cifrado, paginação/snapshots e dashboard de 90 dias",
      "Local: chat compartilhado com voz, memória, KPIs e cards escopados por surface/RBAC",
      "Local: vínculos ad account e Page/Instagram→cliente com UI e filtros fail-closed",
      "Local: seis ações first-party com proposta durável, caps, confirmação, árvore PAUSED e recibos at-most-once",
      "Pendente externo: Meta App/OAuth/assets reais e E2E Graph em staging antes de habilitar writes"
    ],
    dependsOn: []
  },
  {
    id: "tasks",
    label: "Tasks",
    route: "/tasks",
    status: "done",
    priority: "P0",
    category: "atendimento",
    description:
      "Orquestrador de tarefas multi-tenant (boards + tabela). EM USO REAL: board geral da agencia (Crow Visuals, 247 tasks) + boards por cliente (Duby). Backend completo (T1-T9), realtime, tracking, RBAC, render progressivo. Multi-tenant fechado (board vive na conta-agencia; acesso org-aware). Refino continuo de performance e do editor segue como melhoria, nao bloqueio.",
    scope: [
      "Eliminar refresh full/cascata de requests: reconciliar somente task, board ou campo alterado via REST + WS versionado",
      "Refinar performance do board para >500 cards",
      "Melhorar feedback de drag-and-drop entre colunas",
      "Adicionar filtros salvos por usuario",
      "Notificacoes in-app quando @mention"
    ],
    dependsOn: []
  },
  {
    id: "editor",
    label: "Editor",
    route: "/editor",
    status: "beta",
    priority: "P1",
    category: "tools",
    description:
      "Editor rich-text Omni baseado em Tiptap (StarterKit + TaskList + Emoji + Mention + TextAlign). Usado em descricao de tasks. Falta: salvar/abrir documentos avulsos, versionamento, sharing.",
    scope: [
      "Persistir documentos avulsos (nao apenas dentro de Tasks)",
      "Adicionar /slash commands",
      "Suporte a colaboracao em tempo real (avaliar @tiptap/y-tiptap)"
    ],
    dependsOn: ["tasks"]
  },
  {
    id: "tracking",
    label: "Tracking",
    route: "/tracking",
    status: "beta",
    priority: "P1",
    category: "atendimento",
    description:
      "Time-tracking por cliente/usuario/periodo. ENTREGUE 2026-06-10: pagina /tracking com layout de board (TrackingBoardView, so tasks em play/pause) + aba Inteligencia consumindo GET /v1/tasks/tracking/metrics (byClient/byUser/byType em 1 query, GROUP BY server-side; agrega por client_account_id). Falta: export CSV e comparativos avancados.",
    scope: [
      "Export CSV dos buckets de tempo",
      "Comparativo Pessoa A vs B no periodo",
      "Metas de tempo por cliente"
    ],
    dependsOn: ["tasks"]
  },
  {
    id: "omnichannel",
    label: "Omnichannel",
    route: "/omnichannel",
    status: "pending",
    priority: "P1",
    category: "atendimento",
    description:
      "Modulo de ATENDIMENTO WhatsApp inteiro (F0-F14), nao so um port: inbox humano + setores/filas com atribuicao + triagem por IA + multi-provider de canal, com o Go como fonte de verdade. Fusao de dois trilhos (2026-07-16): o port do inbox legado (78 arquivos / 22.642 linhas de codigo maduro: audio, GIF, stickers, mencoes, reacoes, encaminhar, apagar p/ todos, multiplas instancias) vira o caminho do FRONT — 73 byte a byte, 5 repontados (socket.io e rotas Nitro nao existem aqui), adaptacao inteira em 6 arquivos de costura; e a spec externa rege o BACKEND (dominio messaging.*, filas, triagem, seguranca/LGPD) e as telas NOVAS de config, que nascem no design system da casa. Provider = adapter multi-provider (meta_whatsapp_cloud + evolution/waha + mock, escolha por conta/numero) e LLM nativo no Go com config do painel — o n8n fica FORA do caminho critico. Plano canonico: docs/omnichannel/PLANO_ATENDIMENTO.md; specs por fase: docs/omnichannel/specs/OMNI-F*.md; anexo tecnico do front: docs/omnichannel/PLANO_PORT_OMNICHANNEL.md. IMPLEMENTACAO CONGELADA ate a branch refactor/multi-tenant-complete fechar.",
    scope: [
      "F0 decisoes (multi-provider, port=front/spec=backend, LLM no Go) + LEGADO + roadmap",
      "F1 front verbatim + costura (73 arqs byte a byte + 6 adaptadores, badge SEM BACKEND)",
      "F2 schema messaging.* + leitura em Go (colunas de estado/fila/provider nascem juntas)",
      "F3 [NOVA] infra transversal em platform/: jobs (outbox FIFO), secretbox (AES-256-GCM), llm",
      "F4 ChannelProvider + adapters (mock, evolution) + webhook inbound com dedupe no banco",
      "F5 realtime: socket.io -> WS nativo com ticket (3 eventos, shapes por call-site)",
      "F6 envio via outbox (idempotency_key) + midia em disco com stream/Range",
      "F7 acoes (reacao, encaminhar, apagar, status, assign) via maquina de estados",
      "F8 [NOVA] dominio: setores/filas/queue_members/routing_rules + decisions + state->status",
      "F9 [NOVA] triagem IA no Go: ai_agents/versions/runs, JSON schema-validado, IA sugere e o motor decide",
      "F10 [NOVA] telas de config: numeros/providers, setores/filas/regras, editor de agente + simulador",
      "F11 [NOVA] adapter Meta WhatsApp Cloud: X-Hub-Signature-256, templates + janela 24h, capabilities",
      "F12 stickers/GIF/avatar substituindo as rotas Nitro",
      "F13 LGPD + observabilidade: retencao/purge, masking, custo LLM por conta (minimo e P0)",
      "F14 refactor + convergencia: split >450, virar layer, dropar automation.*, avaliar o Tony"
    ],
    dependsOn: []
  },
  {
    id: "assistente-ia",
    label: "Assistente IA (WhatsApp)",
    route: "/automation",
    status: "beta",
    priority: "P1",
    category: "atendimento",
    description:
      "Assistente proativa de WhatsApp (n8n + WAHA, persona Tony): multimodal (texto/audio/imagem via Whisper+visao), debounce, memoria por segmento + memoria longa, naturalidade (digitando/baloes). Migrada para dentro do Omni como modulo automation/ (containers no profile docker 'automation'). EM PRODUCAO e INTOCADA no curto prazo — nao migrar, nao refatorar, nao 'aproveitar'. FRONTEIRA COM O OMNICHANNEL (decidida 2026-07-16): sao DOIS modulos separados — automation.* = o cerebro do Tony (gate /v1/automation), messaging.* = o que aconteceu no atendimento (gate /v1/omnichannel). Regra anti-colisao 'UM NUMERO = UM CEREBRO': o mesmo numero de WhatsApp nunca e atendido pelos dois ao mesmo tempo, validado NO CADASTRO da instancia (omnichannel F4), nao no runtime — dois bots respondendo o mesmo cliente e incidente visivel para o cliente final. Manter dois cerebros e band-aid CONSCIENTE, registrado no docs/LEGADO.md; a convergencia (Tony virar um ai_agent sobre o provider waha) e AVALIADA na F14 do omnichannel, sem compromisso. O que nao se admite e o dado duplicar: automation.messages/contacts (vazias na pratica) sao depreciadas na F0 e dropadas na F14.",
    scope: [
      "Mini-CRM no Postgres do Omni (schema automation.*, tenant-aware)",
      "Tools do agente via API Go (catalogo/estoque/preco, registrar lead/pedido)",
      "Motor proativo (follow-up/pos-venda/nurture)",
      "Painel de config (modelos, personas, liga/desliga, contexto temporario)",
      "Fronteira com o omnichannel: um numero = um cerebro; convergencia so avaliada na F14"
    ],
    dependsOn: []
  },
  {
    id: "team",
    label: "Team (Equipe + Escalas)",
    route: "/team/equipe",
    status: "pending",
    priority: "P2",
    category: "operacao-comercial",
    description:
      "Gestao de equipe e escalas. Pagina existe mas sem CRUD real. Compartilha schema core.users + roles ja existentes.",
    scope: [
      "CRUD de equipe com avatar e cargo",
      "Calendario de escalas (turnos)",
      "Aprovacao de troca de turno"
    ],
    dependsOn: []
  },
  {
    id: "site",
    label: "Site (Leads + Produtos + Tracking)",
    route: "/site/leads",
    status: "beta",
    priority: "P2",
    category: "operacao-comercial",
    description:
      "Modulo site/ ENTREGUE (C17-C19): schema site (leads, products, webhook_sources, tracking_events); ingestao por webhook HMAC SHA-256; admin CRUD de leads/produtos com filtros + colunas travaveis; receptor de tracking (webhook Perola) + tela /site/tracking. Em uso real. O page-builder visual de paginas/forms fica como evolucao futura separada.",
    scope: [
      "Page-builder visual de paginas/forms (futuro)",
      "Campanha = pagina + canal + meta",
      "Mais fontes de webhook por cliente"
    ],
    dependsOn: []
  },
  {
    id: "bio",
    label: "Bio (link-in-bio)",
    route: "/site/bio",
    status: "beta",
    priority: "P2",
    category: "tools",
    description:
      "CRUD multitenant das paginas de bio (link-in-bio), servidas pelo front Nuxt separado crow-nuxt (rota /bio/{slug}, consome /v1/public/bio). Cliente edita so a propria bio; admin/agencia gerencia todas com filtro por cliente. Backend modulo bio/ + schema bio (migration 0152). Plano: docs/bio/PLANO_MODULO_BIO.md.",
    scope: [
      "Editor de blocos (menu/links/slides/lojas) colapsavel e compacto",
      "Temas/tokens por bio",
      "Analytics de cliques por link"
    ],
    dependsOn: []
  },
  {
    id: "cardapio",
    label: "Cardapio Online",
    route: "/cardapio",
    status: "beta",
    priority: "P2",
    category: "tools",
    description:
      "CRUD multitenant de cardapios online (restaurantes), servidos por um front Nuxt estatico no host do cliente, com resolucao de tenant por dominio, pedidos recalculados no servidor e tracking. Backend modulo cardapio/ + schema cardapio (migration 0153). Por enquanto na conta da agencia. Plano: docs/cardapio/PLANO_MODULO_CARDAPIO.md.",
    scope: [
      "Recalculo de pedido server-side (preco/estoque)",
      "Resolucao de tenant por dominio do cliente",
      "Integracao com WhatsApp para receber pedido"
    ],
    dependsOn: []
  },
  {
    id: "inteligencia",
    label: "Inteligencia",
    route: "/inteligencia",
    status: "pending",
    priority: "P2",
    category: "indicadores",
    description:
      "Insights gerados por LLM sobre dados de vendas e atendimento. Cards 'Por que conversao caiu?' / 'Quais produtos faltam mais?'. Sera consumidor pesado do backend de BI.",
    scope: [
      "Prompts canonicos para 5 perguntas frequentes",
      "Cache de resposta por janela (dia/semana)",
      "Exportar insight como PDF"
    ],
    dependsOn: ["bi"]
  },
  {
    id: "relatorios",
    label: "Relatorios",
    route: "/relatorios",
    status: "pending",
    priority: "P2",
    category: "indicadores",
    description:
      "Reports estaticos exportaveis (PDF/CSV). Backend reports/ existe parcial. Faltam templates e UI de configuracao.",
    scope: [
      "Template Ranking Mensal Consultor (PDF)",
      "Template Vendas por Loja (CSV + PDF)",
      "Agendamento de envio recorrente por email"
    ],
    dependsOn: []
  },
  {
    id: "bi",
    label: "BI",
    route: "/bi",
    status: "pending",
    priority: "P2",
    category: "indicadores",
    description:
      "Dashboards customizaveis. Modulo backend bi/ ja criado mas sem UI. Definir entre dashboard hardcoded vs builder.",
    scope: [
      "Decidir entre Metabase embedded vs builder proprio",
      "MVP com 3 dashboards fixos (vendas, atendimento, estoque)",
      "Filtros por loja/consultor/periodo"
    ],
    dependsOn: []
  },
  {
    id: "finance",
    label: "Finance",
    route: "/finance",
    status: "pending",
    priority: "P3",
    category: "indicadores",
    description:
      "Comissoes, metas financeiras, fechamento mensal. Hoje nao existe. Depende de Vendas (ERP) ja integrada.",
    scope: [
      "Calculo de comissao por consultor com regras configuraveis",
      "Fechamento mensal exportavel",
      "Integracao com folha (fora do escopo inicial)"
    ],
    dependsOn: []
  },
  {
    id: "monitoramento",
    label: "Monitoramento",
    route: "/monitoramento",
    status: "pending",
    priority: "P2",
    category: "indicadores",
    description:
      "Pagina interna de health: uptime API, jobs ERP, sync FTP, fila de atendimento em tempo real. Pega de healthz + module registry + alerts.",
    scope: [
      "Painel de modulos ativos (do module registry)",
      "Historico de jobs ERP",
      "Latencia /healthz dos ultimos 7 dias"
    ],
    dependsOn: []
  },
  {
    id: "qr-tools",
    label: "Tools (QR Code + Encurtador)",
    route: "/tools/qr-code",
    status: "in_progress",
    priority: "P3",
    category: "tools",
    description:
      "Encurtador de link e gerador de QR Code customizavel. O projeto antigo era mock (globalThis no BFF Nitro eliminado); em reconstrucao COMO MODULO REAL: back Go modulo tools/ + schema tools (migration 0197), isolado por account_id. QR rastreado (codifica /q/{slug} -> 302 + scan_count, respeita is_active); encurtador rastreado (/s/{slug} -> 302 + hits). Reaproveita o design system (OmniDataTable/Filters). Dono do link escolhido no modal (cross-conta para platform_admin). Plano: docs/tools/PLANO_MODULO_TOOLS.md.",
    scope: [
      "Migration 0197 + modulo Go (short_links + qr_codes, redirect publico /s e /q)",
      "Paginas reais tools/qr-code + tools/encurtador-de-link (substituem os mocks do demo-pages)",
      "QR customizavel (cor/fundo/tamanho) gerado no cliente + download",
      "Scripts (snippets de mensagens): fase futura, ainda mock"
    ],
    dependsOn: []
  },
  {
    id: "social_publishing",
    label: "Agendamento de postagens",
    route: "/postagens",
    status: "in_progress",
    priority: "P1",
    category: "operacao-comercial",
    description:
      "Workspace isolado para conectar uma conta profissional do Instagram, preparar/agendar publicações de imagem e consultar analytics. A integração com Calendário e Crow Assistant está desenhada, mas bloqueada até a homologação e um novo comando.",
    scope: [
      "MVP: conexão técnica cifrada + rascunho/agendamento/publicação de imagem",
      "Fila PostgreSQL durável com idempotência, retry e dead letter",
      "Analytics por publicação e visão geral do cliente",
      "Contrato futuro por source_ref para Calendário e Crow Assistant"
    ],
    dependsOn: []
  },
  {
    id: "content_operations",
    label: "Operação de conteúdo",
    route: "/operacao-conteudo",
    status: "in_progress",
    priority: "P1",
    category: "operacao-comercial",
    description:
      "Módulo pausado e incompleto. Cruza Tasks e Calendário para calcular lembretes por cliente e os entrega à central global de notificações. O Crow possui apenas acesso de leitura ao resumo. A base funcional existe, mas ainda faltam testes amplos, homologação das regras e definição dos disparos fora da sessão.",
    scope: [
      "Já existe: API de brief, regras por cliente, página própria e integração com a central global",
      "Já existe: contexto somente leitura para o Crow responder perguntas",
      "Pendente: testes HTTP/repository/permissões/multi-tenant/fuso e testes frontend/E2E",
      "Pendente: homologação com dados reais representativos e revisão de falsos alertas",
      "Pendente: histórico, confirmar/adiar/dispensar, preferências e disparos fora da sessão"
    ],
    dependsOn: ["tasks", "calendar"]
  }
];
