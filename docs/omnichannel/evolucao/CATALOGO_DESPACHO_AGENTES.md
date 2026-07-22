# Catálogo de despacho e allowlists por pacote

Este documento fecha a liberdade de escrita dos pacotes definidos nas specs. O prompt final é
montado com `TEMPLATE_PACOTE_TAREFA.md`. Caminho não listado é leitura somente ou proibido.

## 1. Regras comuns de allowlist

| Sufixo do pacote | Pode escrever | Não pode escrever |
|---|---|---|
| `CONTRACT`/`AUDIT` | spec/fixtures contratuais explicitamente listadas | código de produção, migration, workflow |
| `DB` | **uma** migration nova reservada + teste SQL indicado | migration antiga, Go, front, workflow |
| `BE`/`API` | arquivos Go listados + `_test.go` co-localizado | migration, front, workflow, `module.go` sem `INT` |
| `N8N` | um JSON do owner + fixture/schema do contrato | qualquer outro JSON, Go sender, banco |
| `FE` | componentes/composables/types omnichannel listados | Go, migration, n8n, componente de outro módulo |
| `RUNBOOK` | docs/runbooks explicitamente listados | runtime, compose/VPS, workflow ativo |
| `QA` | testes/fixtures/relatório listados; produção é leitura | corrigir feature encontrada; isso volta ao owner |
| `INT` | somente wiring/lista fechada após todos os autores | regra de negócio nova ou refactor lateral |

Arquivos novos Go seguem os nomes candidatos abaixo. Se o repositório já possuir a responsabilidade
em outro arquivo, o executor para e pede ao orquestrador uma allowlist ajustada; não cria duplicata.

## 2. Pontos serializados

Somente o pacote de integração explicitamente designado pode tocar pontos serializados. Em E0,
os pacotes serializados são `E0-GUARD-02` e seu fechamento `E0-INT-04`; nas demais fases, é o
pacote `INT` correspondente. Esses pontos são:

- `back/internal/modules/omnichannel/module.go`;
- wiring em `back/internal/app/app.go` ou registry global;
- permissões/manifesto do módulo;
- navegação/host global do front;
- scripts compartilhados;
- número/nome final de migration quando houver colisão;
- ativação/config de runtime (que continua fora de código e exige autorização do dono).

## 3. E0

| Pacote | Escrita permitida |
|---|---|
| `E0-OWN-01` | `AGENT.md`; `automation/AGENT.md`; `back/internal/modules/omnichannel/AGENT.md`; `docs/omnichannel/{PLANO_TECNICO_EVOLUCAO.md,ARQUITETURA_HIBRIDA_N8N.md,ESTADO.md}`; `docs/omnichannel/evolucao/{E0_OWNERSHIP_WORKFLOWS.md,CONTRATO_EXECUCAO_AGENTES.md,CATALOGO_DESPACHO_AGENTES.md,MATRIZ_DEPENDENCIAS.md}` |
| `E0-GUARD-02` | `scripts/dev/n8n-*.ps1`, testes/fixtures específicos desses scripts |
| `E0-QA-03` | somente relatório/fixture de teste; JSONs são leitura |
| `E0-INT-04` | `scripts/dev/n8n-*.{ps1,js}` e testes; `package.json`; `.husky/pre-commit`; `scripts/deploy/deploy-fast.ps1`; docs E0 explicitamente listados |
| `E0-DOC-05` | somente `automation/AGENT.md`, `docs/DEPLOY_VPS.md` e docs E0 explicitamente listados |

Nenhum E0 altera `automation/export/*.json`.

## 4. E1

| Pacote | Escrita permitida |
|---|---|
| `E1-DB-01` | `<reservada>_messaging_message_delivery.sql` |
| `E1-BE-02` | `channel/evolution/parse.go`, `adapter.go`, `*_test.go`, `service_inbound.go`, `store_webhook_events.go`, store inbound explicitamente criado |
| `E1-BE-03` | `channel/evolution/client.go`, `service_media.go`, `media_storage.go`, worker/handler de media fetch e testes |
| `E1-API-04` | `http.go`, `service.go`, `model.go`, `store_postgres.go` e testes focados; sem refactor total |
| `E1-FE-05` | `inbox/types.ts`, `InboxChatMessageRow.vue`, componentes de mídia/quote/status e composables `useOmnichannelInbox*` estritamente necessários |
| `E1-QA-06` | fixtures/testes E1 e evidência; produção leitura |
| `E1-INT-07` | `module.go`/job registry somente para wiring dos handlers já testados |

