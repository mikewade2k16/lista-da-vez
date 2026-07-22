# E1 — fechar o piloto WhatsApp funcional

**Status:** `E1-R1 AUTOMATIZADA APROVADA; SMOKE EVOLUTION/CHROME PENDENTE`; E1 não concluída

Uma revisão independente posterior ao primeiro fechamento local encontrou quatro falhas de
corretude e quatro lacunas operacionais. A rodada E1-R1 corrigiu e provou a camada automatizada;
o pareamento/número real, o smoke externo e o início de E2 continuam bloqueados até os gates
Evolution/Chrome abaixo.

**Resultado:** um atendimento real pela Evolution aparece corretamente no inbox com texto,
reply/quote, mídia persistida, mensagem `fromMe`, estados do provider, histórico paginado e
feedback honesto de falha.

## 1. Escopo e decisões fechadas

- Evolution continua sendo o provider do piloto; não remover WAHA nem implementar Meta aqui.
- Webhook responde rápido: valida, deduplica, persiste evento/mensagem e enfileira trabalho; download
  de mídia não bloqueia a resposta pública.
- `fromMe=true` é `OUTBOUND` com origem `provider_device`, não inbound e não nova resposta da IA.
- reply guarda referência local quando conhecida e referência externa como fallback.
- URL temporária do provider nunca é a mídia definitiva; bytes vão ao storage privado existente.
- ACK pode chegar fora de ordem; status só avança segundo uma ordem monotônica definida.
- Front não sincroniza provider por polling agressivo; realtime + fallback existente prevalece.

## 2. Banco e migration E1

Auditar primeiro `messaging.messages` e reutilizar `metadata_json` apenas para metadado aberto.
Sem equivalente semântico, a migration reservada adiciona:

| Campo | Tipo/regra | Motivo |
|---|---|---|
| `reply_to_message_id` | `uuid null` FK para `messaging.messages(id) on delete set null` | quote navegável e tenant-safe via conversa |
| `reply_to_external_message_id` | `text null` | reply chega antes/sem mensagem local |
| `origin` | `text not null default 'contact'` + CHECK | distinguir `contact`, `human`, `ai`, `provider_device`, `system` |
| `provider_status_at` | `timestamptz null` | ignorar ACK antigo fora de ordem |
| `provider_error_code` | `text not null default ''` | erro classificável sem body do provider |

O CHECK de `status` passa a aceitar `PENDING`, `SENT`, `DELIVERED`, `READ`, `FAILED`, `DELETED`.
A transição é monotônica (`PENDING<SENT<DELIVERED<READ`); `FAILED` só substitui estado anterior se
o evento for mais novo e ainda não houver `DELIVERED/READ`; `DELETED` é terminal visual.

Índices:

- unique parcial `(account_id, instance_scope_key, external_message_id)` quando não vazio;
- `(account_id, conversation_id, created_at desc, id desc)` para cursor;
- `(account_id, reply_to_external_message_id)` parcial para reconciliação tardia.

Não criar tabela de mídia nova. `media_storage_key`/`media_source_kind` de `0207` e a outbox
existente são as fontes.

No contrato HTTP, o nome legado/front `quotedMessageId`/`quoted_message_id` é aceito e normalizado
para `reply_to_message_id`; não persistir dois campos com a mesma verdade.

## 3. Backend

### 3.1 Normalização do adapter Evolution

O evento canônico deve carregar `eventId`, `instanceName`, `externalMessageId`, `chatExternalId`,
`fromMe`, `sender`, `occurredAt`, `messageType`, `text`, `caption`, `mediaDescriptor`, `replyRef` e
`providerStatus`. Fixtures cobrem ao menos texto, extendedText/reply, imagem, áudio, vídeo,
documento, sticker/unknown, ACK e evento duplicado.

O parser não aplica regra de tenant/estado. Ele só normaliza provider → domínio e retorna erro
tipado para payload inválido/não suportado.

### 3.2 Transação inbound

Em uma transação:

1. resolver instância por `account_id + provider + instance_name`;
2. inserir `webhook_events`; conflito da unique retorna sucesso idempotente;
3. resolver/criar contato e conversa no escopo;
4. inserir/reconciliar mensagem pelo external ID;
5. vincular quote local quando possível;
6. atualizar `last_message_at` sem regredir timestamp;
7. enfileirar `omnichannel.media.fetch` se houver mídia;
8. enfileirar evento pós-commit para realtime/IA quando `fromMe=false`.

Para `fromMe=true`, persistir `direction=OUTBOUND`, `origin=provider_device`; reconciliar com envio
local pelo external ID/idempotency quando existir; nunca disparar IA.

