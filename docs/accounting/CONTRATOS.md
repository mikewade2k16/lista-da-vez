# Contábil + Crow Plus — contratos v1

## Prioridade atual — 2026-11-04

O usuário confirmou como foco central **emissão de notas de serviços pelo chat,
reaproveitando emitente, clientes, serviços e valores recorrentes, perguntando só
as lacunas**. Comprovantes externos, despesas e orientação fiscal via RAG também
fazem parte do produto. Produtos/catálogo/NF-e de mercadorias (CT-09/M3) continuam
adiados. Ver [escopo confirmado](ESCOPO_INICIAL.md),
[aceite CT-10](ACEITE_PILOTO.md) e [handoff CT-10](handoffs/CT-10.md).
O usuário ainda não testou nem aprovou a funcionalidade (2026-11-05).
A base passou apenas nos testes automáticos locais executados; jornada conversacional completa,
ambiente do painel atualizado e emissão real ainda estão pendentes.

Versão: 1.0. Data: 2026-11-03. Estado: contrato documental congelado para implementação, **não implementado**. Substitui a proposta inicial deste arquivo, preservando suas restrições. Dependência CT-01 entregue em [Diagnóstico e cenários](DIAGNOSTICO_E_CENARIOS.md). Exemplos/portas: [CONTRATOS_API](CONTRATOS_API.md). Escritores e integração: [OWNERSHIP](OWNERSHIP.md).

Alteração incompatível de campo, permissão, estado ou semântica exige revisão coordenada e versão major nova. Extensão opcional compatível incrementa minor. Esta versão não habilita conector, emissão, cálculo tributário ou publicação. CT-03/06 podem implementar suas fundações independentes; integração continua condicionada aos respectivos aceites.

## 1. Inventário real e decisões de reutilização

| Implementado e inspecionado | Limite atual | Decisão v1 / executor |
|---|---|---|
| `platform/modules/module.go`: Registry/Metadata/Dependencies/PermissionDef | Catálogo não autoriza por emitente | CT-03 registra `accounting`, schema homônimo, IsCore false, OptionalModules finance, sem exigir Omnichannel habilitado |
| `finance/fiscal_model.go`, perfil/templates pessoais por conta+usuário | Não são emitente compartilhado | CT-03 usa porta Finance explícita para importação selecionada; não renomeia nem duplica perfil pessoal |
| `finance/document_prepare.go`, `document_review.go` | Preparação pessoal, revisão por hash/versões; emissão false | Preservar API/fluxo; nova execução empresarial usa snapshot derivado rastreável, jamais aprovação retroativa |
| `finance/assistant.go`, `app/finance_assistant.go` | Config/runtime próprios por conta; resolução pelo cofre Automation | CT-05 estende `finance.assistant_configs.data` com opções Plus; nenhuma segunda tabela de modelo/chave/instruções |
| `calendar/chat_store.go`, chat/receipts/proposals e `assistant_finance.go` | Conversas Finance pessoais, surface finance, histórico canônico em calendar.chat_* | CT-05 acrescenta modo Plus e contexto imutável pelo owner Calendar; preserva rotas e legados |
| `omnichannel/knowledge_model.go`, `service_knowledge.go`, `store_knowledge.go` | CRUD/busca exigem agente e omnichannel.agents.manage; FTS simple, não vetorial | CT-06 extrai serviço neutro mínimo e repositório canônico, preservando adapters Omnichannel |
| `omnichannel/ai_tool_registry.go`, gateway/approval/execution | Contratos incluem AgentID/DispatchID e governam atendimento | Reutilizar padrões de schema/limites/auditoria; não usar aprovação Omnichannel como autorização fiscal |
| `platform/jobs/{jobs,store,worker}.go` | Enqueue usa pool.QueryRow; não recebe pgx.Tx; dedupe atual retorna ID vazio se já existe | CT-03 adiciona método concreto EnqueueTx sem mudar Store; valida hash no domínio antes de enfileirar |
| `platform/llm/llm.go`: Client.Complete, schema/usage | Providers nativos openai/gemini/glm; não há suporte Anthropic neste contrato | CT-05 reutiliza Client; provider sem adapter retorna capability_unavailable, sem cliente LLM paralelo |
| Jobs com FIFO, SKIP LOCKED, liberação de presas | Liberação pode reexecutar efeito; MarkDone não prova posse atual | CT-03 exige claim/fencing próprio na execução fiscal, além do job; nenhuma promessa exactly-once remoto |

