# Catálogo de despacho — Customer Intelligence

- **Status:** DRAFT
- **Uso:** selecionar um pacote atômico após a fase ficar `READY`

> Paths são allowlists máximas. A spec da fase deve reduzi-las. Número de migration nunca é
> escolhido neste catálogo.

## Regras globais

Pode ler:

- `docs/customer-intelligence/**`;
- documentos/AGENTs citados pela spec;
- código atual necessário para provar o contrato.

Nunca pode alterar sem pacote explícito:

- `back/internal/modules/automation/**`;
- workflow `workflow-whatsapp.json`, WAHA e recursos Automation;
- workflows Calendar, Operação ou outro owner;
- `back/internal/modules/socialpublishing/**` e `docs/social-publishing/**`;
- mudanças BI atuais do usuário;
- migration existente;
- deploy/VPS/n8n runtime.

## CI-00 — contratos

| Pacote | Resultado | Allowlist máxima |
|---|---|---|
| `CI00-DOC-APPROVAL` | registrar decisão canônica do owner | somente docs CI-00 nominalmente listados |
| `CI00-MODULE-CATALOG` | registrar módulos, capabilities e permissions | registry/metadata/permissions nominalmente listados na spec |
| `CI00-PROMPT-CONTRACT` | congelar process keys, pipeline, schemas, layers e fixtures | somente contratos/fixtures novos definidos na spec |
| `CI00-SEGMENT-CONTRACT` | congelar AST, lifecycle, runs, permissions e separação de export | somente docs/fixtures contratuais nominalmente despachados |

## CI-01 — binding canal-cliente

| Pacote | Resultado | Allowlist máxima |
|---|---|---|
| `CI01-DB-ADDITIVE` | DDL aditivo do binding | nova migration, ERD e AGENT Omnichannel |
| `CI01-BE-DOMAIN` | domínio/store/service do binding | arquivos novos/focados de Omnichannel |
| `CI01-API` | administração permissionada | handlers/services/testes focados |
| `CI01-REPAIR` | fila de exceções e reparo assistido | Omnichannel backend/testes focados |
| `CI01-FE-BINDING` | aba de vínculos no painel Omnichannel | config/composable/domain/testes listados |
| `CI01-BACKFILL` | backfill e relatório de ambiguidades | comando/pacote dedicado + docs |
| `CI01-INBOUND` | resolução server-side no inbound | arquivos de inbound nominalmente listados |
| `CI01-AUTOMATION-SEAM` | compatibilidade sem apropriar Automation | somente seam/adapters listados; `automation/**` proibido |
| `CI01-CUTOVER` | writer state, métricas e ativação gradual | flags/adapters/evidências nominalmente listados |

## CI-02 — identidade/relacionamento

| Pacote | Resultado | Allowlist máxima |
|---|---|---|
| `CI02-CONTRACT` | contrato de identidade, match, merge e undo | somente docs CI e ADR novo autorizado |
| `CI02-FIXTURES` | corpus de casos sem writer | fixtures novas definidas na spec |
| `CI02-QA` | revisão adversarial dos casos | testes/relatório; sem código de produção ou migration |

## CI-03 — Customer Data

| Pacote | Resultado | Allowlist máxima |
|---|---|---|
| `CI03-DOC-ADR` | registrar boundary aprovado | ADR novo + docs CI |
| `CI03-DB-FOUNDATION` | subjects/relationships/identities/consents | nova migration + ERD/AGENT |
| `CI03-DB-SEGMENTS` | segmentos/versões/runs/materializações/memberships | nova migration separada + ERD/AGENT |
| `CI03-DB-SEGMENT-EXPORTS` | export workflow/eligibility após gates jurídicos | nova migration separada + ERD/AGENT |
| `CI03-BE-MODULE` | módulo e composition contract | `back/internal/modules/customerdata/**` + wiring nominal |
| `CI03-BE-IDENTITY` | store/service de identidade e relação | módulo Customer Data |
| `CI03-BE-NOTES-CONSENTS` | notas, consentimentos e lifecycle | módulo Customer Data |
| `CI03-BE-OFFLINE` | interações offline, anexos e import jobs | módulo Customer Data + ports de storage/scanner |
| `CI03-BE-MATCH-MERGE` | matching, quarantine, merge e undo | módulo Customer Data |
| `CI03-BE-SEGMENTS` | lifecycle imutável, field catalog, AST e avaliações | módulo Customer Data; sem LLM/SQL livre |
| `CI03-BE-SEGMENT-EXPORTS` | consent eligibility e objeto privado/TTL | módulo Customer Data + port de storage aprovado |
| `CI03-API` | APIs, DTOs e permissions | módulo + registry necessário |
| `CI03-API-OFFLINE` | APIs idempotentes de offline/import/anexo | módulo + wiring nominal |
| `CI03-API-SEGMENTS` | fields/definitions/versions/runs/materializations | Customer Data HTTP |
| `CI03-API-SEGMENT-EXPORTS` | request/status/cancel/download intent | Customer Data HTTP; nenhum sender |
| `CI03-EVENT-JOB` | outbox/consumidores idempotentes | módulo + jobs/wiring listados |
| `CI03-JOB-SEGMENTS` | evaluation/materialization/export workers | módulo + wiring nominal; sem n8n |
| `CI03-BACKFILL` | import controlado do CRM legado | comando/pacote dedicado + docs |
| `CI03-BACKFILL-SEGMENTS` | tradução allowlisted e quarentena do filtro legado | comando/pacote dedicado + docs |
| `CI03-LEGACY-FACADE` | compatibilidade de consumidores antigos | arquivos CRM/Omnichannel explicitamente listados |
| `CI03-QA-SEGMENTS` | AST/tenant/immutability/consent/export/load | testes/fixtures/evidência; sem produção |
| `CI03-CUTOVER` | writer state por entidade/cliente | flags/adapters/evidências listados |

