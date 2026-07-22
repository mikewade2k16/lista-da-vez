# E4 — CRM e atribuição 360°

**Status:** `DB-02 + API/BE + merge/undo + landing capture + segmentos + FE-07 + ingestão de identidade inbound APLICADOS localmente; integração visual de segmentos e QA externo pendentes`

**Resultado:** cada pessoa possui um contato canônico pesquisável/editável, identidades por canal,
origens/touchpoints, classificação automática e trilha segura de merge.

## 1. Fonte existente e regra de evolução

Reutilizar obrigatoriamente:

- `messaging.contacts` como contato canônico;
- `contact_identities` como identidade provider/canal;
- `contact_touchpoints` como histórico de origem;
- `contact_notes` como notas humanas;
- `conversations.contact_id` como vínculo;
- `0212` apenas como backfill histórico; nunca editar.

Não criar `customers`, `leads` ou um CRM paralelo. “Já é cliente” não é inferido só por conversa:
é confirmado por ferramenta/fonte comercial autorizada e recebe evidência.

## 2. Complementos de banco

Após auditoria, migration aditiva pode incluir em `contacts`:

| Campo | Regra |
|---|---|
| `primary_email` | nullable; índice case-insensitive parcial |
| `owner_user_id` | usuário da mesma conta; `on delete set null` |
| `merged_into_contact_id` | self FK nullable; contato mesclado fica arquivado |
| `archived_at` | preserva histórico |
| `classification_source` | `manual`, `ai`, `erp`, `rule`, `backfill` |
| `classification_confidence` | 0..1 nullable |
| `last_qualified_at` | quando classificação foi confirmada |

Criar `messaging.contact_merge_events` com source/target, actor, reason, snapshot de IDs movidos e
timestamp. Merge é operação transacional e auditada, não delete.

Criar `messaging.contact_consents` para propósito/canal/status/fonte/evidência e vigência. Status
fechado (`granted`, `revoked`, `unknown`); consentimento nunca é inferido de mensagem genérica.
Criar `messaging.contact_external_refs` para IDs de CRM/ERP por sistema, unique por conta+sistema+ID;
essa tabela sustenta confirmação de cliente sem usar telefone como única verdade.

Criar `messaging.contact_segments` como filtro salvo versionado (`name`, `filter_json`, owner,
is_active), não como cópia de contatos. O service valida um conjunto fechado de filtros e executa
sempre tenant-scoped.

Para landing pages, criar `messaging.lead_sources` somente se não existir catálogo equivalente:
`id/account_id/slug/name/domain/allowed_origins/capture_token_hash/is_active`. Token só em hash.
`contact_touchpoints.landing_page_id` passa a referenciar/armazenar ID canônico em nova coluna UUID,
mantendo o texto legado para compatibilidade até backfill.

Tags continuam em `jsonb` nesta etapa por compatibilidade. Service normaliza array de strings
lowercase, tamanho/quantidade limitados; GIN somente se EXPLAIN provar necessidade. Não criar tabela
de tags sem demanda de governança compartilhada.

Os artefatos de `E4-DB-02` estão em
`back/internal/platform/database/migrations/0217_messaging_contact_crm_evolution.sql` e
`0218_messaging_contact_merge_undo.sql`. Eles são
aditivo, fecha FKs tenant-safe para o CRM, mantém valores legados durante a transição e foi
validado em dupla aplicação transacional e em constraints de classificação, merge, consentimento,
fonte de landing, touchpoint e referência externa. Foi aplicado localmente no banco do Compose
após dump de segurança; nenhum endpoint/fluxo de CRM foi ativado por essa migration.

`relationship_status` evolui para os valores de produto `new_lead`, `known_lead`, `customer`,
`inactive` sem criar outra coluna-verdade. Fazer transição compatível: migration aceita valores
antigos e novos, backend novo lê/mapeia `lead→new_lead` e `prospect→known_lead`, escreve somente os
novos, backfill converte, e uma migration de limpeza só fecha o CHECK depois de zero linha antiga.

