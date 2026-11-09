# Contratos HTTP e portas Go — v1.0

Estado: especificação futura, não endpoints implementados. Decisões normativas em [CONTRATOS](CONTRATOS.md); execução em [OWNERSHIP](OWNERSHIP.md). Todos os exemplos são sintéticos. UUIDs abaixo só identificam fixtures; não correspondem a empresas/usuários reais. Omitir dados fiscais reais é intencional.

## Convenções e limites

Prefixo novo `/v1/accounting`. Conta/ator resolvidos pelo Principal, nunca no JSON. UUID malformado de recurso retorna 404; payload estrutural inválido 400. Decodificação estrita, objeto único, campos desconhecidos rejeitados. HTTP 401 sem autenticação; 403 módulo desligado/permissão insuficiente em recurso visível; 404 recurso invisível, inclusive emitente alheio; 409 conflito de versão/idempotência/estado; 422 dado necessário ou cenário sem cobertura; 503 dependência indisponível. Não colocar respostas privadas no detalhe de erro.

`Idempotency-Key` obrigatório em POST mutável e em PUT/PATCH mutável novo: ASCII 8–200; chave externa inclui escopo interno conta+ator+emitente+operação. Criação de emitente usa escopo account; troca do ator não contorna unicidade de negócio da execution. Hash servidor inclui método/rota/body canônicos e contexto efetivo; mesma chave e mesmo hash retorna resultado persistido após revalidar acesso; outro hash 409. Versão é inteiro positivo, criação expectedVersion 0; CAS em UPDATE. Replay vem antes de CAS, mas depois de autorização. Chaves de operações diferentes não colidem. Leituras GET não exigem chave; busca POST é read e não exige confirmação/chave.

JSON novo: strings de até 4000 caracteres por resposta de diagnóstico; corpo diagnóstico/documento 64KiB, config 32KiB, chat 16KiB/pergunta 4000 caracteres. Documento até 100 itens; lista page>=1, limit padrão 20/máximo 100; buscas query 1–500 caracteres, topK 1–20 padrão 5, trecho até 1200 caracteres e retorno até 32KiB; empate estável por ID. Plus até 4 tools por turno, sem recursão, timeout 30s/turno e 10s/tool local; execução fiscal timeout do adapter limitado a 60s e lease maior que janela local, sem inferir desfecho por timeout. Erros operacionais armazenam código seguro, nunca corpo bruto do provedor.

Envelope novo padrão: `{contractVersion:"1.0",status:"success",data:...,requestId:...}`; erro status error com error `{code,message,fields:[]}`. RequestId é correlationID servidor. Datas/decimais/null seguem CONTRATOS. Exemplos são objetos JSON completos; IDs rótulos nos títulos apontam as mesmas fixtures entre os blocos.

## A1 — Emitente e vínculo (CT-03)

POST `/issuers` exige accounting.issuers.manage. Fonte: formulário protegido, nenhuma importação implícita. Cria administrador inicial e perfil v1, status de confirmação declarado. IDs gerados no servidor. GET `/issuers` filtra vínculos, GET `/issuers/{id}` exige view; PATCH com expectedVersion altera perfil criando versão nova. Membros: PUT `/issuers/{id}/members/{userId}` com grants/status/expectedVersion, manage e teto de permissões, usuário já membro da conta. Revogação invalida access_version. Nenhuma credencial aceita nestes payloads.

```json
{"expectedVersion":0,"profile":{"legalName":"Emitente sintético","document":null,"issuerType":null,"taxRegime":null,"municipality":null,"municipalityCode":null,"state":null,"municipalRegistration":null,"stateRegistration":null},"intent":"service"}
```

201, cadastro incompleto permitido:

```json
{"contractVersion":"1.0","status":"success","requestId":"req-issuer-001","data":{"id":"10000000-0000-4000-8000-000000000001","version":1,"profileVersion":1,"accessVersion":1,"confirmation":"declared","missingFields":["document","municipalityCode","state","taxRegime"],"capabilities":{"diagnose":true,"prepare":true,"review":false,"submit":false}}}
```

Capabilities são projeção derivada, recalculadas pelo serviço, sem campo de escrita. Identidade incompleta permite rascunho, nunca fiscalmente apto. Perfil aceita somente campos de CT-01 Q3–Q7, endereço/contato e referências protegidas de configuração, não URL arbitrária de execução. Campos de confirmação documental exigem evidenceRef revisado, não booleano do cliente.

## A2 — Diagnóstico persistido (CT-03; condução CT-04/05)

PUT `/issuers/{issuerId}/diagnosis`, GET na mesma rota. Uma linha de diagnóstico por intenção+competência não é imposta: PUT deve receber diagnosisId para atualizar ou null para criar, preservando operações independentes. GET aceita diagnosisId e é obrigatório ao retomar; sem ele lista paginada os diagnósticos do emitente. manage para salvar, view para ler; porta revalida vínculo e versão de perfil. Answers usa campos allowlist de Q1–Q13 do CT-01 (identidade, localização, regime, inscrições, serviço/tomador ou produto/destinatário, competência/valor/acesso); rejeita chaves desconhecidas. Estado confirmado só por evidência revisada validada no servidor.