Leitura complementar realizada: AGENT raiz/módulos, portabilidade, AGENT Finance/Calendar e seção E6 do Omnichannel. Trechos históricos não substituem código atual. Migrations atuais chegam a 0309; isso é inventário, **não reserva** de 0310. Não houve consulta a banco vivo, cofre ou credenciais.

## 2. Domínio, acesso e configuração

`accounting` é domínio de emitentes/diagnóstico/documentos e execução fiscal. Handler → service → repository. Regras de produto ficam em módulo, não platform. A UI começa em Finance, com blocos Accounting gated; backend Accounting pode funcionar sem Finance e retorna capability indisponível apenas nas operações que dependem dele. Desabilitar Accounting nunca desabilita planilhas, perfil, templates, preparação ou chat Finance anteriores.

Contexto interno validado: accountID, actorID, issuerID opcional conforme operação, surface efetiva, correlationID e capabilities calculadas. Body/query escolhem recursos, nunca conta/ator/permissão. Toda leitura/escrita exige conta autenticada, módulo habilitado, RBAC na conta e vínculo ativo no emitente. SQL repete filtro de conta/emitente. Usuário da mesma conta sem vínculo ao emitente recebe 404; permissão insuficiente para recurso visível recebe 403. Não há bypass empresarial implícito para agência/admin.

Criação inicial de emitente exige `accounting.issuers.manage`, cria o vínculo administrador do criador na mesma transação; o responsável pode conceder vínculo a membro ativo da mesma conta. Permissão de conta é teto, não concessão automática a todo emitente. Grants de emitente são explícitos e limitados ao subconjunto `view|manage|review|issue|knowledge_view|knowledge_manage`; manage não inclui issue. Conceder issue exige issue efetivo do concedente. Bootstrap inicial é explícito: enableIssue na criação exige RBAC issue; sem emissor ativo, manage no emitente + RBAC issue + confirmed:true permitem o primeiro grant. Havendo emissor ativo, não há exceção. Revogação incrementa access_version e invalida capabilities; worker revalida ator/autorização antes de enviar.

| Operação | Permissão account (nova salvo Finance) | Grant emitente / condição |
|---|---|---|
| Listar/ver emitente | accounting.issuers.view | view |
| Criar/editar emitente, diagnóstico, vínculos | accounting.issuers.manage | manage; exceção criação inicial acima |
| Ver documentos/pendências | accounting.documents.view | view |
| Preparar/editar/importar | accounting.documents.manage | manage; importação exige também acesso pessoal Finance |
| Confirmar/revogar revisão | accounting.documents.review | review |
| Autorizar/revogar envio e executar | accounting.documents.issue | issue; snapshot revisado e capability do conector |
| Buscar evidência | accounting.knowledge.view | knowledge_view |
| Curar fonte/binding | accounting.knowledge.manage | knowledge_manage; não concede acesso a base alheia |
| Ver/alterar config IA Plus | finance.config.view / finance.config.manage | Conta; membership Finance, não dá acesso a emitente |
| Consultar planilha | finance.sheets.view | Escopo Finance; sem vínculo automático planilha→emitente |

Chaves accounting não existem no runtime atual; serão declaradas por CT-03/06 via PermissionDef Scope account, formato compatível com registry. CT-03 é escritor único do catálogo accounting, incorporando definições de CT-06. Role templates novos: accounting.viewer (leituras), accounting.operator (diagnóstico/preparação/revisão), accounting.issuer (adiciona issue), sem atualizar silenciosamente roles já clonadas. Obrigações futuras reservam accounting.obligations.view/manage, **não são capabilities M1**.

Config Plus usa exclusivamente `finance.assistant_configs`: mesmos enabled, credentialId, model, instructions, temperature, historyWindow; acrescentar `plus:{enabled:false,toolPolicyVersion:1}`. Enabled IA não condiciona consultas determinísticas; plus.enabled controla a entrada nova. Segredos permanecem no cofre atual, só referências públicas na configuração. Accounting não replica config de IA. Se Finance estiver desligado, o domínio Accounting continua determinístico pela própria API, sem abrir chat Finance.

## 3. Plus e continuidade de conversa