## 3. Resolução de identidade

Ordem determinística dentro da conta:

1. identidade exata `(channel,provider,instance_scope_key,external_id)`;
2. telefone E.164 normalizado e confiável;
3. email normalizado e verificado;
4. nenhum match: criar contato;
5. múltiplos candidatos: não auto-merge; marcar conflito para revisão.

Nome/avatar/source nunca bastam para merge. Instagram scoped ID de uma conta não é comparável ao
de outra conta/provider. Toda criação/atualização de inbound faz upsert de identidade, atualiza
`last_seen_at/last_channel` monotonicamente e registra touchpoint idempotente.

## 4. Origem e classificação

`source_kind` fechado na camada Go: `whatsapp_inbound`, `instagram_dm`, `instagram_comment`,
`landing_page`, `manual`, `import`, `campaign`, `legacy_backfill`. Touchpoint guarda `source_ref`,
landing/campaign/UTMs permitidas e metadata allowlisted.

A triagem E2 pode sugerir `relationship_status`, tags e campos. Go aceita apenas campos definidos,
confidence mínima e transição permitida. ERP/tool E6 pode confirmar `customer`; essa evidência
prevalece sobre inferência IA. Alteração manual prevalece até nova confirmação explícita e é
auditada.

`first_seen_at/last_seen_at` são projeções atualizadas monotonicamente a partir de touchpoints. Um
job de reparo pode recalculá-las por `min/max(occurred_at)`; operador/API nunca informa esses campos.
Confirmação CRM/ERP usa adapter/endpoint Go idempotente, grava `contact_external_refs` e fonte de
classificação; n8n não consulta ERP diretamente.

## 5. APIs

| Método/rota | Contrato |
|---|---|
| `GET /v1/omnichannel/contacts/crm` | cursor, `q`, channel, status, tag, owner, source, lastSeen range |
| `GET /v1/omnichannel/contacts/{id}/profile` | perfil 360° com identities, resumo/touchpoints paginados |
| `PATCH /v1/omnichannel/contacts/{id}/crm` | patch pequeno; optimistic `expectedUpdatedAt` |
| `GET/POST /v1/omnichannel/contacts/{id}/notes` | lista cursor + nota humana auditada |
| `POST /v1/omnichannel/contacts/{id}/merge` | targetId, reason, idempotency key; permissão elevada |
| `POST /v1/omnichannel/contacts/merges/{id}/undo` | desfaz somente IDs do snapshot, sem apagar vínculos novos |
| `GET/POST/PATCH /v1/omnichannel/settings/lead-sources` | catálogo tenant-scoped; token só retorna na criação/rotação |
| `GET /contacts/merge-candidates` | sugestões explicáveis; nunca executa merge |
| `GET/POST/PATCH /settings/contact-segments` | filtros salvos validados e tenant-scoped |
| `POST /contacts/export` | job assíncrono, colunas/escopo permitidos, artefato privado e expiração |
| `POST /v1/internal/omnichannel/contacts/confirm-customer` | integração autenticada CRM/ERP e idempotente |
| `POST /v1/public/omnichannel/leads/{sourceSlug}` | capture token, origin allowlist, rate limit, honeypot |

Endpoint público resolve conta pelo source server-side, limita body, normaliza campos, cria
contact/identity/touchpoint idempotente e pode iniciar conversa somente por policy/canal permitido.
Ele nunca aceita `account_id`, provider token ou fila arbitrária.

## 6. Frontend

