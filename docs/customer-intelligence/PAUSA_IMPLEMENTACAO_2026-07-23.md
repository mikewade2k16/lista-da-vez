# Marco de pausa — Customer Data e Customer Intelligence

- **Data:** 2026-07-23
- **Estado:** implementação local parcial, sem deploy e sem cutover
- **Objetivo deste registro:** separar claramente o que foi entregue, o que foi apenas preparado e
  o que ainda falta
- **Documentos canônicos:** [GOVERNANCA.md](GOVERNANCA.md),
  [SPECS_GERAIS.md](SPECS_GERAIS.md) e
  [IMPLEMENTACAO_LOCAL_2026-07-23.md](IMPLEMENTACAO_LOCAL_2026-07-23.md)

## 1. Resumo honesto

Foi construído um slice grande e executável no workspace local, mas o produto completo ainda não
está pronto para produção. O código separa Omnichannel, Customer Data e Customer Intelligence,
cria persistência e APIs, integra o inbound do chat a jobs duráveis e adiciona o painel inicial.

O problema visual reportado na rota de Segmentos foi corrigido e validado no navegador. A tela
agora mostra o workspace, o cliente AM Malls, criação de draft, filtros e estado vazio:
[evidência visual](../../customer-intelligence-segmentos-fixed.png).

Nenhum commit, push, deploy, exclusão de legado ou ativação de resposta automática em canal real
foi executado.

## 2. O que foi entregue em código local

| Área                  | Entrega existente                                                                                                                                                                          |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Governança            | Plano auditável, decisões arquiteturais, limites de prompt, RBAC, tenant, retenção, sender e relação entre IA e Omnichannel                                                                |
| Especificações        | Specs gerais e conjunto CI-00 a CI-10 em `docs/customer-intelligence/specs`                                                                                                                |
| Banco                 | Migrations aditivas `0239` a `0255` para bindings, Customer Data, segmentos, runtime, prompts, outboxes, claims, retenção e context snapshots                                              |
| Customer Data         | Subjects, relacionamentos, identidade protegida, notas, consentimentos, interações offline, matching/merge, segmentos e capabilities/writer states                                         |
| Customer Intelligence | Fontes registradas, observações, fatos, claims, resumos, recomendações, prompts por processo, agentes, modelos, credenciais write-only, runs, auditoria, retenção e jobs headless parciais |
| Omnichannel           | Binding canal-cliente, ingestão durável, intent de IA no mesmo commit do inbound, worker de despacho, revalidação de FSM/lease/generation e mensagem `PENDING` + outbox                    |
| Envio da IA           | A IA pode propor a resposta; somente o Omnichannel validado envia pelo adapter. Não existe envio direto do n8n/LLM ao WhatsApp ou Instagram                                                |
| Fontes                | Adapters/fachadas owner-scoped para Omnichannel, Calendário, ERP, Site e BI, sem SQL/URL livre escolhido pela LLM                                                                          |
| Painel                | Clientes, perfil, segmentos, fontes, prompts, runs, auditoria, portfólio e configurações/capabilities                                                                                      |
| Verificação           | Script `scripts/verify-customer-intelligence.ps1`, testes Go/TS e testes de integração PostgreSQL                                                                                          |

Inventário read-only do recorte desta iniciativa no worktree: 381 arquivos de
fonte/documentação/configuração, sendo 330 novos e 51 tracked modificados ou compartilhados, com
65 arquivos de teste. Arquivos compartilhados como `app.go`, ERD e alguns `AGENT.md` também contêm
mudanças de outras frentes e não podem ser atribuídos integralmente a esta iniciativa.

## 3. Correção da página vazia

Havia dois defeitos combinados:

1. componentes Vue guardados em subpastas eram registrados pelo Nuxt com nome prefixado, mas as
   páginas os chamavam pelo nome curto; o shell carregava e o conteúdo não montava;
2. cada componente que instanciava `useCustomerIntelligenceAccess` executava um watcher imediato
   que limpava novamente o cliente selecionado da agência.

Correções aplicadas:

- imports explícitos nos entrypoints das páginas e nos componentes aninhados;
- preservação do cliente selecionado enquanto o owner account não muda;
- teste de regressão para garantir que uma segunda instância do composable não apague o escopo;
- teste de contrato que detecta componentes aninhados usados sem import explícito;
- validação real no navegador da rota `/inteligencia-clientes/segmentos`.

## 4. Estado local de configuração para a validação

O backend corretamente exige RBAC de account, inclusive para `platform_admin`. A conta Crow Visuals
possuía apenas o papel legado `queue.owner`, sem as novas permissões.

