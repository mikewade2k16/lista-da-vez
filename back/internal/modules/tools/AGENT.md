# Módulo `tools` — Encurtador de Link + Gerador de QR Code

Duas ferramentas por conta, reconstruídas como back Go real (o projeto antigo era mock em
`globalThis` no BFF Nitro eliminado). Plano/decisões: `docs/tools/PLANO_MODULO_TOOLS.md`.

## Escopo
- **Encurtador**: `tools.short_links` → redirect público `GET /s/{slug}` (302, só se `is_active`).
  Toggle Ativo: desativado responde 404 sem contar e volta a funcionar ao reativar (não exclui).
- **QR Code**: `tools.qr_codes` → redirect **rastreado** `GET /q/{slug}` (302, só se `is_active`).
  O QR codifica o `qrUrl` (`/q/{slug}`), não a URL final — por isso trocar o destino, desativar e
  contar scans funcionam com o QR já impresso. A imagem PNG é gerada **no cliente** (lib `qrcode`,
  correção de erro H) a partir de `qrUrl` + cores + `size`; o backend não renderiza/armazena PNG.

## Regras que protegem QR/link já divulgado (2026-11-12)
- **Slug do QR é imutável depois do create.** PATCH com slug diferente (ou vazio) → 409
  `qr_slug_immutable` e nada do payload é gravado; slug igual ao atual é aceito (no-op, telas
  antigas mandam o form inteiro). Para "trocar" o endereço: criar outro QR.
- **Renomear link curto exige `confirmSlugChange: true`** (sem isso 409
  `slug_change_requires_confirmation`; vazio → 400 `invalid_slug`). Slug igual ao atual não exige.
  No painel o slug saiu da edição inline e passou para o botão "Renomear slug" (modal com aviso,
  cliques e checkbox de confirmação). Para pausar sem trocar endereço: toggle Ativo.
- **Cores do QR** (`qr_contrast.go`, espelho em `web/app/utils/qr-contrast.ts`): `#rrggbb`
  obrigatório (inválido → 400 `invalid_color`, nunca default silencioso); QR mais escuro que o fundo
  e contraste WCAG ≥ 4.5:1 (senão 400 `low_contrast`). No PATCH valida o par efetivo (patch + valor
  atual) e grava com trava otimista nas cores lidas (`ExpectFill/ExpectBack`): mudou no meio → 409
  `concurrent_change`. O painel avisa e bloqueia o salvar antes do back recusar.
- **Contagem** (`preview_agents.go`): só `GET` de navegador conta. `HEAD`, User-Agent vazio e robôs
  de prévia/busca (WhatsApp, facebookexternalhit, TelegramBot, Slackbot, Discordbot, Googlebot…)
  recebem o mesmo redirect sem contar. Não incluir na lista nomes que aparecem no navegador interno
  de apps (Instagram, Teams, Snapchat, Viber): ali é uma pessoa clicando. Clique não mexe em
  `updated_at`.
- **Criação do QR no painel** não mostra QR escaneável antes de salvar (só amostra de cores): um
  print do preview com o destino direto seria um QR sem rastreio nem desativação. Ao salvar, o modal
  vira edição com o QR real e o botão "Baixar QR salvo".

## Schema
- `0197_tools_module.sql`: `tools.short_links(id, account_id→core.accounts, slug UNIQUE, target_url,
  hits, created_at, updated_at)` e `tools.qr_codes(id, account_id, slug UNIQUE, target_url,
  fill_color, back_color, size, is_active, scan_count, last_scanned_at, created_at, updated_at)`.
- `0328_tools_short_links_active.sql`: `short_links.is_active` (default true — existentes nascem
  ativos) e `short_links.last_hit_at`.

`slug` é **único global por tabela** (o redirect resolve sem X-Account-Id). Colisão no create/rename
→ o store adiciona sufixo `-2/-3`… (loop sobre a violação de unique `23505`). `/s` e `/q` são
tabelas distintas: o mesmo slug pode existir nas duas.

## Rotas
Painel (gating de módulo `tools` no Chain via `/v1/tools` + permissão por rota):
- `GET /v1/tools/short-links` (`?status=active|inactive`, `q`) — `tools.shortlinks.view`
- `POST`, `PATCH /{id}` (`targetUrl`, `isActive`, `slug`+`confirmSlugChange`), `DELETE /{id}` —
  `tools.shortlinks.manage`
