# Customer Intelligence — pacote de especificações executáveis

- **Status do pacote:** READY como blueprint — implementação local parcial, não `DONE`
- **Data-base:** 2026-07-23
- **Governança:** [../GOVERNANCA.md](../GOVERNANCA.md)
- **Blueprint:** [../SPECS_GERAIS.md](../SPECS_GERAIS.md)

> Este diretório transforma a governança em contratos executáveis. O owner autorizou código,
> migrations aditivas, testes e documentação local em 2026-07-23. Workflow/import, deploy,
> cutover, exclusão e uso individual cross-client continuam proibidos até os gates específicos.

> `READY` autoriza despachar pacotes; não afirma que CI-00 a CI-10 foram entregues. O estado real
> está em [../IMPLEMENTACAO_LOCAL_2026-07-23.md](../IMPLEMENTACAO_LOCAL_2026-07-23.md). Hoje existe
> um slice conversacional e cinco writers headless; E2E autenticado/provider real, rollout, deploy
> e partes relevantes das specs seguem pendentes.

## Resultado-alvo que o owner valida

Quando concluído, o conjunto deve permitir validar de uma só vez:

- limites entre Omnichannel, Customer Data e Customer Intelligence;
- identidade agência→cliente→subject→relacionamento;
- banco de evidências, fatos, sínteses e recomendações;
- interações offline, segmentos versionados e uso governado em CRM/marketing;
- catálogo de fontes e ações autorizadas;
- prompts específicos por processo, pipelines estruturados e toda customização segura pelo painel;
- migração auditável dos agentes/prompts atuais, sem publicação automática;
- runtime IA headless e integração com o chat;
- workspace, Prompt Studio e perfil 360°;
- privacidade, portfólio, rollout, rollback e retirada de legado.

## Ordem obrigatória de leitura

1. `AGENT.md` da raiz;
2. skills `principios-engenharia` e `omnichannel-hibrido`;
3. [../GOVERNANCA.md](../GOVERNANCA.md);
4. [../SPECS_GERAIS.md](../SPECS_GERAIS.md);
5. [CONTRATO_EXECUCAO_AGENTES.md](CONTRATO_EXECUCAO_AGENTES.md);
6. [MATRIZ_DEPENDENCIAS.md](MATRIZ_DEPENDENCIAS.md);
7. [CATALOGO_DESPACHO_AGENTES.md](CATALOGO_DESPACHO_AGENTES.md);
8. somente a spec e o pacote atômico a executar;
9. AGENTs locais dos diretórios autorizados;
10. [CHECKLIST_REVISAO.md](CHECKLIST_REVISAO.md), para o revisor.

Receber uma spec não autoriza executar a fase inteira. A unidade de implementação é o pacote
atômico definido dentro dela.

## Índice

| Documento | Resultado |
|---|---|
| [CI-00_GOVERNANCA_CONTRATOS.md](CI-00_GOVERNANCA_CONTRATOS.md) | decisões, módulos, permissões, prompts e contratos congelados |
| [CI-01_BINDING_CANAL_CLIENTE.md](CI-01_BINDING_CANAL_CLIENTE.md) | conversa pertence a cliente independentemente da IA |
| [CI-02_IDENTIDADE_RELACIONAMENTOS.md](CI-02_IDENTIDADE_RELACIONAMENTOS.md) | subject, relacionamento, matching, merge e isolamento |
| [CI-03_CUSTOMER_DATA.md](CI-03_CUSTOMER_DATA.md) | identidade, relação, offline e segmentos determinísticos |
| [CI-04_INTELLIGENCE_BANK.md](CI-04_INTELLIGENCE_BANK.md) | evidências, fatos, sínteses, prompts e auditoria persistidos |
| [CI-05_FONTES_CONECTORES.md](CI-05_FONTES_CONECTORES.md) | adapters, fontes, health, sync e ações autorizadas |
| [CI-06_RUNTIME_IA.md](CI-06_RUNTIME_IA.md) | Prompt Registry, migração legada, agentes e runtime headless |
| [CI-07_INTEGRACAO_OMNICHANNEL.md](CI-07_INTEGRACAO_OMNICHANNEL.md) | IA produz; Omnichannel valida e envia |
| [CI-08_FRONTEND_UX.md](CI-08_FRONTEND_UX.md) | perfil 360°, segmentos, fontes, Prompt Studio, runs, auditoria e inbox |
| [CI-09_RECOMENDACOES_PORTFOLIO.md](CI-09_RECOMENDACOES_PORTFOLIO.md) | follow-up, ofertas, marketing e cross-client governado |
| [CI-10_HARDENING_CUTOVER.md](CI-10_HARDENING_CUTOVER.md) | shadow, SLO, rollout, rollback e deprecação |

## Decisão de customização

