# AGENT - `back/internal/modules/finance`

## Escopo

Modulo de planilhas financeiras mensais por cliente: entradas/saidas, efetivacao,
ajustes (adjustments), contas fixas, categorias e recorrencias. Painel em
`/finance` (front na layer `web/layers/finance`).

Decisao AC-12 (2026-07-02): substitui o mock BFF Nitro (`web/server/*`, removido)
por back Go real. Contrato JSON identico ao mock (camelCase 1:1 com
`web/layers/finance/types/finances.ts`) — o front so trocou a origem (`$fetch`
para o BFF -> `createApiRequest` para a API Go). ADR: `docs/adr/0002-remove-bff-nitro-mock.md`.
Plano de referencia: `docs/finance/PLANO_MODULO_FINANCE.md`.

## Banco

Schema proprio: `finance`. Migration: `0187_finance_module.sql` (idempotente, sem
`-- +goose Down`). `account_id uuid not null` em TODAS as tabelas raiz (multi-tenant
dia 1). `coreTenantId` e filtro/rotulo do cliente DENTRO da account (`''` = escopo
padrao).

Tabelas:

- `finance.sheets` — planilha (account_id, core_tenant_id, title, period, status, notes, timestamps).
- `finance.lines` — linhas de entrada/saida (kind, amount, adjustment_amount, effective, effective_date, position). `effective_date date` nullable (`''` <-> NULL no contrato).
- `finance.line_adjustments` — ajustes pontuais de uma linha (amount, note, date, position).
- `finance.categories` / `finance.fixed_accounts` (+ `finance.fixed_account_members`) / `finance.recurring_entries` — config por escopo.
- `finance.config_state` — guarda o `updated_at` do payload de config (PK `(account_id, core_tenant_id)`).

Decisoes de schema (espelham o mock 1:1):

- `fixed_account_id`/`category_id` sao TEXT (nao FK): o front usa ids deterministicos
  de recorrencia (`finance-ids.ts`) e trata como snapshot textual.
- SEM unique de `lower(name)` em categories e SEM unique em recurring_entries: o
  PUT de config e full-replace e o autosave nao valida duplicata.

## Rotas (`/v1/finance/*`)

Todas sob `RequireAuthWithAccount`; gating de modulo por prefixo no Chain (`moduleGatingRules`
em `app.go`: `{Prefix: "/v1/finance", ModuleID: "finance"}`). `platform_admin` tem
bypass do guard. Conta obrigatória e validada por membership; handlers usam somente `Principal.AccountID`.

- `GET /v1/finance/sheets` -> `{status, data:[SheetListItem], meta}` (query `q`, `coreTenantId`, `period`, `page`, `limit` default 240 teto 500).
- `POST /v1/finance/sheets` -> `{status, data:SheetDetail}`.
- `GET /v1/finance/sheets/{id}` -> `SheetDetail` | 404 `sheet_not_found`.
- `PUT /v1/finance/sheets/{id}` -> `SheetDetail` (full-replace) | 404.
- `DELETE /v1/finance/sheets/{id}` -> `{status:"success"}` | 404.
- `PATCH /v1/finance/sheets/{id}/lines/{lineId}` -> `{sheetId,lineId,line,summary,preview,updatedAt}` | 404 `line_not_found`. Des-efetivar (`effective=false`) limpa `effectiveDate`.
- `GET /v1/finance/config?coreTenantId=` -> `ConfigData`.
- `PUT /v1/finance/config` -> `ConfigData` (full-replace).
- `GET /v1/finance/config/recurring-clients` -> `[RecurringClient]`.

## Permissoes (catalogo, no banco via SyncCatalog)

`finance.sheets.view|manage`, `finance.config.view|manage`, `finance.recurring.manage`
(Scope "account"). Templates: `finance.manager` (todas) e `finance.viewer`
(`sheets.view` + `config.view`).

Enforcement por request: membership + RBAC efetiva da conta. Leitura exige
`finance.sheets.view`/`finance.config.view`; mutações exigem as variantes `manage`.
Alterar recorrências também exige `finance.recurring.manage`. Platform admin
mantém acesso; proprietário usa `core.account.manage` resolvida na conta, sem
usar apenas o papel coarse como bypass de permissões de outra conta.

## Isolamento multi-tenant

- `accountID` SEMPRE do Principal validado pelo middleware, nunca diretamente do header/body.
- `coreTenantId` e filtro normalizado (trim+lower) DENTRO da account.
- Sheet/linha de outra account -> **404** (nao 403; nao vaza existencia).
- Defesa em profundidade: toda query filtra por `account_id`.
- `recurring-clients`: so `platform_admin` recebe a lista real (agencia
  acompanhando mensalidades); caller comum recebe `[]` (idem ao mock, zero
  vazamento cross-tenant).