```json
{"diagnosisId":null,"expectedVersion":0,"expectedProfileVersion":1,"intent":"service","competenceDate":null,"answers":{"taxRegime":{"value":null,"confirmation":"unknown","origin":null,"evidenceRef":null}}}
```

200:

```json
{"contractVersion":"1.0","status":"success","requestId":"req-diag-001","data":{"id":"20000000-0000-4000-8000-000000000001","issuerId":"10000000-0000-4000-8000-000000000001","version":1,"profileVersion":1,"intent":"service","competenceDate":null,"answers":{"taxRegime":{"value":null,"confirmation":"unknown","origin":null,"evidenceRef":null}},"nextQuestionId":"Q3","pending":[{"field":"document","code":"missing_data","blocks":["review","submit"]}],"capabilities":{"diagnose":true,"prepare":true,"review":false,"submit":false}}}
```

Origem permitida: declaration, personal_import, operational_document, official_evidence; null somente unknown. Registro de origem inclui owner/resourceId/version quando há recurso persistido. Conflito de duas evidências vira pending evidence_conflict e não muda o valor confirmado automaticamente. Diagnósticos não são cálculos nem autorizações.

## A3 — Consulta financeira determinística (CT-05, fachada Finance)

Tool finance.sheet.read.v1 invoca GET existente `/v1/finance/sheets/{sheetId}` via service/fachada, nunca SQL externo. accounting desligado não bloqueia essa consulta. Escopo Finance e finance.sheets.view, sem emitir nem efetivar linha. Argumentos e resultado normalizado abaixo; rota HTTP legada mantém resposta anterior.

```json
{"toolId":"finance.sheet.read.v1","input":{"sheetId":"30000000-0000-4000-8000-000000000001","page":1,"limit":20}}
```

```json
{"toolId":"finance.sheet.read.v1","status":"completed","data":{"sheetId":"30000000-0000-4000-8000-000000000001","period":"2026-11","sourceVersion":"2026-11-03T12:00:00Z","totals":{"effectiveOut":"125.00"},"lines":[{"id":"30000000-0000-4000-8000-000000000002","amount":"125.00","effective":true,"effectiveDate":"2026-11-02","dueDate":null}],"dueDatesAvailable":false,"precisionSource":"finance_legacy","page":1,"limit":20,"total":1}}
```

Totais calculados pelo owner sobre todas as linhas, não soma da página/contexto truncado. Se fonte não fornece paginação, adapter pagina só depois de leitura autorizada e indica total; não promete escala além do Finance atual. Solicitação de vencimentos retorna capability_unavailable, sem substituir data de efetivação. Teto de memória respeita limite do owner; não adicionar exportação irrestrita ao chat.

## A4 — Preparação e revisão (CT-03)

POST `/issuers/{issuerId}/documents/prepare` com accounting.documents.manage. Pode preparar parcialmente sem acesso remoto. Fonte empresarial é diagnóstico/perfil selecionados; import pessoal opcional chama facade Finance no escopo do próprio ator e exige confirmação de compartilhamento. Import vazio não copia nada. Kind nfse usa service; nfe usa items/operation/recipient; regras concretas de cobertura vêm de CT-08/09. JSONB não libera campos arbitrários.

```json
{"expectedVersion":0,"diagnosisId":"20000000-0000-4000-8000-000000000001","expectedDiagnosisVersion":1,"expectedProfileVersion":1,"kind":"nfse","source":null,"fields":{"competenceDate":null,"amount":null,"service":{"description":null,"serviceCode":null,"cnae":null,"nbs":null,"issRate":null,"issWithheld":null,"location":null},"customer":{"name":null,"document":null,"address":null}}}
```

201:

```json
{"contractVersion":"1.0","status":"success","requestId":"req-prepare-001","data":{"id":"40000000-0000-4000-8000-000000000001","issuerId":"10000000-0000-4000-8000-000000000001","kind":"nfse","version":1,"state":"draft","pending":[{"field":"competenceDate","code":"missing_data","blocks":["review","submit"]}],"emissionAvailable":false}}
```

Origem pessoal opcional: `{owner:"finance",draftId,expectedDraftUpdatedAt,expectedProfileUpdatedAt,templateId:null,expectedTemplateUpdatedAt:null,selectedFields:["service.description"],confirmShareWithIssuer:true}`. Dados importados são apenas projeção fiscal selecionada e referências de origem; acesso/senha/sessão não são importáveis. Refazer import usa edição com nova versão, nunca substitui silenciosamente fields explícitos.

GET `/issuers/{issuerId}/documents/{documentId}` e GET `.../review` exigem documents.view; PATCH `.../{documentId}` exige manage, expectedVersion e fields; POST `.../validate` exige manage e expectedVersion; retorna validated só quando validador cobre cenário, senão 422 coverage_unavailable/validation_failed. GET review retorna snapshot/hash calculados pelo servidor e pending; POST `.../reviews` exige documents.review, versões e confirmed. DELETE `.../reviews/{reviewId}` exige review e expectedVersion; não apaga histórico.

Exemplo sintético de POST reviews **para fixture completa validada separada**, documento 400…002 v3, perfil v2, diagnóstico v2; não é continuação do rascunho incompleto acima. Hash é 64 caracteres sintéticos, servidor o recalcula; fixtures executáveis futuras devem derivá-lo, não aceitar este literal como prova.