Prompts são a principal camada de comportamento das IAs, com um prompt específico para cada
`process_key`. Como visão-alvo, o painel controla drafts, versões, bindings, variáveis, modelos,
tools, fontes, testes, publicação, canary e rollback.

No slice atual, o Prompt Studio cobre listagem por processo, draft/edição, validação, teste
estrutural, publicação/binding com agente publicado e rollback. A coorte canary também possui
seletor determinístico configurável. Descriptors/persistência das policies por processo, editor de
pipelines, tools/knowledge, corpus/evals com LLM, diff completo e rollout operacional continuam
pendentes.

Os treze processos têm contrato fechado. `conversation.triage` e `conversation.reply` atendem a
conversa; `profile.summary`, `recommendation.follow_up`, `recommendation.offer`,
`recommendation.important_dates` e `source.suggest` possuem job/writer headless. Os seis restantes
continuam sem orquestração/writer de negócio.

Observações possuem proveniência, projeção mascarada e reveal auditado. Observações e context
snapshots expiram em worker limitado/idempotente; o contexto sofre crypto-shred in-place e preserva
metadados mínimos. Legal hold direto ou herdado de observação bloqueia a expiração. Isso ainda não
encerra o lifecycle jurídico, DSAR, backups ou o tratamento de todas as entidades derivadas.

Na tela de Fontes, versões de policy de retenção são visíveis; um administrador pode criar draft e
publicar explicitamente com revisão, motivo catalogado, referência de aprovação e confirmação.
Versão obsoleta é bloqueada, mudança de escopo/permissão cancela a operação e a publicação não
reponta fontes existentes automaticamente. Legal hold ainda não possui painel/API.

Prompts não substituem:

- autenticação, tenant e permissões;
- FSM, leases, dedupe e idempotência;
- schema de saída e catálogos válidos;
- consentimento, retenção e políticas do canal;
- allowlist de fonte/tool;
- mensagem `PENDING`, outbox e adapter de envio.

Lacunas que impedem considerar o pacote `DONE`:

- seis processos sem writer/orquestração de negócio;
- gerador seguro e agregado de `portfolio.opportunity`;
- policy/pipeline/tool/knowledge configuráveis de ponta a ponta pelo painel;
- API/painel de legal hold e lifecycle LGPD amplo;
- credenciais e configuração de ambiente de teste;
- E2E autenticado no browser e com LLM/provider/canal controlados;
- deploy, observabilidade/SLO, rollout, ensaio de rollback e cutover.

Aceitar `source.suggest` continua sendo apenas feedback auditável; não habilita, configura,
sincroniza ou concede credencial a uma fonte.

## Status

| Status | Quem aplica | Significado |
|---|---|---|
| `DRAFT` | autor da spec | contrato ainda pode mudar |
| `READY` | owner/orquestrador | decisões fechadas; pacote pode ser executado |
| `IN_PROGRESS` | executor | trabalho iniciou dentro da allowlist |
| `BLOCKED` | executor + revisor | bloqueio objetivo registrado |
| `IMPLEMENTED` | executor | diff entregue e testes do pacote passaram |
| `VERIFIED` | revisor independente | código, testes e evidências conferidos |
| `DONE` | owner/orquestrador | fase integrada, documentada e demonstrável |

Executor não marca o próprio pacote como `VERIFIED` ou `DONE`.

## Gate histórico para sair de DRAFT

- [ ] decisões `CI-DEC-*` revisadas;
- [ ] nomes definitivos dos módulos;
- [ ] dependências obrigatórias/opcionais definidas;
- [ ] catálogo de `process_key` aprovado;
- [ ] pipeline `conversation.respond` e `ProcessResult` por etapa aprovados;
- [ ] matriz prompt×policy×invariante aprovada;
- [ ] mapping/split dos prompts atuais e bloqueios de migração aprovados;
- [ ] contrato de segmentos e uso de marketing aprovado;
- [ ] permissões de editar/testar/publicar prompts aprovadas;
- [ ] política de identidade e cross-client aprovada;
- [ ] retenção e papéis de privacidade encaminhados;
- [ ] ordem CI-00→CI-10 aceita;
- [ ] itens a manter, mover, criar e retirar compreendidos;
- [ ] nenhum pacote marcado `READY` com decisão material aberta.

## Restrições que permanecem após a autorização local

- não editar/importar/ativar workflow sem pacote e gate próprios;
- não executar deploy, cutover ou provider real sem autorização;
- não remover compatibilidade ou legado;
- não habilitar cross-client individual;
- não tratar export de segmento como entregue: `0241` não o cria e ainda faltam migration, API e
  worker específicos;
- não ativar produção antes de fechar LGPD ampla, E2E autenticado/provider real, SLO, rollout
  canary e rollback;
- não alterar `automation`, WAHA ou recurso de outro owner.
