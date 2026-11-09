# CT-10 — aceite do piloto por capacidade

Data: 2026-11-04, America/Maceio. Estado: **parcial**. Referência de código:
HEAD `5d6ef3b` + working tree local CT-03 a CT-08 e CT-10, preservando alterações
preexistentes. Não corresponde a versão publicada nem a checkout limpo.
Evidência agregada: [evaluations/CT-10.json](evaluations/CT-10.json).

## Complementação técnica — 2026-11-05

CT-10 continua **parcial, sem aprovação do usuário**. A rodada abaixo complementa
as evidências anteriores; os resultados históricos de 2026-11-04 foram preservados.
Relatório: [complemento JSON](evaluations/CT-10-complemento-2026-11-05.json).

| Verificação antes pendente | Execução e resultado atual | Limite |
|---|---|---|
| Migrations completas | TestAllMigrationsApply passou em PostgreSQL16 novo, SkipDataSeeds=true | Banco descartável; não mexeu no banco do painel |
| Proposals Calendar reais | TestChatProposalExecutionPostgresIdempotencyAndSurfaceRevocation passou sem skip no banco integralmente migrado | Replay/efeito único e revogação por superfície; não envolve emissão |
| PDF com texto e digitalizado | TestPDFTextAndScannedFallback passou com Poppler real | Documento sintético; OCR é callback controlado |
| PDF como fonte hostil | TestKnowledgePDFUsesBoundedExtractorAndTreatsInstructionsAsData passou | Extração real; conteúdo não concede execução ou permissão |
| Validação XSD offline | TestOfflineSchemaValidatorWithSyntheticSchema passou com xmllint real | Schema sintético; contrato fiscal oficial permanece pendente |
| Cofre | Pacote secretbox passou com -race | Chaves de teste, sem segredo real ou cofre empresarial fiscal qualificado |
| Interface/autosave | 89 testes/11arquivos Vitest passaram | Inclui4casos de fila de autosave; não é teste visual |

Os5casos Go nomeados acima passaram com -race/-count=1. A fixture completa de
migrations foi usada antes da regressão Calendar, em container PostgreSQL exclusivo,
sem volumes nem portas públicas. PDF/XSD rodaram com ferramentas instaladas apenas
no container efêmero de teste. Nenhum build, reinício ou mudança do painel foi
necessário nesta complementação.

Dos11testes pulados na rodada histórica,4foram agora executados e passaram.
Restam7testes dependentes de arquivos privados OCR/PDF ou R2 live, não executados;
a lista está no complemento JSON. Não obter credenciais/arquivos reais apenas para
eliminar skips. Visual/manual, modelo real e homologação continuam não executados.
Não há falha pendente nas verificações executadas nesta rodada.

Para repetir, usar as URLs do quadro de bancos abaixo e executar, sequencialmente,
no banco descartável **completo** (não no banco mínimo Jobs):

```sh
go test -race -count=1 ./internal/platform/database -run '^TestAllMigrationsApply$'
go test -race -count=1 ./internal/modules/calendar -run '^TestChatProposalExecutionPostgresIdempotencyAndSurfaceRevocation$'
```

TEST_DATABASE_URL deve apontar exclusivamente para accounting_test_ct10_full nessa
sequência. Não usar qualquer URL do banco do painel. Para parsers, instalar Poppler
e xmllint no runner temporário e rodar:

```sh
go test -race -p 1 -count=1 ./internal/modules/finance ./internal/modules/accounting/connectors/nfse -run 'TestPDFTextAndScannedFallback|TestKnowledgePDFUsesBoundedExtractorAndTreatsInstructionsAsData|TestOfflineSchemaValidatorWithSyntheticSchema'
go test -race -count=1 ./internal/platform/secretbox
```

O roteiro manual da próxima seção continua aguardando o usuário. Não assumir aceite
M1 nem iniciar emissão real por causa desses resultados técnicos.

## Retomada para teste do usuário — 2026-11-05

**Aceite do usuário: NÃO REALIZADO. Nenhuma funcionalidade foi aprovada pelo
usuário.** Resultados automáticos de 2026-11-04 abaixo são evidências técnicas
históricas, não aprovação funcional ou autorização de produção.

Após o usuário iniciar Docker, verificação sem navegador: Finance respondeu200,
componente FinanceAssistantSurface presente no container web, Accounting respondeu
404 tanto pelo proxy quanto diretamente na API. CORE_V2_ENABLED=true: o problema
não era desabilitação global. Build da imagem da API local concluído para carregar o código atual.
Último passo operacional: aplicar a imagem com Compose up --no-build na API e
reiniciar somente o proxy local; sem inspeções posteriores, conforme preferência
do usuário. A conclusão do comando de atualização não comprova aceite
nem disponibilidade autenticada da conta; o usuário validará a interface.