```json
{"expectedVersion":3,"expectedProfileVersion":2,"snapshotHash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","confirmed":true}
```

201:

```json
{"contractVersion":"1.0","status":"success","requestId":"req-review-002","data":{"documentId":"40000000-0000-4000-8000-000000000002","version":4,"state":"reviewed","reviewId":"50000000-0000-4000-8000-000000000001","snapshotHash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","emissionAvailable":false}}
```

GET review aceita seleção explícita connectorId/environment antes da confirmação; mudar qualquer um invalida a revisão. Sem seleção, retorna snapshot de preparação sem capacidade de envio; para autorizar, revisar novamente com a seleção desejada. GET review desta fixture retorna dados estruturados do documento/perfil, versions `{document:3,profile:2,diagnosis:2}`, hash, pending vazio e reviewId quando confirmado. A revisão muda versão de estado para 4 sem mudar payload revisado v3; hash vincula payload/version revisada, autorização também verifica versão de estado corrente. Revisão local não prova cobertura de conector. NF-e estrutural usa itens `{description,unit,quantity,unitPrice,discount,addition,ncm,cfop,cst,csosn,origin}`, operation `{nature,purpose,originState,destinationState}`, recipient `{name,document,address,stateRegistration}`, transport/payment nullable. Códigos podem ser null em draft, mas não recebem defaults em validate.

## A5 — Autorização, execução e job (CT-03; conectores CT-08/09)

POST `.../documents/{id}/authorizations`: documents.issue + grant issue; confirmed true; review/hash/versões atuais e conector/ambiente explícitos. Ambiente enum fake_local, homologation, production. Fake nunca produz autorização real: projection simulated true; valor state authorized significa somente resultado daquele ambiente, UI deve exibir simulação. Sem adapter/cobertura configurados retorna 422 coverage_unavailable, sem criar autorização. Fixture abaixo pressupõe conector fake registrado e snapshot da revisão compatível com esse ambiente.

```json
{"expectedVersion":4,"reviewId":"50000000-0000-4000-8000-000000000001","snapshotHash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","connectorId":"fake.nfse.v1","environment":"fake_local","confirmed":true}
```

201:

```json
{"contractVersion":"1.0","status":"success","requestId":"req-auth-002","data":{"authorizationId":"60000000-0000-4000-8000-000000000001","documentId":"40000000-0000-4000-8000-000000000002","version":5,"authorizationVersion":1,"state":"authorized_for_submission","expiresAt":"2026-11-03T12:15:00Z","environment":"fake_local","simulated":true}}
```

POST `.../documents/{id}/execute`: idempotência + documents.issue; corpo:

```json
{"expectedVersion":5,"authorizationId":"60000000-0000-4000-8000-000000000001","businessKey":"synthetic-document-002-revision-3","operation":"issue"}
```

202 (replay retorna mesmo 202/envelope, GET mostra evolução):

```json
{"contractVersion":"1.0","status":"success","requestId":"req-exec-002","data":{"executionId":"70000000-0000-4000-8000-000000000001","jobId":"80000000-0000-4000-8000-000000000001","documentId":"40000000-0000-4000-8000-000000000002","documentVersion":6,"state":"queued","environment":"fake_local","simulated":true}}
```

Estado documento permanece authorized_for_submission até claim, versão 6 representa comando reservado. Segundo comando para o documento ativo é conflito mesmo com businessKey novo. Job `{kind:"accounting.issue.v1",payload:{executionId},orderingKey:"issuerId:documentId"}`. GET `/issuers/{issuerId}/executions/{id}` exige documents.view e retorna state queued/processing/unknown/authorized/rejected/cancelled, documentState, version, safeErrorCode nullable, remoteReference nullable, protocol nullable, artifactsPending e artifacts (IDs privados, sem URL pública). unknown é resultado operacional, não HTTP 504 que induza reenvio. Revogação POST `.../authorizations/{id}/revoke` usa expectedVersion da autorização e documents.issue, retorna estado efetivo sem prometer cancelar POST remoto em andamento.

## A6 — Busca e evidência (CT-06/07)

POST `/issuers/{issuerId}/knowledge/search`, read com accounting.knowledge.view, grant knowledge_view. Fonte é serviço neutro com binding autorizado. Sem data/jurisdição/regime confirmados: 200 insufficient_evidence e lista de pendências, sem afirmar regra. Exemplo de consulta de fixture sintética com contexto confirmado:

```json
{"query":"regra sintética de teste","competenceDate":"2026-11-01","topK":5}
```

200:

```json
{"contractVersion":"1.0","status":"success","requestId":"req-search-001","data":{"status":"completed","evidence":[{"documentId":"90000000-0000-4000-8000-000000000001","version":1,"chunkId":"90000000-0000-4000-8000-000000000002","excerpt":"Texto sintético sem conteúdo normativo real.","location":"seção de teste, parágrafo 1","source":"manual:synthetic-fixture","jurisdiction":"synthetic","validFrom":"2026-01-01","validTo":null,"applicability":"Fixture revisada compatível com o contexto sintético.","score":0.12}],"truncated":false}}
```

