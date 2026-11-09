# Ownership e integração — CT-02 v1.0

Estado: planejamento de escrita, não autorização de publicação. Contratos: [domínio](CONTRATOS.md), [API/portas](CONTRATOS_API.md). Um responsável por arquivo/tabela a cada momento. “Coordenador” significa o executor que integra a etapa, não agente criado automaticamente. Este pacote não cria branches/worktrees, commits, push ou deploy.

## Zonas e escritores únicos

| Owner | Arquivos novos reservados / responsabilidade | Tabelas |
|---|---|---|
| CT-03 | `back/internal/modules/accounting/` exceto arquivos knowledge_* e connectors/; módulo, modelos tipados, HTTP/service/store, diagnóstico, motor/revisão/autorização, fake, testes e AGENT; adapter Finance de importação por fachada no owner | accounting.issuers, issuer_members, issuer_profiles, diagnoses, documents, document_reviews, submission_authorizations, executions, execution_events, artifacts, request_receipts, outbox |
| CT-04 | `web/layers/finance/components/accounting/`, `composables/accounting/`, `types/accounting.ts`, respectivos testes; integração visual pelo coordenador | Nenhuma |
| CT-05 | `back/internal/modules/finance/plus_*.go`, `plus_*test.go`; fachadas Finance `accounting_bridge.go`, `plus_finance.go`; adapter Calendar `assistant_plus.go` e testes; schema/opções Plus na config existente; `web/layers/finance/composables/useCrowPlus.ts` e testes | Extensão finance.assistant_configs.data; extensão calendar.chat_conversations/chat_messages/receipts existentes pelo owner Calendar |
| CT-06 | `back/internal/modules/knowledge/` (serviço/repositório canônico extraído e testes); `accounting/knowledge_*.go`; adapters `omnichannel/knowledge_bridge.go`; fontes/manuais pela API do serviço neutro | accounting.knowledge_bindings; messaging.knowledge_document_applicability; ownership do repositório messaging.knowledge_bases/documents/chunks transferido para serviço knowledge, sem cópia |
| CT-07 | `finance/plus_evidence.go`, testes e componente de evidência dedicado `components/accounting/AccountingEvidence.vue` após handoff CT-04; sem editar plus_service.go em paralelo | Nenhuma tabela duplicada de evidência |
| CT-08 | `accounting/connectors/nfse/` e testes; documentação de cobertura/conector/homologação | Nenhuma cópia do motor/outbox/documento |
| CT-09 | `accounting/connectors/nfe/` e testes; documentação de cobertura/conector/homologação | Nenhuma cópia do motor/outbox/documento |
| CT-10 | Qualificação integrada/fixtures/relatórios por marco, testes focados em arquivos próprios | Banco descartável sintético, sem alterar dados vivos |

Diretórios reservados são decisões desta especificação; não afirmar que existem hoje. CT-03 cria contratos usados pelo bridge; CT-05 implementa a parte Finance empresarial depois do CT-03. Para não bloquear CT-03 pelo CT-05, CT-03 implementa a fachada de importação pessoal mínima em `finance/accounting_source.go` e seus testes; CT-05 não a reimplementa. Ela é injetada por composição e continua sujeita à política Finance.

## Arquivos compartilhados: somente coordenador

- `back/internal/platform/app/app.go`, novo `app/accounting.go`, `app/finance_assistant.go`: wiring, lifecycle, registry/gating. Executors entregam patch proposto no handoff, coordenador aplica uma vez.
- `back/internal/modules/accounting/module.go`, `permissions.go`, `AGENT.md`: CT-03 é dono inicial; após fundação, coordenador integra contribuição knowledge/adapter sem permitir dois escritores. CT-06 entrega lista de permissões/rotas ao coordenador; não cria outro catálogo accounting.
- `back/internal/platform/jobs/store.go` e testes transacionais: CT-03 recebe janela exclusiva para EnqueueTx/helper; preservar Store/Enqueue e consumidores atuais. Se mudança maior no Worker se revelar necessária, parar integração dessa parte e revisar contrato antes de tocar comportamento global.
- `calendar/chat.go`, `chat_store.go`, `chat_access.go`, `http_chat.go`, `assistant_finance.go`, models/receipts compartilhados e Calendar AGENT: janela exclusiva CT-05 coordenada; callbacks neutros em arquivo próprio, sem mover módulo inteiro.
- `finance/assistant.go`, `module.go`, `AGENT.md`: coordenador incorpora config Plus e facade na janela CT-05; configuração não espelhada.
- `omnichannel/{service_knowledge,store_knowledge,knowledge_model}.go`, tool builtin e AGENT: janela CT-06 para delegar ao único repositório canônico; preservar schemas/policies Omnichannel. Acesso neutro não chama requireAgentPerm nem cria agente.
- `back/database/ERD.md`, migrations e AGENTs de composição/plataforma: coordenador serializa e atualiza junto à integração correspondente.
- Página Finance, chat compartilhado, navegação/gating front, store de configuração existente: coordenador aplica pontos de entrada CT-04/05; CT-04 não escreve useCrowPlus.ts, CT-05 não escreve componentes de formulário CT-04.