Roteiro curto para o primeiro teste, após a atualização local:

1. Abrir `http://localhost:3003/finance` e o Crow, no modo Finance sem IA.
2. Procurar a aba **Empresarial**. Ela exige módulo Contábil habilitado e permissão
   accounting.issuers.view na conta ativa; não há bypass. Se não aparecer, informar
   a conta/aba observada, sem enviar senha ou token. Não habilitamos contas ou
   permissões arbitrariamente para mostrar a tela.
3. Cadastrar/selecionar emitente, salvar, recarregar e conferir se o cadastro foi
   retomado. Informações desconhecidas podem continuar pendentes.
4. Fazer o diagnóstico de serviço, preparar um rascunho NFS-e, informar cliente,
   descrição e valor e abrir revisão. **Esse fluxo ainda não emite nota real.**
5. Informar o que ficou confuso ou faltou. Clientes recorrentes e preenchimento
   completo por texto livre continuam lacunas conhecidas, não critérios aceitos.

Crow Plus exige habilitação própria na configuração Finance; IA e fontes também
precisam de configuração. Não é necessário ligar IA para testar os formulários.
Teste visual pelo agente não realizado; o teste manual agora é do usuário.
Nesta retomada, 39 testes frontend/3arquivos passaram (controller Accounting,
controller Plus e compilação dos componentes). Isso não é aceite visual.

## Decisão

**M1: testes automáticos locais anteriores passaram; nenhuma aprovação do usuário.
Teste funcional e aceite do piloto pelo usuário ainda não realizados.** Existem cadastro/diagnóstico persistidos,
preparação/revisão sem IA, consulta financeira, histórico separado e recuperação
de fontes com respostas limitadas. A jornada pretendida pelo usuário — pedir uma
nota para cliente recorrente/novo e responder só às lacunas pelo chat — ainda não
está completa. Não liberar essa promessa como funcionalidade disponível.

**M2: simulação local passou; emissão real bloqueada.** CT-08 não está registrado
no runtime; XSD/Swagger, preparação assinada, cofre empresarial, cobertura do
emitente e homologação continuam pendentes. Não está pronto para homologar sem
resolver esses pré-requisitos. **M3: adiado pelo usuário**, sem impedir M1/M2.
Produção, pagamento e transmissão de declarações não foram habilitados.

## Foco e lacunas priorizadas

O foco é emissão de notas de serviços solicitada no chat, com dados do emitente,
clientes e serviços reutilizados; comprovantes e orientação fiscal via RAG também
fazem parte do produto. Ver [escopo atualizado](ESCOPO_INICIAL.md).

| Prioridade | Capacidade desejada | Evidência atual / trabalho restante |
|---|---|---|
| P0 | Ver a base no painel local | O container web em uso não contém FinanceAssistantSurface; a API em uso responde404 a /v1/accounting/issuers. Sincronizar frontend e atualizar API/migrations pelo fluxo local antes de validar a tela. Não confundir200 do Finance com nova interface disponível. |
| P0 | Cliente recorrente: reaproveitar destinatário, serviço e valor | Perfil/rascunho/template pessoais e snapshots empresariais existem; cadastro e resolução empresarial de clientes/recorrências para esse fluxo ainda precisam de contrato/implementação e aceite. Não criar catálogo de produtos para isso. |
| P0 | Cliente novo: informar empresa e valor; perguntar só lacunas | Diagnóstico reconhece campos já salvos; o roteador atual é delimitado e não transforma esse pedido livre em cadastro/preparação completos. Nome sozinho não resolve destinatário com segurança; exigir dados faltantes e seleção em caso de ambiguidade. |
| P0 | Emissão simples com autorização no chat | Preparação/revisão não executam envio. Falta orquestração conversacional da autorização explícita e conclusão/qualificação do conector CT-08, sem duplicar o motor. |
| P1 | Orientação fiscal ampla e atual | Busca FTS e resumo fundamentado existem. Contexto fiscal confirmado, corpus vigente, cobertura por assunto e avaliação de respostas abertas ainda limitam o uso; não prometer “saber tudo”. |
| P1 | Uber/compras/pagamentos externos | Fluxo pessoal de anexos/rascunhos foi preservado. Fluxo empresarial de comprovantes, vínculo/lançamento e conciliação precisam de especificação/aceite próprios. Não converter despesa em nota emitida. |
| P1 | Lucros, planejamento tributário e declarações | Não há cálculo/entrega completos implementados. Especificar primeiros casos no CT-11 com contexto, regra vigente e evidências de cada etapa. |

