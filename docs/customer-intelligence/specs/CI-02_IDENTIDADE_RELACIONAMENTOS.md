# CI-02 — Identidade, matching e relacionamentos

- **Status:** READY — contrato validado; execução física pertence à CI-03
- **Versão:** 0.1
- **Data-base:** 2026-07-23
- **Owner proposto:** Customer Data
- **Dependências:** CI-00 validada; contrato de binding CI-01
- **Autoriza implementação:** sim, por CI-03 e dentro dos gates desta spec

> CI-02 define semântica, entidades lógicas, fixtures e gates. A criação física e o writer único
> pertencem à CI-03. Esta spec proíbe criar uma segunda fonte de verdade em paralelo ao CRM atual.

## 1. Resultado único e verificável

Separar:

- a entidade deduplicável `subject`;
- a relação isolada do subject com cada cliente;
- as identidades e referências que sustentam matching;
- matching candidato de merge/vínculo efetivamente aprovado.

O contrato deve representar a mesma pessoa atendida por dois clientes sem compartilhar lifecycle,
consentimentos, notas, fatos ou histórico e sem criar dois writers.

## 2. Baseline comprovada

Hoje:

- `messaging.contacts` é único por `account_id` e telefone quando preenchido;
- identidades do provider vivem em `messaging.contact_identities`;
- lifecycle/tags/custom fields/owner/email estão na própria linha de contato;
- external refs, consentimentos, notas e merge vivem em tabelas `messaging.contact_*`;
- conversa aponta para um `contact_id`, mas ainda não explicita client antes de CI-01;
- nome do WhatsApp/Instagram pode ser decorativo;
- merge atual possui evento/idempotency/undo, mas continua contact-scoped;
- `messaging.contact_intelligence` é memória derivada, não identidade determinística.

Risco comprovado na fonte ERP: documentos genéricos ou repetidos não podem ser usados como chave
automática. Nenhuma contagem de contatos misturados foi medida nesta rodada.

### 2.1 Leitura obrigatória para execução/revisão

- CI-00 e CI-01;
- migrations `0211`, `0212`, `0217`, `0236` e posteriores relacionadas;
- `crm_model.go`, `contact_name.go`, `contact_merge.go`;
- `service_crm.go`, `store_crm.go` e testes;
- modelos/importação de customers em `back/internal/modules/crm/erp`;
- contratos de lead/tracking do módulo Site;
- regras de Principal/RBAC/organization no Core.

Nenhum arquivo lido entra automaticamente na allowlist de escrita.

## 3. Invariantes de identidade

1. `subject_id` só é deduplicável dentro de `account_id`.
2. `subject` não concede acesso a todas as suas relações.
3. `relationship_id` pertence a um único `client_account_id`.
4. toda consulta de cliente filtra account + client + relationship no service e repository.
5. nome, nascimento, endereço, comportamento e fuzzy match nunca executam merge automático.
6. identidade forte em outro cliente gera candidato restrito da agência, não vínculo automático.
7. matching confidence, fact confidence e recommendation confidence são métricas diferentes.
8. identidade do provider é forte somente dentro do issuer/recurso/escopo definido.
9. telefone/e-mail só são fortes após normalização e verificação compatíveis.
10. CPF/documento precisa ser válido, escopado ao conector/cliente e protegido; valor genérico
    conhecido ou inválido é rejeitado.
11. `visitor_id`/`session_id` são pseudônimos de navegação; só viram identidade de pessoa após
    conversão explícita e evidência.
12. merge preserva evidência, possui idempotência, revisão e undo.
13. correção manual verificada não é sobrescrita por LLM.
14. IA pode propor claim/match; somente Customer Data aplica identidade/merge.

## 4. Modelo lógico

### 4.1 `Subject`

Entidade mínima, owner-scoped:

| Campo | Tipo lógico | Regra |
|---|---|---|
| `id` | UUID | opaco |
| `ownerAccountId` | UUID | escopo de deduplicação |
| `subjectType` | `person`, `organization` | imutável sem processo de conversão auditado |
| `status` | `active`, `merged`, `anonymized` | não usa delete silencioso |
| `mergedIntoSubjectId` | UUID nullable | mesmo owner; sem ciclo |
| `revision` | bigint | optimistic locking |
| `createdAt`, `updatedAt` | timestamp | server-side |