Em produção, source é origem oficial verificável ou referência privada autorizada; nunca URL de download com segredo. Consulta não baixa URL recebida do usuário/modelo. UI de fontes CT-06 usa CRUD pelo serviço knowledge neutro com política Accounting, e PUT `/issuers/{issuerId}/knowledge/bindings/{baseId}` `{isEnabled,expectedVersion}` exige knowledge.manage e prova de acesso à base da mesma conta. Retorna binding/version; criar expectedVersion 0. Nenhum agente fictício.

## A7 — Plus e tools (CT-05; fundamentação CT-07)

POST `/v1/assistant/chat/ask` preserva payload legado; **somente** mode plus retorna envelope v1 abaixo. Idempotency-Key e receipts continuam no owner Calendar. Surface finance, plus enabled, Finance acessível; issuerId nullable para consulta só financeira. Contábil exige módulo/vínculo/permissão. Criar/retomar/listar/get/delete usa rotas compartilhadas existentes, com mode plus e filtro pessoal; conversa legacy não aceita virar plus. Listing informa mode/issuerId sem misturar mensagens.

```json
{"surface":"finance","mode":"plus","contractVersion":"1.0","question":"Quais dados faltam para preparar minha nota?","conversationId":null,"issuerId":"10000000-0000-4000-8000-000000000001","financeSheetId":null}
```

200:

```json
{"contractVersion":"1.0","requestId":"req-plus-001","conversationId":"a0000000-0000-4000-8000-000000000001","context":{"surface":"finance","mode":"plus","issuerId":"10000000-0000-4000-8000-000000000001"},"intent":"diagnosis","status":"missing_data","message":"Complete o documento do emitente no formulário protegido.","data":{"nextQuestionId":"Q3"},"evidence":[],"proposals":[],"tools":[{"id":"accounting.diagnosis.read.v1","status":"completed"}]}
```

Status do envelope: completed, missing_data, insufficient_evidence, provider_unavailable, awaiting_authorization, capability_unavailable. provider_unavailable só quando a parte solicitada depende de IA; leitura determinística pode completar com aviso de indisponibilidade. Não retornar sucesso de ferramenta que falhou. Propostas são referências a recursos persistidos, `{id,kind,resourceId,expectedVersion,status}`; confirmar encaminha ao comando dono do domínio, nunca ao executor genérico de proposals Calendar para emissão.

Formato interno de tool `{id,version:1,mode,inputSchema,outputSchema,permission,timeoutMs,maxOutputBytes,confirmation}`. Schemas fechados correspondem a A2/A3/A4/A6; invocation `{toolId,input}` não aceita accountId/userId/permission. Result `{toolId,status,data,errorCode}`. Permissões em CONTRATOS §2, revalidadas pelo serviço chamado. Leituras confirmation none; prepare exige intenção explícita/versão; issue não está no registry M1. Mutation de diagnóstico usa o receipt do domínio com chave derivada turno+toolCallID, além do receipt Calendar; crash não repete efeito.

## A8 — Erros completos

409, mesma chave/payload diferente:

```json
{"contractVersion":"1.0","status":"error","requestId":"req-conflict-001","error":{"code":"idempotency_conflict","message":"Esta chave já foi usada com outros dados.","fields":[]}}
```

404 recurso fora do vínculo/conta (mesmo envelope para inexistente):

```json
{"contractVersion":"1.0","status":"error","requestId":"req-notfound-001","error":{"code":"not_found","message":"Recurso não encontrado.","fields":[]}}
```

422 preparação incompleta quando se tenta validar:

```json
{"contractVersion":"1.0","status":"error","requestId":"req-missing-001","error":{"code":"validation_failed","message":"Complete os dados indicados antes de validar.","fields":[{"field":"competenceDate","code":"missing_data"}]}}
```

Outros códigos fechados: conversation_context_conflict, execution_in_progress, version_conflict, snapshot_stale, authorization_expired, authorization_revoked, request_in_progress, execution_unknown, evidence_conflict, coverage_unavailable, capability_unavailable, module_disabled, permission_denied, provider_unavailable. Erros incluem somente campos autorizados. Estado desconhecido retorna GET 200 state unknown; nova tentativa de execute no mesmo documento retorna 409 execution_unknown, sem reenvio.

## Portas Go mínimas (novas; não copiar como se já existissem)

Esboço compilável dos DTOs mínimos de fronteira, sem implementação. Métodos públicos revalidam contexto pelo owner; não confiar em structs construídas pelo modelo. Implementação interna pode desdobrar DTOs fechados sem mudar JSON. context/json/time são biblioteca padrão; pgx/jobs são dependências já existentes. Interfaces ficam no consumidor, adapters no owner/composição. Repository e transação não entram no handler.