## CI-04 — Intelligence Bank

| Pacote | Resultado | Allowlist máxima |
|---|---|---|
| `CI04-DOC-01` | congelar modelo físico/lógico | somente docs CI |
| `CI04-DB-01` | evidence/facts/summaries/recommendations | nova migration + ERD/AGENT |
| `CI04-DB-02` | persistência de Process/Pipeline/Prompt Registry | nova migration separada + ERD/AGENT |
| `CI04-DB-03` | policies versionadas de recomendação | nova migration separada + ERD/AGENT |
| `CI04-BE-DOMAIN-01` | tipos e policies de evidência/fato | `back/internal/modules/customerintelligence/**` |
| `CI04-BE-STORE-01` | stores tenant-scoped | mesmo módulo |
| `CI04-BE-PROMPTS-01` | store/service do registry | mesmo módulo |
| `CI04-BE-PIPELINES-01` | lifecycle/versionamento de pipelines | mesmo módulo |
| `CI04-API-01` | perfil, evidência, revisão e auditoria | mesmo módulo |
| `CI04-JOB-01` | resolução/rebuild idempotente | mesmo módulo + wiring nominal |
| `CI04-BACKFILL-01` | legado `contact_intelligence` | comando/pacote dedicado + docs |
| `CI04-QA-01` | revisão independente | testes do módulo + evidências |
| `CI04-CUTOVER-01` | writer state e façade compatível | flags/adapters/evidências listados |

## CI-05 — fontes/conectores

| Pacote | Resultado | Allowlist máxima |
|---|---|---|
| `CI05-BE-REGISTRY-01` | descriptors, scopes e policy de fontes | Customer Intelligence |
| `CI05-DB-01` | configs/runs/checkpoints necessários | nova migration + ERD/AGENT |
| `CI05-BE-SERVICE-01` | ingestão, health, disable e suggestions | Customer Intelligence |
| `CI05-API-01` | catálogo/config/test/sync/health | Customer Intelligence HTTP |
| `CI05-ADAPTER-OMNI-01` | adapter de conversa online | novo adapter + interfaces públicas mínimas |
| `CI05-ADAPTER-MANUAL-01` | fonte manual/offline | Customer Data/Intelligence definidos |
| `CI05-ADAPTER-ERP-01` | adapter ERP | novo adapter + service público mínimo do ERP |
| `CI05-ADAPTER-CALENDAR-01` | business context Calendar | novo adapter + service público mínimo Calendar |
| `CI05-ADAPTER-SITE-01` | adapter de lead/tracking | novo adapter + service público mínimo Site |
| `CI05-ADAPTER-BI-01` | adapter de dataset BI | novo adapter; preservar trabalho BI do usuário |
| `CI05-FE-CONTRACT-01` | contratos TS de fontes | somente quando CI-08 despachar |
| `CI05-FE-UI-CONTRACT-01` | componentes de fonte | somente via `CI08-SOURCES-03` |
| `CI05-FE-MANUAL-CONTRACT-01` | timeline/form/import offline | somente quando CI-08 despachar |
| `CI05-QA-01` | falhas, limites, tenant e injection | testes/evidências focados |
| `CI05-CUTOVER-01` | ativação gradual de adapters | bindings/flags/evidências nominalmente listados |

## CI-06 — runtime e prompts