O outbound quote deve ser incluído explicitamente no pacote BE indicado pelo orquestrador:
`service_outbound.go`, `channel/provider.go` e adapter Evolution; nenhuma outra capability entra.

### 4.1 E1-R1 — allowlists corretivas

Estes pacotes substituem somente as afirmações reprovadas do E1. Eles não autorizam Meta, n8n,
Evolution session/QR, WAHA, Docker/compose, deploy ou refactor lateral.

| Pacote | Escrita permitida |
|---|---|
| `E1-R1-DB-08` | **somente** nova `back/internal/platform/database/migrations/0215_messaging_delivery_reconciliation.sql`; `back/database/{AGENT.md,ERD.md}`; `back/internal/platform/database/AGENT.md`; testes SQL/Go de migration explicitamente co-localizados |
| `E1-R1-BE-09` | `back/internal/modules/omnichannel/{http_send.go,service_outbound.go,service_media.go,service_actions_messages.go,service_inbound.go,service_transition.go,realtime.go,outbound_handler.go,store_outbound.go,store_postgres_routing.go,store_webhook_events.go,media_storage.go (somente cleanup transacional),store_delivery_integration_test.go,outbound_handler_test.go,service_outbound_test.go,http_send_test.go}`; novos `_test.go` somente se co-localizados e focados; **não** pode tocar `module.go` |
| `E1-R1-INT-10` | somente `back/internal/modules/omnichannel/module.go` e `module_test.go`, para injetar dependências já implementadas/testadas; zero SQL/regra de negócio |
| `E1-R1-FE-11` | `web/app/composables/omnichannel/{useInboxChatMediaActions.ts,useOmnichannelInboxHistory.ts,useOmnichannelInbox.ts,useOmnichannelInboxShared.ts}`; `web/app/components/omnichannel/{OmnichannelInboxModule.vue,inbox/InboxConversationsSidebar.vue,inbox/InboxChatPanel.vue}`; `web/app/types/index.ts`; testes E1 co-localizados ou em `web/test/omnichannel/` |
| `E1-R1-QA-12` | `docs/omnichannel/evolucao/E1_PAUSA_2026-07-20.md`, checklist/evidência E1 e testes/fixtures E1; produção, banco restaurado, provider e todos os JSONs são leitura |

Regras adicionais de isolamento:

- DB-08 e FE-11 podem executar em paralelo; BE-09 inicia somente depois do contrato DB-08 estar
  fechado; INT-10 inicia depois de BE-09 verde; QA-12 recebe o diff final e não implementa correções;
- qualquer necessidade de arquivo fora da linha do pacote interrompe o executor e volta ao
  orquestrador para ajuste documental **antes** da escrita;
- `automation/export/workflow-omnichannel-brain.json` e
  `automation/export/workflow-instagram-first-contact.json` também ficam somente leitura no E1-R1;
- `workflow-whatsapp.json`, workflows de Calendário/Automação e todos os arquivos/volumes WAHA são
  proibidos, mesmo que um teste externo falhe.

## 5. E2

| Pacote | Escrita permitida |
|---|---|
| `E2-CONTRACT-01` | `docs/omnichannel/contracts/brain-v2/*.json`, fixtures e spec E2 |
| `E2-DB-02` | `<reservada>_messaging_ai_dispatches.sql` |
| `E2-BE-03` | novos `ai_dispatch*.go`, stores correspondentes e testes; ajuste mínimo em `service_inbound.go` |
| `E2-N8N-04` | somente `automation/export/workflow-omnichannel-brain.json` + fixtures brain v2 |
| `E2-BE-05` | `ai_policy.go`, `service_ai*.go`, `store_ai*.go`, outbox producer e testes |
| `E2-FE-06` | `config/ConfigAiAgent*.vue`, composables admin AI e tipos dedicados; badges inbox dedicados |
| `E2-QA-07` | testes de concorrência/contract/e2e, produção leitura |
| `E2-INT-08` | wiring module/job/realtime e permissões já declaradas na spec |