## Arquivos

- `module.go` — registro, Metadata, Permissions, RoleTemplates, Build.
- `model.go` — structs + tags JSON camelCase (contrato do front).
- `service.go` — regras de sheets (normalizacao, summary, preview) portadas do mock.
- `service_config.go` — regras de config (split para respeitar ~450 linhas).
- `store_sheets.go` — pgx: List/Get/Create/Update(full-replace)/Delete/PatchLine.
- `store_config.go` — pgx: GetConfig/SaveConfig(full-replace)/ListRecurringClients.
- `http.go` — handlers `/v1/finance/*`.
- `errors.go` — sentinelas `ErrSheetNotFound`/`ErrLineNotFound`.

IDs: sempre `string` + cast `::uuid` no SQL (sem lib uuid externa). Id novo via
`coalesce(nullif($n,'')::uuid, gen_random_uuid())`; id invalido no service vira `''`.
Nullable (`effective_date`, `date`, `payment_due_day`) escaneado com ponteiro.

## Pendencias

- **Realtime/WebSocket**: nao ha broadcast; o front so re-busca sob demanda (fica
  para fase futura, como o mock).
- **Integracao `contacts`**: o modulo `contacts` nao existe ainda; `clientName` sai
  de `core.accounts.name` (lookup quando `core_tenant_id` e uuid de account). Nao
  ha `OptionalModules` declarado para evitar referencia morta.

## Correções de 2026-11-01

- Datas PostgreSQL são lidas com `pgtype.Date` e formatadas como `YYYY-MM-DD`.
- Composição usa índices estáveis no slice de contas fixas.
- Escritas vivem em `store_sheet_mutations.go`, com lock da planilha na transação.
- PUT aceita `expectedUpdatedAt` e retorna 409 `sheet_conflict` para versão velha;
  timestamps usam RFC3339Nano para preservar a precisão do banco.
- Planilha fechada não aceita PUT, PATCH de linha ou DELETE (409 `sheet_closed`).
- `PATCH /v1/finance/sheets/{id}/status` altera status/reabre explicitamente sem
  regravar linhas. Payload: `status` e `expectedUpdatedAt`.
- Nomes de contas em listagens não são resolvidos a partir de IDs externos ao
  account validado. Snapshots históricos não são sincronizados automaticamente.
- Regressão HTTP em `http_access_test.go`; testes PostgreSQL em
  `store_integration_test.go`, usando exclusivamente `FINANCE_TEST_DATABASE_URL`
  num banco descartável com nome `finance_test*`. Não apontar para banco real.

## Roteiro de QA

Funcionalidades, preparação, passos e resultados esperados: `docs/finance/ROTEIRO_TESTES.md`. Resultados históricos: `docs/finance/TESTES_FUNCIONAIS_FINANCE.md`.

## Perfil fiscal pessoal e templates (2026-11-02)

Cadastro autoritativo em `/perfil#fiscal`, com atalho na configuração Finance. Contrato separado do PUT de categorias/contas fixas; salvar um não regrava o outro. Dados pessoais escopados por `account_id` + `Principal.UserID` (inclusive admin), nunca por userId vindo do cliente. Acesso exige membership, módulo Finance e `finance.sheets.view`; autoatendimento altera apenas o próprio perfil/templates, não autoriza mutações em planilhas nem em perfis de terceiros.

- Migration0304: `finance.user_fiscal_profiles`, `finance.user_fiscal_credentials`, `finance.user_fiscal_templates`. FKs com cascade; um padrão por usuário/conta.
- GET/PUT `/v1/finance/fiscal/profile`; PUT/DELETE `/v1/finance/fiscal/credentials`; GET/POST `/v1/finance/fiscal/templates`; PUT/DELETE `/v1/finance/fiscal/templates/{id}`.
- Escritas/deleções existentes exigem `expectedUpdatedAt` (409 se desatualizado). Novo perfil/template usa versão vazia. Templates alheios retornam404. Payload estrito e limitado a32KiB.
- Credenciais de portal write-only, cifradas pelo `secretbox` compartilhado (`OMNI_SECRETS_KEY` existente); GET retorna apenas `credentialsSet` e timestamp. Não existe GET da senha. Salvar perfil preserva acesso; alterar URL invalida acesso anterior. Cofre indisponível →503 sem gravar plaintext. Nenhuma chamada de login ao portal.
- Formulários manuais com confirmação para descartar/remoção; troca de contexto invalida respostas antigas. Pinia `app/stores/fiscal-profile.ts` centraliza HTTP; `useFiscalProfile` mantém rascunhos. Nenhum segredo em localStorage ou conversa.
- Template tem descrição, códigos opcionais, observações, percentual e retenção opcionais/nulos. Nenhuma tributação é deduzida. Escolha de template mostra prévia com emitente do perfil; não gera NFS-e.
- Testes em `fiscal_*test.go` e `useFiscalProfile.test.ts`; roteiro em `docs/finance/PERFIL_FISCAL_E_TEMPLATES.md`. A etapa inicial não incluía chat; a integração textual está descrita abaixo. Extrato por imagem e emissão continuam pendentes.