| Pacote | Resultado | Allowlist máxima |
|---|---|---|
| `CI06-DB-01` | agents/models/runtime | nova migration + ERD/AGENT |
| `CI06-DB-02` | mappings de tools/knowledge | nova migration separada + ERD/AGENT |
| `CI06-DB-03` | lineage `ai_legacy_mappings` | migration aditiva separada + ERD/AGENT; sem backfill/dual-write |
| `CI06-BE-MODULE-01` | módulo e composition contract | Customer Intelligence + wiring nominal |
| `CI06-BE-PROMPT-01` | registry/compiler/lifecycle | Customer Intelligence |
| `CI06-BE-CONTEXT-01` | context builder minimizado | Customer Intelligence |
| `CI06-BE-RUNTIME-01` | jobs/runs/leases/cancelamento | Customer Intelligence |
| `CI06-BE-MODELS-01` | profiles/providers/credentials | Customer Intelligence + platform adapter |
| `CI06-BE-TOOLS-01` | registry/gateway de tools | Customer Intelligence + adapters explícitos |
| `CI06-BE-KNOWLEDGE-01` | bindings de knowledge | Customer Intelligence + façade legada |
| `CI06-BE-EXEC-NATIVE-01` | executor nativo | Customer Intelligence |
| `CI06-BE-EXEC-N8N-01` | executor n8n sob o mesmo contrato | Customer Intelligence; sem workflow/deploy |
| `CI06-BE-JOBS-01` | workers do runtime | Customer Intelligence + wiring nominal |
| `CI06-API-01` | prompts/test/publish/rollback/runs | Customer Intelligence HTTP |
| `CI06-BACKFILL-01` | mapping/import de `messaging.ai_*` | comando/pacote dedicado + docs |
| `CI06-COMPAT-OMNI-01` | façade para APIs/config atuais | arquivos IA Omnichannel nominalmente listados |
| `CI06-FE-PROMPT-STUDIO-01` | contratos/UI do Prompt Studio | somente após gate CI-08 |
| `CI06-QA-01` | eval/schema/injection/fallback/paridade | testes/fixtures/evidências |
| `CI06-CUTOVER-01` | writer state por cliente/processo | mappings/flags/façades/evidências listados |
| `CI06-N8N-01` | **bloqueado:** alterar/importar/ativar workflow | nenhum até owner autorizar deploy separado |

## CI-07 — integração Omnichannel

| Pacote | Resultado | Allowlist máxima |
|---|---|---|
| `CI07-GATEWAY-01` | interface/adapter/wiring | Omnichannel + platform/app + Intelligence |
| `CI07-OUTBOX-02` | outcome accepted transacional | nova migration/store/teste focado |
| `CI07-FK-03` | referências aditivas | nova migration + backfill separado |
| `CI07-SHADOW-04` | comparação sem envio novo | services/jobs/metrics focados |
| `CI07-QA-05` | lease/humano/outbox/provider | testes de integração |

## CI-08 — frontend

| Pacote | Resultado | Allowlist máxima |
|---|---|---|
| `CI08-SHELL-01` | workspace/gates/nav | novos paths CI + 4 arquivos de wiring listados |
| `CI08-PROFILE-02` | lista/perfil/timeline | `web/app/**/customer-{data,intelligence}/**` |
| `CI08-OFFLINE-03` | timeline/form/import offline | paths Customer Data novos + página do perfil |
| `CI08-SOURCES-03` | fontes/health | mesmos paths novos |
| `CI08-PROMPTS-04` | Prompt Studio | mesmos paths novos |
| `CI08-INBOX-05` | card/links/seams | Omnichannel files explicitamente listados |
| `CI08-QA-06` | unit/browser/a11y/roles | testes e artifacts definidos |
| `CI08-SEGMENTS-07` | builder/version/preview/materialização/export separado | paths novos Customer Data/rota CI listados na spec |
| `CI08-RUNS-08` | lista/drawer de runs sanitizados CI-06 | página/componentes/composable/domain GET-only |
| `CI08-AUDIT-09` | audit-events/observations sanitizados CI-04 | página/componentes/composable/domain GET-only |

## CI-09 — recomendações/portfólio

| Pacote | Resultado | Allowlist máxima |
|---|---|---|
| `CI09-POLICY-01` | lifecycle/resolução de policies | Customer Intelligence, após `CI04-DB-03` |
| `CI09-BE-01` | recommendation domain | Customer Intelligence |
| `CI09-PROMPTS-02` | process prompts/evals | Prompt Registry fixtures/config |
| `CI09-API-03` | recommendations/feedback | Customer Intelligence |
| `CI09-FE-04` | UI/filtros/aprovação | paths novos de Customer Intelligence |
| `CI09-QA-05` | privacy/cohort/opt-out | testes focados |

## CI-10 — hardening/cutover

| Pacote | Resultado | Allowlist máxima |
|---|---|---|
| `CI10-OBS-01` | métricas/SLO/evals | módulos CI/Omni e docs explicitados |
| `CI10-LOAD-02` | carga/EXPLAIN | testes/scripts não destrutivos |
| `CI10-CUT-03` | flags/cutover/rollback | arquivos estritamente listados |
| `CI10-DEPRECATE-04` | métricas/headers | fachadas antigas |
| `CI10-REMOVE-*` | uma retirada por pacote | alvo único + migration nova, só com aprovação |
| `CI10-QA-05` | smoke/recovery/auditoria | testes/docs/artifacts |

## Handoff de despacho

Todo prompt de executor inclui:

- resultado único;
- spec/seção;
- decisões não reabríveis;
- pode ler;
- pode alterar;
- não pode alterar;
- contratos;
- passos;
- testes;
- aceite;
- stop conditions;
- handoff do contrato comum.