No outbound com reply, o service resolve `reply_to_message_id` na **mesma conta e conversa**, obtém
o external message ID citado e entrega um `ReplyReference` explícito à interface do canal. O adapter
Evolution mapeia isso para o campo de quote/context aceito pelo endpoint usado. Se a referência
local não possuir external ID, a API devolve erro acionável ou envia sem quote somente quando o
usuário confirmar essa degradação; nunca mostra citação só na UI fingindo que chegou ao WhatsApp.

### 3.3 Fetch de mídia durável

O job contém apenas IDs internos. O worker relê instância/credencial cifrada, obtém bytes pelo
adapter, aplica timeout, tamanho máximo da conta, MIME permitido, proteção SSRF quando houver URL,
hash e path tenant-scoped. Grava em arquivo temporário, faz rename atômico e só então atualiza a
mensagem. Retry classifica 401/403, 404 temporário, 429 e 5xx; falha final preserva mensagem com
estado de mídia `failed` em metadata segura e ação de retry autorizada.

### 3.4 Leituras

- `GET /conversations`: `limit<=100`, cursor estável, busca normalizada por nome/telefone/texto
  com limite e filtro por canal/status/fila/responsável;
- `GET /conversations/{id}/messages`: cursor `before=(createdAt,id)`, ordem ascendente na resposta,
  `hasMore` e `nextCursor`;
- serialização inclui `replyTo`, `origin`, `status`, `providerErrorCode` seguro e capability de mídia;
- endpoint de mídia continua autenticado e nunca expõe storage key/path.

## 4. Frontend

### 4.1 Tipos e estado

Atualizar `types.ts` e composables, sem cast `any`, para `replyTo`, `origin`, novos status e
`mediaState`. O cache faz upsert por ID e external ID, preserva ordenação e não duplica mensagem
quando webhook confirma um optimistic send.

### 4.2 Renderização

- quote mostra autor, resumo seguro e tipo da mensagem; clique rola/carrega a original;
- `provider_device` aparece do lado outbound e pode ter rótulo discreto “enviado pelo aparelho”;
- mídia mostra skeleton durante ingest, player/preview quando pronta, retry/erro quando falha;
- ACK usa ícones/labels acessíveis e nunca converte erro em “enviado”;
- paginação preserva posição do scroll ao carregar anteriores;
- busca/filtros têm debounce de UI e cancelamento de request anterior;
- empty/loading/error/offline são estados explícitos.

## 5. Pacotes atômicos

| Pacote | Resultado | Escrita principal | Depende de |
|---|---|---|---|
| `E1-DB-01` | schema de reply/origin/ACK e índices | nova migration | E0 |
| `E1-BE-02` | parser + persistência idempotente/fromMe | adapter, inbound, store, testes | DB-01 |
| `E1-BE-03` | fetch durável e reconciliação de mídia/status | worker/store/media, testes | BE-02 |
| `E1-API-04` | cursores, busca e response contract | http/service/store | BE-02 |
| `E1-FE-05` | render completo e estados de UX | front omnichannel | API-04, BE-03 |
| `E1-QA-06` | smoke independente no browser/API/banco | sem código de feature | todos |

Nenhum executor recebe simultaneamente `BE-02` e `FE-05`; fixture JSON do contrato separa as
camadas.

### 5.1 Rodada corretiva E1-R1 (obrigatória antes do smoke real)

| Pacote | Resultado fechado | Escrita principal | Depende de |
|---|---|---|---|
| `E1-R1-DB-08` | ACK anterior à mensagem fica durável e busca textual ganha índice compatível | migration aditiva `0215` + ERD/contrato de banco | E1-QA-06 |
| `E1-R1-BE-09` | envio humano toma a conversa; dispatch IA e takeover ficam serializados; ACK/echo não regridem; permissão canônica | services/stores/handler HTTP e testes focados | R1-DB-08 |
| `E1-R1-INT-10` | wiring mínimo das dependências já testadas, sem regra nova | somente `module.go` e teste de composição se necessário | R1-BE-09 |
| `E1-R1-FE-11` | retry de mídia reconcilia resposta; erros e retry são visíveis; cursor de conversas é consumido | composables/componentes do inbox e testes focados | contrato HTTP vigente |
| `E1-R1-QA-12` | revisão independente e matriz dos 12 critérios com evidência, sem corrigir feature | testes/relatório; produção somente leitura | R1-DB-08, R1-BE-09, R1-INT-10, R1-FE-11 |

#### R1-DB-08 — contrato do banco

- a próxima migration é `0215_messaging_delivery_reconciliation.sql`; `0213` e `0214` são
  imutáveis;