Política pequena em `finance/plus_service.go` (CT-05). Consome portas Accounting, Finance e conhecimento por composição. Calendar mantém transporte `/v1/assistant/chat/*`, armazenamento e receipts; não importa Accounting/Finance concretos. App injeta closures/adapters. Nova chamada usa surface finance e mode plus, com versão de contrato; modo legacy continua aceitando o payload/resposta atuais.

Uma conversa Plus tem dono pessoal, conta, mode plus, surface finance e issuerID opcional **imutáveis**. Contexto extra pertence a `calendar.chat_conversations` (colunas mode/issuer_id/contract_version); issuer_id é referência lógica validada por porta, sem dependência SQL Calendar→Accounting. financeSheetId é seleção por turno validada, não identidade jurídica. Todos os acessos pessoais continuam filtrando criador, inclusive admin. Mensagens mantêm autores e IDs originais; nenhum histórico Finance/Calendar/access_messages é fundido, copiado ou convertido automaticamente. Pedido de entrar em Plus cria conversa nova. Troca de emitente/conta cria outra conversa; retomada do mesmo escopo mantém a conversa. Mensagens Plus podem guardar envelope estruturado versionado em coluna própria, sem alterar content legado.

Calendar é dono dos receipts/turnos, chama uma porta Plus apenas depois de claim durável e autorização. Mesmo request em voo retorna 409 request_in_progress; terminado repete envelope salvo; hash diferente retorna 409 idempotency_conflict. Histórico IA usa só mensagens autorizadas da conversa atual; contexto/evidências são revalidados por turno. Não transportar credenciais nem resultados privados para outro emitente. UI cancela requests antigos e compara conta+ator+conversa+emitente antes de hidratar resposta/cache.

Tools M1 são allowlist: finance.sheet.read.v1 (read), accounting.diagnosis.read.v1 (read), accounting.diagnosis.save.v1 (prepare), accounting.document.prepare.v1 (prepare), knowledge.search.v1 (read). Revisão/autorização/envio são comandos explícitos de UI/API, não execução autônoma do modelo. Prepare só grava rascunho/diagnóstico após intenção explícita; não exige nova aprovação para cada leitura. Cada tool define schemas fechados, permissão, timeout e limite. Nenhuma tool de URL/SQL/shell/pagamento livre. LLM classifica/explica; resultado não altera contexto de segurança, cálculo ou estado de execução.

## 4. Persistência nova e integridade

Desenho lógico obrigatório, a ser convertido em migrations novas por CT responsável. Todos os recursos raiz possuem UUID id, account_id FK core.accounts, created_at/updated_at UTC. Tenant-scoped tem UNIQUE(account_id,id); filhos de emitente também UNIQUE(account_id,issuer_id,id), com FKs compostas para pai no mesmo escopo. Versão bigint inicia 1 e só cresce por CAS; não usar timestamp novo como versão accounting. IDs JSON strings. Exclusão lógica/revogação preserva histórico fiscal; não cascade apagar documento autorizado.