`AccountingEvidence.vue` tem dono CT-04 até seu handoff, depois CT-07. Transferências de ownership registram explicitamente arquivo, owner anterior/novo e contrato; sem escrita concorrente. Subdiretórios não autorizam reformatar todos os arquivos do módulo.

## Reserva de migrations

Atualização CT-03: 0310_accounting_foundation.sql foi reservada, implementada e testada em banco descartável. Não foi aplicada ao banco de uso. Antes de criar migration, coordenador relê todos os nomes, identifica próximo livre e registra aqui número/nome/CT/status. Uma reserva ativa tem um único escritor. O próximo executor não deduz número só por este inventário.

| Sequência lógica | Responsável | Conteúdo | Número/status |
|---|---|---|---|
| Fundação | CT-03 | schema accounting, escopo/versões/diagnóstico, documento/motor/outbox | 0310_accounting_foundation.sql — implementada/testada; não aplicada ao banco de uso |
| Conhecimento | CT-06 | binding Accounting + applicability no corpus existente | 0312_accounting_knowledge.sql — implementada/testada em PostgreSQL descartável; não aplicada em ambiente de uso |
| Respostas e uso IA | CT-07 | runs de uso sem duplicar evidência/config | 0313_finance_plus_llm_runs.sql — implementada/testada em banco descartável; não aplicada em uso |
| Plus | CT-05 | campos opcionais Calendar para modo/contexto/envelope; opções config na tabela existente sem duplicação | 0311_assistant_plus.sql — implementada/testada em banco descartável no CT-05; não aplicada em ambiente de uso |

CT-06 pode desenvolver serviço neutro/FTS em paralelo com CT-03, usando contrato de binding; a migration com FK para emitente só integra depois da fundação. Isso é ordem de integração, não dependência circular. Não renumerar/editar migration aplicada. Cada migration roda/reaplica em PostgreSQL descartável com fixtures sintéticas, valida FKs compostas/CAS/rollback e atualiza ERD. Não copiar segredos nem apontar testes ao banco real.

## Grafo e ondas

Dependências normativas da integração (sem retorno para CT-02; alteração de contrato é processo de revisão, não aresta de execução):

```json
{
  "CT-01": [],
  "CT-02": ["CT-01"],
  "CT-03": ["CT-02"],
  "CT-04": ["CT-03"],
  "CT-05": ["CT-03"],
  "CT-06": ["CT-02"],
  "CT-07": ["CT-04", "CT-05", "CT-06"],
  "CT-08": ["CT-03", "CT-04", "CT-05"],
  "CT-09": ["CT-03", "CT-04", "CT-05"],
  "CT-10-M1": ["CT-04", "CT-05", "CT-06", "CT-07"],
  "CT-10-M2": ["CT-10-M1", "CT-08"],
  "CT-10-M3": ["CT-03", "CT-04", "CT-05", "CT-09"],
  "CT-11": ["CT-07", "CT-10-M1"],
  "CT-12": ["CT-06", "CT-07"],
  "CT-13": ["CT-07", "CT-10-M1"],
  "CT-14": ["CT-07", "CT-10-M1", "CT-13"]
}
```

CT-10 é qualificação por marcos, não uma trava única esperando todos. M1 não espera NF-e/NFS-e/Jeff. CT-05 oferece capability_unavailable para busca até integrar CT-06; não inventa resultado. CT-08/09 podem iniciar adapters isolados após CT-03, mas finalizam integração após CT-04/05. CT-11 requalifica o marco após obrigações; CT-12/13/14 não bloqueiam M1.