```go
package contracts

import (
    "context"
    "encoding/json"
    "time"

    "github.com/jackc/pgx/v5"
    "github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

type Scope struct {
    AccountID, ActorID, IssuerID, Surface, CorrelationID string
}
type Versioned struct { ID string; Version int64 }
type IssuerInput struct { ExpectedVersion int64; Profile json.RawMessage }
type DiagnosisInput struct {
    ID string
    ExpectedVersion, ExpectedProfileVersion int64
    Intent string
    CompetenceDate *string
    Answers json.RawMessage
}
type PreparationInput struct {
    DiagnosisID, Kind string
    ExpectedVersion, ExpectedDiagnosisVersion, ExpectedProfileVersion int64
    Source, Fields json.RawMessage
}
type Snapshot struct {
    DocumentID, Hash string
    DocumentVersion, ProfileVersion, DiagnosisVersion int64
    Payload json.RawMessage
    Pending []string
}
type ReviewInput struct {
    ExpectedVersion, ExpectedProfileVersion int64
    SnapshotHash string
    Confirmed bool
}
type AuthorizationInput struct {
    ExpectedVersion int64
    ReviewID, SnapshotHash, ConnectorID, Environment string
    Confirmed bool
}
type Authorization struct { Versioned; ExpiresAt time.Time }
type ExecuteInput struct {
    ExpectedVersion int64
    AuthorizationID, BusinessKey, Operation string
}
type Execution struct {
    Versioned
    DocumentID, JobID, State, Environment string
    RemoteReference, Protocol *string
    Simulated, ArtifactsPending bool
}
type AccountingPort interface {
    SaveIssuer(context.Context, Scope, string, IssuerInput) (Versioned, error)
    SaveDiagnosis(context.Context, Scope, DiagnosisInput) (Versioned, error)
    ReadDiagnosis(context.Context, Scope, string) (json.RawMessage, error)
    Prepare(context.Context, Scope, PreparationInput) (Versioned, error)
    Review(context.Context, Scope, string) (Snapshot, error)
    ConfirmReview(context.Context, Scope, string, ReviewInput) (Versioned, error)
    Authorize(context.Context, Scope, string, AuthorizationInput) (Authorization, error)
    Execute(context.Context, Scope, string, ExecuteInput) (Execution, error)
}
type FinanceSelection struct { SheetID string; Page, Limit int }
type PersonalSource struct {
    DraftID, ExpectedDraftUpdatedAt, ExpectedProfileUpdatedAt string
    TemplateID, ExpectedTemplateUpdatedAt *string
    SelectedFields []string
    ConfirmShareWithIssuer bool
}
type FinancePort interface {
    ReadSheet(context.Context, Scope, FinanceSelection) (json.RawMessage, error)
    ReadPersonalSource(context.Context, Scope, PersonalSource) (json.RawMessage, error)
}
type SearchInput struct { Query, CompetenceDate string; TopK int }
type Evidence struct {
    DocumentID, ChunkID string
    Version int
    Excerpt, Location, Source, Jurisdiction, Applicability string
    ValidFrom, ValidTo *string
    Score float64
}
type KnowledgePort interface {
    Search(context.Context, Scope, SearchInput) ([]Evidence, error)
}
type Capability struct {
    ConnectorID, Version, Kind, Environment string
    CanSubmit, CanLookup bool
    UnavailableCode *string
}
type Submission struct {
    ExecutionID, Operation, Environment, RemoteIdentity, SnapshotHash string
    Payload json.RawMessage
}
type Outcome struct {
    State string // authorized | rejected | unknown | not_submitted
    RemoteReference, Protocol, SafeErrorCode *string
}
type Artifact struct { Kind, SHA256 string; Content []byte }
type FiscalConnector interface {
    Capability(context.Context, Scope) (Capability, error)
    Validate(context.Context, Scope, Submission) ([]string, error)
    Submit(context.Context, Scope, Submission) (Outcome, error)
    Lookup(context.Context, Scope, Submission) (Outcome, error)
    FetchArtifacts(context.Context, Scope, Submission) ([]Artifact, error)
}
type TransactionalQueue interface {
    EnqueueTx(context.Context, pgx.Tx, jobs.NewJob) (string, bool, error)
}
type ArtifactStore interface {
    PutPrivate(context.Context, Scope, string, Artifact) (string, error)
}
type PlusPort interface {
    Ask(context.Context, Scope, string, json.RawMessage) (json.RawMessage, error)
}
```

RawMessage é marcador de DTO versionado validado pelos schemas A1–A7, nunca autorização para payload livre/SQL. Implementação CT-03 deve fornecer tipos concretos dos fields fiscais; CT-05 valida tool e Plus antes de despacho. Adapter de conector resolve segredo pelo cofre apenas no servidor; segredo não existe em Submission/Outcome. RemoteIdentity é identidade consultável escolhida/persistida antes de Submit, com formato determinado pelo conector; ausência bloqueia CanSubmit. Artifact.Content permanece só no servidor, limitado a 16MiB/artefato e quantidade máxima 10; protocolos não são inventados. Storage verifica account/owner/cota e mantém referências privadas.

## Fixtures de contrato e resultados esperados

Casos documentais normativos; CT-03/05/06 convertem em fixtures executáveis sem dados reais. A1/A2/A4 incompletos não podem produzir A5; fixture completa A4 v3 é separada. Todos os efeitos são fake_local nesta revisão.