## Chat Finance e acesso gov.br (2026-11-02)

- Surface `finance` usa o chat compartilhado `/v1/assistant/chat/*`, configuração própria em `finance.assistant_configs` (migration0305), GET/PUT `/v1/finance/assistant/config`. Leitura exige `finance.config.view`, escrita `finance.config.manage` e versão `expectedUpdatedAt` (409 em conflito).
- Modelo/chave resolvidos pelo cofre canônico sem herdar persona/modelo/configuração do Calendar. Configuração inicial desativada; chaves nunca entram no retorno público. O painel oferece edição do cofre somente a administradores transversais, mantendo a política existente.
- Contexto somente leitura: planilha selecionada da conta, perfil e templates do próprio usuário. Até50 linhas por seção/10 ajustes por linha e10 templates; totais originais preservados, truncamento sinalizado. Sem credenciais de portal no contexto.
- Conversas Finance pessoais inclusive para agência/admin; sem escopo amplo de clientes, sem acesso implícito ao Calendar, sem propostas de escrita/emissão.
- Perfil admite `portalLoginMethod` portal/govbr. Alterar método ou URL invalida credenciais antigas; govbr rejeita armazenamento de senha. Botão Emissor Nacional preenche o URL oficial e método govbr; link de entrada direciona ao portal, não realiza login automático.
- Status e roteiro: `docs/finance/CHAT_E_AUTOMACAO.md`. Chat textual não implementa leitura de imagem nem automação fiscal. Login gov.br/MFA é feito pelo usuário no portal. Nenhuma nota foi emitida em QA.

## Comprovante → rascunho (2026-11-02)

Migration0306 `finance.document_drafts`, conta+usuário da sessão. GET `/v1/finance/documents` (10recentes), POST `/extract` multipart imagem única, PUT `/{id}` com versão. Upload exige `finance.sheets.view`, autoatendimento pessoal; fora do escopo404 e conflito409. Hash deduplica sem sobrescrever revisões. OCR Tesseract local em pipe, JPEG/PNG8MB/24MP, timeout35s/2slots, não guarda imagem nem chama provedor. Valor/data/pagador/recebedor preenchem o rascunho; tomador e competência não são inferidos do pagamento. Templates pessoais podem ser aplicados e campos editados. Dados lidos (não credenciais) entram no contexto pessoal da IA. Não há emissão fiscal nesta etapa. Documentação e testes: `docs/finance/COMPROVANTES_E_RASCUNHOS.md`.

### PDF e extratos comuns

Upload aceita PDF/JPEG/PNG/TXT/CSV/OFX8MB, multipart `file` (legado `image` aceito). Formato validado pelo conteúdo; extensão só para textos. PDF: Poppler por stdin, até10páginas/90s/32ktexto; extração nativa com normalização geométrica de centavos elevados, fallback OCR por página digitalizada, sem arquivo temporário. PDF com senha de abertura ou cópia proibida rejeitado; leitura permitida de PDF criptografado sem senha não é bloqueada. UTF-8/Windows-1252 para texto; OFX exige formato. `document_pdf_test.go` cobre arquivos reais por env opt-in e casos sintéticos. Dockerfiles principal/Mac incluem Poppler. Não existe emissão automática ou importação de transações múltiplas nesta etapa.

### Revisão dos dados preparados

Migration0307 `finance.document_reviews` mantém confirmações/revogações com snapshot sem segredos, hash e versões do perfil/rascunho. GET `documents/{id}/review`, POST/DELETE `documents/{id}/review/approval`, sob membership/Finance e autoatendimento `finance.sheets.view`; conta/usuário da sessão, recurso alheio404 e versão velha409. Aprovação transacional trava o perfil pelo advisory lock compartilhado e o rascunho pela linha; hash fornecido só é comparado ao resumo do servidor. Mudanças invalidam o registro efetivo; histórico preservado, confirmação idempotente. UI lazy-load mostra pendências, bloqueia dados incompletos e confirma pelo helper canônico. `emissionAvailable:false`; confirmar revisão não autoriza emissão. Executor/portal autenticado continuam pendentes. Roteiro: `docs/finance/REVISAO_DO_RASCUNHO.md`.

