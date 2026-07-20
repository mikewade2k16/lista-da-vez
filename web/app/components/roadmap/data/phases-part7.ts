import type { RoadmapPhase } from "./types";

// ─── Módulo de Atendimento WhatsApp (Omnichannel) ────────────────────────────
//
// Fusão de dois trilhos que existiam separados (2026-07-16): o PORT do inbox
// legado vira o caminho do FRONT; a spec externa rege o BACKEND (domínio,
// setores/filas, triagem IA, segurança/LGPD) e as telas novas de config.
// Substitui a numeração antiga OMNI-F0..F9 do port por F0..F14 — renumerar é
// seguro porque nada foi implementado (tudo pending).
//
// LIBERADO PARA IMPLEMENTAÇÃO (2026-07-17, decisão do dono): a branch
// refactor/multi-tenant-complete fechou e o congelamento que segurava todas as
// fases SAIU. Este roadmap deixa de ser só desenho — a F0 já pode começar.
//
// O módulo é INDEPENDENTE: integração com outros módulos está FORA deste plano
// — se, depois de fechar, for preciso integrar, vira plano próprio.
//
// Doc canônico: docs/omnichannel/PLANO_ATENDIMENTO.md (fonte de verdade).
// Specs por fase: docs/omnichannel/specs/OMNI-F*.md.
// Anexo técnico do FRONT (contratos verbatim, não duplicados no canônico):
// docs/omnichannel/PLANO_PORT_OMNICHANNEL.md + SPECS_PORT_OMNICHANNEL.md.
export const ROADMAP_PHASES_PART7: RoadmapPhase[] = [
  {
    id: "omni-f0-decisoes",
    code: "OMNI-F0",
    title: "Decisões + fundação",
    goal: "Registrar com data as 7 decisões do dono que definem o módulo (D-A multi-provider, D-B port=front/spec=backend, D-C LLM nativo no Go, e as 4 de 2026-07-17: D-D liberação, D-E pending=7º estado, D-F código morto fora, D-G idempotência por conta), publicar o plano canônico, marcar o legado e espelhar no roadmap. Sem código de produto.",
    status: "pending",
    estimateWeeks: "registro + documentação",
    group: "omnichannel-port",
    tasks: [
      { id: "omni-f0-da", label: "[decisao] D-A — Provider = adapter MULTI-PROVIDER: meta_whatsapp_cloud + evolution/waha + mock, escolha por conta/número", done: false, note: "REVISA a D1 do port (que cravava Evolution single-provider) — a D1 fica SUPERADA. Racional: conta séria quer o número oficial (sem risco de ban); conta pequena/piloto quer o não-oficial (sem app review, sem custo por conversa). Um adapter custa uma interface; um provider errado custa uma migração." },
      { id: "omni-f0-db", label: "[decisao] D-B — Fusão: o port verbatim é o caminho do FRONT do inbox; a spec externa rege o BACKEND (domínio, setores/filas, triagem, segurança/LGPD) e as telas novas de config", done: false, note: "O front legado é código maduro em produção que ninguém aqui escreveu — reescrever enquanto porta é trocar 2 problemas por 4. Já o backend legado é Fastify+Prisma com auth N+1 por request e base64 no Postgres: não é o que se quer manter. Telas novas de config NÃO são verbatim — nascem no design system da casa." },
      { id: "omni-f0-dc", label: "[decisao] D-C — LLM nativo em Go (provider/modelo/chave do painel, padrão já validado no calendário). A triagem sobrevive sem n8n; n8n só integrações periféricas", done: false, note: "REVISA a F8 do port, que punha a IA dentro do n8n. IA que decide roteamento é caminho crítico: não depende de um container que pode cair nem de um workflow cuja versão vive no SQLite do n8n e diverge do arquivo (falha real e recorrente — project_vps_n8n_import_gap)." },
      { id: "omni-f0-dd", label: "[decisao] D-D (2026-07-17) — Congelamento LIBERADO: a refactor/multi-tenant-complete fechou e o aviso 'IMPLEMENTAÇÃO CONGELADA' SAI de todos os documentos do módulo", done: false, note: "Decisão do dono em 2026-07-17. Enquanto o aviso ficar num doc, o doc mente — a F0 deixa de ter blocker. O registro HISTÓRICO de que houve congelamento continua onde explica uma decisão passada; o que sai é o AVISO ATIVO de bloqueio (cabeçalho deste arquivo, canônico, specs)." },
      { id: "omni-f0-de", label: "[decisao] D-E (2026-07-17) — pending é o 7º state da máquina (opção A do Contrato 3.1 da F8), com o 12º evento human.pending (PATCH status → PENDING). Projeta pending → PENDING", done: false, note: "Decisão do dono em 2026-07-17, fechando o Contrato 3.1 que bloqueava a F8. Racional VERIFICADO no legado: PENDING é rótulo MANUAL do operador ('parei nesta, estou esperando algo') — sem produtor automático e sem limpeza automática. O candidato queued → PENDING está DESCARTADO com evidência: queued é produzido pelo MOTOR, e mapear trocaria 'filtro sempre vazio' por 'filtro sempre cheio'. PENDING é ORTOGONAL ao roteamento. Sai de pending por: msg.outbound.human → human_active; human.assign → human_active; conv.close → closed. msg.inbound em pending = self (o rótulo é do OPERADOR; o cliente não o limpa). Consequências: o CHECK de conversations.state nasce com 7 valores na F2 (a F8 NÃO faz ALTER); a matriz da F8 vira 7 × 12 = 84 pares (era 6 × 11 = 66); o 409 invalid_transition interino para PATCH status → PENDING DEIXA DE EXISTIR." },
      { id: "omni-f0-df", label: "[decisao] D-F (2026-07-17) — Código morto do port (D4) FICA DE FORA: OmnichannelAuditModule.vue + useOmnichannelAudit.ts não são portados. A D4 sai de 'pendente' e vira DECIDIDA: fora", done: false, note: "Decisão do dono em 2026-07-17. Não renderizam nem no legado (as páginas que os chamariam redirecionam para fora). NÃO é remover funcionalidade (princípio 3) — é não importar código inalcançável. Bônus: era a única dependência de ~/components/docs/ProjectDocsModule.vue. A F1 deixa de ter esse blocker e os arquivos copiados byte a byte caem 2." },
      { id: "omni-f0-dg", label: "[decisao] D-G (2026-07-17) — idempotency_key é POR CONTA: unique(account_id, idempotency_key), NÃO UNIQUE global", done: false, note: "Decisão do dono em 2026-07-17. A chave vem do cliente; UNIQUE global deixa a conta A colidir com a chave da conta B e SUPRIMIR o envio dela — fere o princípio 2 (isolamento). A spec da F3 já tinha divergido do canônico exatamente assim: agora a divergência VIROU A NORMA e o canônico §7.1 muda (deixa de dizer 'idempotency_key UNIQUE' global). Onde alguma spec exigir prefixar a chave com o account_id como mitigação do UNIQUE global, isso vira DESNECESSÁRIO — o unique composto é o mecanismo." },
      { id: "omni-f0-plano", label: "[doc] Publicar docs/omnichannel/PLANO_ATENDIMENTO.md como canônico e rebaixar PLANO_PORT_OMNICHANNEL.md + SPECS_PORT_OMNICHANNEL.md a ANEXO TÉCNICO do front", done: false, note: "Contrato do front verbatim (paginação limit+beforeId, os 3 shapes de message.updated, proteções do webhook, mapa de rotas Node→Go) NÃO é duplicado no canônico — ele remete. Duplicar contrato é criar duas verdades (princípio 1)." },
      { id: "omni-f0-legado", label: "[doc] docs/LEGADO.md: os 5 itens (front sem backend em F1, 6 adaptadores de costura, arquivos >450 linhas, módulo em web/app em vez de layer, segredos do calendário sem cifragem)", done: false, note: "Princípio 4: legado nunca escondido como pronto. Cada item com alvo de remoção declarado (quase todos F14)." },
      { id: "omni-f0-roadmap", label: "[doc] Espelhar no roadmap: phases-part7.ts (F0..F14) + groups.ts + modules.ts", done: false, note: "O módulo novo mora em messaging.*. Espelho do canônico: painel, plano e AGENT.md do módulo sempre em sync (feedback_three_docs_sync)." }
    ],
    verifiable: "Roadmap mostra F0–F14 com o grupo da fusão; LEGADO.md lista os 5 itens com alvo; o canônico existe e o port está marcado como anexo; nenhum doc do módulo ainda exibe o aviso de congelamento (D-D).",
    blockers: []
  },
  {
    id: "omni-f1-front-verbatim",
    code: "OMNI-F1",
    title: "Front verbatim + costura",
    goal: "/omnichannel abre e é visualmente o inbox do legado. Nenhum dado carrega — a tela é real, o backend não existe ainda. Dos 78 arquivos do inventário, 71 vêm byte a byte; 5 são repontados (socket.io e rotas Nitro não existem aqui); 2 são código morto que fica FORA (D-F, 2026-07-17); 6 arquivos de costura fazem a adaptação inteira. INALTERADA pela fusão (D-B).",
    status: "in_progress",
    estimateWeeks: "independente de F2/F3 — pode começar já",
    group: "omnichannel-port",
    tasks: [
      { id: "omni-f1-copia", label: "[front] Copiar os 71 arquivos VERBATIM (byte a byte, sem Prettier/ESLint --fix) para web/app/: composables/omnichannel/ (49), components/omnichannel/ (22), pages/omnichannel/index.vue (do inbox.vue)", done: true, note: "Em web/app/, NÃO em layer: dentro de layer o '~' resolve p/ web/app e os imports do legado quebrariam (finance/AGENT.md:48-53). Precedente: o calendário também não é layer. NÃO copiar os 4 redirects. NÃO copiar OmnichannelAuditModule.vue + useOmnichannelAudit.ts: são CÓDIGO MORTO e ficam fora por decisão do dono (D-F, 2026-07-17) — nunca renderizam nem no legado, e eram a única dependência de ~/components/docs/ProjectDocsModule.vue. Contagem honesta no disco (spec OMNI-F1 C1): 78 total − 4 redirects = 74 copiados − 5 repontados = 69 byte a byte, e 67 com a D-F aplicada. Armadilhas: OmnichannelInboxLoading.vue usa USkeleton SEM <script> (auto-import puro); useOmnichannelInboxRealtime.ts:42 usa ref() sem importar de vue." },
      { id: "omni-f1-costura", label: "[front] Os 6 arquivos de costura em web/app/: composables/useApi.ts (prefixa /v1/omnichannel + ApiClientError), useAdminSession.ts, usePageBootstrapLoading.ts, stores/session-simulation.ts, stores/ui.ts, types/index.ts (barrel dos ~25 tipos)", done: true, note: "useApi().apiFetch delega p/ createApiRequest e NÃO seta X-Account-Id (o provider global injeta). useAdminSession mapeia p/ useAuthStore + useCoreAccountStore; legacyRole = mapear papel do Omni p/ ADMIN|SUPERVISOR|AGENT|VIEWER (o front gateia por ele). São ADAPTADORES temporários: entram no LEGADO.md com alvo F14." },
      { id: "omni-f1-repontar", label: "[front] Repontar os 5 que não vêm verbatim: useOmnichannelInboxRealtime (socket.io→F5), useInboxChatGifAssets (/api/gif→Go), useAvatarProxy (/api/avatar→Go), useInboxChatMediaActions (URL /api/bff literal), useOmnichannelInboxOutboundPipeline (remover o fetch direto no :4000 da linha 252)", done: true, note: "web/ NÃO tem Nitro (BFF eliminado 2026-07-02, ADR 0002) — as rotas /api/* do legado não existem aqui. Nenhum dos 5 muda comportamento, só p/ onde aponta." },
      { id: "omni-f1-registro", label: "[front] Os 6 pontos de registro (todos obrigatórios, senão dá drift): routeRules /omnichannel/**, workspaces.ts, permissions.ts (3 lugares), nav.config.ts (tirar hidden), MODULE_PATH_GUARDS, e o módulo no registry Go + moduleGatingRules", done: false, note: "PARCIAL: os 5 pontos de FRONT estão feitos (routeRules /omnichannel/**, workspaces.ts, permissions.ts nos 3 lugares, nav.config.ts sem hidden + beta, MODULE_PATH_GUARDS). FALTA o 6º: registry.MustRegister + moduleGatingRules no Go — entregue à F2 nesta execução. ATENÇÃO: enquanto o módulo não existir no Go, o SyncCatalog não cria a linha em core.modules, nenhuma conta tem omnichannel em enabledModules e o MODULE_PATH_GUARDS manda /omnichannel para /perfil — a página só abre depois disso. CORREÇÃO de rumo: o icon do workspaces.ts NÃO é chave do NAV_ICON_MAP, é ligature do Material Icons Round (usamos forum); no nav.config.ts, messages está correto. CONFIRMADO no disco: nav.config.ts:9-13 já tem o item omnichannel com hidden:true → vira beta; nuxt.config.ts:58 tem '/omnichannel': {ssr:false} mas FALTA '/omnichannel/**'. icon é CHAVE do NAV_ICON_MAP, não nome livre — 'messages' já existe. moduleGatingRules em app.go:518 ({Prefix,ModuleID}). SyncCatalog no boot registra as permissões E auto-habilita nas contas is_agency (catalog_postgres.go:147)." },
      { id: "omni-f1-demo", label: "[front] Remover o placeholder: web/app/pages/omnichannel.vue + a chave 'omnichannel' de demo-pages.ts:22", done: true, note: "Precedente: finance/AGENT.md:28-29 (o demo foi removido ao portar o módulo real)." },
      { id: "omni-f1-badge", label: "[legado] Badge admin 'SEM BACKEND (F1)' na página: a tela é real, os dados não. Remover ao fechar F2/F4", done: true, note: "Princípio 4: nunca esconder legado/mock como pronto. Sem o badge, F1 vira dívida invisível e alguém desenvolve achando que está pronto. ARMADILHA: platform_admin tem has()=false no front — todo gating precisa de isPlatformAdmin || has(...), senão o módulo some justamente para quem administra." }
    ],
    verifiable: "/omnichannel abre logado com layout idêntico ao legado, badge de sem-backend visível para admin, requests 404 no console, npm run dev sem erro de resolução de import. ESLint acusa max-lines (esperado e consciente, alvo F14).",
    blockers: []
  },
  {
    id: "omni-f2-go-leitura",
    code: "OMNI-F2",
    title: "Go: schema messaging.* + leitura",
    goal: "O inbox lista dados reais do banco (vazios, mas reais). Migrations messaging.* + módulo Go no padrão da casa + rotas de leitura com o shape exato que o front espera. As colunas de estado/fila/provider JÁ NASCEM na migration — não são ALTER depois.",
    status: "pending",
    estimateWeeks: "pode correr em paralelo com F1 e F3",
    group: "omnichannel-port",
    tasks: [
      { id: "omni-f2-migrations", label: "[banco] Migrations messaging.* a partir de 0200: conversations, messages, contacts, whatsapp_instances, saved_stickers, audit_events, hidden_messages, account_config — SQL plano idempotente, schema-qualificado, account_id uuid NOT NULL REFERENCES core.accounts(id)", done: false, note: "CONFERIR O DISCO ANTES DE NUMERAR: existem DOIS arquivos 0197 (operation_validation_reason e tools_module) — a numeração não é validada por ninguém. Última no disco é 0199, então 0200 está livre. SEM '-- +goose Down' (o migrator roda o arquivo INTEIRO e o Down se auto-destrói). Migration nova exige build --no-cache api (embed.FS não re-embute com cache de camada)." },
      { id: "omni-f2-colunas-novas", label: "[banco] As colunas da fusão nascem JUNTO, não em ALTER: whatsapp_instances += provider (CHECK meta_whatsapp_cloud|evolution|waha|mock), provider_config jsonb, credentials_ciphertext; conversations += state (CHECK com os 7 valores: new|ai_active|routing|queued|human_active|pending|closed), department_id, queue_id, assigned_user_id, extracted_fields jsonb", done: false, note: "O CHECK de state NASCE com os 7 valores — pending incluído (D-E, decisão do dono em 2026-07-17). A F8 NÃO faz ALTER para acrescentar pending depois. Menos migration, menos backfill, menos janela de inconsistência. Índices do port continuam obrigatórios: conversations UNIQUE(account_id, external_id, channel, instance_scope_key) = dedupe + (account_id, last_message_at DESC); messages (conversation_id, created_at); contacts UNIQUE(account_id, phone); instances UNIQUE(account_id, instance_name)." },
      { id: "omni-f2-modulo", label: "[back] Módulo back/internal/modules/omnichannel/ no padrão da casa (module.go, http*.go, service*.go, store_postgres*.go), camadas estritas handler→service→repository, teto ~450 linhas (aqui o limite VALE, é código novo)", done: false, note: "O módulo NÃO existe ainda em back/internal/modules/ nem em web/app/components/ — não há AGENT.md a sincronizar. Ele NASCE junto com o código aqui e na F1." },
      { id: "omni-f2-rotas", label: "[back] Rotas de leitura: GET conversations (ordena last_message_at DESC, SEM paginação), GET/{id}/messages, GET/{cid}/messages/{mid}, GET+POST+PATCH contacts, GET+PATCH account, GET whatsapp/instances + /access", done: false },
      { id: "omni-f2-paginacao", label: "[back] Paginação de mensagens EXATA: limit 1..200 (default 100) + beforeId (NÃO é cursor). Resolve beforeId→created_at, filtra <, ordena DESC, take limit, INVERTE o array (devolve ASC). hasMore = existe mais antiga que a primeira. Resposta { conversationId, messages[], hasMore }", done: false, note: "Contrato verbatim — vive no anexo técnico (SPECS_PORT_OMNICHANNEL.md F2), não é redecidido aqui. Divergir quebra a janela de mensagens e o scroll infinito do front." },
      { id: "omni-f2-shapes", label: "[back] Shapes campo a campo do legado (Message, Conversation com lastMessage aninhado, Contact), JSON camelCase. tenantId→account_id; instanceScopeKey = instance_name (não o id)", done: false, note: "MessageStatus no legado é só PENDING|SENT|FAILED — NÃO existe DELIVERED/READ (não há tracking de ACK). Se quisermos, é feature nova avaliada na F14, não port." },
      { id: "omni-f2-isolamento", label: "[seguranca] account_id SEMPRE do Principal, nunca do body; repositório filtra por conta também (defesa em profundidade); fora de escopo → 404, nunca 403", done: false, note: "Princípio 2. 403 vs 404 vaza que o recurso existe (enumeration). O legado resolve tenant por x-selected-tenant-slug/x-client-id; o Omni resolve por X-Account-Id no Principal — não portar o modelo do legado." }
    ],
    verifiable: "Logado, /omnichannel lista do banco. Inserir uma conversa na mão no banco → aparece na tela. X-Account-Id de outra conta → 404.",
    blockers: []
  },
  {
    id: "omni-f3-infra-transversal",
    code: "OMNI-F3",
    title: "[NOVA] Infra transversal (jobs · secretbox · llm)",
    goal: "Três peças que NÃO são do omnichannel — são da plataforma, e nascem em back/internal/platform/ porque o segundo consumidor já é previsível (o calendário, para segredos e LLM). Outbox durável, cifragem em repouso e client LLM nativo.",
    status: "pending",
    estimateWeeks: "independente — pode correr em paralelo com F1 e F2",
    group: "omnichannel-port",
    tasks: [
      { id: "omni-f3-jobs", label: "[back] platform/jobs — outbox + worker: FOR UPDATE SKIP LOCKED, retry/backoff CLASSIFICADO, FIFO por ordering_key, dead-letter", done: false, note: "Retry herdado do legado: transitório→5; 401/403/404/405 e 400/422 conhecidos→1 (unrecoverable); 429→5; 5xx→4; sem status→4; outros→3. Monitor de presas >10min COM FILTRO DE CONTA — o legado varre a tabela inteira sem tenant; NÃO portar esse comportamento. Estado no banco, sem BullMQ/Redis (princípio 1)." },
      { id: "omni-f3-fifo", label: "[back] Teste de concorrência DEDICADO provando FIFO por conversa com N workers (ordering_key = conversation_id)", done: false, note: "RISCO 5 do canônico: SKIP LOCKED dá throughput, mas duas mensagens da mesma conversa em workers diferentes podem inverter a ordem — o cliente vê a resposta antes da pergunta. Sem teste dedicado isso só aparece em produção, no cliente final." },
      { id: "omni-f3-secretbox", label: "[seguranca] platform/secretbox — cifragem em repouso: AES-256-GCM, chave via env OMNI_SECRETS_KEY, prefixo 'v1:' para rotação, saída SEMPRE {set,last4}", done: false, note: "GAP DE SEGURANÇA REAL, não conveniência: calendar/secrets.go NÃO cifra em repouso — {set,last4} é mascaramento de SAÍDA e a chave é gravada em TEXTO PURO. O contrato de saída dele está certo e é o modelo; o que falta é a cifragem. OMNI_SECRETS_KEY é OBRIGATÓRIA: fail-fast no boot, nunca default. Perder a chave = perder os segredos." },
      { id: "omni-f3-llm", label: "[back] platform/llm — client LLM nativo (D-C): adapters openai/gemini/glm, structured output VALIDADO contra schema versionado, usage → ai_runs", done: false, note: "Padrão já validado no calendário (calendar.config → body.ai): provider/modelo/chave vêm do PAINEL, nunca de env, nunca supostos. A triagem (F9) não pode depender do n8n." },
      { id: "omni-f3-limites", label: "[back] Limites por conta em core.account_modules.config jsonb (max_whatsapp_numbers, monthly_ai_runs); defaults em core.platform_settings; estouro → 409 com erro acionável", done: false, note: "A coluna config jsonb JÁ EXISTE (0100_core_schema.sql:120, not null default '{}'::jsonb) — sem migration nova para isso. Princípio 5: aviso honesto e acionável, não falha silenciosa." },
      { id: "omni-f3-calendario", label: "[legado] Registrar (não executar): migrar os segredos do calendário para o secretbox é pendência NÃO bloqueante deste plano", done: false, note: "É a razão de o pacote nascer em platform/ e não em omnichannel/. Entra no LEGADO.md com alvo 'após a F3'." }
    ],
    verifiable: "Teste de concorrência verde provando FIFO por conversa com N workers; segredo gravado cifrado (prefixo v1:) e lido de volta; a API não sobe sem OMNI_SECRETS_KEY; o client LLM devolve JSON validado contra o schema.",
    blockers: []
  },
  {
    id: "omni-f4-provider-webhook",
    code: "OMNI-F4",
    title: "ChannelProvider + adapters + webhook inbound",
    goal: "A camada tradutora: o front e o domínio só veem o shape canônico. Conectar um número pelo painel e ver mensagem recebida chegar no banco. Adapters mock + evolution (o 1º real), webhook público com todas as proteções do legado.",
    status: "pending",
    estimateWeeks: "",
    group: "omnichannel-port",
    tasks: [
      { id: "omni-f4-interface", label: "[back] Interface ChannelProvider: VerifyWebhook, ParseWebhook, SendMessage, DownloadMedia, Capabilities — + eventos canônicos", done: false, note: "D-A. Mudança da Meta ou troca de provedor = ajustar 1 adapter, zero mudança no domínio, zero no front. Capabilities() é o que sustenta o multi-provider na UI: a tela DEGRADA POR NÚMERO (risco 2) em vez de mentir que todo número faz tudo." },
      { id: "omni-f4-adapters", label: "[back] Adapters mock + evolution (header apikey, timeout 30s): createInstance, connect, fetchInstances, logout, setWebhook, getBase64FromMediaMessage", done: false, note: "Integração WHATSAPP-BAILEYS. No legado a API key é GLOBAL do ambiente (não por tenant) — avaliar por-conta no Omni, que é multi-tenant de verdade. O mock é o que permite testar F5–F9 sem número real." },
      { id: "omni-f4-um-cerebro", label: "[seguranca] Validação INTERNA no cadastro da instância: o mesmo número não pode ser cadastrado em duas instâncias da conta (UNIQUE por conta)", done: false, note: "No cadastro, NÃO no runtime: dois bots respondendo o mesmo cliente é incidente visível para o cliente final. A norma 'um número = um cérebro' vale como regra de OPERAÇÃO — não apontar o mesmo número de WhatsApp para dois sistemas é responsabilidade de quem opera. Colisão com sistema externo é RISCO OPERACIONAL registrado (seção de riscos do canônico), não gate de código." },
      { id: "omni-f4-sessao", label: "[back] Sessão: bootstrap (valida limite de canais→409, cria/renomeia instância, promove default, re-escopa conversas 'default'), connect, logout, status (cache + dedupe in-flight; includeWebhook compara e AUTO-REPARA), qrcode (data URL; cache Redis TTL 120s)", done: false, note: "Não existe tabela de sessão/QR no legado — o QR vive só no Redis (chave wa:qrcode:{accountId}:{instanceName})." },
      { id: "omni-f4-webhook", label: "[seguranca] Webhook inbound — rota PÚBLICA sem JWT, fora do moduleGatingRules. Proteções na ordem: rate-limit 600/min por slug:ip (block 5min→429), token constant-time→401, allowlist content-type→415, content-length→413, conta inexistente→404, idempotência", done: false, note: "Precedente CONFIRMADO de rota fora do gate: /v1/public/* (bio, cardápio) e /s/{slug}, /q/{slug} (tools). Rota pública sem essas proteções é incidente — herdar TODAS do legado (anexo técnico, SPECS F3)." },
      { id: "omni-f4-dedupe", label: "[back] Dedupe inbound em messaging.webhook_events com UNIQUE(provider, external_event_id) — no BANCO, não aplicativo", done: false, note: "O legado faz dedupe APLICATIVO (não há UNIQUE). Aqui a garantia é do banco. Payload é DINÂMICO — parsear defensivamente (event de payload.event ?? type ?? data.event, normalizado [^a-zA-Z0-9]+→_ e uppercase). Inbound grava status SENT e created_at = messageTimestamp do provider." }
    ],
    verifiable: "Ler o QR no painel, conectar o número, mandar mensagem do celular e ela existir em messaging.messages na conversa certa. Webhook sem assinatura → 401. Evento repetido não duplica. Cadastrar o mesmo número em duas instâncias da conta → bloqueado no cadastro.",
    blockers: ["F2", "F3"]
  },
  {
    id: "omni-f5-realtime",
    code: "OMNI-F5",
    title: "Realtime (socket.io → WS nativo)",
    goal: "Mensagem aparece ao vivo, sem refresh. Canal Go no padrão ticket + reescrita do único arquivo do front que não pode vir verbatim.",
    status: "pending",
    estimateWeeks: "",
    group: "omnichannel-port",
    tasks: [
      { id: "omni-f5-canal", label: "[back] GET /v1/realtime/omnichannel no padrão ticket (POST /v1/ws/ticket → ?scope=account&accountId=&ticket=), canal omnichannel:account:{id}, realtimeService injetado como Publisher (igual calendar.WithPublisher)", done: false },
      { id: "omni-f5-eventos", label: "[back] Exatamente 3 eventos (message.created, message.updated, conversation.updated) com os shapes REPLICADOS POR CALL-SITE — são 3 shapes distintos de message.updated e 2 de conversation.updated. NÃO unificar", done: false, note: "message.created: envio HTTP→Message completo+correlationId; webhook→subconjunto. message.updated: worker→mínimo {id,status,externalMessageId,updatedAt,correlationId}; rehidratação→completo SEM correlationId. conversation.updated: webhook→SEM instanceName/instanceDisplayName; status/contacts→COM. Unificar quebra o front. Detalhe no anexo técnico (SPECS F4)." },
      { id: "omni-f5-sanitize", label: "[perf] Sanitizar mídia no realtime: data URL → null. Nunca trafegar base64 no WS (o front busca pelo endpoint /media)", done: false },
      { id: "omni-f5-front", label: "[front] Reescrever useOmnichannelInboxRealtime.ts sobre useRealtimeSocket, preservando handlers, nomes de evento e os fallbacks de polling (status 45s, stale 20s, heartbeat 5min, cooldown 5s). Erros de auth eram por string (ModuleAccessDenied/Unauthorized) — mapear", done: false, note: "accountId pela MESMA fonte do REST (accountStore.activeAccountId || auth.activeTenantId || ...). Usar auth.activeTenantId direto faz o platform_admin cair no seed e o handshake nunca virar 101 → close 1006 em loop (tasks/AGENT.md:315-331). Bug já caro, não repetir." }
    ],
    verifiable: "Duas abas em contas diferentes: mensagem do celular aparece ao vivo SÓ na conta certa. Derrubar o WS → polling assume → volta ao reconectar.",
    blockers: ["F4"]
  },
  {
    id: "omni-f6-envio-midia",
    code: "OMNI-F6",
    title: "Envio via outbox + mídia",
    goal: "Responder pelo painel e a mensagem chegar no celular. POST de mensagens sobre o platform/jobs da F3 (o outbox não é mais do módulo) + endpoint de mídia com stream e Range.",
    status: "pending",
    estimateWeeks: "",
    group: "omnichannel-port",
    tasks: [
      { id: "omni-f6-post", label: "[back] POST conversations/{id}/messages com o body do legado. Fluxo: valida escopo → valida upload (413/415 com {message,code,details}) → cria PENDING/OUTBOUND → atualiza last_message_at → enfileira → publica message.created → audita → 200. Falha ao enfileirar → FAILED + publica + 202", done: false, note: "TEXT exige content (max 4000); resto exige mediaUrl. Reply/quote vai em metadataJson (não campo dedicado); sticker = type IMAGE + metadataJson.media.sendAsSticker. Sem permissão de reply → 403 (é permissão, não escopo)." },
      { id: "omni-f6-outbox", label: "[back] Envio pelo platform/jobs (F3): unique(account_id, idempotency_key) + ordering_key = conversation_id (FIFO por conversa). O módulo NÃO reimplementa outbox nem retry", done: false, note: "MUDANÇA em relação ao port: o port previa a tabela de outbox só para o envio; na fusão ela é infra transversal e nasce na F3. IDEMPOTÊNCIA POR CONTA (D-G, decisão do dono em 2026-07-17): NÃO é UNIQUE global — a chave vem do cliente, e chave global deixa a conta A colidir com a da conta B e suprimir o envio dela (fere o princípio 2). O unique composto É o mecanismo: a chave do cliente NÃO é prefixada com o account_id. Enviar 2× com a mesma idempotency_key na mesma conta → 1 mensagem só." },
      { id: "omni-f6-media", label: "[back] GET messages/{mid}/media: exclui hidden_messages do usuário→404; rehidrata via DownloadMedia do provider quando mediaUrl vazio/requiresMediaDecrypt/url_encrypted (one-shot por request) e emite message.updated; anti-SSRF (host interno→403, protocolo≠http/https→422); Cache-Control private max-age=60", done: false, note: "D2 do port mantida: disco (path na coluna) + stream com suporte a Range, sem carregar em memória. O legado faz Buffer.from(await res.arrayBuffer()) do arquivo inteiro, sem streaming e sem Range, e guarda base64 no Postgres (data URL até 60MB). O front SÓ consome GET /media — o storage é invisível p/ ele." }
    ],
    verifiable: "Responder do painel e chegar no celular; status vira SENT; foto/áudio sobem e reproduzem; derrubar o provider → FAILED após os retries; enviar 2× com a mesma idempotency_key na MESMA conta → 1 mensagem; a mesma chave em OUTRA conta → envia normal (unique é composto, D-G); mensagem presa >10min é re-enfileirada (com filtro de conta).",
    blockers: ["F3", "F4"]
  },
  {
    id: "omni-f7-acoes",
    code: "OMNI-F7",
    title: "Ações do inbox",
    goal: "Cada botão da UI funciona ponta a ponta: reação, encaminhar, apagar (para mim / para todos), status, atribuir, participantes de grupo, sync de histórico, import de contatos. O que mexe em status passa PELA MÁQUINA DE ESTADOS da F8.",
    status: "pending",
    estimateWeeks: "",
    group: "omnichannel-port",
    tasks: [
      { id: "omni-f7-mensagens", label: "[back] POST messages/{mid}/reaction, messages/forward ({messageIds 1..100, targetConversationId}), messages/delete-for-me (grava em hidden_messages), messages/delete-for-all", done: false },
      { id: "omni-f7-conversas", label: "[back] PATCH conversations/{id}/status e /assign VIA MÁQUINA DE ESTADOS (F8) — nunca escrevendo status na mão; GET /group-participants; POST conversations/sync-open; POST messages/sync-history", done: false, note: "MUDANÇA da fusão: state é a verdade, status é PROJEÇÃO derivada na serialização. assign ⇒ state=human_active ⇒ hard-block da IA. Escrever status direto fura a máquina e é exatamente o risco 4 do canônico." },
      { id: "omni-f7-contatos", label: "[back] POST contacts/import-whatsapp + contacts/{id}/open-conversation", done: false },
      { id: "omni-f7-audit", label: "[back] Auditar em messaging.audit_events: MESSAGE_OUTBOUND_QUEUED|SENT|FAILED, CONVERSATION_STATUS_CHANGED, CONVERSATION_ASSIGNED", done: false },
      { id: "omni-f7-instancias", label: "[seguranca] Escopo de instância por usuário CORRIGIDO (não portar o bug): no legado o ternário isTenantAdmin || len<=1 ? activeInstances : activeInstances retorna o mesmo nos 2 ramos — o filtro é INOPERANTE e todo usuário vê todas as instâncias", done: false, note: "É isolamento (princípio 2) — portar corrigido e avisar o usuário da mudança de comportamento em relação ao legado." }
    ],
    verifiable: "Cada ação da UI funciona no browser: reagir, encaminhar p/ outra conversa, apagar p/ mim (some só p/ mim), apagar p/ todos, mudar status, atribuir, ver participantes do grupo. Atribuir a um humano → a IA cala.",
    blockers: ["F6", "F8"]
  },
  {
    id: "omni-f8-dominio-atendimento",
    code: "OMNI-F8",
    title: "[NOVA] Domínio de atendimento (setores · filas · roteamento)",
    goal: "O que o legado não tem: setores, filas, atribuição e roteamento determinístico auditável. Máquina de estados com TODAS as transições tabeladas e projeção state→status para o front verbatim.",
    status: "pending",
    estimateWeeks: "pode correr em paralelo com F4–F7 (domínio não depende do canal)",
    group: "omnichannel-port",
    tasks: [
      { id: "omni-f8-tabelas", label: "[banco] departments UNIQUE(account_id, slug); queues UNIQUE(account_id, department_id, slug); queue_members UNIQUE(queue_id, user_id); routing_rules (ordenadas por prioridade); routing_decisions (auditoria de cada decisão)", done: false, note: "O legado só tem assign manual — o port declara isso em §14. queue_members É O GATE DE DADO, não é tabela de conveniência." },
      { id: "omni-f8-maquina", label: "[back] Máquina de estados com TODAS as transições TABELADAS — sem exceção, sem 'e os outros casos análogos': os 7 states (new · ai_active · routing · queued · human_active · pending · closed) × 12 eventos = 84 pares", done: false, note: "RISCO 4 (o ponto mais frágil da fusão): o front verbatim não pode mudar (D-B) e o domínio precisa da máquina nova. Um estado que projete errado = inbox mostrando conversa fechada como aberta. A mitigação É tabelar tudo; transição implícita é bug esperando. D-E (decisão do dono em 2026-07-17) fechou o Contrato 3.1 pela opção A: pending é o 7º state e human.pending (PATCH status → PENDING) é o 12º evento — a matriz era 6 × 11 = 66 e agora é 7 × 12 = 84 pares, e o 409 invalid_transition interino do PATCH → PENDING deixa de existir. Sai de pending por: msg.outbound.human → human_active; human.assign → human_active; conv.close → closed. msg.inbound em pending = self (o rótulo é do operador; o cliente não o limpa)." },
      { id: "omni-f8-projecao", label: "[back] Projeção state→status na SERIALIZAÇÃO: new/ai_active/routing/queued→OPEN; human_active→OPEN + assignedTo preenchido; pending→PENDING; closed→CLOSED", done: false, note: "state é a fonte de verdade; status é projeção derivada. O front verbatim conhece OPEN/PENDING/CLOSED (ConversationStatus, types/index.ts:91) e não pode ser tocado (D-B) — pending→PENDING é a linha que a D-E acrescentou (2026-07-17), fechando o único status que antes não tinha state produtor. Nunca persistir status como campo independente — seria a segunda verdade (princípio 1)." },
      { id: "omni-f8-roteamento", label: "[back] Motor determinístico lê routing_rules por prioridade e DECIDE; grava a decisão (entrada, regra que casou, saída) em routing_decisions", done: false, note: "IA sugere; o motor decide (F9). Isso é o que torna o roteamento auditável e TESTÁVEL SEM CHAMAR MODELO. LLM não escolhe fila sozinha — preenche campos; a regra decide." },
      { id: "omni-f8-gate-dado", label: "[seguranca] Permissão gateia FEATURE; fila gateia DADO. conversations.view NÃO é 'vê tudo': o atendente vê só as conversas das filas onde é queue_member + as atribuídas a ele. Filtro NO REPOSITÓRIO", done: false, note: "Defesa em profundidade (princípio 2): não só no service, não só no front. Conversa fora do escopo → 404, nunca 403." },
      { id: "omni-f8-permissoes", label: "[back] Permission keys omnichannel.* seedadas pelo Module Registry no boot (conversations.view/reply/assign/close, contacts.manage, instances.manage, settings.manage, agents.manage, audit.view) + role templates attendant/supervisor/manager", done: false, note: "CONFIRMADO: a validação de chave contra módulo habilitado já é de graça — InvalidPermissionKeys (rbac_repository.go:385) faz JOIN core.permissions × core.account_modules com am.enabled=true e p.deprecated_at is null. Permissão de módulo desabilitado é inválida sem código novo." },
      { id: "omni-f8-handoff", label: "[back] Handoff IA→humano: assign ⇒ state=human_active ⇒ HARD-BLOCK da IA", done: false, note: "Substitui o paused_until da spec externa: janela de tempo EXPIRA SOZINHA e o bot volta a falar por cima do atendente. Estado é mais honesto que timer." }
    ],
    verifiable: "Conversa entra → cai na fila certa por regra; atendente de outra fila NÃO vê (404); routing_decisions explica cada decisão (entrada, regra, saída); atribuir a humano → state=human_active e a IA cala.",
    blockers: ["F2"]
  },
  {
    id: "omni-f9-triagem-ia",
    code: "OMNI-F9",
    title: "[NOVA] Triagem IA no Go (sem n8n no caminho crítico)",
    goal: "IA de triagem com client nativo em Go (D-C): extrai campos, o motor determinístico da F8 roteia. Config 100% do painel; a triagem sobrevive com o n8n desligado.",
    status: "pending",
    estimateWeeks: "",
    group: "omnichannel-port",
    tasks: [
      { id: "omni-f9-tabelas", label: "[banco] ai_agents; ai_agent_versions (publish/rollback); ai_runs (input, output, schema, usage/custo); collect_field_defs (campos que a IA extrai)", done: false, note: "ai_runs.usage é a base do custo por conta na F13. Sem gravar usage na hora, não há como reconstruir custo depois." },
      { id: "omni-f9-prompt", label: "[back] Prompt em 8 camadas montado no Go, com provider/modelo/prompt vindos do PAINEL (nunca de env, nunca supostos)", done: false, note: "Padrão validado no calendário: config no banco → body.ai. NUNCA supor provider — checar o banco (feedback_ai_config_from_panel)." },
      { id: "omni-f9-schema", label: "[back] Saída JSON VALIDADA contra schema versionado (platform/llm da F3). IA SUGERE → o motor da F8 DECIDE", done: false, note: "LLM não escolhe fila sozinha: preenche campos; routing_rules decide. Saída não validada = roteamento que ninguém consegue auditar nem testar sem chamar modelo." },
      { id: "omni-f9-hardblock", label: "[back] human_active = hard-block da IA: atribuiu a um humano, a IA cala", done: false },
      { id: "omni-f9-n8n", label: "[back] n8n FORA do caminho crítico: sem lógica, sem prompt, sem config. Só integrações periféricas", done: false, note: "REVISA a F8 do port (IA dentro do n8n). Razão real: workflow cuja versão vive no SQLite do n8n e diverge do arquivo é falha recorrente (project_vps_n8n_import_gap) — e re-import é obrigatório ao mudar o .json. Caminho crítico não depende disso." }
    ],
    verifiable: "Msg → IA extrai campos → regra roteia. Trocar o prompt no painel muda o comportamento sem tocar em código. DESLIGAR O n8n e a triagem continuar funcionando. Atribuir a humano → IA cala.",
    blockers: ["F3", "F8"]
  },
  {
    id: "omni-f10-telas-config",
    code: "OMNI-F10",
    title: "[NOVA] Telas de config (números · setores/filas · agente)",
    goal: "Configurar número, setor, fila, regra e agente pelo painel, sem tocar no banco. São telas NOVAS (não existem no legado) — nascem no design system da casa, o verbatim não se aplica. Fecha o piloto P0.",
    status: "pending",
    estimateWeeks: "",
    group: "omnichannel-port",
    tasks: [
      { id: "omni-f10-numeros", label: "[frontend] Números/instâncias/providers: escolher provider por número, credenciais (só {set,last4}, nunca de volta cruas), QR/conexão, limite por conta com 409 acionável", done: false, note: "Gateado por omnichannel.instances.manage. Credencial NUNCA volta pro front nem entra em log (F3/secretbox)." },
      { id: "omni-f10-capabilities", label: "[frontend] UI DEGRADA POR NÚMERO via Capabilities(): a tela nunca oferece o que aquele número não faz (template, janela 24h, reação, sticker)", done: false, note: "RISCO 2: assimetria da janela de 24h — Cloud exige template fora da janela, o não-oficial não. Sem isso, o atendente descobre o limite quando a mensagem FALHA. Princípio 5: aviso honesto inline, não default silencioso." },
      { id: "omni-f10-setores", label: "[frontend] Setores, filas, membros de fila e regras de roteamento (ordenadas por prioridade)", done: false, note: "Gateado por omnichannel.settings.manage. ARMADILHA: platform_admin tem has()=false — usar isPlatformAdmin || has(...) senão a seção some para quem administra." },
      { id: "omni-f10-agente", label: "[frontend] Editor de agente de IA com publish/rollback + SIMULADOR mínimo", done: false, note: "Gateado por omnichannel.agents.manage. O simulador é o que permite testar prompt sem mandar mensagem para cliente real." },
      { id: "omni-f10-ds", label: "[frontend] Design system da casa: tokens, nunca hex hardcoded; OmniEntityDrawer/OmniDataTable onde couber", done: false, note: "ARMADILHA: cor hardcoded não troca de tema — e bundle stale do container mostra CSS velho; conferir a versão SERVIDA, não só o disco (project_hardcoded_colors_theming)." }
    ],
    verifiable: "Configurar um número, um setor, uma fila, uma regra e um agente pelo painel, sem tocar no banco. Número sem capability de template não oferece template na UI. Estourar o limite de números → 409 com mensagem acionável.",
    blockers: ["F4", "F8", "F9"]
  },
  {
    id: "omni-f11-meta-cloud",
    code: "OMNI-F11",
    title: "[NOVA] Adapter Meta WhatsApp Cloud (número oficial)",
    goal: "O número oficial, sem risco de ban: adapter Cloud com HMAC próprio da Meta, templates e janela de 24h expostos por Capabilities(). Prova que o multi-provider da D-A é real.",
    status: "pending",
    estimateWeeks: "P1 — depois do piloto",
    group: "omnichannel-port",
    tasks: [
      { id: "omni-f11-hmac", label: "[seguranca] VerifyWebhook da Meta: HMAC SHA-256 do body no header X-Hub-Signature-256 + verify token", done: false, note: "CUIDADO com o precedente: site/http_ingest.go já faz webhook público por HMAC SHA-256 com hmac.Equal (constant-time) e MaxBytesReader — a MECÂNICA é o modelo a seguir e é reaproveitável. Mas o header de lá é X-Signature: sha256=<hex> (padrão próprio, estilo GitHub/Stripe); a META EXIGE X-Hub-Signature-256. Mesma mecânica, header diferente — o adapter tem o seu." },
      { id: "omni-f11-templates", label: "[back] Templates + janela de 24h; Capabilities() declara o que este número faz", done: false, note: "Fora da janela a Cloud EXIGE template; o não-oficial não tem essa restrição. A UI degrada por número (F10), nunca mente." },
      { id: "omni-f11-signup", label: "[back] Embedded Signup → P2 (exige app review da Meta), fora desta fase", done: false, note: "RISCO 1: cobrança POR CONVERSA + quality rating que degrada o número. É risco de PRODUTO, não de código — quem decide preço é a Meta." },
      { id: "omni-f11-ban", label: "[doc] Registrar no contrato com o cliente: Evolution/WAHA usam WhatsApp não-oficial; conta séria usa Cloud. Rate limit por número é MITIGAÇÃO, não garantia", done: false, note: "RISCO 3. Isto entra no contrato, não só no código — prometer que o não-oficial não toma ban é promessa que não se pode cumprir." }
    ],
    verifiable: "Número oficial recebe e envia; webhook sem X-Hub-Signature-256 válido → 401; fora da janela de 24h a UI exige template.",
    blockers: ["F4"]
  },
  {
    id: "omni-f12-stickers-gif",
    code: "OMNI-F12",
    title: "Stickers / GIF / avatar (substituir o Nitro)",
    goal: "As 3 rotas Nitro do legado viram endpoints Go — o web/ não tem BFF e não vai ter. Chave do Tenor no painel, não em env. INALTERADA do port.",
    status: "pending",
    estimateWeeks: "P1",
    group: "omnichannel-port",
    tasks: [
      { id: "omni-f12-stickers", label: "[back] GET+POST+DELETE /v1/omnichannel/stickers: limit 1..200 default 36, array direto, allowlist image/webp|png|jpeg|jpg|gif→415, limite min(conta.max_upload_mb, 20MB)→413, POST→201, DELETE→204, poda FIFO acima de 200/conta", done: false },
      { id: "omni-f12-gif", label: "[back] GET /v1/omnichannel/gif/search + /gif/media (substituem /api/gif/* do Nitro). Chave do Tenor vem do PAINEL/BANCO, não de env", done: false, note: "Princípio 1: toda config que o sistema precisa vem do banco, configurada no painel. Com a F3 pronta, a chave vai cifrada pelo secretbox." },
      { id: "omni-f12-avatar", label: "[back] GET /v1/omnichannel/avatar?url= — proxy com anti-SSRF (mesma allowlist do /media)", done: false }
    ],
    verifiable: "Salvar/usar/apagar sticker; buscar e mandar GIF; avatar do contato carrega sem erro de CORS.",
    blockers: ["F6"]
  },
  {
    id: "omni-f13-lgpd-observabilidade",
    code: "OMNI-F13",
    title: "LGPD + observabilidade (mínimo é P0)",
    goal: "Retenção, purge, masking e custo de LLM por conta. O mínimo entra no piloto P0 — dado de conversa de cliente final não pode nascer sem política de retenção.",
    status: "pending",
    estimateWeeks: "mínimo é P0 (entra no piloto); export/anonimização é P1",
    group: "omnichannel-port",
    tasks: [
      { id: "omni-f13-retencao", label: "[seguranca] Retenção por classe (365/180/90/30 dias default) + JOB DE PURGE que realmente apaga", done: false, note: "Política sem job é política que não existe. O job roda sobre o platform/jobs da F3." },
      { id: "omni-f13-masking", label: "[seguranca] Payload bruto MASCARADO: nunca em log, nem em erro, nem em trace", done: false, note: "Conversa de WhatsApp de cliente final em log é vazamento de dado pessoal — e log costuma sair da VPS." },
      { id: "omni-f13-midia", label: "[seguranca] Política de mídia: retenção por classe + purge do disco (o volume de mídia da F6 precisa entrar no backup E no purge)", done: false, note: "Gap que a spec externa expôs: nenhum módulo tem isso hoje." },
      { id: "omni-f13-custo", label: "[frontend] Custo de LLM por conta na tela, a partir do usage gravado em ai_runs (F9)", done: false, note: "Sem isso, IA por conta é custo sem teto e sem dono. Casa com monthly_ai_runs em account_modules.config (F3)." },
      { id: "omni-f13-export", label: "[back] Export / anonimização a pedido do titular → P1 (fora do mínimo do piloto)", done: false }
    ],
    verifiable: "Job de purge apaga o que passou da retenção (banco e disco); log não contém payload bruto nem em erro; custo de LLM por conta aparece na tela.",
    blockers: ["F9"]
  },
  {
    id: "omni-f14-refactor",
    code: "OMNI-F14",
    title: "Refactor (pagar a dívida do port)",
    goal: "Só depois de F0..F13 verdes: pagar a dívida assumida conscientemente no port. É aqui que o LEGADO.md esvazia.",
    status: "pending",
    estimateWeeks: "P1 — depois de F13",
    group: "omnichannel-port",
    tasks: [
      { id: "omni-f14-split", label: "[faxina] Split dos >450 linhas: useOmnichannelInbox.ts (1467), InboxConversationsSidebar.vue (1128), InboxChatPanel.vue (1110), useOmnichannelInboxHistory.ts (774), useOmnichannelAdmin.ts (764) — INCREMENTAL, com smoke no browser a cada split", done: false, note: "Lição do useTasksPageContext (3063 linhas): o split de uma vez foi adiado por ser arriscado. Fazer incremental e validar." },
      { id: "omni-f14-costura", label: "[faxina] Remover os 6 adaptadores de costura; o módulo passa a usar createApiRequest/auth do Omni direto", done: false },
      { id: "omni-f14-layer", label: "[faxina] Mover web/app/**/omnichannel/** → web/layers/omnichannel/ (aí sim com imports relativos, que é o que o layer exige)", done: false, note: "ARMADILHA: stores de layer NÃO são auto-importadas — exigem import explícito, senão 500 no SSR (project_web_autoimport_layer_stores)." },
      { id: "omni-f14-ds", label: "[frontend] Trocar componentes do legado pelos do design system (OmniEntityDrawer p/ modais etc.); tokens, nunca hex hardcoded", done: false },
      { id: "omni-f14-legado", label: "[doc] Tirar os itens do docs/LEGADO.md + AGENT.md do módulo (front e back) atualizados", done: false },
      { id: "omni-f14-ack", label: "[feature] Avaliar DELIVERED/READ — NÃO existe no legado (o enum é só PENDING|SENT|FAILED, sem tracking de ACK). É feature nova, não port", done: false }
    ],
    verifiable: "ESLint sem max-lines; nenhum adaptador de costura; LEGADO.md limpo dos itens do port; tudo que funcionava em F13 continua funcionando (smoke completo no browser).",
    blockers: ["F0..F13 verdes"]
  }
];