| ID | Entrada/condição | Resultado obrigatório |
|---|---|---|
| C01 | Ator da conta A fornece issuer/document/execution da B | 404 not_found, sem leitura/fila |
| C02 | Dois usuários na conta A, emitentes diferentes; um pede o do outro sem vínculo | 404; nem admin/agência recebe sessão pessoal alheia |
| C03 | Accounting desligado, Finance ligado | Accounting 403 module_disabled; leitura/prepare pessoais Finance existentes funcionam |
| C04 | Leitor vê documento e tenta autorizar | 403 permission_denied; nenhuma autorização/job |
| C05 | Snapshot revisado, payload/perfil/evidência muda | 409 snapshot_stale ou version_conflict; revisão/autorização não reaproveitadas |
| C06 | Mesmo execute/key/hash após sucesso do comando | Mesmo executionId/jobId e envelope 202; nenhum segundo job |
| C07 | Mesma key com body diferente | 409 idempotency_conflict, inclusive expectedVersion diferente |
| C08 | Timeout após POST ou crash antes de persistir resposta | execution/document unknown; Lookup antes de retry; sem prova permanece unknown |
| C09 | Conta troca enquanto request retorna | Cliente descarta resposta por contexto; servidor não relabela conversa nem cache |
| C10 | Fonte vencida para competência / vigência não verificada | 200 insufficient_evidence, sem cálculo/afirmação fiscal; versão histórica válida pode ser selecionada |
| C11 | IA indisponível | Consulta determinística conclui; explicação dependente de IA retorna provider_unavailable, sem texto de sucesso inventado |
| C12 | Revogação/expiração antes do claim | Worker cancela comando sem Submit; reautorizar se ainda aplicável |
| C13 | Outbox insert falha | Rollback execução/receipt/evento; sem job órfão e autorização não consumida |
| C14 | Lease expira enquanto worker antigo pode estar remoto | Novo worker reconcilia, não submete; resposta antiga só grava com fence válido |
| C15 | Fonte pessoal pertence a outro usuário da mesma conta | 404, mesmo com manage empresarial; nenhum campo importado |
| C16 | Novo mode plus com conversationId legacy ou issuer diferente | 409 conversation_context_conflict; criar conversa nova, sem mesclar histórico |
| C17 | Fake retorna authorized | simulated true, environment fake_local; UI não apresenta como nota real |
| C18 | Provedor autorizou, storage falhou | authorized com artifactsPending true; recuperação não chama Submit |
| C19 | Mesmo documento ativo, businessKey diferente | 409 execution_in_progress; uma execução efetiva por autorização/documento |

Compatibilidade: Finance conserva expectedUpdatedAt/float API, preparação pessoal e config; Calendar conserva aliases/receipts/confirm de propostas não fiscais; Omnichannel conserva rotas, bindings por agente e políticas de atendimento, usando o mesmo repositório knowledge. CT-10 verifica regressões integradas, não só mocks.

## Notas de implementação CT-03

- `contentVersion` distingue revisão de dados de mudanças de estado em `version`;
  editar os mesmos valores também invalida o snapshot anterior. `validatedConnector`
  e `validatedEnvironment` guardam a seleção local; review sem seleção explícita
  retoma essa seleção, sem mudar ambiente silenciosamente.
- PATCH de documento admite `source` opcional para reimportação consentida e
  versionada; somente campos selecionados são copiados. Declarações do diagnóstico
  não alteram o perfil do emitente por conta própria. Pendências indicam campos
  faltantes; confirmação documental/curadoria pertence à CT-06.
- DELETE de emitente desativa logicamente, com expectedVersion; DELETE de documento
  só aceita draft/rejected e preserva histórico. Recurso alheio retorna404.
- GET execution inclui documentState e metadados privados de artifacts. POST
  `.../executions/{id}/recover-artifacts` recebe expectedVersion + Idempotency-Key,
  retorna202 com jobId. Recuperação usa a mesma execução, sem novo Submit.
- Validação sem conector é estrutural e permite revisão local, com emissão
  indisponível. Validação fiscal/conector real continua dependente CT-08/09.
- Replay preserva o resultado de negócio e IDs; requestId no envelope HTTP é a
  correlação da chamada atual. Hashes sintéticos deste documento não são aceitos
  como snapshots reais pelo servidor.

Bootstrap de permissão de emissão: criação aceita `enableIssue:false` por padrão;
true exige também accounting.documents.issue e cria o grant inicial do criador.
Gerência de emitente sozinha nunca concede issue. Emitente sem nenhum emissor ativo
pode receber seu primeiro grant issue por PUT members com `confirmed:true`, exigindo
accounting.issuers.manage, accounting.documents.issue e grant manage no emitente.
Havendo emissor ativo, o concedente precisa de grant issue efetivo; confirmed não
contorna essa regra. Essa confirmação concede acesso, não autoriza uma nota.

## CT-05 — transporte Plus1.0

- GET `/v1/finance/assistant/plus/status?issuerId=`: data.enabled/mode/toolPolicyVersion
  e catálogo autorizado; não exige IA configurada. Flag na configuração Finance
  existente `plus: {enabled:false,toolPolicyVersion:1}`.
- POST `/v1/assistant/chat/ask`: surface finance, mode plus, contractVersion1.0,
  question; contexto opcional conversationId/issuerId/financeSheetId/month/
  financeDraftId/diagnosisId/documentId/competenceDate. Ação estruturada:
  `{toolId,input}`. `Idempotency-Key` obrigatório. Conta/ator só do Principal.