`Subject` não carrega lifecycle, consentimento ou tag global. Atributos pessoais/empresariais
owner-scoped possuem perfil próprio e visibilidade mais restrita.

### 4.2 `PersonProfile`

| Campo | Regra |
|---|---|
| `subjectId` | subject `person` |
| `legalName` | opcional; classificação PII |
| `preferredName` | opcional; não substitui nome de cada relação |
| `birthDate` | opcional; nunca chave automática isolada |
| `locale`, `timezone` | valores normalizados |
| `verifiedAt`, `verificationSourceRef` | somente se houver prova |
| `revision` | optimistic locking |

### 4.3 `OrganizationProfile`

| Campo | Regra |
|---|---|
| `subjectId` | subject `organization` |
| `legalName`, `tradeName` | opcionais |
| `registrationCountry` | ISO allowlisted |
| `registrationId` | cifrado + fingerprint HMAC; nunca log/URL |
| `verifiedAt`, `verificationSourceRef` | prova necessária |
| `revision` | optimistic locking |

### 4.4 `Relationship`

| Campo | Tipo lógico | Regra |
|---|---|---|
| `id` | UUID | opaco |
| `ownerAccountId` | UUID | workspace |
| `clientAccountId` | UUID | cliente exato |
| `subjectId` | UUID | mesmo owner |
| `displayName` | string | nome seguro para este cliente |
| `preferredName` | string nullable | confirmado para esta relação |
| `lifecycleStatus` | `lead`, `prospect`, `customer`, `inactive` | determinístico |
| `classificationSource` | `manual`, `erp`, `rule`, `backfill` | IA produz proposta, não valor direto |
| `classificationConfidence` | 0..1 nullable | diferente de match confidence |
| `ownerUserId` | UUID nullable | usuário do mesmo account autorizado |
| `tags` | lista tipada | bounded e normalizada |
| `customFields` | objeto schema-bound | sem segredo/PII arbitrária |
| `firstSeenAt`, `lastSeenAt` | timestamps | fonte/ocorrência real |
| `lastQualifiedAt` | timestamp nullable | |
| `archivedAt` | timestamp nullable | soft archive |
| `revision` | bigint | optimistic locking |

Unicidade lógica: um subject possui no máximo uma relação canônica por client dentro do owner.

### 4.5 `SubjectIdentity`

Identidade fica visível e única no escopo da relação/cliente por default:

| Campo | Regra |
|---|---|
| `id`, `ownerAccountId`, `clientAccountId`, `relationshipId`, `subjectId` | escopo cumulativo |
| `identityKind` | `phone`, `email`, `whatsapp`, `instagram`, `erp_customer`, `site_visitor`, `document`, `other` |
| `issuer` | provider/conector registrado |
| `valueCiphertext` | opcional, cifrado |
| `valueFingerprint` | HMAC rotacionável, nunca hash simples enumerável |
| `maskedValue` | projeção segura para UI |
| `verificationStatus` | `unverified`, `verified`, `revoked` |
| `verificationMethod` | enum allowlisted |
| `sourceRef` | referência tipada |
| `firstSeenAt`, `lastSeenAt`, `verifiedAt` | timestamps de origem |
| `metadata` | campos allowlisted por kind |
| `revision` | optimistic locking |

O contrato não exige que uma mesma fingerprint em clientes diferentes aponte automaticamente para
o mesmo subject. A busca cross-client é restrita e cria `MatchCandidate`.

### 4.6 `SourceLink`

Liga entidade do módulo owner a subject/relação sem importar seu schema:

- source module/key;
- source entity type/id;
- source version/hash;
- subject/relationship/client;
- link method (`verified_exact`, `manual`, `backfill`, `reviewed_candidate`);
- match confidence;
- status (`active`, `superseded`, `quarantined`);
- linked/reviewed by e timestamps.

Exemplo inicial: `messaging.contact` → `subject + relationship`.

### 4.7 `MatchCandidate`