Para validar o módulo localmente foi criado pelo endpoint oficial de RBAC o papel
`customer_intelligence_admin`, com 33 permissões account-scoped de Customer Data e Customer
Intelligence, e ele foi atribuído ao usuário local atual. Essa é configuração do banco local; não é
migration nem bypass de autorização.

Em AM Malls, `segmentation` foi alterada pelo próprio painel de `off` para `shadow`, revisão 1, com
motivo auditável. O writer `segment_definition` permanece `legacy`. Portanto:

- leitura e inspeção do workspace de Segmentos estão liberadas;
- nenhum cutover de escrita foi feito;
- criação persistida de segmentos continua bloqueada até reconciliação e promoção governada do
  writer/capability.

## 5. Evidências de validação até a pausa

| Verificação                                                               | Resultado                                                                                                                        |
| ------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `go test ./...`                                                           | passou                                                                                                                           |
| `go vet ./...`                                                            | passou                                                                                                                           |
| Migrations `0001` a `0255` em PostgreSQL 16 descartável                   | passaram                                                                                                                         |
| Integração PostgreSQL de Customer Data/Customer Intelligence              | passou                                                                                                                           |
| Atomicidade inbound + FSM + generation + intent de IA, incluindo rollback | passou                                                                                                                           |
| Suite Vitest final                                                        | 61 arquivos, 295 testes, todos passando                                                                                          |
| ESLint completo                                                           | zero erros; warnings existentes fora do recorte                                                                                  |
| ESLint do recorte Customer Intelligence corrigido                         | zero erros e zero warnings                                                                                                       |
| Teste novo de estabilidade do client scope                                | passou                                                                                                                           |
| Browser autenticado: Configurações                                        | renderizou capabilities, writers, modelos, credenciais e agentes                                                                 |
| Browser autenticado: Segmentos                                            | renderizou workspace funcional em `shadow`, sem área vazia                                                                       |
| Typecheck global                                                          | ainda falha por dívida preexistente fora do recorte; nenhum erro foi encontrado nos arquivos CI no levantamento anterior         |
| Build Nuxt final                                                          | inconclusivo no Windows por reparse point Linux/WSL em `.output/server/node_modules/hookable`; o prepare isolado do Nitro passou |

## 6. O que ainda não foi entregue

| Área             | Pendência real                                                                                                                      |
| ---------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| E2E              | Fluxo autenticado completo com provider LLM e WhatsApp/Instagram reais                                                              |
| Rollout          | SLOs, métricas, canary operacional, ensaio de rollback, aprovação e ativação                                                        |
| Deploy           | Build final reproduzível, commit isolado, push, deploy e smoke test                                                                 |
| Processos de IA  | Seis processos ainda têm contrato/schema, mas não possuem orquestrador/writer de negócio completo                                   |
| Segmentos        | Worker contínuo de avaliação/materialização e exportação governada                                                                  |
| Offline          | Importação CSV, anexos, storage/scan e backfill amplo                                                                               |
| Prompt Studio    | Simulação LLM real, corpus/evals, policy descriptor, pipeline, tools/knowledge e rollout completos                                  |
| Portfólio        | Gerador agregado seguro, ranking validado e promoção comercial                                                                      |
| Retenção/LGPD    | Painel/API de legal hold, DSAR, exclusão/anonimização ampla, backups e decisão jurídica                                             |
| Legado/n8n       | Paridade, importação, cutover de writers e retirada controlada do legado                                                            |
| Painel           | Varredura visual autenticada de todas as rotas e todos os estados de erro/vazio/loading                                             |
| RBAC existente   | Estratégia reproduzível para clonar/atribuir os novos papéis em accounts antigas; hoje a atribuição feita para teste é apenas local |
| Qualidade global | Resolver a dívida de typecheck e concluir build limpo em ambiente sem symlinks incompatíveis                                        |

## 7. Como retomar sem perder contexto

1. Preservar este worktree e separar mudanças de Social Publishing/BI antes de qualquer commit.
2. Reexecutar `scripts/verify-customer-intelligence.ps1`.
3. Rodar a suite web completa após as correções de imports e client scope.
4. Fazer a varredura autenticada de todas as rotas de `/inteligencia-clientes`.
5. Definir o bootstrap de RBAC para accounts existentes sem conceder permissões amplas
   automaticamente.
6. Fechar primeiro um E2E em `shadow`; somente depois avaliar `canary` ou `on`.
7. Não promover writer para `new` sem reconciliação, checksums iguais, confirmação e rollback
   ensaiado.

## 8. Limites desta pausa

- o worktree continua sujo e contém mudanças alheias a esta iniciativa;
- nada está staged;
- o branch local estava três commits à frente do remoto na auditoria, mas nenhum push foi feito
  nesta rodada;
- o container PostgreSQL descartável criado para a verificação foi removido ao encerrar a rodada;
- este documento registra estado local e não é aceite de produção.