- GET `/v1/assistant/chat/conversations?mode=plus&issuerId=&page=1&limit=20`: own
  conversations. GET/DELETE com `/id` e mesmo contexto. Detalhe retorna mensagens
  recentes em ordem cronológica; páginas anteriores por page. Máximo100.
- Envelope1.0 contém requestId, conversationId, context, intent/status/message,
  data/evidence/proposals/tools/routingReason. Resultado é persistido/reproduzido.
  Chave igual com conteúdo divergente409; execução incerta409 sem retry automático.
  Emitente de conversa imutável; outro dono/conta404.
- Ferramentas v1: finance.sheet.read; accounting.diagnosis.read/save;
  accounting.document.prepare/review; knowledge.search; finance.personal.prepare/guide.
  Knowledge ausente até CT-06; interpretação ausente até CT-07. Não existe tool
  de emissão, SQL, shell ou URL arbitrária. Schema fechado e até uma chamada/turno.

## CT-06 — conhecimento neutro1.0

Prefixo `/v1/accounting/issuers/{issuer}/knowledge` (conta/ator do Principal):

| Método / caminho | Payload / resultado |
|---|---|
| GET /bases | até100bases privadas do emitente |
| POST /bases | name; base desativada por padrão |
| PATCH /bases/{base} | expectedVersion, enabled; CAS no binding |
| GET /bases/{base}/documents | page/limit,20padrão/100teto |
| POST /imports | baseId,title,sourceRef,fileName,contentType,content(base64),metadata |
| PUT /documents/{id}/review | expectedVersion, metadata, confirmed |
| POST /documents/{id}/publish | expectedVersion, confirmed:true |
| POST /documents/{id}/deactivate | expectedVersion |
| POST /search | query,competenceDate,jurisdiction,regime,topK,page |
| GET /evidence/{document}/{chunk} | trecho/referência revalidados, sem original inteiro |

metadata: authority,jurisdiction,regime,classification,publishedOn?,validFrom?,
validTo?,validitySource. Datas civis; fim exclusivo. Ausência explícita permanece
null/unknown, não prova vigência. Revisão registra ator/data; publicação exige
extração completa e revisão, é atômica. Documento novo tem version de conteúdo e
lockVersion CAS separados; nova importação com bytes iguais retorna versão existente
sem sobrescrever revisão. Mudança do original na mesma origem cria nova versão.

Original até2MiB em storage privado; JSON até3MiB; tipos text/plain/PDF textual,
PDF até10páginas/32kB extraídos. Erro parcial não deixa versão publicada. Nenhuma
URL é executada/baixada; origem https:// ou manual:// sem credencial/query/fragmento.
Vigência desconhecida permite material informativo, sem cálculo conclusivo.

Search: topK1–20/page até10000; até24KiB e truncated explícito. Retorna
status/evidence/potentialConflict/message. Cada evidência contém documentId/version/
chunkId, título/origem/trecho, location com offsets Unicode, score FTS, metadados e
razão de aplicabilidade. ContextConfirmed é interno e não aceito no JSON; o runtime
atual não transforma contexto declarado em confirmado. Score não é confiança jurídica.

Autorização Accounting e grants knowledge_view/manage independentes de atendimento.
Base de outro emitente/conta404; CAS409. Corpus físico/IDs messaging existentes
permanecem; Omnichannel conserva rotas/contratos/bindings de agente por bridge.
Bases fiscais privadas ficam fora de gestão e busca legadas. Sem cache de permissões.

## CT-07 — explicações Plus, extensão compatível

Envelope Plus continua1.0, schema `finance_plus_source_summary` v1 e política1.
Config existente `plus.explanation`:

- enabled:false por padrão; credentialId/model escolhidos por administrador da
  conta no cofre canônico. Não altera enabled/model/credentialId do chat anterior.
- taskClass=`source_summary`, maxInputBytes1000–16000, maxOutputTokens128–2048,
  timeoutMs1000–15000, dailyRequests1–1000 (conta inteira, dia UTC).
- maxRunCostUsd decimal string opcional. Sem tabela vigente e teto definido,
  não chama provedor. Sem teto, custo desconhecido continua null, nunca zero.
- pricing opcional: version,source,validFrom,validTo (fim exclusivo),
  inputUsdPerMillion/outputUsdPerMillion decimais até6casas. Taxas/fontes declaradas
  e conferidas pelo gestor; sem preço comercial embutido ou seleção automática.
  Snapshot de preço é preservado na run. Estimativa USD por tokens, não fatura.

Entrada interpretativa inicial: “resuma fonte termos” / “explique fonte termos”,
com emitente selecionado. Retrieval top3 autorizado; contexto mínimo sem histórico,
sem PII desnecessária, sem credencial, fonte delimitada como dado não confiável.
Sem fonte ou com conflito/truncamento, abstém-se sem gastar IA. Demais perguntas
tributárias informam ausência de cobertura; não há cálculo ou autonomia fiscal.