| Campo | Regra |
|---|---|
| `id`, IDs de escopo | owner/client obrigatório |
| `incomingSourceRef` | referência, não payload bruto |
| `candidateSubjectId` | nunca concede visibilidade por si |
| `candidateRelationshipId` | nullable quando outro client |
| `matchMethod` | enum registrado |
| `matchConfidence` | 0..1 |
| `evidenceRefs` | IDs/hashes allowlisted |
| `riskFlags` | `cross_client`, `sensitive`, `generic_identifier`, `mixed_history` etc. |
| `status` | `pending`, `accepted`, `rejected`, `expired` |
| `decisionReason` | obrigatório para decisão |
| `reviewedBy`, `reviewedAt` | obrigatório após decisão |
| `idempotencyKey` | único por owner |

Aceitar candidato cross-client pode vincular uma nova relação somente após autorização específica;
nunca copia fatos/consentimentos da relação existente.

### 4.8 `MergeEvent`

Evento imutável:

- source/target subject;
- relações afetadas;
- reason;
- actor;
- snapshot minimizado;
- idempotency key;
- createdAt;
- undo event/status.

Regras:

- origem e destino no mesmo owner;
- sobreposição no mesmo client exige plano explícito de relação vencedora;
- source fica `merged`, não é apagado;
- links/identidades são reatribuídos pelo writer em transação;
- undo falha com conflito explícito se houve mutação posterior incompatível.

### 4.9 Dados por relação

Continuam sempre relationship-scoped:

- consentimentos e opt-out;
- notas;
- lifecycle e owner;
- tags/custom fields;
- touchpoints projetados;
- fatos, sínteses e recomendações;
- campanhas e resultados.

## 5. Força de matching

### 5.1 Match forte elegível dentro do mesmo client

| Evidência | Condições adicionais |
|---|---|
| external ID do provider | issuer + recurso/canal + verificação exatos |
| telefone E.164 | validado, verificado e não compartilhado/genérico |
| e-mail normalizado | verificado; regras de alias documentadas |
| ID ERP | conector + client + loja/namespace quando aplicável |
| documento | algoritmo válido, não genérico, client/conector escopado e proteção aprovada |

Mesmo forte, conflito com dois subjects produz candidato/quarentena, não escolha.

### 5.2 Match fraco

- nome exato ou fuzzy;
- nickname/display name do WhatsApp/Instagram;
- data de nascimento;
- endereço;
- produto/interesse;
- comportamento ou horário;
- avatar/imagem;
- session/visitor ID sem conversão.

Match fraco só ranqueia candidato. Limiar nunca muda essa proibição.

### 5.3 Precedência de nome

Para saudação:

1. preferred name manual verificado daquela relação;
2. nome autoritativo permitido daquela relação;
3. nome pessoal validado do provider;
4. saudação genérica.

Empresa, frase decorativa, handle, telefone e string não pessoal não viram nome de saudação.

## 6. Fluxos

### 6.1 Identidade nova, mesmo client

1. evento traz binding/client resolvido;
2. resolver procura identidade forte no mesmo client;
3. zero match: cria candidato para novo subject/relationship ou criação automática allowlisted;
4. um match sem conflito: vincula source link idempotente;
5. mais de um: quarentena;
6. evento de resolução ocorre após commit.

Criação automática só pode ocorrer para identidade provider verificada e policy ativa; ela não
preenche lifecycle `customer` por inferência.

### 6.2 Identidade encontrada em outro client

1. não retorna subject ao operador do client atual;
2. cria candidato `cross_client` visível somente à agência autorizada;
3. não cria relationship automaticamente;
4. não compartilha fatos, consentimentos ou histórico;
5. revisão pode reconhecer o mesmo subject e criar relação vazia para o novo client.

### 6.3 Contato histórico misturado

- marcar source link candidato como `quarantined`;
- listar client IDs/contagens e intervalos sem decidir pessoas automaticamente;
- revisão pode dividir relações/sources, reconhecer um único subject ou manter separado;
- nenhum backfill altera conversas/mensagens originais.

### 6.4 Merge e undo

Merge:

- valida scope/permissão/revision;
- adquire locks ordenados;
- grava evento, move referências autorizadas e supersede source;
- publica evento durável após commit.