## 6. E3

| Pacote | Escrita permitida |
|---|---|
| `E3-CONTRACT-01` | `docs/omnichannel/contracts/media-analysis/*.json`, fixtures e spec E3 |
| `E3-DB-02` | `<reservada>_messaging_media_analyses.sql` |
| `E3-BE-03` | novos `media_analysis*.go`, signed-media interno, stores e testes; `service_media.go` mínimo |
| `E3-N8N-04` | somente `workflow-omnichannel-brain.json` + fixtures multimodais |
| `E3-FE-05` | componente de análise em `inbox/`, config AI media e composables dedicados |
| `E3-QA-06` | testes/fixtures de MIME, limite, token e fallback |
| `E3-INT-07` | wiring das rotas/jobs criados, sem regra nova |

## 7. E4

| Pacote | Escrita permitida |
|---|---|
| `E4-AUDIT-01` | spec/relatório do gap CRM; schema/código leitura |
| `E4-DB-02` | `<reservada>_messaging_contact_crm_evolution.sql` |
| `E4-BE-03` | `service_contacts.go`, store de identity/touchpoint, inbound mínimo e testes |
| `E4-API-04` | `http_contacts_crm.go`, `crm_model.go`, `service_crm.go`, `store_crm.go` e testes |
| `E4-BE-05` | `contact_merge.go`/`contact_merge*.go` + testes |
| `E4-LP-06` | `lead_capture.go`, service/store/source security e testes |
| `E4-FE-07` | `OmnichannelCRMProfilePanel.vue`, `useOmnichannelCRM.ts` e alterações mínimas na aba Contatos |
| `E4-QA-08` | testes/fixtures CRM e landing, produção leitura |
| `E4-INT-09` | registro das rotas/permissões e host da aba Contatos |

## 8. E5

| Pacote | Escrita permitida |
|---|---|
| `E5-CONTRACT-01` | spec/matriz/fixtures de handoff/SLA |
| `E5-DB-02` | `<reservada>_messaging_handoff_sla.sql` |
| `E5-BE-03` | novos `handoff*.go`, `service_transition.go` mínimo, stores e testes |
| `E5-BE-04` | actions take/release/transfer, stores e testes de corrida |
| `E5-BE-05` | novos `sla*.go`, scheduler/job e testes |
| `E5-FE-06` | componentes inbox de filas/handoff/SLA e composables dedicados |
| `E5-QA-07` | testes concorrência/e2e, produção leitura |
| `E5-INT-08` | wiring de rotas/jobs/realtime/permissões |

## 9. E6

| Pacote | Escrita permitida |
|---|---|
| `E6-AUDIT-01` | relatório de interfaces do módulo Tools; código leitura |
| `E6-DB-02` | `<reservada>_messaging_ai_tools_knowledge.sql` |
| `E6-BE-03` | novos `ai_tools*.go`, adapter para interface pública Tools, stores e testes |
| `E6-BE-04` | novos `knowledge*.go`, ingest/search/stores e testes |
| `E6-N8N-05` | somente `workflow-omnichannel-brain.json` + fixtures tool-call |
| `E6-FE-06` | config tools/knowledge/approvals e composables dedicados |
| `E6-QA-07` | testes injection/tenant/timeout/retry, produção leitura |
| `E6-INT-08` | wiring de API/job/permissões já implementados |

Pacote omnichannel não edita internals do módulo Tools. Se faltar interface pública, abrir pacote
separado com owner daquele módulo e aprovação explícita.

## 10. E7