Identificação cadastrada evita repetir perguntas de identificação; não determina
sozinha descrição, valor, município ou tributação. Dados desconhecidos permanecem
pendências. Dados reutilizados precisam ser atuais e pertencer ao emitente correto.

## Matriz executada

Todas as linhas automáticas abaixo usam fixtures sintéticas e, quando exigida
persistência, PostgreSQL16 descartável. “Passou” vale apenas para o critério indicado.
Nenhuma linha significa emissão, gasto de tokens ou homologação externa reais.

| Critério / pré-condição | Passo e esperado | Observado / evidência | Status |
|---|---|---|---|
| M1 cadastro e retomada | Gravar diagnóstico, reconstruir Service e repetir comando; manter dados/versão | TestPilotSavedDiagnosisSurvivesRestartWithoutIssuance: mesma identidade/versão, Q14, zero autorizações/execuções | PASSOU local DB |
| M1 perguntar só ausências | Perfil conhecido, adicionar serviço, cliente e valor; perguntar próxima lacuna | TestPilotAsksOnlyMissingOperationData: Q8→Q9→Q12→Q14; unknown volta a Q12; identificação sozinha não completa cadastro | PASSOU local; não é chat livre |
| M1 perfil incompleto | Unknown/declarado não vira confirmação documental | TestDiagnosisProgressesWithoutPromotingDeclarationsToProfile e TestConfirmedDiagnosisRequiresReviewedOwnerEvidence | PASSOU |
| M1 Finance canônico | Consultar/preparar com Accounting indisponível e IA falhando | TestPlusConfigAndFinancialToolsWorkWithoutAI, TestDeterministicFinanceIgnoresFailingLLM, HTTP integrado real | PASSOU; zero chamada LLM na consulta determinística |
| M1 rascunho sem IA | Preparar/revisar, conservar versões e fonte pessoal | TestDocumentPreparationUsesSavedDataAndExplicitConversationFields, TestReviewPersistenceConflictsAndScope e suíte Accounting | PASSOU; sem emissão |
| M1 API/auth real | Invocar HTTP autenticado com Finance/Accounting/Knowledge reais | TestPlusHTTPAccountingAndFinanceWithoutModel: persistência, replay e fonte revogada | PASSOU local DB/HTTP fake LLM |
| M1 histórico e contexto | Replay, exclusão e conversa de outro modo/emitente | Suíte Calendar Plus + controllers; histórico legado separado | PASSOU; execução PostgreSQL de proposals legados ficou pendente |
| M1 fontes/citações | Importar/revisar/publicar, buscar e revogar referências | TestFiscalLifecycleIsolationValidityAndDeduplication e HTTP Plus integrado | PASSOU; PDF não requalificado |
| M1 IA opcional | Sem fonte, conflito, schema/citação inventada,429/timeout, resposta tardia | TestPlusExplanationPortugueseEvaluation e cliente LLM interceptado | PASSOU simulado; qualidade de modelo comercial não avaliada |
| M1 limites/custo | Duas chamadas concorrem; replay não paga outra chamada | TestExplanationConcurrentAccountBudgetIsAtomic e TestExplanationBudgetAndDuplicateNeverCallTwice | PASSOU; preços sintéticos não são tabela comercial |
| Segurança conta/emitente/ator | Forjar IDs e trocar emitente na mesma conta | TestPersistenceIsolationVersionsAndForeignKeys, TestSameAccountEmittersAndDeleteInvisible e HTTP | PASSOU local DB |
| Segurança RBAC/módulo atual | Revogar membership/módulo/permissão e repetir operação | TestAccountingHostPolicyUsesLiveMembershipModuleAndRBAC | PASSOU banco host separado |
| Segurança dados pessoais | Fonte de outro ator não importada; conta não relabela histórico | Suítes Finance/Calendar/App, controllers com resposta atrasada | PASSOU |
| Segurança fonte hostil/segredo | Instrução recuperada permanece dado; saída/URLs inválidas são descartadas | Avaliação portuguesa, registry fechado e normalização Knowledge Omnichannel | PASSOU sintético; não prova ausência universal de ataques |
| Confiabilidade transacional | Falhar outbox antes do commit | TestQueueRollbackAndProductionUnavailable, TestEnqueueTxRollbackAndLegacyCompatibility | PASSOU; sem comando/job órfão |
| Confiabilidade concorrência | Executar replay concorrente e editar versões antigas | TestExecuteConcurrentReplayCreatesOneCommandAndJob, TestConcurrentDedupeAndEditing | PASSOU |
| Confiabilidade restart/lease | Expirar lease e devolver resposta antiga | TestLostLeaseFencingAndReconciliation | PASSOU; consulta antes de novo efeito |
| Confiabilidade revogação | Revogar antes de claim/alterar snapshot | TestRevocationAndExpiryBeforeClaim, TestEditingIdenticalValuesStillInvalidatesReviewedSnapshot | PASSOU |
| Jobs | FIFO/backoff/idempotência por conta e liberar travadas | TestWorkerFIFO, TestWorkerFIFOBackoffNotOvertaken, TestEnqueueIdempotencyIsPerAccount, TestReleaseStuckFiltersByAccount | PASSOU banco separado |
| Regressão Finance | Persistência/isolamento de planilha, atualização concorrente e fechamento | TestSheetsPersistenceDatesAndScope, TestClosedSheetAndConcurrentUpdate e Vitest fiscais/revisão/conversa | PASSOU local; autosave visual não executado |
| Regressão Omnichannel | Reusar repository e preservar busca/normalização legadas | TestLegacyOmnichannelSearchPreservedAndFiscalCorpusExcluded + TestNormalizeKnowledge* | PASSOU focado; não é toda a suíte do atendimento |
| M2 protocolo local | Fake autoriza/rejeita/perde resposta; reconciliar sem repetir POST | TestSEFINLocalProtocolOutcomesAndReconciliation e TestMotorLocalSEFINReconciliationAndPrivateArtifacts | PASSOU fake HTTP + DB |
| M2 artefato | Storage falha; recuperar sem reemitir; revogar durante download | TestMotorArtifactRecoveryNeverResubmitsAndChangedProfileCancels e TestPrivateArtifactHTTPAndRevocationDuringRead | PASSOU local |
| M2 conformidade oficial | Validar XSD/assinatura e credenciamento com contrato oficial | Pacote/validação reais não qualificados; teste sintético xmllint pulado nesta imagem | BLOQUEADO; sem homologação |
| M3 produtos/NF-e | Conector e cenário fiscal próprios | Adiado por decisão do usuário | NÃO EXECUTADO |
| Visual e acessibilidade | Abrir shell/abas, teclado, mobile e desktop | Nenhum navegador acessado | NÃO EXECUTADO |
| Ambiente em uso | Sondar HTTP e existência do componente | Finance200; healthz200; accounting/issuers404; componente ausente no container web | FALTA ATUALIZAÇÃO LOCAL |