Undo:

- valida evento e mutações posteriores;
- restaura snapshot ou retorna conflito;
- grava novo evento de undo; não apaga o merge original.

## 7. Contratos de serviço

Interfaces declaradas pelos consumidores, implementadas por adapter:

```go
type SubjectResolver interface {
    ResolveSubject(ctx context.Context, req ResolveSubjectRequest) (ResolveSubjectResult, error)
}

type RelationshipReader interface {
    GetRelationship(ctx context.Context, req RelationshipRequest) (RelationshipSnapshot, error)
}

type IdentityReviewService interface {
    ReviewCandidate(ctx context.Context, req ReviewMatchRequest) (ReviewMatchResult, error)
}
```

### 7.1 `ResolveSubjectRequest`

| Campo | Regra |
|---|---|
| `requestId` | idempotência |
| owner/client IDs | derivados e validados |
| `sourceKey`, `sourceEntityType`, `sourceEntityId`, `sourceVersion` | referência tipada |
| `identities` | normalizadas por adapter; valores classificados |
| `occurredAt` | timestamp de origem |
| `purpose` | finalidade registrada |
| `allowCreate` | policy server-side, não decisão do caller externo |

### 7.2 `ResolveSubjectResult`

- status: `resolved`, `created`, `candidate`, `quarantined`, `not_found`;
- subject/relationship IDs apenas quando caller pode recebê-los;
- match method/confidence;
- candidate ID quando aplicável;
- reason codes;
- idempotent replay flag;
- nenhuma relação de outro client no payload.

## 8. API lógica para CI-03

| Ação | Rota futura | Permissão |
|---|---|---|
| listar candidates | `GET /v1/customer-data/match-candidates` | merge.manage |
| detalhar candidate | `GET /v1/customer-data/match-candidates/{id}` | merge.manage |
| decidir candidate | `POST /v1/customer-data/match-candidates/{id}/decision` | merge.manage |
| merge | `POST /v1/customer-data/subjects/{id}/merge` | merge.manage |
| undo | `POST /v1/customer-data/merges/{id}/undo` | merge.manage |

Body de decisão:

```json
{
  "decision": "accept|reject",
  "targetSubjectId": "uuid opcional",
  "createRelationship": false,
  "reason": "string",
  "expectedRevision": 1,
  "idempotencyKey": "string"
}
```

`createRelationship=true` cross-client requer os gates adicionais; não copia dados.

## 9. Permissões e visibilidade

| Papel lógico | Pode ver |
|---|---|
| operador do client | somente sua relação, identidades permitidas e nome seguro |
| steward da agência | candidates/merges dentro do owner e clients autorizados |
| platform admin | não recebe PII/cross-client automaticamente |
| Customer Intelligence | snapshot minimizado da relação/purpose autorizados |

Todas as queries repetem account + client. Uma rota que recebe somente `subjectId` resolve e filtra
as relações permitidas antes de montar resposta.

## 10. Fixtures obrigatórias

| Fixture | Entrada | Resultado esperado |
|---|---|---|
| `same_person_two_clients` | mesmo telefone verificado em A e B | candidato cross-client; duas relações isoladas |
| `same_name_two_people` | mesmo nome, phones diferentes | dois subjects; zero auto-merge |
| `whatsapp_decorative_name` | display “Loja do João 🚀” | nome não confiável; saudação genérica |
| `verified_manual_name_wins` | manual verificado vs provider | manual vence naquela relação |
| `erp_generic_document` | documento genérico/repetido | quarentena; zero match |
| `erp_scoped_id` | mesmo external ID em conectores/clientes distintos | identidades distintas |
| `site_session_only` | visitor/session sem conversão | não cria identidade pessoal |
| `provider_retry` | mesmo source/version duas vezes | mesmo resultado, zero duplicata |
| `ambiguous_strong_identity` | fingerprint ligada a dois candidates | quarentena |
| `mixed_historical_contact` | contact com conversas A+B | relatório/revisão, sem split automático |
| `merge_then_undo` | merge sem mutação posterior | restaura e mantém dois eventos |
| `undo_conflict` | merge seguido de alteração incompatível | 409, sem perda |
| `client_scope_negative` | operador A acessa relação B | 404 |
| `standalone_self` | account não-agência | relação client=account |
| `organization_is_not_permission` | mesma org, client não acessível | 404 |