### Identificação de partes do comprovante

`document_parties.go` reconhece titulares de cartão/estabelecimentos e aliases de partes com valores inline/multilinha. Metadados de CPF/CNPJ/cartão/NSU/banco encerram captura; não reconstruir documento mascarado nem promover CNPJ do banco a tomador. POST `documents/{id}/parties` reidentifica pelo OCR salvo com versão/escopo da sessão, preenchendo apenas nomes vazios; não sobrescreve correções e invalida revisão por mudança de versão. Extração segue OCR+regras locais, sem participação do chat IA. Deduplicação de uploads não reprocessa dados automaticamente. Testes em `document_parties_test.go` e `document_integration_test.go`.

Reenvio de hash existente agora aplica recuperação dos nomes vazios a partir do OCR salvo, preservando edições/ID e sem executar OCR novamente. Não retorna simplesmente o rascunho desatualizado. Coberto em `document_integration_test.go`.

### Documentos das partes do comprovante

`DocumentFields.payerDocument` e `payeeDocument` persistem no JSON do rascunho. A extração associa CPF/CNPJ completo às seções de pagador/recebedor; ignora máscaras e rodapé bancário. Reenvio e `/parties` completam também documentos vazios sem sobrescrever correções. Papéis fiscais continuam explícitos.

### OCR de imagem pequena e reenvio incompleto

Imagens com largura inferior a 600 px são ampliadas em memória por interpolação bilinear (até 4x, altura 10.000 e área 24 MP). Valores com símbolo monetário mal reconhecido só são recuperados em linhas explicitamente rotuladas Valor/Valor original/Valor final; divergências continuam sem seleção automática. Reenvio de imagem cuja extração salva não tem valor/data/partes refaz OCR, atualiza texto e preenche campos vazios, preservando correções e ID. Update otimista escopado e atômico no DocumentStore; não armazena original.

## Acesso conversacional sem IA — 2026-11-03

Migration0308: `finance.access_messages` (histórico pessoal com idempotência por request_id/role) e `finance.portal_access_checks` (último teste por conta/usuário, vinculado à versão fiscal). GET/POST `/v1/finance/conversation` requerem membership e finance.sheets.view. Intenções delimitadas: cadastrar/método/testar acesso, perfil e comprovantes. Escolha do método usa o mesmo FiscalService; não cria cadastro paralelo. Senhas mencionadas no texto não entram no histórico; captura real pelo endpoint write-only de credentials, nunca IA.

GET/POST `/v1/finance/fiscal/portal-check`: consulta/teste de login próprio do Emissor Nacional. POST exige expectedUpdatedAt, cofre e credencial salvos; envia GET/POST ao formulário web oficial com CSRF/cookie, destinos fixos, redirects restritos ao host oficial HTTPS, timeout20s e corpos1MiB. Só informa authenticated com link de saída e ausência de formulário de senha. Não emite NFS-e, não guarda cookies/sessão e não abre sessão no Chrome. CAPTCHA/gov.br/MFA requerem etapa manual; não são aprovados automaticamente. Último teste é persistido sem credenciais; mudança de perfil invalida resultado anterior. Limite2 testes simultâneos e intervalo10s para a mesma versão.

Roteiro e limites em docs/finance/ACESSO_CONVERSACIONAL.md. Login real depende das credenciais do usuário; somente testes sintéticos autorizados nesta implementação.

Classificação do portal (2026-11-03): bibliotecas/scripts que mencionam captcha não significam desafio ativo. Página 200 com link de saída e sem formulário de senha confirma a autenticação; classificação manual deve examinar conteúdo de desafio, excluindo scripts/estilos. Regressão em portal_probe_test.go, diagnóstico real em docs/finance/ACESSO_CONVERSACIONAL.md.

## Sessão persistente e preparação — 2026-11-03

Migration0309 cria finance.portal_sessions (cookies cifrados, vínculo à revisão da credencial, validade30min, verificação) e draft_id no histórico pessoal. GET/POST /v1/finance/fiscal/portal-session retorna somente estado/horários; POST exige expectedUpdatedAt e verifica via GET antes de reutilizar, renovando com o acesso canônico quando expirado. Trocar/remover credenciais ou portal invalida a sessão atomicamente. Alterar dados fiscais preserva a sessão, mas invalida a revisão da nota. Nenhum cookie/chave chega ao front/IA/logs.