| Tabela / dono CT | Colunas essenciais além do padrão | Uniques, FKs e índices obrigatórios |
|---|---|---|
| accounting.issuers / 03 | status active/inactive, version, access_version | índice account/status/id; identidade documental no perfil versionado, não chave de autorização |
| accounting.issuer_members / 03 | issuer_id,user_id,grants,status,version | UNIQUE(account,issuer,user); FK emitente composta e core.users; índice account/user/status; membership de conta validada no service |
| accounting.issuer_profiles / 03 | issuer_id,version,data JSONB,valid_from,valid_to,created_by | UNIQUE(account,issuer,version); FK emitente; índice account/issuer/valid_from; versões imutáveis e vigência sem sobreposição ambígua, validada sob lock |
| accounting.diagnoses / 03 | issuer_id,version,intent,competence_date nullable,answers JSONB,pending JSONB,profile_version | FK profile composta quando selecionado; índice account/issuer/updated_at/id; answers contém origem/confirmação por campo |
| accounting.documents / 03 | issuer_id,kind nfse/nfe,state,version,diagnosis_id,profile_version,payload JSONB,amount NUMERIC(20,2) nullable,source_ref JSONB nullable | FKs escopadas; índice account/issuer/state/updated_at/id; check amount>=0, operação valida positivo; nenhuma nota autorizada editável |
| accounting.document_reviews / 03 | issuer_id,document_id,document_version,profile_version,snapshot JSONB,snapshot_hash,actor_id,revoked_at | FK documento; UNIQUE(account,issuer,document,hash,actor); índice document/created_at; snapshot só do servidor |
| accounting.submission_authorizations / 03 | issuer_id,document_id,review_id,actor_id,snapshot_hash,expires_at,revoked_at,consumed_at,version | FKs revisão/documento compostas; índice document/expiry; uma autorização ativa por documento sob lock; validade máxima 15min, sem renovação automática |
| accounting.executions / 03 | issuer_id,document_id,authorization_id,state,operation,environment,connector_id,request_hash,business_key,remote_ref nullable,lease_owner,lease_until,fence,version | UNIQUE(account,issuer,operation,environment,business_key); FK autorização; índice account/state/lease_until; comando imutável |
| accounting.execution_events / 03 | issuer_id,execution_id,sequence,event_code,actor_id nullable,metadata mínima | UNIQUE(account,issuer,execution,sequence); FK execução; append-only, sem payload/segredo em erro |
| accounting.artifacts / 03 | issuer_id,execution_id,storage_object_id,kind,sha256,size,protocol nullable | FK execução; UNIQUE(account,issuer,execution,kind,sha256); storage por porta, índice execution/kind |
| accounting.request_receipts / 03 | issuer_id,actor_id,operation,idempotency_key,request_hash,state,result JSONB,resource_id | UNIQUE(account,actor,issuer,operation,key); índice resource; sem TTL automático em comandos fiscais |
| accounting.outbox / 03 | Colunas do Store jobs: ordering_key,idempotency_key,kind,payload,status,attempts,max_attempts,run_after,locked_at,locked_by,last_error | UNIQUE(account,idempotency_key); índices de claim/FIFO equivalentes ao Store; payload só executionId; ordering_key issuer:document |
| accounting.knowledge_bindings / 06 | issuer_id,knowledge_base_id,is_enabled,version | UNIQUE(account,issuer,base); FK issuer e messaging.knowledge_bases(account,id); índice account/issuer/enabled |
| messaging.knowledge_document_applicability / 06 | document_id,version,authority,jurisdiction,classification,published_on,valid_from/valid_to nullable,validity_source,reviewed_by,reviewed_at,status,original_storage_id,checksum,version_lock | FK doc(account,id), UNIQUE(account,document,version); índice account/jurisdiction/status/valid_from; não duplica texto/chunks |

Dados JSON são tipados pelo contrato v1 e validados no service, não objetos arbitrários do modelo. Doc/profile versions referenciadas permanecem disponíveis. Colunas concretas e checks equivalentes podem ser desdobrados sem mudar semântica, com registro no handoff. Outbox é fila técnica, não sinônimo da execução fiscal. Artefato é referência de objeto privado, não duplicação do storage. Credencial empresarial usa porta do cofre com namespace accounting/issuer e referência em configuração de conector do perfil; não copiar tabelas de senhas Finance.

Origem pessoal em documento empresarial: source_ref contém owner finance, draftId, actorId e versões do draft/perfil/template selecionados. Importação ocorre por porta pública do owner com consentimento explícito e verificações de conta/usuário. Só campos selecionados são copiados para snapshot empresarial, com destino/visibilidade mostrados ao usuário; não criar segunda coleção de rascunhos pessoais. Atualização da origem marca vínculo desatualizado e exige reimportação explícita antes de nova revisão, nunca sync silencioso. Preparação pessoal anterior continua na API Finance sem criar documento accounting.

## 5. Conhecimento neutro com dois consumidores reais

CT-06 extrai `back/internal/modules/knowledge` como serviço/repositório compartilhado (sem novo módulo comercial obrigatório e sem regras fiscais em platform). Repositório passa a ser owner único de `messaging.knowledge_bases/documents/chunks`; schema físico e IDs permanecem, sem duplicar texto/índice. Omnichannel mantém adapter de permissão/bindings de agente e rotas existentes. Accounting valida módulo, vínculo e accounting.knowledge.* e usa bindings próprios por emitente. Nenhum consumidor acessa SQL privado do outro.