- `GET /v1/tools/qr-codes` (`?status=`, `q`) — `tools.qr.view`
- `POST`, `PATCH /{id}` (`targetUrl`, cores, `size`, `isActive`; sem troca de slug), `DELETE /{id}` —
  `tools.qr.manage`

Público (sem auth, fora do gating — prefixos `/s` e `/q` não estão em `moduleGatingRules`):
- `GET /s/{slug}` → 302 destino; `GET /q/{slug}` → 302 destino. `Cache-Control: no-store`.

O `Resolve` (redirect) só devolve o destino se o item está ativo, a conta dona está `is_active`
**e** o módulo `tools` está habilitado para ela (`core.account_modules`) — igual ao `PublicLookup` do
bio. Qualquer falha = 404 uniforme, sem vazar existência. Atenção: QR/link de conta sem o módulo
ligado não abre (local, 2026-11-12: `bari-cardapio`, `instagram-katarina`, `avriomar`).

## Isolamento multi-tenant e permissões (`requireToolsPermission` em `http.go`)
- `platform_admin`: `RequireAuth`; confia no `X-Account-Id` (troca de conta); `""` = todas as contas
  (list) / edita e remove por id em qualquer conta. No create pode mirar qualquer `accountId` do body.
- Demais: `RequireAuthWithAccount` (membership + permissões efetivas da conta do `X-Account-Id`);
  sem a permissão da rota → 403 `forbidden`. `.manage` inclui `.view` da mesma ferramenta; permissão
  de uma ferramenta não libera a outra. `accountId` do body é **ignorado**. Patch/delete/get filtram
  `id + account_id` → linha de outra conta vira 404.

Toda query do store filtra por `account_id` (quando não-admin). `accountId` inexistente no create
cross-conta do admin → violação de FK `23503` → 400 `invalid_account`.

Permissões declaradas em `module.go` (sync no boot): `tools.shortlinks.view/manage`,
`tools.qr.view/manage`. Templates: `tools.manager`, `tools.viewer`. O menu "Tools" continua na
permissão de página `workspace.tools.view` (módulo core). O painel esconde criar/editar/excluir/
renomear e trava os toggles sem `.manage` (`canManage` nos composables); quem decide é o back.

## Env
- `TOOLS_PUBLIC_BASE_URL` (opcional): base absoluta de `shortUrl`/`qrUrl` (produção:
  `https://crowvisuals.com.br`). Fallback: `PUBLIC_API_BASE_URL`; vazio = URL relativa.
  O Compose Mac preserva essa configuração pública; não substituir por localhost, pois
  o QR precisa funcionar em outros dispositivos. Quando a base aponta para produção,
  edições locais não afetam esse resolver. Validar o resolver local separadamente em
  `http://localhost:8080/q/{slug}` ou `/s/{slug}`, sem mudar a base codificada.
- Em produção o reverse proxy precisa rotear `crowvisuals.com.br/s/*` e `/q/*` para a api.

## Testes
- `tools_rules_test.go` (contraste, contagem por User-Agent, permissões) e `service_tools_test.go`
  (regras de slug, ativação, cores com stores falsos que contam gravações).
- `store_tools_integration_test.go`: roda as migrations reais 0197+0328 num Postgres descartável
  (`TOOLS_TEST_DATABASE_URL`; sem a env o teste é pulado — conferir que não saiu `SKIP`).
- Front: `web/app/utils/qr-contrast.test.ts` (mesma tabela de cores do Go).

## Arquivos
`module.go` (registry/permissions/Build) · `model.go` (DTOs + inputs) · `service.go` (normalização,
regras de slug/cores, orquestração) · `qr_contrast.go` · `preview_agents.go` ·
`store_short_links.go` (+ helpers `isUUID/tsString/isUniqueViolation/isForeignKeyViolation`) ·
`store_qr_codes.go` · `http.go` (painel + `requireToolsPermission`) · `http_public.go` (`/s`,`/q`) ·
`errors.go` (sentinelas).