GET /v1/finance/documents/{id} e POST /{id}/prepare são pessoais e filtrados por conta/usuário. Preparar aplica somente template padrão quando o serviço inteiro está vazio, retorna revisão autoritativa/pendências e estado da sessão, sem submeter formulário fiscal. Chat /conversation aceita draftId validado; mantém contexto no banco e aceita campos explícitos Tomador, CPF/CNPJ do tomador, Competência, Serviço, Valor e Template. Não infere tomador ou competência do recibo. Preparação/revisão não emitem nem autorizam emissão. Docs: docs/finance/SESSAO_E_PREPARACAO.md.

## Porta pessoal para Accounting — CT-03

`accounting_source.go` projeta apenas campos explicitamente selecionados de serviço
(descrição/códigos), com consentimento de compartilhamento empresarial e versões de
perfil/rascunho/template. Conta/usuário são do contexto autenticado; outro usuário,
mesmo admin, não lê a fonte pessoal. Não copia credenciais, sessão, OCR bruto ou
alíquotas float para o domínio fiscal decimal. O adapter da composição valida
membership, Finance habilitado e `finance.sheets.view`. API Finance não muda.
Teste PostgreSQL: `TestAccountingSourceExplicitSharingAndOwnerIsolation`.

## Crow Assistant Plus — CT-05 (2026-11-04)

`plus_*.go`: registry fechado/versionado, roteamento determinístico, uma tool por
turno, input12KiB/output32KiB/timeout10s. Consultas usam Service Finance;
Accounting/conhecimento por portas e composição em app. Preparação pessoal
reutiliza AccessConversation, sem cópia de perfil/cofre. Nenhuma tool emite nota.
Config existente contém `plus.enabled` (default false) e toolPolicyVersion1,
independente de IA. GET `/v1/finance/assistant/plus/status` projeta flag e catálogo
permitido sem credenciais/modelo/instruções. Validação viva por operação.
Sem vencimento individual, não inferir effectiveDate como dueDate. Dinheiro legado
projetado em decimal string com precisão de origem identificada.
Handoff e validação: docs/accounting/handoffs/CT-05.md.

## Porta de conhecimento — CT-06

Plus knowledge.search agora recebe serviço neutro por composição, preservando
política Accounting e evidências no envelope. Prefixo de comando não entra no FTS.
knowledge_extract expõe reutilização limitada do extrator PDF, sem OCR/IA para
conhecimento. Fixtures de teste verificam PDF malicioso tratado como dado, inválido,
limite de páginas e PDF digitalizado recusado. Não muda preparação financeira.

## Respostas fundamentadas — CT-07

plus_evidence.go oferece somente source_summary: “resuma fonte termos” ou
“explique fonte termos”. Usa retrieval autorizado, contexto mínimo sem histórico,
PII/segredos mascarados, fonte como dado não confiável e resposta JSON validada.
Sem evidência suficiente/conflito/contexto fiscal confirmado, abstém-se de conclusão
tributária; números/URLs/citações inventados e alegações de execução são recusados.
Modelo não recebe tools e proposals permanece vazio. Não substitui cálculos.

Config única finance.assistant_configs.data.plus.explanation: enabled (kill switch
próprio), credentialId/model/taskClass, maxInputBytes/maxOutputTokens/timeoutMs,
dailyRequests e maxRunCostUsd opcional. Não altera config/modelo do Crow anterior.
Resolver/cofre existente, providers nativos openai/gemini/glm; Anthropic indisponível
neste adapter. Modelo exato é autorizado por configuração, sem fallback automático.

Migration0313: finance.plus_llm_runs, ledger account/actor/issuer/request com versão
de prompt/schema/política/config/modelo e referências de evidência; sem prompt,
resposta ou segredo. Reserva durável antes da rede, lock de orçamento diário UTC
da conta e request único. Incerto não repete automaticamente. Tokens/custo ausentes
são null; preços declarados com versão/fonte/vigência exclusiva e decimal exato USD
por milhão. Custo é estimativa pela tabela, não fatura. Teto pré-chamada usa limite
conservador de entrada por bytes+overhead e saída máxima, não cotação do provedor.

Retenção: ledger sem payload expira em30dias, remoção lazy na próxima reserva da
mesma conta; não promete remoção física instantânea no dia30. Conteúdo de mensagem
permanece somente no histórico pessoal Calendar existente, com exclusão lógica
existente (sem TTL novo). Falha de persistência deixa pedido em quarentena; não
refazer chamada paga para tentar preencher metadados.