| Pacote | Escrita permitida |
|---|---|
| `E7-CONTRACT-01` | fixtures/schemas Meta e spec E7 |
| `E7-DB-02` | `<reservada>_messaging_whatsapp_cloud.sql` |
| `E7-BE-03` | novo `channel/meta_whatsapp/{parse,verify,adapter}*.go`, webhook mínimo e testes |
| `E7-BE-04` | `channel/meta_whatsapp/{client,sender,media,templates}*.go` e testes |
| `E7-BE-05` | nova policy de janela/template, outbox handler mínimo e testes |
| `E7-FE-06` | config provider/templates/capabilities e composer policy |
| `E7-RUNBOOK-07` | docs de setup/cutover/rollback; sem executar deploy |
| `E7-QA-08` | fixtures/contract/smoke controlado; produção leitura |
| `E7-INT-09` | registry do adapter, rotas, jobs e permissions |

Nenhum E7 escreve `channel/evolution` salvo adaptador de interface compartilhada previamente
aprovado; nunca escreve WAHA/Automação/n8n.

## 11. E8

| Pacote | Escrita permitida |
|---|---|
| `E8-CONTRACT-01` | schemas/fixtures Instagram e spec E8 |
| `E8-DB-02` | `<reservada>_messaging_instagram.sql` |
| `E8-BE-03` | novo `channel/instagram/{parse,verify,adapter}*.go`, CRM inbound mínimo e testes |
| `E8-N8N-04` | somente `workflow-instagram-first-contact.json` + fixtures |
| `E8-BE-05` | `channel/instagram/{client,sender}*.go`, policy/actions/outbox e testes |
| `E8-FE-06` | componentes Instagram/moderação dentro de omnichannel e composables dedicados |
| `E8-QA-07` | fixtures/contract/policy/e2e; produção leitura |
| `E8-INT-08` | registry, rotas, jobs, realtime e permissions |

## 12. E9

| Pacote | Escrita permitida |
|---|---|
| `E9-AUDIT-01` | relatório de gaps; tudo mais leitura |
| `E9-OBS-02` | instrumentation/health do módulo, dashboards/runbooks versionados indicados |
| `E9-SEC-03` | somente call-sites omnichannel e abstrações platform explicitamente aprovadas |
| `E9-LGPD-04` | purge/logmask/retention omnichannel, migration apenas se gap provado |
| `E9-PRIVACY-05` | jobs/services/stores de consent/export/anonimização e testes; só após decisão jurídica |
| `E9-BACKUP-06` | scripts de backup/restore e docs; sem executar em produção |
| `E9-SCALE-07` | worker/store/ratelimit/QR cache e testes de carga, sem feature nova |
| `E9-FE-08` | painel de saúde/custo/DLQ omnichannel |
| `E9-QA-09` | testes/fault injection/relatório; produção leitura |
| `E9-INT-10` | wiring final de métricas/health/jobs/permissões |

## 13. E10

| Pacote | Escrita permitida |
|---|---|
| `E10-CONTRACT-01` | spec/schema de config/gates/permissions |
| `E10-BE-02` | rollout policy/service/store/audit e testes; sem adapter |
| `E10-FE-03` | painel rollout e composables dedicados |
| `E10-EVAL-04` | fixtures/dataset sem PII e scripts de avaliação isolados |
| `E10-RUNBOOK-05` | docs/roteiros/checklists; sem deploy |
| `E10-QA-06` | testes kill switch/rollback; produção leitura |
| `E10-INT-07` | wiring das flags/permissões/UI já prontas |

## 14. Remoção de legado interno

Remoção nunca é efeito colateral de pacote funcional. Um pacote `LEGACY` separado só fica `READY`
se tiver:

1. owner provado como omnichannel;
2. lista de consumidores vazia por busca e runtime;
3. substituto `VERIFIED` e ativo no coorte;
4. backup/commit anterior identificável;
5. rollback documentado;
6. testes sem referência ao removido;
7. confirmação de que WAHA, `workflow-whatsapp.json` e outros módulos não estão no diff.

Adapters temporários do próprio omnichannel podem sair; recursos de Automação, Calendário,
Operação ou Tools não podem ser classificados como legado por este catálogo.