## Resultados e limites da evidência

- PASSOU: 407 eventos de testes/subtestes Go distintos após complementações e
  correção da fixture nova, sem falha de teste pendente. São eventos distintos,
  não407 operações fiscais independentes. `-race`, `-count=1`, pacotes serializados
  para evitar concorrência de DDL entre suítes; cada domínio tem banco próprio.
- PASSOU: 85 testes Vitest em10arquivos; controllers Accounting/Knowledge/Plus,
  componentes compilados, evidências, conversa/revisão pessoais e rotas/acesso.
- PASSOU: lint Accounting/Knowledge/App/LLM/Jobs, `0 issues`.
- FALHOU e corrigido: fixture CT-10 usou origin=operator, recusado pelo contrato;
  alterado para declaration e os dois testes novos passaram. Nenhuma mudança de
  produção para aceitar input inválido. Checagens HTTP iniciais sem acesso de rede
  e rota incorreta /health foram substituídas por sondagens locais de /healthz.
- NÃO EXECUTADO: 11 testes Go pularam por pré-condições ausentes (ver JSON): OCR
  privado/Poppler, XSD sintético xmllint, integração DB de proposals Calendar e
  integrações R2 live. Não foram aceitos como aprovados nem obtidas credenciais.
- NÃO EXECUTADO: modelo/LLM real, pagamento, portal/certificado real, homologação,
  transmissão fiscal, deploy, teste visual, build completo e alterações no banco
  de uso. Zero chamadas reais fiscais/LLM. Não há benchmark de qualidade real.

Latência pública local: uma amostra sem autenticação por endpoint: Finance200 em
39.492ms; healthz200 em7.973ms; Accounting404 em7.313ms. Isso mede somente HTTP de
serviços locais atuais, não tempo para gerar nota ou responder RAG. Tempos de suíte
estão no JSON; não usar duração de teste como SLA do produto.

## Como repetir

Usar imagem `omni-go-tools:1.26.5-v2.4.0` e PostgreSQL16 exclusivo, sem portas ou
volumes. Não usar URL do banco do painel. Criar bancos com estes nomes/prefixos:

| Variável | Banco descartável |
|---|---|
| ACCOUNTING_TEST_DATABASE_URL | accounting_test_ct10 |
| FINANCE_TEST_DATABASE_URL | finance_test_ct10 |
| KNOWLEDGE_TEST_DATABASE_URL | knowledge_test_ct10 |
| PLUS_TEST_DATABASE_URL | plus_test_calendar_ct10 |
| PLUS_APP_TEST_DATABASE_URL | plus_test_app_ct10 |
| ACCOUNTING_HOST_TEST_DATABASE_URL | accounting_test_host_ct10 |
| NFSE_TEST_DATABASE_URL | accounting_test_nfse_ct10 |
| TEST_DATABASE_URL (somente Jobs) | jobs_test_ct10 |

A execução usou container temporário `omni-ct10-postgres`, autenticação trust
restrita à rede do container, sem credencial real. Runner conecta por
`--network container:omni-ct10-postgres`; URL sintética de cada banco:
`postgres://postgres@localhost/NOME_DO_BANCO?sslmode=disable`.
Fixtures criam seus schemas. Jobs requer antes `core.accounts` com id UUID/default
gen_random_uuid(), name text, slug text e is_active boolean. Sem a variável certa,
um teste pode pular; conferir skips, não apenas exit0.

Dentro de back/, com variáveis acima configuradas no runner:

```sh
go test -race -p 1 -count=1 ./internal/modules/accounting/... ./internal/modules/finance ./internal/modules/calendar ./internal/modules/knowledge ./internal/platform/app ./internal/platform/llm
go test -race -count=1 ./internal/platform/jobs
go test -race -count=1 ./internal/modules/omnichannel -run TestNormalizeKnowledge
go test -race -count=1 ./internal/modules/accounting -run TestPilot
```

Não passar TEST_DATABASE_URL do Jobs para Calendar: o teste de proposals requer
migrations/fixture próprias e continua não requalificado nesta execução.

Na raiz, testes da interface:

```sh
npm test --prefix web -- layers/finance/composables/accounting/accounting-controller.test.ts layers/finance/composables/accounting/knowledge-controller.test.ts layers/finance/composables/crow-plus-controller.test.ts layers/finance/accounting-components.test.ts layers/finance/fiscal-components.test.ts layers/finance/utils/plus-evidence.test.ts layers/finance/composables/useFinanceConversation.test.ts layers/finance/composables/useFinanceDocumentReview.test.ts app/utils/assistant-access.test.ts app/utils/assistant-routes.test.ts
```

## Roteiro do painel e aceite visual pendente

Acesso local: `http://localhost:3003/finance`. Instância atual ainda serve versão
anterior; atualizar API/migrations e sincronizar web pelo fluxo de desenvolvimento
local, preservando dados. Não basta reiniciar a imagem antiga. Depois:

1. Abrir o Crow na superfície Finance e manter o modo sem IA. Fluxo Pessoal mantém
   comprovantes/perfil existentes. Com Accounting habilitado e permissão efetiva,
   deve aparecer aba Empresarial para cadastro/diagnóstico/preparação/revisão.
2. Crow Plus aparece quando habilitado na configuração Finance da conta. Escolher
   emitente explicitamente, consultar pendências e fontes; não misturar histórico
   pessoal/empresarial. Nenhum dado público hardcoded deve simular cadastro salvo.
3. Salvar e recarregar cadastro; identidade já salva não deve ser perguntada de
   novo. Serviço, cliente e valor ausentes devem continuar como lacunas. Essa
   etapa valida diagnóstico, não cadastro recorrente completo ou emissão real.
4. Trocar conta/emitente com resposta pendente; conferir isolamento, dirty/409,
   teclado/Esc, viewport móvel e desktop. Confirmar revisão sem efeito fiscal.
5. RAG sem fonte/sem contexto/provedor: resposta honesta, sem regras/valores
   inventados; busca/consulta determinísticas devem continuar disponíveis.

Sem autorização específica de navegador, essa etapa permaneceu não executada.
A skill [principios-engenharia](../../.agents/skills/principios-engenharia/SKILL.md)
determina: “Nunca usar o Work, Computer Use/CUA ou automação de navegador/computador
sem permissão explícita prévia do usuário para esse uso na tarefa.” Não impede as
validações locais concluídas; não houve solicitação para interrompê-las.

## CT-15 — roteiro de aceite dos pacotes 1 a 8 (2026-11-06)

Roteiro único em [ROTEIRO_ACEITE_CT15.md](ROTEIRO_ACEITE_CT15.md). Testes técnicos
passaram (ver CT-15); nenhum item foi aprovado pelo usuário até esta data.