Revalida config/acesso/fontes após LLM; resultado obsoleto é descartado.
ValidateOutput revalida fontes de replay/histórico em lote. Logs existentes não
recebem prompt/chave. Testes/avaliação e limites: handoffs/CT-07.md.

## Vínculo de obrigações CT-11 — 2026-11-05

ObligationEntryInput/PrepareObligationTx fornecem porta estreita ao Service/Store
canônicos, com transação recebida do comando Accounting. Host exige membership,
Finance habilitado e finance.sheets.manage. Não expor o método como HTTP sem policy.
finance.obligation_links guarda proveniência; migration0314 coordenada, sem dados.
Sheet lock/CAS/período/status, valor decimal string para NUMERIC(14,2), linha saída
não efetivada; replay não duplica, manual exige seleção, alterada/apagada conflita.
Não usa float para valor fiscal. Mantém formato/funcionalidades Finance anteriores.
Efetivação manual nunca vira conciliação bancária ou pagamento confirmado na obrigação.
Plus adiciona leitura de obrigações e preparação estruturada, sem pagamento ou IA
calculando imposto. Regras reais indisponíveis no runtime; ver cobertura Accounting.

## CT-13 — harness isolado (2026-11-05)

plus_routing_experiment_test.go avalia routePlus com dataset sintético português em
docs/accounting/experiments. Não executa tools/SQL/modelos nem muda roteador default.
Baseline36/40; quatro pedidos compostos precisam de esclarecimento (P15-36).
Metadados/licenças e decisões em experiments/CT-13-DECISOES.md; inferência Jeff/LLM
real não executada. Fallback de falhas é simulação do harness, não adapter instalado.

## Chat progressivo — 2026-11-06

FinanceConversation inicia somente com mensagens, sem reabrir formulário antigo
a partir do histórico. Formulários de perfil, acesso e documento continuam disponíveis
por pedido explícito; “criar nota” conduz perguntas sequenciais no chat. Respostas
comuns após uma conversa de acesso não são ocultadas como credenciais. Senhas
explícitas continuam bloqueadas/omitidas e devem usar o formulário protegido.

Estado da pergunta é a action da mensagem assistente no histórico canônico
(ask:<campo>/issuer:<campo>), vinculado ao draftId, conta e usuário. Dados são
gravados nas APIs/services fiscais/documentais existentes, sem tabela ou migração
nova. Pode iniciar rascunho sem comprovante; chave de criação derivada do request
evita duplicar no retry. Pergunta um campo faltante por vez, reaproveita dados
do rascunho e template padrão. Não consulta cadastro recorrente empresarial nem
conclui a emissão real. Preparação conversacional não tenta login no portal.

“Cancelar” interrompe perguntas e preserva dados salvos. “Abrir formulário da nota”
continua oferecendo o editor/revisão completo sob solicitação. Histórico antigo
que já foi salvo com dados omitidos não pode ser reconstruído por esta correção.

## Crow Assistant Plus — entrada unificada Finance (2026-11-06)

A conversa principal usa finance.access_messages para respostas locais e IA, com
requestId idempotente e dados pessoais por conta/usuário. Não há seletor com/sem IA.
ConversationOrchestrator escolhe preparação/perfil/acesso locais, leitura Finance
canônica ou resposta opcional via cliente LLM compartilhado. Sem IA/configuração
válida, limite ou falha de provedor, continua com resposta local; não simula uma
resposta do modelo. Consulta financeira não depende da flag de ferramentas avançadas.

POST /v1/finance/conversation aceita financeSheetId/month opcionais; contexto vem
da planilha selecionada da mesma conta e recurso é validado no serviço Finance.
Novas respostas IA usam somente histórico pessoal limitado/mascarado, configuração
Finance e cofre existentes; não herdam chave global, não executam ferramentas nem
confirmam emissão. Revalida configuração/permissão após chamada. Sem retry interno.
Limites de contexto/tokens/tempo/diário e teto monetário são os mesmos configurados
no bloco de limites Plus. Pricing só é reutilizado para a mesma credencial/modelo.

Migration0316 permite issuer_id=null no ledger finance.plus_llm_runs para classe
conversation; nenhuma tabela de histórico nova. Reserva é durável antes da rede;
retentativa com reserva anterior incerta não faz nova chamada. Histórico bem-sucedido
replaya pela mensagem existente. Retenção do ledger continua30dias; não promete
recuperar resposta não gravada após crash nem deduplicação eterna de reserva expirada.