Consulta neutra recebe contexto autorizado por adapter interno, consumer fechado accounting/omnichannel, resourceID de emitente/agente e lista de bases resolvida no servidor. Revalida bindings/base/publicação/vigência dentro da busca antes do ranking, com conta explícita. Não aceitar lista de bases fornecida pelo LLM como concessão. Revogar/desabilitar binding/base invalida busca/cache. M1 usa somente bases da mesma account; corpus público curado é explícito por conta, sem compartilhamento cross-account ou promoção automática de material privado. Compartilhamento global exige evolução posterior, não é lacuna a contornar copiando documentos já existentes.

Documento publicado para Accounting exige original/hash, órgão/título/origem, jurisdição, classificação, publicação quando verificável, vigência com fonte, revisor e status reviewed. Publicação desconhecida pode ser null; vigência não verificada resulta insufficient_evidence, nunca regra vigente. Chunks publicados/referenciados tornam-se imutáveis; nova versão cria documento/versão preservando anteriores. Adapter Omnichannel deve impedir substituir chunks já usados como evidência; aceita edição de rascunho conforme fluxo anterior. Não estender obrigação fiscal a todos os documentos de atendimento.

Evidência: documentId/version/chunkId, trecho e localização, fonte, jurisdição, vigência e explicação de aplicabilidade. Score FTS é ranking, nunca confiança jurídica. A consulta usa competência, estabelecimento/regime confirmados e metadados revisados; não apenas maior versão. Conteúdo recuperado é dado não confiável: instrução nele não altera tools/permissões. CT-06 implementa armazenamento/recuperação; CT-07 produz explicação fundamentada com abstinência.

## 6. Motor fiscal único CT-03

| De → para | Condições e efeito |
|---|---|
| draft → validated | Campos/regras cobertos, versões e validações determinísticas; sem conector real pode validar estrutura, mas capability de emissão false |
| validated → reviewed | Confirmação explícita por review sobre snapshot/hash servidor vigente; registra revisão |
| reviewed → authorized_for_submission | Permissão issue + confirmação explícita + ambiente/conector cobertos + autorização curta vigente |
| authorized_for_submission → submitting | POST execute cria comando e outbox atomicamente; worker confere autorização, versões, access_version e capability antes de consumir autorização e enviar |
| submitting → authorized | Provedor comprovou autorização; persistir referência/protocolo e tarefa de recuperar artefatos |
| submitting → rejected | Rejeição definitiva identificável; guardar código sanitizado, sem tratar timeout como rejeição |
| submitting → unknown | Timeout/resposta ambígua, crash ou lease perdida após possível envio; consultar antes de qualquer retry |
| unknown → authorized/rejected | Reconciliação com resposta conclusiva do provedor |
| unknown → authorized_for_submission | Somente prova conclusiva de não envio, ausência de worker anterior ativo e nova autorização; execução anterior termina cancelled com motivo not_submitted e novo comando usa nova businessKey; sem prova permanece unknown |
| validated/reviewed/authorized_for_submission → draft | Edição relevante ou fonte/perfil obsoleto; invalidar revisões/autorização e não apagar histórico |
| rejected → draft | Correção explícita gera nova versão, revisão e autorização |

authorized é imutável; cancelamento/substituição são operações futuras separadas. Revogação antes do envio cancela job pendente/execução sem efeito; documento volta reviewed se snapshot segue válido, senão draft. Após início de submissão, revogar não promete interromper efeito remoto: registrar solicitação e reconciliar. Expiração depois do envio não apaga o resultado. Snapshot inclui documento, versões do perfil/diagnóstico/regras/evidências, operação, ambiente, conector/capability e hash SHA-256 de JSON canônico servidor; não inclui segredo.

Transação de execute: lock documento/autorização + receipt → CAS de versões/hash e revalidação → execution + evento + outbox via `EnqueueTx(ctx, pgx.Tx, jobs.NewJob)` → receipt com resultado → commit. Implementar método novo em PostgresStore, mantendo Enqueue/Store existentes; Enqueue usa o mesmo helper com pool. Rollback não deixa job nem autorização consumida. Nada de chamada remota dentro da transação. Autorização é consumida somente no claim fiscal pré-envio, que revalida o snapshot; não pode sustentar duas execuções distintas.

