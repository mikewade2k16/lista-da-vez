# Módulo `tools` — Encurtador de Link + Gerador de QR Code

Duas ferramentas por conta, reconstruídas como back Go real (o projeto antigo era mock em
`globalThis` no BFF Nitro eliminado). Plano/decisões: `docs/tools/PLANO_MODULO_TOOLS.md`.

## Escopo
- **Encurtador**: `tools.short_links` → redirect público `GET /s/{slug}` (302 + `hits++`).
- **QR Code**: `tools.qr_codes` → redirect **rastreado** `GET /q/{slug}` (302 + `scan_count++`,
  `last_scanned_at`, só se `is_active`). O QR codifica o `qrUrl` (`/q/{slug}`), não a URL final —
  por isso o toggle Ativo e o contador de scans funcionam de verdade. A imagem PNG é gerada **no
  cliente** (lib `qrcode`) a partir de `qrUrl` + cores + `size`; o backend não renderiza/armazena PNG.

## Schema (migration `0197_tools_module.sql`)
- `tools.short_links(id, account_id→core.accounts, slug UNIQUE, target_url, hits, created_at, updated_at)`
- `tools.qr_codes(id, account_id→core.accounts, slug UNIQUE, target_url, fill_color, back_color,
  size, is_active, scan_count, last_scanned_at, created_at, updated_at)`

`slug` é **único global por tabela** (o redirect resolve sem X-Account-Id). Colisão no create/patch →
o store adiciona sufixo `-2/-3`… (loop atômico sobre a violação de unique `23505`). `/s` e `/q` são
tabelas distintas: o mesmo slug pode existir nas duas.

## Rotas
Painel (RequireAuth + gating de módulo `tools` no Chain via `/v1/tools`):
- `GET/POST /v1/tools/short-links`, `PATCH /v1/tools/short-links/{id}`, `DELETE /v1/tools/short-links/{id}`
- `GET/POST /v1/tools/qr-codes`, `PATCH /v1/tools/qr-codes/{id}`, `DELETE /v1/tools/qr-codes/{id}`

O `PATCH` do encurtador é parcial (`slug`/`targetUrl`; `nil` = não mexe) e alimenta a **edição
inline** da tabela no painel. Trocar o `slug` mantém o unique global (mesmo loop de sufixo `-2/-3` do
create) — atenção: muda o `shortUrl`, então links já divulgados com o slug antigo param de resolver.

Público (sem auth, fora do gating — prefixos `/s` e `/q` não estão em `moduleGatingRules`):
- `GET /s/{slug}` → 302 destino; `GET /q/{slug}` → 302 destino (se ativo).

O `Resolve` (redirect) só devolve o destino se a conta dona está `is_active` **e** o módulo `tools`
está habilitado para ela (`core.account_modules`) — igual ao `PublicLookup` do bio. Qualquer falha =
404 uniforme (`pgx.ErrNoRows`), sem vazar existência.

## Isolamento multi-tenant (defesa em profundidade)
`RequireAuthWithAccount` rejeitaria `platform_admin` (sem linha em `core.account_users`), então as
rotas usam `RequireAuth` + validação **no handler** (`scopeContext` em `http.go`):
- `platform_admin`: confia no `X-Account-Id` (troca de conta); `""` = todas as contas (list) /
  remove/edita por id em qualquer conta (mutations). No create pode mirar qualquer `accountId` do body.
- Usuário comum: exige `X-Account-Id`, valida `IsMember` contra `core.account_users`; escopo = essa
  conta; `accountId` do body é **ignorado** (nunca escreve em conta alheia). Delete/patch filtram
  `id = $1 and account_id = $2` → linha de outra conta vira 404.

Além disso toda query do store filtra por `account_id` (quando não-admin). `accountId` inexistente no
create cross-conta do admin → violação de FK `23503` → 400 `invalid_account`.

## Permissões (declaradas em `module.go`, sync no boot)
`tools.shortlinks.view/manage`, `tools.qr.view/manage`. Templates: `tools.manager`, `tools.viewer`.
A visibilidade do menu "Tools" continua na permissão de página `workspace.tools.view` (módulo core).

## Env
- `TOOLS_PUBLIC_BASE_URL` (opcional): base absoluta de `shortUrl`/`qrUrl`. Fallback:
  `PUBLIC_API_BASE_URL` (já usada por bio/cardapio); vazio = URL relativa (o front prefixa com apiBase).
- Em produção o reverse proxy precisa rotear `/s/*` e `/q/*` para a api (para o link funcionar ao
  ser clicado/escaneado).

## Arquivos
`module.go` (registry/permissions/Build) · `model.go` (DTOs + inputs) · `service.go` (normalização,
slug, publicBase, orquestração) · `store_short_links.go` (+ helpers `isUUID/tsString/
isUniqueViolation/isForeignKeyViolation`) · `store_qr_codes.go` · `http.go` (painel + `scopeContext`)
· `http_public.go` (`/s`,`/q`) · `errors.go` (sentinelas).

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

### Realtime dos roteiros (versão simples, 2026-11-12)

- Após cada mutação persistida, `scriptService` publica `ScriptEvent` (`scripts_publisher.go`)
  pelo `ScriptPublisher` injetado em `tools.New(tools.WithScriptPublisher(realtimeService))`.
  Eventos: `script.saved` (id + `updatedAt`), `script.deleted` (id) e `script.reordered`
  (só a conta dona, com `userId`; a ordem é por usuário). Falha ao gravar não publica.
- O evento é só invalidação, sem conteúdo. Vai para `scripts:account:{conta dona}` e para as
  contas-cliente que viam ou passam a ver o roteiro (cliente atual visível + cliente anterior),
  para o card sumir/aparecer na tela delas. A tela refaz o fetch pela API (escopo `scriptVisibleTo`).
- Canal: `GET /v1/realtime/scripts?scope=account&accountId=...&ticket=...` (módulo realtime,
  `service_scripts.go`): conta ativa + membership + `tools.scripts.view` ou `.manage`.
- Front (`useScriptsRealtime.ts` + `useScriptWorkspace.ts`): ignora o eco da própria aba pelo
  `updatedAt`; atualiza o card (`refreshDocument`; 404 remove da lista); roteiro aberto sem
  edição local recarrega na hora; com edição local mostra "alterado em outra sessão" com
  "Recarregar"; excluído em outra sessão fecha o editor com aviso. Com busca/filtro ativo,
  recarrega a lista (a API decide se o item ainda entra).

### Implementações futuras (realtime dos roteiros)

Pendentes de propósito; a versão atual é "último a salvar vence" com detecção de conflito (409).
- **Edição colaborativa** de verdade (duas pessoas na mesma célula, texto mesclado): exige CRDT/OT
  (ex.: Yjs + y-quill) e persistência de deltas, não só invalidação.
- **Presença**: mostrar quem está com o roteiro aberto/editando — reaproveitar o `PresenceStore`
  do realtime (`/v1/realtime/presence`, já usado em tasks/calendar) com um scope `script`.
- **Lock por célula** enquanto alguém edita (o presence de tasks já tem `field_locked`).
- **Atualização sem refetch**: mandar o resumo no evento quando o volume justificar.