## Telas (web/app/components/tools) — modelo da aba Produtos do cardápio
Desktop: faixa de filtros sem moldura (busca + `ToolsStatusChips` Todos/Ativos/Inativos + Colunas +
ação primária no fim) e `OmniDataTable` sem coluna de seleção, rolando por dentro. Mobile
(`useMediaMatch('(max-width: 720px)')`): `ToolsListMobileToolbar` (busca + criar; status + "Campos
do card") e cards encostados (`tools-cards.css`) só com o essencial. CSS comum em `tools-list.css`.
- Desktop tem duas visualizações (`ToolsViewToggle`): cards e tabela. QR começa em cards;
  encurtador começa em tabela. A escolha fica nas preferências locais do navegador
  (`useToolsViewMode`, `ui.view_mode.*`).
- QR: `ToolsQrCardGrid` (QR grande; clique abre `ToolsQrViewer` em tela cheia com copiar, baixar,
  editar e Ativo), slug em destaque como **link de teste** para o `/q/{slug}` (abrir conta como
  scan), destino numa linha/largura máxima. `ToolsQrThumb` (tamanho por prop), `ToolsQrMobileList`,
  `ToolsQrFormModal`. Imagem/download/cópia em `web/app/utils/qr-image.ts`.
- Encurtador: `ToolsShortLinkCardGrid`; o link curto é um link de verdade (abre em nova aba) com
  botões de copiar e abrir; destino editável na célula da tabela (nos cards e no mobile, pelo menu
  "..."). `ToolsShortLinkMobileList`, `ToolsShortLinkCreateModal`, `ToolsShortLinkRenameModal`.
- Preferências de colunas: `tools.qr-codes.v2`/`.mobile`, `tools.short-links.v2`/`.mobile`.

## Rebuild
Mudou Go → `docker compose up -d --build api` (restart não basta). Migration roda no boot.

## Roteiros (`0325_tools_scripts.sql`)

`tools.scripts` persiste documentos por `account_id`: título, linhas JSONB de áudio/vídeo,
observações HTML e timestamps. A rota estática `/tools/scripts` substitui a demonstração.
Não importa documentos do PHP antigo; essa migração de dados está deliberadamente pendente.

- `GET /v1/tools/scripts?offset=0`: até 50 resumos, ordenados por atualização/id; `hasMore`
  indica próxima página. Conteúdo completo não participa da listagem; `preview` traz texto puro
  (sem tags, até 300 caracteres) das 6 primeiras linhas para a miniatura da lateral.
  `q` (opcional, até 100 caracteres) filtra por título ou texto puro de qualquer linha (ILIKE,
  curingas `%`/`_` escapados), sempre dentro da conta; a paginação vale sobre o resultado.
  `client` (opcional): vazio = todos, `none` = sem cliente, uuid = roteiros daquele cliente.
- `GET /v1/tools/scripts/{id}`: documento completo, no mesmo escopo de leitura da listagem.
- **Cliente do roteiro (migration 0327).** `tools.scripts.client_account_id` marca sobre qual
  conta-cliente o roteiro fala; o roteiro continua pertencendo à conta que o criou.
  `client_visible` (padrão `true`) libera a leitura para os usuários da conta-cliente. Só uma
  conta-agência (`is_agency`) pode marcar, e só cliente ativo, não-agência e da MESMA
  organização (senão 400 `invalid_client`). `GET /v1/tools/scripts/clients` (exige `.manage`)
  lista essas opções com `scriptCount` (roteiros da conta marcados com o cliente); para
  conta-cliente devolve `[]`.
- **Escopo de leitura único** (`scriptVisibleTo` em `scripts_store.go`): roteiros da própria
  conta + roteiros de uma conta-agência da mesma organização marcados com a conta atual e
  visíveis. A organização é reconferida na leitura. Esses itens voltam com `shared: true` e são
  somente leitura: PUT/DELETE/ordem filtram por `account_id` e retornam 404. A conta-cliente
  precisa do módulo tools e de `tools.scripts.view` para ler. Respostas trazem
  `clientAccountId`, `clientName`, `clientVisible` e `shared`.
- `POST /v1/tools/scripts`, `PUT /v1/tools/scripts/{id}`, `DELETE /v1/tools/scripts/{id}`.
- Leitura exige `tools.scripts.view` ou `.manage`; escrita exige `.manage`. Permissões efetivas
  da conta são resolvidas por `RequireAuthWithAccount`. Admin usa `RequireAuth` e conta explícita
  em `X-Account-Id`, sem leitura/escrita global implícita. IDs fora da conta retornam 404.
- PUT exige o `updatedAt` retornado pela API, com precisão de microssegundos: conflito retorna
  409 e preserva o documento persistido. Título até 200 caracteres, até 200 linhas, cada célula
  e observações até 200 KB; soma de conteúdo até 800 KB (limite HTTP de 1 MiB).
- `PUT /v1/tools/scripts/order` grava a ordem da lateral por usuário (`tools.script_orders`,
  migration 0326); IDs fora da conta retornam 404.
- `POST /v1/tools/scripts/images` (multipart `file`, PNG/JPEG/GIF/WebP até 10 MB, tipo detectado
  pelo conteúdo) grava em `PRIVATE_FILES_DIR/scripts/{conta}/{id}` e registra em
  `tools.script_images`; `GET /v1/tools/scripts/images/{id}` serve imagens da própria conta ou
  embutidas (id presente em `rows`/`description`) num roteiro compartilhado com ela.
  Sem `PRIVATE_FILES_DIR` o upload retorna 503.
- Front faz autosave com debounce de 400 ms sem bloquear o editor, reidrata a resposta
  autoritativa e confirma saída com draft pendente. Draft por conta fica apenas em memória
  enquanto a página permanece montada. PDF é baixado direto (html2canvas + jsPDF, A4 fatiado).

### Edição por célula + realtime + travas (2026-11-12)

Modelo atual (nível intermediário, parecido com os blocos do Notion):
- **Linhas com id estável** (`rows[].id`, migration 0328 fez o backfill). Toda chave de trava
  e de edição usa o id, nunca o índice (que muda quando alguém insere/move/remove linhas).
- **Conteúdo salva por célula**: `PATCH /v1/tools/scripts/{id}/cell` `{field, rowId, value}`
  com `field` em `title|description|audio|video` (`rowId` só para audio/video). Debounce de
  400 ms por célula no front. Responde `{updatedAt}` e publica `script.cell` com o conteúdo.
- **Estrutura salva à parte**: `PUT /v1/tools/scripts/{id}` faz merge por id numa transação
  (`saveStructure`, `select ... for update`): linhas existentes mantêm o conteúdo do banco,
  linhas novas entram com o conteúdo enviado, removidas saem; também grava cliente/visível.
  Nunca sobrescreve célula de outra pessoa, por isso não há mais 409 por `updatedAt`.
- **Travas por célula = presença** (`/v1/realtime/presence?scope=script&scriptId=` — a base
  WS do front manda o id em `taskId`, aceito como alternativa). fieldKeys:
  `title`, `description`, `cell:<rowId>:<audio|video>` (`ScriptCellKey`). Focar trava; sair
  libera; expira em 30 s sem heartbeat. Exclusiva: o segundo a focar vê "Fulano editando".
- **O servidor confere as travas** (`ScriptLocks`, implementado pelo realtime sobre o
  `PresenceStore`): PATCH numa célula travada por outro → 409 `script_locked` ("Fulano esta
  editando..."); PUT que remove linha travada → 409; `DELETE` com alguém editando → 409.
- **Eventos** (`scripts:account:{id}`): `script.cell` (campo, linha, conteúdo, autor) é
  aplicado direto na célula — só aquela célula re-renderiza, sem refetch nem "Abrindo
  documento"; `script.saved` (estrutura/criação) refaz o fetch e reaproveita as linhas por id;
  `script.deleted`; `script.reordered` (só abas do próprio usuário). Vão para a conta dona e
  para o cliente que lê (visível), e o cliente anterior ao trocar/ocultar.
- **Front**: `useScriptWorkspace.ts` (células pendentes, estrutura, eventos),
  `useScriptPresence.ts` (wrapper de `useCalendarPresence` com `scope: 'script'`),
  `useScriptsRealtime.ts`. Célula travada fica não editável com rótulo "Fulano editando";
  excluir linha/roteiro fica desabilitado enquanto alguém edita; rodapé mostra "Editando agora".
- A troca de conta descarta o antigo "rascunho por conta": agora só há poucas centenas de ms
  pendentes, que são enviadas (flush) antes de trocar.

### Implementações futuras (substituir o modelo por célula)

Decisão (2026-11-12): o modelo por célula é a etapa atual; o alvo é colaboração completa.
- **Colaboração na mesma célula (alvo)**: Yjs + y-quill (CRDT). Cada tecla vira operação
  pequena, mesclada sem conflito, com cursores de quem edita. Exige guardar o estado Yjs por
  roteiro (o HTML vira derivado), um provider sobre o WebSocket existente e remove a
  necessidade das travas por célula.
- **Atualização de lista sem refetch** em `script.saved` (mandar o resumo no evento).
- **Trava do seletor de tamanho de fonte**: o `<select>` nativo tira o foco do editor e libera
  a trava enquanto a lista está aberta (janela curta; aceitável no modelo atual).