- aba Contatos usa lista virtual/paginada, busca e filtros persistidos na URL;
- drawer 360° mostra identidade, status, tags, owner, campos, origens, notas e conversas;
- edição inline tem dirty state, conflito 409 e feedback de salvamento;
- origem inicial e último toque são visíveis; landing/campanha aparecem com nome canônico;
- merge mostra diferenças source×target, dados que serão movidos, exige motivo e confirmação;
- consentimentos mostram propósito, canal, fonte, vigência e revogação; opt-out afeta policies;
- segmentos podem ser salvos e reutilizados sem materializar uma lista divergente;
- export mostra progresso, expiração e quem solicitou; arquivo nunca é link público permanente;
- conflito de identidade aparece em fila de revisão, nunca auto-resolvido no browser;
- tela indica “cliente confirmado por ERP”, “classificado pela IA” ou “definido manualmente”.

## 7. Pacotes atômicos

| Pacote | Entrega |
|---|---|
| `E4-AUDIT-01` | mapa de gaps de `0211/0212` e contrato final |
| `E4-DB-02` | complementos CRM/merge/source |
| `E4-BE-03` | resolver identidade e touchpoint em inbound |
| `E4-API-04` | perfil, filtros, notas e patch |
| `E4-BE-05` | merge e undo transacionais, auditados e idempotentes |
| `E4-LP-06` | captura segura de landing page com token hash/origin/rate-limit |
| `E4-FE-07` | lista e drawer 360° |
| `E4-QA-08` | dedupe, merge, atribuição e isolamento |

## 8. Critérios de aceite

- WhatsApp repetido atualiza um contato; Instagram cria identidade no mesmo contato só com vínculo
  confiável/manual, nunca por nome;
- touchpoint duplicado não duplica origem;
- landing A não consegue gravar na conta B;
- filtros por origem/status/tag/owner têm paginação estável;
- merge move identities, conversations, notes e touchpoints em uma transação, preserva source e
  registra evento; retry é idempotente;
- undo só ocorre quando o snapshot ainda é reversível; caso contrário retorna 409 e exige fluxo
  assistido, sem “desmesclar” parcialmente;
- contato mesclado responde redirect/estado arquivado, não desaparece;
- classificação mostra fonte e confiança;
- CRM/ERP confirma cliente por external ref e a IA recebe esse status do banco;
- nenhum telefone/email completo aparece em log.

### Implementação local atual

- `crm_model.go`/`store_crm.go` e `service_crm.go`: filtro cursorizado, perfil 360°, patch com
  optimistic concurrency, tags/campos normalizados e RBAC tenant-scoped.
- `contact_merge.go`: lock tenant-scoped, movimentação transacional de identidades/touchpoints/
  notas/conversas, snapshot, idempotency key e undo auditado.
- `lead_capture.go`: `POST /v1/public/omnichannel/leads/{sourceSlug}` resolve a conta pela fonte
  e hash do token, valida origem, aplica limite de corpo/rate-limit, faz upsert por telefone/email
  e touchpoint idempotente. Não aceita `account_id`, token de provider ou fila no payload.
- Segmentos salvos: `GET/POST/PATCH /v1/omnichannel/settings/contact-segments` persiste somente filtros fechados e normalizados (sem materializar contatos), sempre sob a conta e a permissão de configurações.
- Verificações locais: `go test ./internal/modules/omnichannel/...`, `go vet`, migration `0218` e
  o fixture dinâmico `TestStoreDeliveryE1` passaram em Postgres descartável; o cenário de identidade
  exata versus telefone e `last_seen_at` monotônico está coberto. Ainda faltam QA externo e testes
  de integração dedicados de landing/merge; o endpoint público não foi exposto na VPS.
- Frontend FE-07: `useOmnichannelCRM.ts` alimenta a aba Contatos com cursor/filtros estaveis,
  `OmnichannelCRMProfilePanel.vue` abre o perfil 360° com identidades, touchpoints, notas e ação
  para abrir a conversa; em caso de ausência da permissão nova, a lista legada permanece como
  fallback.

  O recorte FE-07 tambem inclui dedupe de paginas, carregamento incremental, edicao inline com
  expectedUpdatedAt, notas auditaveis e merge assistido com motivo; Vitest cobre filtros, cursor,
  dedupe, erro acionavel e desfazer merge pelo endpoint de snapshot.