UI Finance passa a Crow Assistant Plus; conversa principal sem ficha inicial.
Empresarial mantém contexto próprio com emitente explícito, ferramentas/RAG existentes
e cadastros/revisão abertos por solicitação. Não unificar contas/emitentes por página.
Históricos anteriores IA/Plus permanecem consultáveis em leitura; sem copiar mensagens
entre tabelas. Unificação integral de históricos antigos/empresarial e expansão de
Calendar não estão concluídas. Áudio e ditado reutilizam composables existentes;
transcrição não envia a mensagem automaticamente. Teste visual/voz real pendente.

Detalhes, evidência e roteiro em docs/finance/PLUS_UNIFICADO.md. Aceite do usuário
continua pendente no CT-15; nenhuma conta/configuração/chave foi ativada por esta tarefa.

## Autorização Finance por conta — 2026-11-06

`ResolveAccountPrincipal` revalida visibilidade da conta, usuário ativo, papéis e
RBAC efetiva no owner Auth. O adapter Finance usa a política canônica
`CanFinancePermission`: admin transversal e owner com `core.account.manage`
seguem o mesmo contrato das rotas Finance; custom precisa da permissão efetiva.
Módulo Finance habilitado continua obrigatório neste adapter. Accounting
mantém membership e grants de negócio explícitos. Teste PostgreSQL descartável
em `finance_scope_test.go` cobre permissões, revogação e módulo desabilitado.
O cliente selecionado no chat é enviado em header, validado no middleware e
revalidado no serviço; não muda a conta global da página.

## Busca pública no chat — pacotes 1/2, 2026-11-06

0317 cria `knowledge.public_sources`: acervo global exclusivamente público,
curado e informativo; não possui tenant porque não recebe uploads ou dados privados.
Não copia nem abre `messaging.knowledge_*`/bindings Accounting. Sem endpoint de
publicação pública, sem scraping no chat. Versões publicadas imutáveis (trigger),
retirada por status; nova busca seleciona versão publicada mais recente. Referências
históricas publicadas permanecem válidas; retiradas são ocultadas em lote no GET.
0318 adiciona cinco sínteses curtas com origem/revisão/limites em manifesto, sem
alterar configurações, contas, credenciais ou seeds anteriores. Vigência fiscal
não confirmada; são evidências informativas, nunca regra de cálculo do emitente.

SearchPublic → Store, FTS simple com normalização de acentos, até12termos e3trechos.
Aliases de vocabulário no roteador não contêm valores fiscais. Finance usa portas
PublicKnowledge/ValidatePublicEvidence, autorizadas pelo Finance canônico antes/
depois. Não exige Accounting contratado, emitente, modelo, cofre, Jeff ou API paga
para ler material público. Corpus privado empresarial conserva todos os gates.

finance.access_messages.public_evidence persiste snapshot estruturado separado do
texto; List revalida até180IDs e oculta fontes retiradas/indisponíveis. Conta/usuário
continuam filtrados no SQL. Perguntas fiscais explícitas no chat chamam busca antes
de qualquer modelo. Falta de fonte difere de falta de serviço. Nenhuma resposta é
marcada como confirmação jurídica ou execução de nota/declaração.

Frontend reutiliza AccountingEvidence e cartões/mensagens do Crow; mostra título,
trecho, fonte HTTPS e versão. Roteiro manual em docs/finance/ROTEIRO_TESTES.md.
Testes PostgreSQL sintéticos, corpus público inicial e regressões de conversa
passaram. Comparação visual, microfone real e aceite do usuário não executados.

## Intenções e continuidade da conversa — pacote 3, 2026-11-06

`conversation_intent.go` normaliza o texto (acentos, pontuação, cortesias como
"Crow, você pode...") e reescreve somente comandos reconhecidos para as frases
canônicas de `accessReply`/`routePlus`; texto não reconhecido segue intacto. Não
infere cliente, valor, regime, período nem autorização. Tipos: command, cancel,
resume e multiple. `conversationCommand` usa essa camada, então comando nunca é
gravado como resposta da pergunta pendente; consulta lateral responde e mantém
`ask:`/`issuer:` com aviso. "continuar" recalcula a lacuna pelo rascunho salvo.
Pedidos compostos geram esclarecimento, também em `routePlus` (`multiple_intents`).
No Plus, a preparação continua exigindo a frase exata; só leituras são reescritas.
Mês por extenso/`mm/aaaa` vira `aaaa-mm`; mês sem ano usa o ano do mês do chat.
Regressão: `conversation_intent_test.go` + docs/accounting/experiments/CT-15-P3-regressao.json;
integração `TestConversationContinuityAcrossSideRequests`. Registro em CT-15.

## Clientes recorrentes no chat — pacote 4, 2026-11-06