Job usa Worker existente sobre accounting.outbox, com execution claim/CAS + fence monotônico e lease. Commit de resultado exige fence atual. Lease vencida de execução submitting transforma retomada em reconciliação, nunca nova submissão. Handler não devolve timeout ambíguo para retry genérico de envio: persiste unknown e agenda consulta limitada. Worker anterior pode ainda estar remoto; consulta “não encontrado” isolada não é prova de não envio. Sem chave remota consultável persistida antes do POST, conector não oferece emissão. Fencing local não garante exactly-once remoto; conector precisa idempotência remota comprovada ou reconciliação sem reenvio automático.

Retry transitório comprovadamente pré-envio pode manter comando enquanto autorização/capability vigentes. Nova tentativa de negócio após rejeição requer novo snapshot e businessKey coerente com nova revisão. Mesma chave com payload diferente retorna conflito antes do enqueue; não confiar no ON CONFLICT DO NOTHING de jobs. Timeout de storage após autorização não reenvia nota: guarda resultado fiscal, marca artifacts_pending em execução e recupera pelo identificador persistido. Artefatos originais privados com hash/protocolo são gravados por porta Storage; UI diferencia autorizado com artefatos pendentes de resultado desconhecido.

## 7. Dinheiro, tempo e ausência

Novo contrato: dinheiro BRL string decimal sem expoente/sinal positivo, escala 2, até 18 inteiros (NUMERIC(20,2)); quantidade escala até 6 e até 12 inteiros; taxa escala até 6 e faixa conforme regra revisada. Domínio usa decimal exato, nunca float64; cálculo usa precisão da regra e arredondamento explícito HALF_UP apenas quando a regra implementada assim determinar. Sem regra de arredondamento coberta, cálculo indisponível. Adapter Finance limita precisão ao contrato existente sem reformar API; leitura convertida deve indicar origem, sem afirmar precisão fiscal que o dado legado não prova.

Data civil YYYY-MM-DD; competência de diagnóstico/documento usa competenceDate completa, período Finance YYYY-MM separado. Instantes RFC3339 UTC; fuso de negócio deve ser configurado/confirmado para exibição, não inferido do host. Vigência usa intervalo [validFrom,validTo), limites civis; null no fim significa aberto somente com evidência revisada. Null no início ou regime/jurisdição desconhecidos impede afirmação de aplicabilidade fiscal. Nulidade financeira não equivale a zero. Campo omitido em PATCH preserva; null remove somente se nullable; PUT novo exige representação completa. Novos objetos usam expectedVersion; recursos Finance continuam expectedUpdatedAt.

## 8. Decisões fechadas e pendências externas

Fechados: owner accounting; Plus em Finance; config IA única; transporte/histórico Calendar preservados; conhecimento neutro com schema físico existente; grants por emitente; revisão separada de issue; outbox transacional; timeout incerto sem reenvio; dinheiro decimal; dados pessoais importados explicitamente; contratos 1.0 e escritores únicos.

Pendências externas não bloqueiam fundações: emitente/jurisdição/regime do piloto, cobertura fiscal concreta, credenciamento, método de autenticação/certificado, leiautes/protocolos e homologação. CT-08/09 precisam confirmar isso para habilitar envio; fake local nunca vira nota autorizada real. Anthropic nativo, corpus global, pagamento, escrituração completa, Jeff/MCP/vetores ficam fora de M1. Nenhuma dessas pendências justifica defaults fiscais nem troca de credenciais/volumes.

## Aditivo 1.1 compatível — CT-11, 2026-11-05

Estrutura de obrigações MEI/não MEI autorizada pelo usuário, com regras a definir.
M1 segue sem aceite funcional. Novo contrato opcional por emitente: obligation
(code,competência,versão,perfil,estado,base/valor/data nullable,regra,pendências,
memória e trilha). Sem alteração dos envelopes v1.0 já usados. API adicional e
portas em CONTRATOS_API.md; matriz real em OBRIGACOES_COBERTURA.md. Nenhuma regra
ou taxa seedada/ativada; default seguro é ausência de cálculo, não imposto zero.

## Aditivo 1.2 compatível — CT-12, 2026-11-05

Ingestão opt-in por fonte permitida, subscrições/runs/receipts no owner Knowledge;
mesmo corpus/Storage, fila Jobs existente, checkpoints e revisão/publicação humana.
Detalhes em INGESTAO_E_BUSCA.md. FTS simple continua autoritativo; português foi
avaliado offline, pgvector/embeddings não adotados. Não muda envelopes anteriores,
permissões fiscais nem status de aceite do usuário. Nenhuma fonte foi ativada na conta.