Saída do modelo fechada: answer,evidenceIds,assumptions,missingData,coverage
(`informational` ou `abstained`). Go valida schema, IDs pertencentes à recuperação,
limites e bloqueios de valores/URLs/alegações de execução indevidas. Não há campo de
tool/proposta executável. Citação inventada invalida resposta. Abstenção descarta
texto livre do modelo e usa mensagem determinística. Uma chamada, sem fallback.

`data` mostra cobertura/hipóteses/pendências e usage:
runId,model,status,tokens?,costUsd?,latencyMs. Campo ausente/null é desconhecido.
Referências/URLs exibidas vêm do registry servidor em evidence, nunca da resposta
livre. Campo `explanationsEnabled` no status público não expõe chave/modelo privado.

Run única por conta/ator/request, reserva pré-rede e orçamento transacional.
Mudança de config/permissão/fontes durante a chamada descarta resposta; a run
continua registrada com uso quando conhecido. Histórico/replay revalidam referências
em lote e podem retornar403/409 após revogação, preservando os dados originais.
Consultas determinísticas continuam sem dependência da IA. Smoke real só autorizado.

## CT-08 — artefatos e adapter NFS-e (parcial)

GET `/v1/accounting/issuers/{issuer}/executions/{id}/artifacts/{artifact}` retorna
o original binário após RBAC accounting.documents.view, grant view e escopo de
conta/emitente/execution/artefato. Hash/tamanho conferidos e acesso revalidado após
I/O. Attachment, Cache-Control private,no-store e nosniff. Recurso alheio404, leitor
indisponível503, original inválido502; não retornar objeto/key privada de storage.
Sem confirmação extra de leitura, sem substituir os endpoints de status/histórico.

Connector `nfse.sefin_nacional.v1` implementa a porta CT-03, mas **não é registrado
em runtime**. Env homologation/production fechados; fake_local só helpers de teste.
Config/manifest/cofre/schema/assinatura obrigatórios por portas, sem API paralela
de credencial nem uso de cookies pessoais. Não há DTO de chat que habilite produção.
DPSID precisa ser durável e vinculado ao conteúdo revisado antes do POST; não deriva
sequência de recibo/UUID. Config/profile/content versions e hash são rechecados.

XML recebido só autoriza após validação/chave/identidade/documento coerentes.
Timeout/404/200vazio/5xx permanecem unknown, nunca POST cego. Recibo técnico local
não é protocolo fiscal; PDF não implementado. Qualificação formal do wire/schema e
conector do piloto permanece pendente: ver NFSE_COBERTURA.md e handoff CT-08.

## Aditivo compatível 1.1 — obrigações CT-11

Rotas sob `/v1/accounting/issuers/{issuer}/obligations`, conta/ator do Principal.
Permissões novas: accounting.obligations.view/manage/review com grants view/manage/
review do emitente. Nenhuma concede issue, pagamento ou transmissão. JSON64KiB,
idempotência ASCII8–200 e CAS seguem o owner. IDs alheios404.

- GET lista: period opcional, page/limit20/100. GET /{id}: snapshot/memória atual.
- GET /{id}/history: eventos paginados com versão/ator/operação/snapshot.
- POST raiz: code, period, expectedVersion, base decimal string ou null. Criação
  expectedVersion0; nova avaliação exige versão atual e estado não revisado.
- POST /{id}/review: expectedVersion, confirmed:true. Exige cálculo qualificado.
- POST /{id}/reopen: expectedVersion, confirmed:true, evidenceRef(motivo), <=500.
- POST /{id}/prepare-finance: expectedVersion, confirmed:true, sheetId,
  expectedSheetUpdatedAt, existingLineId opcional. Exige Finance manage também.
- POST /{id}/report-payment: expectedVersion, confirmed:true, paymentDate,
  evidenceRef. Apenas informação manual; sem conciliação/banco.

Estrutura/limites e códigos em OBRIGACOES_COBERTURA.md. Runtime não injeta
ObligationQualification: cálculo real bloqueado, valor/data=null. Regras só são
sintéticas nos testes. Nenhuma mudança na versão/semântica dos envelopes anteriores.
Tools novas: accounting.obligations.read.v1 e accounting.obligation.finance.prepare.v1.

## Aditivo compatível 1.2 — ingestão Knowledge CT-12

`/v1/accounting/issuers/{issuer}/knowledge/ingestion` usa Principal, RBAC e grant
knowledge.view/manage existentes. GET /catalog e /subscriptions retornam listas;
GET /runs usa page/limit20/100, items/status/checkpoint histórico, sem original.
PUT /subscriptions exige Idempotency-Key, baseId, sourceId, expectedVersion,
enabled e intervalHours6–168; criação expectedVersion0, mudanças CAS. Conta/ator/
URL/headers/provider não são campos aceitos. Sem body de publicação ou permissão.

Catálogo é fechado no servidor; configuração da subscrição persiste por emitente.
Base ativa para habilitar, máximo20por emitente. Alteração cancela runs antigas;
mesma key/body replay, divergência409. IDs/recursos alheios404, formato inválido400,
fonte não permitida422, dependência indisponível503. Lista de runs informa
publicationAutomatic:false/retrieval:fts_simple. Nenhum endpoint ativa embeddings,
paga serviço externo, confirma regra ou publica texto por agendamento.