Migration0319 `finance.fiscal_customers` + `fiscal_customer_versions` (append-only),
escopo conta+usuário como perfil/templates. `fiscal_customers.go` (store, lock do
escopo fiscal, versão nova só quando nome/serviço/template/valor mudam) e
`access_customers.go` (conversa). Pedido "nota de 500 pro João" preenche só campos
vazios com o que foi escrito. Nome com um cadastro → reutiliza CPF/CNPJ, template,
serviço e valor e informa o que reutilizou; homônimos → `pick:Tomador` (número ou
CPF/CNPJ); CPF/CNPJ salvo → completa serviço/valor vazios. Nota completa salva/atualiza
o cliente. Novo pedido após nota completa abre outro rascunho (exceto draftId explícito).
GET `/v1/finance/fiscal/customers` e `/{id}/versions` (finance.sheets.view, sessão).
Sem UI de listagem ainda. Teste: `fiscal_customers_test.go`.

## Planilha do chat independente da página — pacote 5, 2026-11-06

`conversation_sheet.go`: leitura financeira do chat resolve a planilha sempre na conta
do chat (X-Account-Id validado): período escrito > planilha escolhida no chat
(`sheet:<id>` na última resposta) > planilha da página da mesma conta > pergunta.
Vários sheets do período → `sheetpick:<período>` com escolha numerada; nenhum → aviso.
Planilha de outra conta enviada pela página é ignorada (GetSheet por conta). O mês da
página não sobrepõe a planilha escolhida no chat. Nada altera a seleção da página.
`ReplyWithAction` devolve o estado persistido; campos ChatSheetID/ChatAction vêm do
histórico no servidor (`json:"-"`), nunca do body. Teste: `conversation_sheet_test.go`.

## Comprovante → lançamento Finance — pacote 6, 2026-11-06

Migration0320 `finance.receipt_links`. `document_receipt_link.go`: LinkReceipt exige
confirmed, versão do documento e da planilha (CAS), planilha aberta da conta; replay
igual não duplica; comprovante já vinculado → 409; linha existente só com mesmo tipo e
valor e sem outro comprovante; nova linha efetivada na data de pagamento. GET revalida
(amount_changed/line_removed); DELETE remove só o vínculo. Rotas
`/v1/finance/documents/{id}/receipt-link`: GET view, POST/DELETE finance.sheets.manage.
Chat (`access_receipt.go`): “lançar comprovante” propõe planilha (pacote 5), tipo/escopo
(padrão saída/pessoal, alterável por “empresarial”, “entrada”...) e reutiliza linha de
mesmo valor; grava só após “sim”, com nova checagem finance.sheets.manage. Comprovante
lançado como saída não vira fonte de nota; comprovante de entrada mantém o fluxo de nota.
Originais continuam processados em memória e descartados (não há cópia privada).
Não confundir `fiscal_customers` com `config/recurring-clients` (cobrança da agência,
platform admin, sem CPF/CNPJ).

## Originais, retenção, comprovantes e clientes — CT-15 fase B, 2026-11-09

0322: finance.document_originals (1 por documento: storage_ref `r2:`/`disk:`, nome, MIME,
tamanho, SHA-256, status stored|not_stored|purged|removed) e finance.retention_settings
(por conta, CAS por versão) + retention_setting_events (append-only). Padrão 90 dias
(3 meses, escolhido pelo usuário) enquanto a conta não grava política.
Extract guarda o arquivo recebido byte a byte via porta OriginalFiles (app:
privateOriginals = R2 canônico ou PRIVATE_FILES_DIR fora de /uploads). Falha do storage
não bloqueia a extração: registra not_stored e novo envio tenta de novo.
Expurgo: goroutine horária (cancelada no Close) reserva linhas vencidas com lease de 10min
(`skip locked`), remove o binário e marca purged; falha soma tentativa com backoff até 24h.
O prazo vale também para originais já guardados (expira = guardado em + dias atuais).
**Dados extraídos, vínculos e históricos não expiram.**
Rotas: GET /v1/finance/receipts (lista paginada: documento, vínculo revalidado, original,
uma consulta), GET/DELETE /v1/finance/documents/{id}/original (download privado,
no-store; apagar agora exige finance.sheets.manage), GET/PUT /v1/finance/retention e
GET /retention/history (view/manage = finance.config.*).
PUT /v1/finance/fiscal/customers/{id}: edição manual (nome, serviço, valor) com CAS pela
versão e nova versão imutável marcada editedManually; CPF/CNPJ não muda.
Testes: retention_test.go (memória para storage), fixture inclui 0322/0323.