Fixtures não contêm PII real.

## 11. Pacotes atômicos e allowlists

CI-02 não autoriza código nem DDL.

### CI02-CONTRACT

- **Pode escrever somente:**
  - `docs/customer-intelligence/specs/CI-02_IDENTIDADE_RELACIONAMENTOS.md`
  - `docs/customer-intelligence/specs/fixtures/identity/README.md`
- **Proibido:** `back/`, `web/`, migrations e workflows.

### CI02-FIXTURES

Somente após aprovação do contrato:

- **Pode criar somente:**
  - `docs/customer-intelligence/specs/fixtures/identity/same_person_two_clients.json`
  - `docs/customer-intelligence/specs/fixtures/identity/same_name_two_people.json`
  - `docs/customer-intelligence/specs/fixtures/identity/provider_retry.json`
  - `docs/customer-intelligence/specs/fixtures/identity/mixed_historical_contact.json`
  - `docs/customer-intelligence/specs/fixtures/identity/merge_undo.json`
  - `docs/customer-intelligence/specs/fixtures/identity/scope_negative.json`
- **Proibido:** dados reais, telefone/e-mail/documento real, output de banco.

### CI02-QA

- **Read-only:** confrontar fixtures contra CI-00/CI-01/Governança e schema atual.
- **Escreve somente:** `docs/customer-intelligence/evidence/CI-02_REVIEW.md`.

## 12. Testes de contrato

Quando CI-03 implementar:

```text
go test ./internal/modules/customerdata/... -run 'Identity|Relationship|Match|Merge'
go test ./internal/modules/customerdata/...
go test ./...
```

Testes property/table-driven:

- ordem de input não altera resolução;
- replay mantém IDs;
- fuzzy/name nunca retorna auto-merged;
- toda resposta autorizada contém somente um client scope;
- merge não cria ciclo;
- locks concorrentes não produzem dois winners;
- valor sensível não aparece em erro/log/snapshot de teste.

## 13. Rollout e rollback

Como CI-02 é contrato:

- rollout = aprovar invariantes/fixtures e referenciá-los na CI-03;
- rollback = nova versão documental com decisão explícita;
- não há dado para migrar nem apagar.

Depois que CI-03 persistir IDs, mudanças semânticas em `subject`/`relationship` exigem migration e
compatibilidade; não se “corrige” retrospectivamente por prompt.

## 14. Critérios de aceite

- [ ] mesma pessoa pode ter duas relações isoladas;
- [ ] subject não concede acesso cross-client;
- [ ] match forte cross-client gera apenas candidate;
- [ ] nome/fuzzy nunca auto-merge;
- [ ] identidade forte inclui issuer/escopo necessário;
- [ ] visitor/session não vira pessoa sem conversão;
- [ ] merge/undo são idempotentes e auditáveis;
- [ ] contatos misturados entram em quarentena;
- [ ] fixtures cobrem standalone, agência e tenant negativo;
- [ ] nenhuma tabela, writer ou workflow foi criado.

## 15. Stop conditions

Parar se:

- CI-00/CI-01 mudarem topologia de IDs;
- a solução exigir subject global entre organizations;
- um client user puder consultar identidade/relação de outro client;
- matching por nome/fuzzy for necessário para auto-merge;
- documento/telefone/e-mail precisar de hash simples ou log bruto;
- ERP genérico for tratado como identificador forte;
- merge não puder ser desfeito/auditado;
- contato misturado precisar ser corrigido automaticamente;
- fixtures exigirem PII real;
- qualquer executor tentar criar DDL/writer nesta CI.

## 16. Handoff obrigatório

Entregar:

- versão dos invariantes aprovada;
- decisões canônicas ainda pendentes;
- fixtures e resultados esperados;
- riscos de dados históricos;
- pontos que CI-03 deve implementar;
- confirmação de zero DDL, writer, backfill, workflow e dado real.