- o evento de status precisa sobreviver mesmo quando o ACK chega antes da mensagem. A fonte durável
  deve continuar tenant-scoped e deduplicada; não usar memória/Redis como verdade;
- o replay aplica `(provider_status_at, id)` em ordem determinística e passa pela mesma regra
  monótona usada pelo ACK recebido depois da mensagem;
- a busca `%ILIKE%` em conteúdo precisa de índice `pg_trgm` compatível. A extensão já é criada pela
  migration `0034`; a nova migration não cria mecanismo de busca paralelo;
- migration reaplica em banco vazio e banco atualizado sem editar migration antiga.

#### R1-BE-09 — invariantes de concorrência e entrega

1. Um POST humano novo e válido executa `msg.outbound.human` antes de publicar/enfileirar a mensagem,
   incrementa o lease de IA e assume o atendente. Repetição idempotente da mesma mensagem não toma a
   conversa uma segunda vez.
2. Takeover invalida outbox IA em `pending` **e** `processing`. O envio ao provider e a validação de
   `conversation.state + ai_generation` ficam serializados pelo lock da conversa; uma IA invalidada
   nunca chama `Provider.SendMessage`.
3. ACK anterior à mensagem é reprocessado quando a mensagem/eco aparece. ACK duplicado continua
   idempotente e ACK atrasado nunca regride `READ/DELIVERED`.
4. Ao unir eco `fromMe` e mensagem local, vence o estado monótono mais avançado, junto com
   `provider_status_at` e erro seguro. O realtime publica o estado realmente persistido, nunca
   `SENT` fixo.
5. Envio e retry de mídia usam a permissão efetiva canônica da conta; papel legado não concede acesso
   sozinho. Conta/conversa/mensagem fora do escopo continuam respondendo 404.
6. Timeout/erro do provider libera a transação e segue a política existente de retry/dead-letter;
   logs não incluem texto, telefone, credencial, token ou URL assinada.

Testes mínimos do pacote: humano invalida IA; job `processing` perde corrida para takeover; dispatch
que obtém lock primeiro termina antes do takeover; ACK-before-message; merge eco em `READ`; ACK
duplicado/fora de ordem; 403 por permissão; 404 cross-account; erro do provider sem vazamento.

#### R1-FE-11 — invariantes de UX

- `POST .../media/retry` faz upsert da mensagem retornada e sempre libera o marcador local; eventos
  posteriores `ready|failed` também limpam estado pendente. Falha nova nunca fica mascarada como
  `pending` eterno;
- lista e histórico possuem `loading`, `empty`, `error` e ação de tentar novamente distinguíveis;
  erro HTTP não pode ser renderizado como lista vazia bem-sucedida;
- `GET /conversations` preserva `nextCursor/hasMore`, concatena por ID sem duplicar e oferece carregar
  mais sem trocar a conversa ativa nem perder filtros;
- request anterior continua cancelável e resposta obsoleta não substitui filtro/busca atual;
- nenhuma refatoração estética, split de arquivo legado ou mudança fora do inbox faz parte do pacote.

#### R1-QA-12 — rejeição automática

O revisor reprova sem corrigir o código se observar qualquer um destes casos: provider chamado por IA
depois de takeover concluído; ACK perdido/regredido; mensagem/arquivo duplicado; falha mostrada como
sucesso/vazio; acesso cross-account diferente de 404; workflow fora do owner com diff; WAHA tocada.
Depois da revisão automatizada, o smoke real continua obrigatório para os critérios 2 a 7 que dependem
do provider e do browser.

## 6. Critérios de aceite

1. duas entregas do mesmo webhook criam uma mensagem;
2. `fromMe` aparece uma vez, outbound, e não chama IA;
3. reply abre/realça a mensagem original, inclusive após paginação;
4. reply outbound chega ao WhatsApp real como citação da mensagem correta;
5. imagem, áudio, vídeo e documento sobrevivem à expiração da URL do provider;
6. ACK fora de ordem não regride `READ` para `SENT`;
7. falha de mídia é visível e retry não duplica arquivo/mensagem;
8. 200+ mensagens paginam sem salto de scroll;
9. usuário de outra conta recebe 404 para conversa/mídia;
10. logs não contêm texto, telefone completo, token ou URL assinada;
11. workflows não pertencentes ao omnichannel têm diff vazio.
12. assumir/atribuir a conversa cancela ou invalida imediatamente qualquer dispatch IA em voo.

## 7. Rollback

Código novo pode ser desligado por capability server-side, preservando colunas aditivas. Não
dropar migration. Se o fetch novo falhar, mensagem permanece legível com placeholder e retry; não
voltar a servir URL externa temporária como fonte definitiva.