## Integração e aceite mínimo de cada handoff

1. Executor confirma contrato 1.0, escopo e estado real das dependências; registra arquivos/tabelas/portas, limitações e patch de composição necessário.
2. Coordenador reserva janela de arquivo/migration e aplica wiring sem regra de negócio em platform. Serviços consumidores dependem de portas; não de SQL privado ou service concreto de outro módulo.
3. Executar verificações focadas do owner e de compatibilidade afetada, sem build completo automático. Persistência/concorrência usam PostgreSQL descartável; adapter usa fake local.
4. Registrar PASSOU/FALHOU/NÃO EXECUTADO/BLOQUEADO, comandos e resultados reais; não usar histórico de teste como evidência nova. UI visual depende de autorização específica para browser/Work.
5. Atualizar handoff CT-XX e estado das capacidades; integração não habilita envio real. Publicação Git/deploy/homologação externa requerem autorização própria.

Para CT-03: repetir A1/A2/A4 e C01–C08/C12–C15/C17–C19, rollback/fencing reais no banco; não basta mock. Para CT-06: dois consumidores reais preservados, bindings e vigência antes do ranking, C01/C02/C10/C15; nenhuma exigência de Omnichannel no Accounting. Para CT-05: replay/isolamento/contexto e C03/C09/C11/C16, determinismo sem IA. CT-10 verifica tudo integrado.

## Execução sequencial — 2026-11-04

Por instrução do usuário, executar uma tarefa por vez: CT-04 → CT-05 → CT-06.
Nenhum arquivo 0311/0312 existia quando as reservas foram ajustadas; nenhuma
migration aplicada foi renumerada. Estado e aceite CT-04 em handoffs/CT-04.md.

## Janela coordenada CT-08 — 2026-11-04

Execução sequencial, sem outros escritores. CT-08 criou accounting/connectors/nfse
e testes; extensão compartilhada mínima do owner: ArtifactReader/download HTTP,
service/module e wiring de leitura Storage em app/accounting.go/app.go. Sem mudança
em worker/autorizações/outbox nem migration nova. NF-e não tocada. Connector NFS-e
não registrado em runtime; XSD/Swagger/preparação/cofre/homologação pendentes.

## CT-09 adiado por mudança de prioridade — 2026-11-04

Usuário excluiu catálogo/produtos do escopo inicial. Ver ESCOPO_INICIAL.md.
A tentativa local de extensão de produtos/NF-e foi retirada; nenhum contrato de
produto foi incorporado. A reserva 0314 foi liberada: o arquivo experimental
0314_accounting_nfe.sql foi removido antes de qualquer aplicação em banco. Não
renumerar migrations existentes; conferir o inventário novamente na próxima tarefa.

## Reserva CT-11 — 2026-11-05

Execução sequencial autorizada pelo usuário, com M1 ainda sem aceite funcional.
0314_accounting_obligations.sql reservada para obrigações/regras/histórico e vínculo
canônico no owner Finance. Regras reais serão definidas depois por escolha expressa:
MEI e não MEI recebem estrutura, sem regra fiscal ativada. Extensão API compatível
v1.1; nenhuma mudança dos contratos de emissão. Finance fornece operação transacional
estreita, nunca SQL Finance no domínio Accounting. Sem pagamento/guia/transmissão.

## CT-15 — registro contínuo, 2026-11-05

Prompt CT-15 é a lista canônica de pendências; cada executor atualiza os itens
ao encerrar e mantém exportação derivada sincronizada. Dependências documentais
CT-01–14, sem arestas de implementação desses prompts de volta ao CT-15. Nenhuma
migration ou mudança runtime nesta tarefa de registro; aceite do usuário pendente.

## Reserva CT-12 — 2026-11-05

0315_knowledge_ingestion.sql reservada: assinaturas de fontes e runs/checkpoints
neutros no schema messaging. Reutiliza outbox/worker Jobs já composto pelo host
Accounting; não cria vetor/worker/framework paralelo. Sem extensão ou reindexação
global. PostgreSQL/backup serão inventariados em ambiente descartável; FTS e
publicação revisada CT-06 permanecem autoritativos. Escritor único nesta execução.
