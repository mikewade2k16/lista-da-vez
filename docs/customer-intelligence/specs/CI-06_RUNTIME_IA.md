# CI-06 — Runtime de IA headless e Prompt Registry

- **Status:** READY — implementação local autorizada; runtime nasce desligado
- **Versão:** 0.1
- **Data-base:** 2026-07-23
- **Owner proposto:** módulo `customer_intelligence`
- **Dependências:** CI-00, CI-03, CI-04 e CI-05
- **Bloqueia:** CI-07, CI-08, CI-09 e CI-10
- **Governança:** [GOVERNANCA.md](../GOVERNANCA.md), versão vigente
- **Blueprint:** [SPECS_GERAIS.md](../SPECS_GERAIS.md), versão vigente

> Esta spec define contratos futuros. Não implementa código, migration, workflow ou deploy. Em
> especial, não autoriza editar o workflow Omnichannel atual, criar workflow sem owner/ID aprovado,
> migrar credenciais reais ou retirar tabelas `messaging.ai_*`.

## 1. Resultado único e verificável

Criar um runtime de IA que:

- opera por API/job sem depender de inbox ou painel aberto;
- possui um prompt específico, versionado e publicável para cada `process_key`;
- compila deterministicamente guardrail de plataforma, política da agência, política do cliente,
  prompt do processo, override permitido do agente e contexto autorizado;
- resolve modelo, credencial, sources, tools, knowledge e limites por binding publicado;
- executa nativamente ou via n8n sob o mesmo contrato;
- valida a saída antes de qualquer efeito;
- permite draft, validação, testes, evals, simulação, shadow, canary, publish, rollback e stop;
- mantém compatibilidade auditável com agentes/versões/runs atuais do Omnichannel;
- registra custo, latência, versões, fontes e erro sem logar prompt bruto, segredo ou PII
  desnecessária.

O resultado é demonstrado com dois casos:

1. `profile.summary` forma um perfil a partir de ERP/manual via job headless sem conversa ou UI;
2. `conversation.reply` gera uma proposta válida, mas somente CI-07/Omnichannel pode transformá-la
   em mensagem `PENDING` + outbox.

## 2. Princípio: prompt governa comportamento; código governa segurança

Prompts são a principal camada de comportamento semântico e de customização do produto. Eles
definem persona, linguagem, objetivo, critérios de análise, estratégia de resposta, abordagem,
extração e raciocínio esperado para cada processo.

Prompts não substituem invariantes:

| Assunto | Governança |
|---|---|
| tom, linguagem, persona, estratégia, critérios semânticos | prompt versionado |
| thresholds, horários, modelo, timeout, tokens, rollout | policy estruturada no banco |
| tenant, RBAC, consentimento, FSM, lease, idempotência | Go/PostgreSQL |
| output shape e tipos | schema registrado + validação Go |
| sources/tools/knowledge disponíveis | interseção de registries/bindings/permissions |
| janela/capacidade do canal e envio | Omnichannel |
| retenção, sensibilidade e cross-client | policy + Go |

Uma alteração de comportamento segura deve ser possível pelo painel. Uma alteração que tente
desligar isolamento, validação, allowlist, consentimento ou sender é rejeitada mesmo que esteja no
prompt.

## 3. Decisões congeladas como proposta DRAFT

- módulo ID `customer_intelligence`;
- package Go `customerintelligence`;
- schema `intelligence`;
- API `/v1/customer-intelligence`;
- workspace `/inteligencia-clientes`;
- Customer Intelligence requer `customer_data`;
- Omnichannel consome o dispatcher como dependência opcional;
- `process_key` é estável, imutável e deprecável;
- processos diferentes não compartilham prompt implicitamente;
- prompt publicado e agent/model version publicados são imutáveis;
- binding publicado congela versões de todas as camadas;
- alteração de draft não muda produção;
- publish cria candidato imutável; rollout decide shadow/canary/full;
- rollback reponta resolução futura, sem reescrever histórico;
- execução resolve binding uma vez e grava snapshot antes de chamar provider;
- n8n é executor; não resolve tenant, binding, source/tool, segredo, FSM ou envio;
- `messaging.ai_dispatches` continua sendo a fila autoritativa da conversa;
- jobs headless pertencem ao runtime e nunca disputam ownership da conversa;
- prompts/modelos/credentials/tools/knowledge atuais migram sem dual-write permanente.

## 4. Estado atual medido no disco

O runtime não nasce do zero:

| Baseline existente | Semântica que deve ser preservada |
|---|---|
| `messaging.ai_agents` | identidade estável, enabled e active version |
| `messaging.ai_agent_versions` | draft/published/archived, versão imutável, provider/model/temp/layers/schema e publish/rollback |
| `messaging.ai_credentials` | segredo cifrado, write-only, `last4` |
| `messaging.ai_runs` | tentativa, status, usage, custo, latência e input mascarado |
| `messaging.ai_dispatches` | debounce, lease, generation, cancelamento e idempotência da conversa |
| `messaging.ai_tool_bindings/runs/approvals` | allowlist, modos read/propose/approved e auditoria |
| `messaging.knowledge_*` | base, documentos, chunks FTS e bindings |
| `messaging.media_analyses` | análise ligada a mensagem/agent version, usage e custo |
| `ai_prompt.go` | prompt em camadas, parte editável e parte server-side |
| `brain_context.go` | `brain.request.v2/v3` e contexto de conversa |
| `brain_executor.go` | executor n8n, gateway token efêmero e timeout de 75s |
| `brain_gateway.go` | gateway server-to-server para `platform/llm` |
| `AIToolRegistry` | registry explícito; sem URL/SQL arbitrário |
| `ConfigAiAgentVersions.vue` e configurações Omnichannel | UI existente de versão/publish/rollback/simulação |

Lacunas:

- prompt atual combina triagem, resposta e memória num contrato;
- `layers` não é Prompt Registry por `process_key`;
- bindings não separam agência/cliente/processo/agente;
- context builder depende de conversa;
- runs não registram bundle completo de prompt/context/source;
- UI e APIs pertencem ao Omnichannel;
- não existe job headless genérico;
- não existem eval suites/canary por processo;
- workflow atual pertence ao Omnichannel e não é runtime genérico autorizado.

É proibido afirmar que “não existem prompts/versionamento” ou criar tabelas concorrentes sem plano
de compatibilidade.

## 5. Catálogo inicial de processos

Cada linha possui `ProcessDefinition`, `ProcessConfigVersion`, Prompt Definitions por camada,
input/output schema, variables, source/tool policy, modelo, limites, eval suite e failure mode
independentes.

### 5.1 Processos e contratos

| `process_key` | Entrada principal | Saída schema-validada | Falha segura |
|---|---|---|---|
| `conversation.triage` | turno, estado, campos e catálogo de destinos | intenção, categorias, lead stage, campos candidatos, confiança, necessidade/motivo humano | handoff/fallback Omnichannel |
| `conversation.reply` | mensagem, contexto e decisão de continuar | `reply_draft`, idioma, confiança, warnings e propostas de tool | sem resposta + humano |
| `conversation.handoff_summary` | contexto aceito e motivo | resumo, motivo, campos coletados/pendentes e redactions | resumo determinístico mínimo |
| `memory.extract` | observações aceitas | claims candidatos com evidence refs e confiança | retry/dead-letter; nenhum fato |
| `profile.summary` | fatos/evidências autorizados | síntese, seções, evidence refs e confiança | perfil anterior stale |
| `recommendation.follow_up` | perfil, consentimento e horários | instante/canal/janela/racional/expiração | nenhuma recomendação |
| `recommendation.offer` | perfil + catálogo permitido | referências de catálogo, racional e validade | nenhuma oferta |
| `recommendation.important_dates` | fatos/evidências temporais | data, tipo, recorrência, confiança e evidência | nenhuma data |
| `source.suggest` | lacunas + catálogo de capabilities | source key registrada, lacunas, racional, confiança | nenhuma sugestão |
| `portfolio.opportunity` | agregados suprimidos | segmento, target client, afinidade, racional e coorte | nenhuma oportunidade |
| `media.image_analysis` | referência de mídia autorizada | descrição, campos candidatos, safety flags | blocked/failed |
| `media.document_analysis` | referência e páginas autorizadas | campos/chunks limitados e safety flags | blocked/failed |
| `quality.review` | atendimento sanitizado | scores, evidências, problemas e coaching | review indisponível |

### 5.2 Regras por processo

#### `conversation.triage`

- não gera envio;
- não escreve fila;
- `suggested_department/queue` precisam existir no catálogo;
- campo extraído precisa existir em fact/collect field definition;
- pedido humano, tema sensível e baixa confiança são sinais; policy Go decide efeito;
- não inclui `reply_draft`.

#### `conversation.reply`

- executa somente quando triage/policy permitem;
- texto respeita canal, idioma, janela e tamanho recebidos como constraints;
- tool call é proposta estruturada;
- não promete ação que não foi executada;
- saída não contém comando `send`, provider, número ou URL de canal.

#### `conversation.handoff_summary`

- não inclui segredo, pagamento, documento ou PII sem necessidade;
- evidence refs são IDs de mensagens/observações permitidos;
- não altera state/handoff por conta própria.

#### `memory.extract`

```json
{
  "claims": [{
    "factKey": "preferred_name",
    "valueType": "string",
    "value": "Ana",
    "confidence": 0.83,
    "evidenceObservationIds": ["uuid"],
    "validFrom": null,
    "validUntil": null
  }]
}
```

Go rejeita chave, tipo, sensibilidade ou evidence ref fora do contexto. Claims seguem CI-04.

#### `profile.summary`

- usa fatos resolvidos e conflitos permitidos;
- devolve evidence refs exatas;
- mesma fingerprint pode reutilizar versão;
- não transforma recomendação em fato.

#### Recomendações

- follow-up respeita opt-out, consentimento e horário antes de publicar;
- oferta referencia catálogo; não inventa produto/preço;
- data importante exige evidência, distingue confirmada de inferida;
- todas possuem expiração e feedback em CI-09.

#### `source.suggest`

- só retorna `source_key` registrada;
- não habilita nem pede credencial;
- não sugere scraping/URL/SQL arbitrário.

#### `portfolio.opportunity`

- recebe somente agregado já sujeito a policy;
- saída não contém subject, nome, telefone, e-mail ou identificador individual;
- coorte/supressão de CI-09 vence o prompt.

#### Mídia

- binário permanece no storage privado;
- runtime recebe URL interna assinada/stream autorizado por TTL;
- análise não persiste arquivo/base64;
- imagem e documento têm prompts/schemas distintos;
- áudio/transcrição existente não é silenciosamente tratado como imagem/documento.

#### `quality.review`

- produz apoio/coaching, não decisão trabalhista automática;
- evidence refs e rubrica são auditáveis;
- conteúdo sensível é minimizado.

### 5.3 Templates iniciais e ausência de fallback escondido

Cada `process_key` nasce com template inicial próprio e versionado. O bootstrap pode usar assets
versionados para criar o primeiro draft/published binding no PostgreSQL, mas:

- PostgreSQL passa a ser a fonte efetiva;
- bootstrap cria ausentes e nunca sobrescreve versão já existente;
- template guarda lineage/checksum;
- editar pelo painel cria nova versão;
- nenhum processo cai em prompt padrão de outro processo;
- Go/n8n não possuem texto de comportamento como fallback invisível;
- sem binding/prompt válido, o processo retorna `not_configured` e aplica seu failure mode;
- importação do prompt legado produz drafts para revisão, não publish automático.

Guardrails técnicos mínimos podem existir em código apenas como validação/contrato de segurança;
persona, estratégia, tom, objetivo e critérios semânticos sempre vêm das versões persistidas.

### 5.4 Áudio e vídeo existentes — compatibilidade sem ampliar o catálogo

O estado atual do Omnichannel já executa `transcription` e `video_summary` em
`messaging.media_analyses`, configurados pelo `media_config` versionado. Eles não estão nas 13
chaves aprovadas pelo catálogo inicial de CI-00/GOVERNANCA e, por isso:

- continuam no writer/runtime legado do Omnichannel durante esta fase;
- não reutilizam silenciosamente `media.image_analysis` ou `media.document_analysis`;
- não recebem binding, prompt ou run novo sob uma chave inventada;
- mantêm configuração, IDs, dedupe, retenção, retry e rollback atuais;
- podem alimentar uma conversa apenas pelo resultado sanitizado que o Omnichannel já autoriza.

As chaves candidatas `media.audio_transcription` e `media.video_summary` ficam reservadas apenas
para uma futura decisão de governança. Ativá-las exige, antes do cutover: acrescentar o catálogo
canônico em CI-00, owner, schemas próprios, Prompt/Process Definitions e config versionada, template
próprio, modelos/capabilities, casos de teste, binding publicado, shadow comparativo, mapping de IDs
legados e rollback ensaiado. Até lá, a UI nova deve mostrar esses dois processos como “legado
gerenciado no Omnichannel”, não como ausência nem como prompt genérico.

## 6. Pipeline de conversa sem mega-prompt

```text
InteractionRequest.v1 (pipeline conversation.respond)
  -> conversation.triage -> ProcessResult.v1
  -> policy Go estruturada valida classificação/campos/handoff/close proposal
  -> se permitido ou se fechamento exigir mensagem final:
       conversation.reply -> ProcessResult.v1
  -> coordenador compõe InteractionDecision.v1
  -> Omnichannel revalida lease/FSM/policy
  -> mensagem PENDING + outbox
  -> após outcome aceito: memory.extract assíncrono
  -> profile.summary/recommendations quando invalidados
```

No primeiro rollout, chamadas separadas podem aumentar custo/latência. A otimização permitida é
selecionar modelo menor por processo, reduzir contexto e reutilizar snapshots. “Fundir” processos
numa chamada só exige contrato explícito futuro, duas saídas independentes, versões auditadas e
prova de paridade; não pode reintroduzir prompt compartilhado implícito.

CI-07 envia um único `InteractionRequest.v1` de alto nível, sem `processKey`. O coordenador resolve
a versão publicada de `conversation.respond`, cria um `ContextRequest.v1` singular para cada etapa
e persiste um `runtime_run`/`ProcessResult.v1` por prompt. A ordem, branches e hard caps vêm de
`PipelineVersion`, editável pelo painel sob catálogo fechado; texto de prompt não escolhe o fluxo.

Triagem nunca é disfarçada como `no_reply` ou decisão final. Se houver proposta de fechamento, a
etapa de resposta gera a mensagem final exigida pela policy e a composição preserva
`closure.requested/reason/confidence`. O Omnichannel continua sendo quem decide e executa
`SystemTryAutoClose`.

## 7. Camadas e compilação determinística

### 7.1 Ordem

```text
1 platform_guardrail
2 agency_policy
3 client_policy
4 process_prompt
5 agent_override permitido
6 runtime_constraints geradas pelo Go
7 runtime_context tratado como dados não confiáveis
8 output_contract registrado
```

As cinco primeiras vêm do binding CI-04. O compilador mantém as referências de camada separadas;
não depende apenas da posição textual para resolver precedência. As três últimas são geradas pelo
runtime, não são texto livre do tenant.

### 7.2 Precedência

- `platform_guardrail` possui autoridade máxima;
- em comportamento semântico não protegido, a especificidade é
  `agent_override > client_policy > agency_policy > process_prompt`;
- camada mais específica pode ajustar comportamento, mas não ampliar tool, fonte, dado sensível,
  permissão ou capacidade fora das policies estruturadas;
- `agent_override` só usa variables/capabilities permitidas pela process config e pelo slot;
- `runtime_context` fica delimitado e instruído como dados, nunca como comando;
- texto de mensagem, ERP, site, documento, knowledge e tool output é não confiável;
- `output_contract` não é editável por prompt;
- conflito entre camadas bloqueia validate/publish quando detectável e gera warning em runtime;
- plataforma pode deprecar variável/processo com janela de migração, sem reinterpretar chave.

### 7.3 Algoritmo

1. validar account/client/subject/relationship/purpose;
2. resolver process definition e `process_config_version_id` published;
3. verificar kill switch e capability;
4. resolver rollout/bucket;
5. carregar binding published e conferir que ele fixa a mesma process config;
6. intersectar source/tool/knowledge policy;
7. resolver variables pelo catálogo;
8. montar context snapshot token-budgeted;
9. serializar camadas em ordem e delimitadores estáveis;
10. calcular `compiled_fingerprint`;
11. persistir run/snapshot antes do provider;
12. executar native/n8n;
13. validar tamanho, JSON/schema, refs e regras do processo;
14. persistir output mascarado/usage;
15. devolver proposta sem efeito operacional implícito.

Mesmos IDs, process config, versões, variables e contexto ordenado produzem o mesmo fingerprint,
ainda que o provider continue não determinístico.

## 8. Variáveis tipadas

### 8.1 Tipos permitidos

- `string`;
- `string_list`;
- `integer`;
- `decimal`;
- `boolean`;
- `enum`;
- `date`;
- `timestamp`;
- `object_closed`;
- `message_list`;
- `evidence_list`;
- `fact_list`;
- `catalog_list`.

Objetos/arrays possuem schema fechado, quantidade e bytes máximos. `any`, template executable,
expressão, path livre, SQL e URL livre são proibidos.

### 8.2 Catálogo

Cada variável declara:

- `variable_key`;
- `value_type`;
- `resolver_key` Go;
- layers permitidas;
- required;
- missing behavior `fail|omit|default`;
- sensitivity;
- purpose allowlist;
- max chars/items/tokens;
- token priority;
- truncation strategy;
- redaction strategy;
- exemplo sintético.

Exemplos iniciais:

| Variável | Resolver | Tipo | Regra |
|---|---|---|---|
| `client.profile` | `business_context.client_profile` | `object_closed` | cliente atual |
| `relationship.summary` | `intelligence.summary.current` | `string` | pode estar stale |
| `relationship.facts` | `intelligence.facts.allowed` | `fact_list` | somente fact keys permitidas |
| `conversation.messages` | `omnichannel.messages.window` | `message_list` | paginação/lease |
| `conversation.pending_fields` | `omnichannel.collect_fields.pending` | `string_list` | catálogo |
| `conversation.collect_field_definitions` | `omnichannel.collect_fields.definitions` | `catalog_list` | tipos/opções permitidos; sem valor livre |
| `routing.catalog` | `omnichannel.routing.catalog` | `catalog_list` | somente sugestão |
| `source.freshness` | `intelligence.sources.freshness` | `object_closed` | warnings explícitos |
| `product.catalog` | `source.catalog.allowed` | `catalog_list` | referências reais |
| `runtime.local_time` | `runtime.clock.account_timezone` | `timestamp` | server-side |

### 8.3 Placeholders

Sintaxe proposta: `{{ variable.key }}` sem funções, includes dinâmicos ou avaliação.

- placeholder desconhecido bloqueia validate;
- required ausente bloqueia execução/publish conforme policy;
- optional ausente é omitido com marcador explícito;
- default contém apenas valor não sensível;
- valor é escapado/delimitado;
- segredo nunca é variável de prompt;
- preview exibe dado sintético ou mascarado.

## 9. Modelo persistente do runtime

Prompt tables pertencem a CI-04. CI-06 acrescenta runtime/agents/models/credentials/tools/knowledge.

### 9.1 `intelligence.ai_agents`

| Coluna | Regra |
|---|---|
| `id`, `account_id` | UUID/FK tenant-safe |
| `slug`, `name` | unique por conta |
| `enabled` | gate administrativo |
| `active_version_id` | versão published |
| `legacy_agent_id` | mapping opcional para `messaging.ai_agents` |
| `created_by_user_id`, `created_at`, `updated_at` | auditoria |

### 9.2 `intelligence.ai_agent_versions`

Bundle imutável de comportamento operacional do agente:

| Coluna | Regra |
|---|---|
| `id`, `account_id`, `agent_id` | tenant-safe |
| `version` | único por agent |
| `status` | `draft`, `validated`, `published`, `archived` |
| `default_failure_policy` | policy estruturada |
| `conversation_limits` | debounce/context/turns dentro dos caps |
| `media_policy` | capabilities |
| `legacy_agent_version_id` | referência de compatibilidade opcional; lineage autoritativo fica em `ai_legacy_mappings` |
| `published_by_user_id`, `published_at`, `created_at` | auditoria |

Prompt/model/tool/source não ficam como JSON opaco aqui; junctions versionadas apontam cada
`process_key`.

O lifecycle funcional dessa tabela não recebe estados de migração. Uma versão importada nasce
sempre como `draft`; somente validate/publish normais podem promovê-la a `validated`/`published`.
`imported`, `shadow` ou `cutover` são estados exclusivos do mapping e não valores de `status`.

### 9.2.1 `intelligence.ai_legacy_mappings`

Contrato autoritativo de lineage e progresso da migração, separado do lifecycle das entidades-alvo:

| Coluna | Regra |
|---|---|
| `id`, `account_id` | UUID/FK tenant-safe |
| `client_account_id` | nullable no inventário; obrigatório antes de criar `agent_override`/binding client-scoped |
| `legacy_entity_type`, `legacy_entity_id` | tipo fechado + ID em `messaging.ai_*` |
| `target_entity_type`, `target_entity_id` | tipo fechado + ID nullable enquanto inventariado |
| `legacy_schema_version` | versão do contrato/builder lido |
| `source_hash` | SHA-256 do envelope legado canônico, nunca do ciphertext de segredo |
| `transform_version` | versão imutável do algoritmo de import/split |
| `migration_state` | `inventoried`, `imported`, `review_required`, `shadow`, `validated`, `cutover`, `failed` |
| `split_map` | objeto fechado source JSON pointer → targets/modo/hash |
| `unmapped_paths` | lista fechada de paths/reason codes pendentes |
| `target_hash` | hash do bundle de targets ordenados; nullable antes do import |
| `idempotency_key` | derivada de account/client/tipo/ID/source hash/transform version |
| `mapping_revision` | CAS das decisões humanas ainda não validadas |
| `last_successful_state` | estado retomado por retry após `failed` |
| `imported_at`, `reviewed_at`, `validated_at`, `cutover_at` | milestones nullable |
| `reviewed_by_user_id` | nullable; obrigatório ao sair de `review_required` |
| `error_code`, `error_detail_masked` | erro fechado e detalhe sem PII/segredo |
| `created_at`, `updated_at` | auditoria |

Unique tenant-safe por
`(account_id, client_account_id, legacy_entity_type, legacy_entity_id, source_hash,
transform_version)`, com índice parcial explícito para `client_account_id is null`, e
`idempotency_key`. Uma linha nunca troca `source_hash`, `transform_version` ou targets já
validados; mudança do source cria novo lineage/candidates. `split_map` registra, por entrada:

```json
{
  "/layers/identity": [{
    "targetProcessKey": "conversation.reply",
    "targetLayerKind": "agent_override",
    "targetVersionId": "uuid",
    "mode": "materialized|copied|transformed|manual",
    "sourceValueHash": "sha256",
    "targetValueHash": "sha256"
  }]
}
```

O payload bruto legado não é duplicado nessa tabela. Mapping de credential registra apenas ID,
provider e hashes/metadados permitidos; backfill nunca decifra ou calcula hash da chave.

Transições permitidas:

```text
inventoried -> imported -> review_required -> validated -> shadow -> cutover
inventoried -> imported ---------------------> validated -> shadow -> cutover
qualquer estado pré-cutover -> failed -> last_successful_state por retry idempotente
```

`imported -> validated` só é aceito quando `unmapped_paths=[]`, todos os oito itens do contrato
abaixo têm target/hash e não houve transformação semanticamente ambígua. `cutover` exige shadow
aprovado e writer state compatível. Rollback operacional não apaga lineage nem reabre target
published; cria binding/writer transition auditada.

### 9.3 `intelligence.agent_pipeline_bindings`

| Coluna | Regra |
|---|---|
| `id`, `account_id`, `client_account_id`, `agent_version_id` | escopo tenant-safe |
| `pipeline_definition_id`, `pipeline_version_id`, `pipeline_key` | versão published |
| `status` | `draft`, `published`, `archived` |
| `rollout_policy` | off/shadow/canary/full com bucket determinístico |
| `runtime_caps` | interseção com hard caps da definição |
| `revision`, autores e timestamps | concorrência/auditoria |

Unique `(account_id, client_account_id, agent_version_id, pipeline_key)` com tratamento explícito
de `NULL`. Alterar ordem/branch pelo painel cria nova `pipeline_version`; o binding publicado apenas
seleciona versão/rollout e nunca carrega código, expressão livre ou prompt embutido.

### 9.4 `intelligence.agent_process_bindings`

| Coluna | Regra |
|---|---|
| `id`, `account_id`, `agent_version_id` | tenant-safe |
| `process_definition_id`, `process_config_version_id`, `process_key` | catálogo/config published |
| `prompt_binding_id` | binding published que fixa a mesma config |
| `model_profile_version_id` | modelo published |
| `enabled` | por processo |
| `source_policy` | interseção allowlisted |
| `tool_policy` | interseção allowlisted |
| `knowledge_policy` | bindings permitidos |
| `runtime_policy` | timeout, output, retry dentro de caps |

Unique `(account_id, agent_version_id, process_key)`.

### 9.5 `intelligence.ai_credentials`

Preserva semântica da migration 0234:

| Coluna | Regra |
|---|---|
| `id`, `account_id`, `name`, `provider` | catálogo |
| `secret_ciphertext`, `secret_last4` | segredo cifrado/write-only |
| `status` | `active`, `rotating`, `revoked` |
| `created_by_user_id`, `rotated_at`, `created_at`, `updated_at` | auditoria |
| `legacy_credential_id` | mapping transitório |

API nunca devolve ciphertext nem chave.

### 9.6 `intelligence.model_profiles` e `model_profile_versions`

Stable profile + versão imutável:

| Campo versionado | Regra |
|---|---|
| provider/model | catálogo `platform/llm` |
| credential_id | mesma conta/provider compatível |
| base_url_policy_key | allowlist server-side; não URL tenant livre |
| temperature | range da plataforma |
| max_output_tokens | cap |
| timeout_ms | cap |
| schema_retry_count | 0..1 inicialmente |
| provider_retry_count | 0..1, apenas erro seguro |
| cost_policy | budget/alert/stop |
| capabilities | structured output/media/tools conforme provider |
| status/version/published | imutabilidade |

### 9.7 Tools

Target preserva:

- registry Go explícito;
- `tool_bindings` por account/agent/process;
- modes `read`, `propose_write`, `approved_write`;
- operations/input/output schemas fechados;
- timeout e max calls;
- `tool_runs` com input/output mascarados;
- approvals separadas;
- idempotency/call ID;
- nenhum endpoint/SQL/credential no prompt.

Effective allowlist:

```text
registered tool
∩ process config capabilities
∩ prompt binding tool policy
∩ agent process binding
∩ account/client permission
∩ runtime purpose/consent
```

### 9.8 Knowledge

Target preserva bases/documentos/chunks/bindings do legado:

- documento versionado, checksum e status;
- chunk limitado;
- FTS inicial; vector somente em spec própria;
- binding por processo/agente, top-k/min-score;
- source/purpose/sensitivity;
- conteúdo tratado como dado não confiável;
- nenhuma credencial;
- exclusão/retention invalida contexts/runs derivados.

Knowledge atual não é copiado como segunda verdade. Writer state e cutover seguem §18.

### 9.9 `intelligence.runtime_runs`

Uma linha por tentativa:

| Coluna | Regra |
|---|---|
| `id`, `account_id`, `client_account_id` | escopo |
| `subject_id`, `relationship_id` | conforme processo |
| `request_id`, `interaction_id`, `decision_id`, `correlation_id`, `causation_id`, `idempotency_key` | correlação |
| `pipeline_definition_id`, `pipeline_version_id` | nullable em processo headless; obrigatório em conversa |
| `process_definition_id`, `process_config_version_id`, `process_key`, `purpose_key` | catálogo e configuração congelada |
| `agent_id`, `agent_version_id`, `prompt_binding_id`, `model_profile_version_id` | versões |
| `context_snapshot_id` | CI-04 |
| `executor` | `native`, `n8n` |
| `status` | `queued`, `processing`, `succeeded`, `schema_invalid`, `provider_error`, `blocked`, `cancelled`, `limit_exceeded`, `stale_result` |
| `output_schema_version` | contrato |
| `input_fingerprint`, `compiled_prompt_fingerprint` | hashes |
| `output_masked` | JSON limitado |
| `provider`, `model` | auditoria |
| tokens/cost/latency/attempts | métricas |
| `error_code`, `error_detail_masked` | seguro |
| `started_at`, `completed_at`, `created_at` | timestamps |

Unique `(account_id, idempotency_key, process_key)`.

### 9.10 `intelligence.runtime_jobs`

Somente processos headless, nunca dispatch de conversa:

| Coluna | Regra |
|---|---|
| IDs/escopo/process/purpose | referências |
| `payload_refs` | IDs e versões, sem prompt/PII bruto |
| `status` | `queued`, `processing`, `completed`, `failed`, `dead_letter`, `cancelled` |
| `run_after`, `locked_at`, `attempts`, `max_attempts` | lease |
| `idempotency_key` | unique escopada |
| `runtime_run_id`, error safe, timestamps | auditoria |

Hot path usa índice parcial `(status, run_after, created_at, id)`.

## 10. Contratos de execução

### 10.1 Request interno

```go
type ExecutionRequest struct {
    SchemaVersion  string
    RequestID      string
    IdempotencyKey string
    AccountID      string // derivado/validado pelo caller interno
    ClientAccountID string
    SubjectID, RelationshipID *string
    ProcessKey, PurposeKey string
    AgentID *string
    Invocation InvocationRef
    OperationalLease *OperationalLease
    Input json.RawMessage
}

type InvocationRef struct {
    Kind string // conversation | headless | simulation | replay
    SourceEntityType, SourceEntityID, SourceVersion string
}

type OperationalLease struct {
    ConversationID, DispatchID string
    Generation int64
}
```

Public API nunca confia em `AccountID` do body. Internal gateway valida token claims contra todos os
IDs de escopo. `Input` obedece schema do processo e não substitui resolvers.

### 10.2 Context envelope

```go
type ContextEnvelope struct {
    SchemaVersion string // context.envelope.v1
    RequestID, SnapshotID string
    AccountID, ClientAccountID string
    SubjectID, RelationshipID *string
    AsOf time.Time
    Sections []ContextSection
    SourceStatuses []ContextSourceStatus
    EvidenceRefs []EvidenceRef
    Budget ContextBudget
    PromptBindingID string
    ExpiresAt time.Time
    Warnings []string
}
```

Esse é o envelope canônico de CI-00. `variables` e `fact refs` compilados são representação interna
das `Sections`; não criam um segundo contrato cross-module.

### 10.3 `ProcessResult.v1` e composição final

```go
type ProcessResult struct {
    SchemaVersion, RequestID, RunID, ProcessKey string
    InteractionID string
    AccountID, ClientAccountID string
    SubjectID, RelationshipID *string
    Status string
    OutputSchemaVersion string
    Output json.RawMessage
    PromptBindingID string
    PromptVersionRefs []VersionRef
    ModelRef ModelRef
    ContextSnapshotID string
    Warnings []string
    Usage RuntimeUsage
}
```

O tipo corresponde ao `ProcessResult.v1` canônico de CI-00. Trace detalhado permanece no
`runtime_run` e expõe apenas IDs/versões e sources/tools usados, nunca prompt compilado ou segredo.
Cada processo converte `Output` para tipo Go próprio antes da composição.

O coordenador de `conversation.respond` valida `conversation.triage.result.v1`, decide o branch com
policy estruturada, valida `conversation.reply.result.v1` quando executado e então compõe uma única
`InteractionDecision.v1` com `pipelineVersionId` e `processRunRefs[]`. Resultado intermediário não
é retornado ao Omnichannel como mensagem/handoff/close e não possui efeito operacional.

## 11. Context builder

### 11.1 Regras

- parte de `account/client/relationship/purpose`;
- consulta somente sources habilitadas e fact keys permitidas;
- pagina cada fonte;
- ordena por autoridade, relevância, frescor e ID estável;
- separa fatos, evidências, business context, messages e knowledge;
- registra stale/ausente/conflito/omissão;
- nunca usa fallback de outro cliente;
- aplica orçamento por seção antes de serializar;
- snapshots são cifrados e expiram;
- variável não resolvida segue `missing_behavior`;
- source failure opcional degrada; required bloqueia apenas o processo;
- prompt injection em conteúdo não ganha precedência.

### 11.2 Orçamento

Ordem default configurável por processo:

1. guardrails/output contract;
2. última mensagem/objetivo;
3. fatos verificados;
4. resumo atual;
5. mensagens recentes;
6. evidências relevantes;
7. business context;
8. knowledge;
9. dados auxiliares.

Truncation nunca corta JSON inválido nem mistura mensagens. `omission_codes` explica cada corte.

## 12. Lifecycle de prompt e binding

```text
draft
  -> validate
  -> test/eval
  -> publish immutable candidate
  -> shadow
  -> canary
  -> full
  -> rollback ou archive
```

### 12.1 Draft

- editável somente com `expectedRevision`/ETag;
- pode clonar published;
- não afeta runtime;
- diff mostra camada, variables, schema/tool/source/model refs;
- autosave não publica.

### 12.2 Validate

Bloqueia:

- placeholder desconhecido/duplicado;
- variável required ausente;
- camada proibida;
- conteúdo/tamanho acima do cap;
- binding incompleto;
- schema incompatível;
- source/tool/model/credential indisponível;
- guardrail ausente;
- referência cross-tenant;
- process key deprecada;
- tentativa de inserir segredo.

### 12.3 Test/eval

- roda casos required;
- registra custo real quando chama provider;
- pode usar provider mock deterministicamente;
- write tools ficam `deny`;
- resultados não geram fato, recommendation, mensagem ou ação.

### 12.4 Publish

- exige permissão `customer_intelligence.prompts.publish`;
- revalida revision e eval policy;
- cria versões/binding immutable;
- registra diff, autor, data e change summary;
- não altera rollout ativo implicitamente.

### 12.5 Shadow/canary/full

- shadow executa candidato sem efeito e compara baseline;
- canary usa bucket determinístico;
- full resolve candidate para novas execuções;
- cada rollout possui baseline e stop criteria;
- runs claimed mantêm binding já resolvido.

### 12.6 Rollback

- ação explícita, reason obrigatório;
- reponta novas resoluções ao baseline published;
- cancela/pausa candidate jobs ainda não claimed;
- não reescreve versões, runs, outputs ou fatos;
- conversa continua com sender Omnichannel;
- rollback técnico do executor e rollback semântico do prompt são ações distintas.

## 13. Resolução de binding e canary

Ordem:

1. account + client + agent + process;
2. account + client + process;
3. default da account + process;

`client_account_id` é null apenas no terceiro caso e `agent_id` também deve ser null. Defaults de
plataforma são templates de provisionamento: precisam ser materializados como binding account-wide
tenant-owned antes de entrar nessa resolução. O runtime nunca lê binding com `account_id null` nem
herda template global silenciosamente. Sem binding publicado, retorna `not_configured` e usa o
failure mode seguro. Não existe fallback entre clientes.

Bucket proposto:

```text
HMAC(bucket_key_version,
  account_id + client_account_id + relationship_id_or_invocation_id
  + process_key + rollout_id
) % 10000
```

`traffic_percent=5` seleciona buckets `<500`. A mesma relação permanece no mesmo braço durante o
rollout. Chave/versão do bucket é interna; não contém PII em claro.

## 14. Evals e gates de publicação

### 14.1 Tipos

- schema/type validation;
- exact/enum/range;
- required/forbidden substring;
- regex segura precompilada;
- source/tool allowlist;
- evidence reference validity;
- no-PII/no-secret;
- tenant fixture isolation;
- semantic rubric com judge versionado;
- comparação humana;
- custo/latência/token;
- regression diff contra baseline.

Judge LLM não é a única prova de segurança. Tenant, schema, allowlist, segredo e refs usam asserts
determinísticos.

### 14.2 Policy proposta

Para publish:

- 100% de schema, tenant, secret, tool/source e safety cases;
- zero critical failure;
- todos os casos `required_for_publish`;
- score funcional mínimo configurado por processo;
- regressão máxima de custo/latência configurada;
- pelo menos uma aprovação humana quando processo é sensível/cross-client;
- modelo/evaluator versions registradas.

Valores funcionais e quantitativos permanecem DRAFT até CI-10; safety 100% não é flexibilizável.

### 14.3 Casos de teste

- preferir fixture sintética;
- replay histórico exige finalidade/permissão;
- dados reais são cifrados, minimizados e expiram;
- nenhuma fixture contém credential;
- caso publicado é versionado;
- alteração da suite invalida gate anterior conforme policy.

## 15. Simulador e Prompt Studio

### 15.1 Modos

- input sintético;
- fixture salva;
- replay histórico autorizado;
- comparação baseline × candidate;
- provider real;
- provider mock.

### 15.2 Side effects

`sideEffectMode=deny` é obrigatório:

- não cria mensagem/outbox;
- não altera conversa;
- não resolve/escreve fato;
- não publica summary/recommendation;
- tool read usa snapshot/replay quando possível;
- tool write é rejeitada;
- source externa cara exige confirmação e conta custo;
- simulação grava run/eval com `invocation=simulation`.

### 15.3 Resultado

- output dos dois bindings;
- diff estruturado;
- schema/errors;
- variable/source omissions;
- tool proposals;
- tokens/custo/latência;
- scores/asserts;
- versões/fingerprints;
- aviso explícito quando usou provider/dado real.

Prompt bruto publicado só é visível a permissões próprias; audit viewer comum recebe metadata/diff
mas não necessariamente conteúdo.

## 16. Executor nativo e n8n

### 16.1 Interface única

```go
type Executor interface {
    Execute(context.Context, CompiledExecution) (RawExecutionResult, error)
}
```

`CompiledExecution` já contém binding, prompt compilado, context snapshot, output schema, model
profile e tool bindings. Executor não escolhe nenhum deles.

### 16.2 Native

- usa `platform/llm`;
- structured output quando provider suporta;
- timeout/cancelamento do context;
- usage/custo normalizados;
- retry somente conforme policy;
- nenhum estado global por subject.

### 16.3 n8n

- recebe envelope versionado compilado pelo Go;
- pode orquestrar modelo/multimodal/tools autorizadas;
- usa gateway token opaco, curto, audience/run/account scoped;
- persistência de executions/pin/static data desabilitada;
- não guarda memória;
- não muda prompt/binding;
- não chama PostgreSQL;
- não envia a canal;
- tool call retorna ao gateway Go assinado;
- saída volta ao Go para schema/policy validation.

### 16.4 Ownership de workflow

- `omnibrain0000001` continua owner Omnichannel e pode atender somente o caminho legado/compatível
  de conversa até cutover aprovado;
- CI-06 não edita esse arquivo;
- headless começa `native` se não houver workflow genérico aprovado;
- workflow genérico de Customer Intelligence só pode nascer após CI-00/E0 registrar owner, nome,
  ID canônico, export path e rollback;
- nenhuma spec pode reutilizar WAHA/Automation, Calendar, Operação ou social-publishing.

### 16.5 Paridade

Native e n8n recebem o mesmo fingerprint/output schema. Test suite executa ambos e compara:

- status/schema;
- allowed sources/tools;
- result classification;
- cancellation;
- timeout;
- secret absence;
- usage normalization.

## 17. Limites determinísticos

Configurações são editáveis no painel dentro de caps da plataforma. Caps são Go, visíveis para
auditoria e não sobrescritos por prompt.

| Limite | Compatibilidade/default proposto | Cap inicial |
|---|---:|---:|
| body interno n8n | atual 4 MiB | 4 MiB |
| timeout brain | atual 75s | 120s; conversa default 75s |
| context messages | atual 30 | 100 |
| AI turns conversa | atual permite 0=sem limite | 0..100; policy de segurança pode impor teto |
| debounce conversa | atual 2500ms | 500..15000ms |
| tool arguments | atual 64 KiB | 64 KiB |
| tool output | atual 128 KiB | 128 KiB |
| tool calls/process | atual default 4, cap 20 | 20 |
| schema retries | atual 1 | 1 |
| provider retries | 0 | 1 apenas erro seguro/idempotente |
| output JSON | 128 KiB | 128 KiB |
| prompt layer | 32 mil caracteres | 64 mil caracteres |
| compiled prompt/context | por modelo e process policy | menor entre policy e model window |
| source page | descriptor | máximo descriptor |
| evidence refs/output | 100 | 200 |
| context snapshot TTL | process policy | máximo aprovado por retenção |

Outros gates:

- rate/concurrency por account/process;
- budget mensal por account e opcional por cliente/processo;
- max custo/run;
- max headless attempts;
- circuit breaker de provider/source;
- cancelamento cooperativo;
- kill switch account/client/process.

Clamp é explícito na API; valor fora do range retorna validação, não fallback silencioso.

## 18. Compatibilidade e migração do runtime atual

### 18.1 Mapeamento

`intelligence.ai_legacy_mappings` preserva lineage sem transformar estado de migração em estado
funcional da entidade-alvo:

- `legacy_agent_id -> intelligence.ai_agents.id`;
- `legacy_agent_version_id -> intelligence.ai_agent_versions.id`;
- `legacy_credential_id -> intelligence.ai_credentials.id`;
- `legacy_ai_run_id <-> intelligence.runtime_runs.id`, quando houver execução espelhada;
- tool/knowledge mapping por entidade.

As colunas `legacy_*_id` nas entidades-alvo são referências transitórias de compatibilidade; em
caso de divergência, `ai_legacy_mappings` é autoritativa e o validate falha. Elas não guardam
`migration_state`.

`messaging.ai_dispatches.agent_version_id` permanece válido até CI-07 criar referências aditivas e
trocar consumidores. Não se quebra FK existente.

### 18.2 Prompt legado

`messaging.ai_agent_versions.layers` é baseline parcial real. O envelope canônico de origem inclui
ID/account/agent/version/status, as quatro propriedades editáveis de `layers`, provider/model/
temperature, `output_schema`, `schema_version`, policies/configs de runtime e a versão identificada
do builder `ai_prompt.go`. Campos são ordenados, `null` e ausência permanecem distintos, timestamps
voláteis são excluídos e o resultado produz `source_hash`.

Os oito componentes efetivos do prompt atual possuem mapping obrigatório e determinístico:

| # | Origem efetiva | Target no runtime novo | Modo/regra |
|---|---|---|---|
| 1 | `layers.identity` | `agent_override` draft distinto de `conversation.triage` e `conversation.reply` | `copied` quando preenchido; vazio usa `materialized` com o default do builder legado identificado |
| 2 | `layers.goal` | `process_prompt` draft de `conversation.triage` | `copied` quando preenchido; vazio materializa o default legado |
| 3 | `layers.context` | `agency_policy` draft distinto para triage/reply | `copied` quando preenchido; vazio materializa o default legado; nunca vira `platform_guardrail` |
| 4 | catálogo server-side de destinos | variável `routing.catalog` + source capability/binding | `transformed`; IDs/slugs são resolvidos no runtime, não copiados para texto |
| 5 | `collect_field_defs` server-side | variável `conversation.collect_field_definitions` + config do processo | `transformed`; chaves/tipos/opções vêm do catálogo autoritativo |
| 6 | `layers.guardrails` | trecho marcado nos `process_prompt` drafts distintos de triage/reply | `transformed`; vazio materializa o default legado; não vira guardrail de plataforma |
| 7 | `messaging.messages`/contexto CRM server-side | `conversation.messages` e resolvers tipados do context snapshot | `transformed`; conteúdo de conversa não é importado nem persistido no prompt |
| 8 | `output_schema`/`schema_version` | schemas registrados distintos de `conversation.triage` e `conversation.reply` | `transformed`; mega-schema legado serve ao comparator, nunca é reutilizado como schema novo |

Para triagem, o `process_prompt` candidato combina `goal` e `guardrails` em headings fixos do
`transform_version`; para resposta, combina template inicial específico de resposta e
`guardrails`. `identity` e `context` geram versões por processo, ainda que o conteúdo/hash seja
igual, para não criar compartilhamento implícito. Provider/model/temperature geram model profile
draft; limites/media/failure config geram bundle/policies drafts. Destinos, campos, histórico e
schema continuam dados/contratos estruturados, não texto editável.

Como `agent_override` exige binding client-scoped, inventário sem `client_account_id` não inventa
escopo: registra `client_scope_missing` em `unmapped_paths` e aguarda o binding CI-01/decisão do
painel. Uma versão legada usada por mais de um cliente gera lineage/candidates distintos por
cliente, nunca um target compartilhado implicitamente.

O import não presume que um mega-prompt pode ser separado semanticamente sem revisão. Qualquer
texto com instruções cruzadas, chave desconhecida em `layers`, schema não reconhecido, target
ausente ou valor sem mapping entra em `unmapped_paths` e fixa `migration_state=review_required`.
Mesmo sem pendência sintática, o primeiro split triage/reply exige aceite humano registrado; uma
regra futura só poderá dispensá-lo após eval aprovada e nova `transform_version`.

Pipeline:

1. inventariar agentes/versions/active version/config UI;
2. criar `ai_legacy_mappings` em `inventoried`, sem mudar writer;
3. importar targets sempre com status funcional `draft` e marcar mapping `imported`;
4. gerar drafts explícitos e independentes para `conversation.triage` e `conversation.reply`;
5. calcular `target_hash`, validar cobertura dos oito componentes e ir a `review_required`;
6. owner revisa split/diff/unmapped e registra decisão — duplicação não é publicação;
7. validar targets pelo lifecycle normal, marcar mapping `validated` e rodar eval;
8. somente após gates, publicar versões/bindings candidatos com rollout exclusivamente `shadow` e
   marcar mapping `shadow`;
9. congelar writer antigo para o escopo, confirmar leitura nova e marcar `cutover`;
10. API/UI antiga vira fachada ou read-only;
11. somente depois mudar runtime resolver padrão.

O import/backfill é replay-safe:

- mesma `idempotency_key` devolve os mesmos mapping/target IDs e hashes, sem criar nova versão;
- mesma origem com `transform_version` nova cria lineage e drafts novos;
- origem alterada cria novo `source_hash`/lineage e nunca edita target validado/published;
- cada source path termina em `split_map` ou `unmapped_paths`; ausência silenciosa é falha;
- defaults vazios são materializados com ID/hash da versão do builder, não com o default atual;
- `target_hash` é calculado sobre IDs, content hashes, schema/model/config versions ordenados;
- replay compara contagem, source/target hashes e relações tenant antes de avançar estado;
- falha parcial marca `failed`; retry retoma pela idempotency key sem publicar;
- nenhum import altera `active_version_id`, binding published ou writer state.

Enquanto não houver separação validada, o adapter `legacy_messaging` continua resolvendo a versão
ativa antiga. O Prompt Registry não vira coautor.

### 18.3 Revisão, diff e bloqueios no painel

O Prompt Studio oferece a visão “Migração legada” por account/agente/versão, com:

- estado de writer e `migration_state` exibidos separadamente;
- source ref/schema/builder/`source_hash` e `transform_version`;
- tabela dos oito componentes com source path, target process/layer/config, modo e hashes;
- diff lado a lado por triage/reply, identificando conteúdo materializado, copiado, transformado e
  inserido pelo template novo;
- lista de `unmapped_paths`, reason codes e ação explícita `mapear`, `descartar com justificativa`
  ou `manter legado`;
- preview compilado somente com fixtures sintéticas/mascaradas;
- comparação shadow/evals, target hashes e decisão do revisor;
- histórico de import/retry/review/validate/cutover.

O painel nunca mostra segredo nem conversa real no diff. Aceite exige comentário, `expectedRevision`
e permissão `customer_intelligence.prompts.manage`; validate/publish continuam exigindo suas
permissões próprias. Botões de validate/publish ficam bloqueados enquanto houver unmapped, target
ausente, hash divergente, split sem aceite, mapping `failed` ou writer transition incompatível.
“Descartar com justificativa” remove o path de `unmapped_paths`, mas preserva a decisão no
`split_map` com `mode=manual`, reason code, usuário e timestamp.

APIs administrativas:

| Método e rota | Permissão | Regra |
|---|---|---|
| `GET /v1/customer-intelligence/legacy-migrations` | `customer_intelligence.prompts.view` | filtros tenant-safe |
| `GET /v1/customer-intelligence/legacy-migrations/{id}` | `customer_intelligence.prompts.view` | mapping/diff mascarado |
| `POST /v1/customer-intelligence/legacy-migrations/inventory` | `customer_intelligence.prompts.manage` | job idempotente |
| `POST /v1/customer-intelligence/legacy-migrations/{id}/import` | `customer_intelligence.prompts.manage` | cria drafts; `expectedRevision` |
| `PATCH /v1/customer-intelligence/legacy-migrations/{id}/review` | `customer_intelligence.prompts.manage` | decisões fechadas; CAS |
| `POST /v1/customer-intelligence/legacy-migrations/{id}/validate` | `customer_intelligence.prompts.manage` | cobertura/hash/schema |
| `POST /v1/customer-intelligence/legacy-migrations/{id}/shadow` | `customer_intelligence.prompts.publish` | requer validated |
| `POST /v1/customer-intelligence/legacy-migrations/{id}/cutover` | `customer_intelligence.prompts.publish` | gate CI-10 + writer CAS |

Nenhuma rota aceita `account_id`, target ID, source path ou estado livre fora do escopo/enum
resolvido no service.

### 18.4 State machine do writer

| Estado | Prompt/agent writer | Runtime | UI |
|---|---|---|---|
| `legacy` | `messaging.ai_*` | serviço atual | ConfigAiAgent atual |
| `shadow` | legado | atual envia; novo compara | nova UI somente leitura/teste |
| `new` | `intelligence.*` | novo binding | Prompt Studio; legado fachada/read-only |

Writer state é por account/client/agent/process. Um publish no novo registry é bloqueado enquanto
o writer state daquele processo ainda for `legacy`, salvo criação de candidate shadow sem ativação.

Writer state (`legacy|shadow|new`) e `migration_state` são máquinas distintas. Exemplo válido:
writer `legacy` + mapping `validated`; exemplo inválido: mapping `cutover` + writer `legacy`.

### 18.5 Runs/tools/knowledge

- runs existentes permanecem auditáveis;
- novo runtime pode guardar `legacy_run_id`;
- tool/knowledge writer muda por entidade, nunca em bloco;
- documentos/chunks não são copiados se o mesmo writer/serviço puder ser adaptado;
- se migration for necessária, checksum/count/consumer zero precedem cutover;
- nenhuma credencial é decifrada por backfill;
- `ConfigAiAgentVersions.vue`, simulador e rotas atuais permanecem até paridade e redirect
  autorizado de CI-08.

### 18.6 Sem dual truth

Dual-read controlado é permitido para comparação. Dual-write permanente é proibido. Durante
shadow:

- fonte antiga escreve;
- nova projeção deriva/importa assincronamente;
- divergência não corrige fonte antiga automaticamente.

## 19. APIs administrativas

### 19.1 Processos e prompts

| Rota | Permissão |
|---|---|
| `GET /v1/customer-intelligence/processes` | `customer_intelligence.prompts.view` |
| `GET /v1/customer-intelligence/pipelines` | `customer_intelligence.prompts.view` |
| `POST /v1/customer-intelligence/pipelines/{pipelineKey}/drafts` | `customer_intelligence.prompts.manage` |
| `POST /v1/customer-intelligence/pipeline-versions/{id}/validate` | `customer_intelligence.prompts.manage` |
| `POST /v1/customer-intelligence/pipeline-versions/{id}/test` | `customer_intelligence.prompts.manage` |
| `POST /v1/customer-intelligence/pipeline-versions/{id}/publish` | `customer_intelligence.prompts.publish` |
| `POST /v1/customer-intelligence/pipeline-versions/{id}/rollback` | `customer_intelligence.prompts.publish` |
| `GET /v1/customer-intelligence/prompts?processKey=...` | `customer_intelligence.prompts.view` |
| `POST /v1/customer-intelligence/prompts/{processKey}/drafts` | `customer_intelligence.prompts.manage` |
| `GET /v1/customer-intelligence/prompt-versions/{id}` | `customer_intelligence.prompts.view` |
| `PATCH /v1/customer-intelligence/prompt-versions/{id}` | `customer_intelligence.prompts.manage`, `expectedRevision` |
| `POST /v1/customer-intelligence/prompt-versions/{id}/validate` | `customer_intelligence.prompts.manage` |
| `POST /v1/customer-intelligence/prompt-versions/{id}/test` | `customer_intelligence.prompts.manage` |
| `POST /v1/customer-intelligence/prompt-versions/{id}/simulate` | `customer_intelligence.prompts.manage` |
| `POST /v1/customer-intelligence/prompt-versions/{id}/publish` | `customer_intelligence.prompts.publish` |
| `GET /v1/customer-intelligence/prompt-bindings?processKey=...` | `customer_intelligence.prompts.view` |
| `POST /v1/customer-intelligence/prompt-bindings` | `customer_intelligence.prompts.publish` |
| `POST /v1/customer-intelligence/prompt-bindings/{id}/rollout` | `customer_intelligence.prompts.publish` |
| `POST /v1/customer-intelligence/prompt-bindings/{id}/rollback` | `customer_intelligence.prompts.publish` |
| `GET /v1/customer-intelligence/prompt-versions/{id}/evaluations` | `customer_intelligence.prompts.view` ou `customer_intelligence.runs.view` |

`POST .../rollout` recebe ação fechada `shadow|canary|full|pause`, percentual, critérios e
`expectedRevision`; não existe endpoint genérico para mutar rollout de outro binding.

Platform guardrail usa APIs separadas e `customer_intelligence.prompts.platform_manage`; tenant
admin não recebe o conteúdo editável dessa superfície por herança.

### 19.2 Agentes/modelos/credentials

| Método e rota | Permissão | Nota |
|---|---|---|
| `GET /v1/customer-intelligence/agents` | `customer_intelligence.agents.manage` | lista |
| `POST /v1/customer-intelligence/agents` | `customer_intelligence.agents.manage` | stable agent |
| `GET /v1/customer-intelligence/agents/{id}` | `customer_intelligence.agents.manage` | detalhe |
| `PATCH /v1/customer-intelligence/agents/{id}` | `customer_intelligence.agents.manage` | enabled/name |
| `GET /v1/customer-intelligence/agents/{id}/versions` | `customer_intelligence.agents.manage` | bundles |
| `POST /v1/customer-intelligence/agents/{id}/versions` | `customer_intelligence.agents.manage` | draft bundle |
| `POST /v1/customer-intelligence/agent-versions/{id}/validate` | `customer_intelligence.agents.manage` | valida |
| `POST /v1/customer-intelligence/agent-versions/{id}/publish` | `customer_intelligence.agents.manage` | imutável |
| `POST /v1/customer-intelligence/agents/{id}/rollback` | `customer_intelligence.agents.manage` | active version |
| `GET /v1/customer-intelligence/model-profiles` | `customer_intelligence.agents.manage` | lista |
| `POST /v1/customer-intelligence/model-profiles` | `customer_intelligence.agents.manage` | profile |
| `GET /v1/customer-intelligence/model-profiles/{id}/versions` | `customer_intelligence.agents.manage` | versions |
| `POST /v1/customer-intelligence/model-profiles/{id}/versions` | `customer_intelligence.agents.manage` | draft |
| `GET /v1/customer-intelligence/credentials` | `customer_intelligence.agents.manage` | status mascarado |
| `POST /v1/customer-intelligence/credentials` | `customer_intelligence.agents.manage` | segredo write-only |
| `PUT /v1/customer-intelligence/credentials/{id}` | `customer_intelligence.agents.manage` | rotação write-only |
| `DELETE /v1/customer-intelligence/credentials/{id}` | `customer_intelligence.agents.manage` | revoga se não usado |
| `GET /v1/customer-intelligence/agent-versions/{id}/process-bindings` | `customer_intelligence.agents.manage` | mapping |
| `POST /v1/customer-intelligence/agent-versions/{id}/process-bindings` | `customer_intelligence.agents.manage` | adiciona |
| `PATCH /v1/customer-intelligence/agent-versions/{id}/process-bindings/{bindingId}` | `customer_intelligence.agents.manage` | revision |
| `GET /v1/customer-intelligence/runs` | `customer_intelligence.runs.view` | cursor/filtros |

Credential response contém apenas id/name/provider/status/last4/timestamps.

### 19.3 Test/simulate inputs

```json
{
  "clientAccountId": "uuid",
  "agentId": "uuid",
  "testCaseIds": ["uuid"],
  "fixture": null,
  "historicalRef": null,
  "executor": "native",
  "compareWithBindingId": "uuid",
  "sideEffectMode": "deny"
}
```

Account vem do Principal. `fixture` e `historicalRef` são mutuamente exclusivos.

## 20. APIs/jobs headless e gateways internos

### 20.1 Admin/job

`POST /v1/customer-intelligence/jobs`:

```json
{
  "clientAccountId": "uuid",
  "subjectId": "uuid",
  "relationshipId": "uuid",
  "processKey": "profile.summary",
  "purposeKey": "customer_profile",
  "idempotencyKey": "caller-key",
  "asOf": "RFC3339"
}
```

Retorna `202 {jobId,status}`. Apenas process configs com invocation mode `headless`.

### 20.2 Internal execute

`POST /v1/runtime/customer-intelligence/execute`:

- token server-to-server com audience, account/client, caller module e expiry;
- body limitado e schema fechado;
- IDs precisam coincidir com claims;
- idempotency;
- sem segredo;
- resposta é sempre `ProcessResult.v1`.

`POST /v1/runtime/customer-intelligence/interactions/decide` recebe
`InteractionRequest.v1` e devolve `InteractionDecision.v1`. Os dois endpoints/schemas não são
intercambiáveis; o coordenador do segundo chama internamente o runtime processual do primeiro
contrato sem round-trip HTTP obrigatório.

CI-07 pode chamar uma interface Go in-process primeiro; endpoint interno existe para futura
separação, não é obrigatório em monólito.

### 20.3 LLM/tool gateway

Paths propostos:

- `/v1/runtime/customer-intelligence/llm-gateway`;
- `/v1/runtime/customer-intelligence/tool-calls`;
- `/v1/runtime/customer-intelligence/context/{snapshotId}` somente se n8n precisar, com token
  audience/run e conteúdo minimizado.

Gateway:

- não usa JWT de usuário;
- valida token interno curto e replay/idempotency;
- rate/body limits;
- confere run/binding/model/tool;
- nunca aceita account/provider/model livre divergente;
- não retorna credential;
- logs mínimos.

## 21. Stop, fallback e cancelamento

### 21.1 Kill switches

Escopos:

- plataforma/provider;
- account;
- client;
- agent;
- process;
- rollout.

Stop é policy estruturada, não texto de prompt. Alteração é auditada e efetiva para novas claims;
runs em processamento recebem cancelamento cooperativo.

### 21.2 Automatic stop criteria

Exemplos configuráveis dentro de caps:

- critical eval failure;
- schema invalid rate;
- provider error/timeout;
- custo/latência;
- tool denial/unsafe attempt;
- tenant/scope anomaly;
- handoff spike;
- output vazio/oversize;
- divergence contra baseline;
- source stale obrigatória.

Tenant/scope/secret violation para imediatamente o rollout, sem threshold flexível.

### 21.3 Fallback por classe

| Classe | Fallback |
|---|---|
| conversa | resultado `needs_human/failure`; Omnichannel decide handoff |
| memory extract | retry/dead-letter; nenhum claim/fato |
| profile summary | manter versão anterior marcada stale |
| recommendation | nenhuma recomendação publicada |
| source suggestion | nenhuma sugestão |
| portfolio | nenhuma saída |
| media | blocked/failed; conversa pode ir a humano |
| quality review | unavailable |

Resultado atrasado/cancelado é persistido como `stale_result` e nunca produz efeito downstream.

## 22. Segurança, tenant e permissões

Permissões propostas por CI-00:

- `customer_intelligence.agents.manage`;
- `customer_intelligence.prompts.view`;
- `customer_intelligence.prompts.manage`;
- `customer_intelligence.prompts.publish`;
- `customer_intelligence.prompts.platform_manage`;
- `customer_intelligence.runs.view`;
- `customer_intelligence.audit.view`;
- sources/profile/portfolio cumulativas conforme contexto.

Regras:

- permissão antiga `omnichannel.agents.manage` não concede automaticamente prompt/source/portfolio;
- fora do escopo retorna 404;
- client é catálogo + mesma organização;
- process de portfolio exige gates adicionais;
- prompt é conteúdo ativo e tratado como input administrativo sensível;
- prompt/context/output bruto não entram em logs;
- credential nunca reidrata;
- provider base URL é allowlisted;
- context snapshot cifrado/TTL;
- mídia/documento usa storage privado e SSRF protection;
- tool write exige aprovação/policy;
- n8n execution persistence desligada.

## 23. Observabilidade e auditoria

Por run:

- request/run/job/correlation/causation;
- account/client/subject/relationship conforme permitido;
- process key;
- agent/version;
- process definition/config e prompt definition/binding/layer versions;
- model profile/provider/model;
- executor;
- sources consultadas/omitidas/stale;
- evidence/fact/context IDs;
- tools/proposals/approvals;
- schema/status/error;
- tokens/custo/latência;
- operational lease ref quando conversa;
- rollout/bucket/arm;
- decisão downstream conhecida, sem fingir envio.

Métricas por process/prompt/model/client/rollout:

- success/schema/provider/blocked/cancelled;
- latency p50/p95/p99;
- tokens/cost;
- eval scores;
- shadow divergence;
- canary arm;
- tool/source calls;
- context omissions;
- stale result;
- fallback/handoff.

Nenhuma label de métrica usa subject/conversation ID ou PII.

## 24. Pacotes atômicos e allowlists

### Leitura permitida

- `AGENT.md`, skills aplicáveis e AGENTs de database, Omnichannel e Customer Intelligence;
- `docs/customer-intelligence/GOVERNANCA.md`, `SPECS_GERAIS.md` e specs CI-00 a CI-06;
- documentos e contratos canônicos em `docs/omnichannel/`;
- migrations `0206`, `0216`, `0219`, `0222` a `0225`, `0228`, `0232`, `0234` e `0236`;
- implementação atual `messaging.ai_*`, `brain_*`, media analysis, service/store/http do
  Omnichannel, apenas para compatibilidade e mapping;
- `back/internal/platform/llm`, `secretbox`, jobs, modules e composition root;
- telas `ConfigAiAgent*` e APIs tipadas atuais do frontend, somente para preservar a transição;
- export/contrato do brain Omnichannel somente leitura e após confirmação de owner;
- CI-04/CI-05 e contratos Customer Data necessários ao context builder.

Leitura de secrets, `.env`, dumps, dados reais de execução, bytes de mídia e módulos
`social-publishing` é proibida.

### CI06-DB-01 — agents/models/runtime

**Escrita permitida:**

- `back/internal/platform/database/migrations/<NNNN>_customer_intelligence_runtime.sql`;
- `back/database/ERD.md`;
- `back/database/AGENT.md`;
- `back/internal/modules/customerintelligence/AGENT.md`.

### CI06-DB-02 — tools/knowledge mapping

**Escrita permitida:**

- `back/internal/platform/database/migrations/<NNNN>_customer_intelligence_tools_knowledge.sql`;
- ERD/AGENTs acima.

Não executa drop nem muda writer legado.

### CI06-DB-03 — lineage de migração legada

**Escrita permitida:**

- `back/internal/platform/database/migrations/<NNNN>_customer_intelligence_legacy_lineage.sql`;
- ERD/AGENTs acima.

Cria `ai_legacy_mappings`, enums/constraints/índices tenant-safe e nenhum trigger de dual-write.
Não altera enum/status de `ai_agent_versions`.

### CI06-BE-MODULE-01

**Escrita permitida:**

- `back/internal/modules/customerintelligence/module.go`;
- `back/internal/modules/customerintelligence/handle.go`;
- `back/internal/modules/customerintelligence/metadata.go`;
- `back/internal/modules/customerintelligence/AGENT.md`;
- testes correspondentes;
- registros explícitos no composition root listados pelo orquestrador.

### CI06-BE-PROMPT-01

**Escrita permitida:**

- `back/internal/modules/customerintelligence/prompt_catalog.go`;
- `back/internal/modules/customerintelligence/prompt_compiler.go`;
- `back/internal/modules/customerintelligence/prompt_variables.go`;
- `back/internal/modules/customerintelligence/prompt_binding_resolver.go`;
- `back/internal/modules/customerintelligence/process_schemas.go`;
- `back/internal/modules/customerintelligence/pipeline_catalog.go`;
- `back/internal/modules/customerintelligence/pipeline_compiler.go`;
- `back/internal/modules/customerintelligence/prompt_templates/manifest.json`;
- `back/internal/modules/customerintelligence/prompt_templates/conversation_triage.md`;
- `back/internal/modules/customerintelligence/prompt_templates/conversation_reply.md`;
- `back/internal/modules/customerintelligence/prompt_templates/conversation_handoff_summary.md`;
- `back/internal/modules/customerintelligence/prompt_templates/memory_extract.md`;
- `back/internal/modules/customerintelligence/prompt_templates/profile_summary.md`;
- `back/internal/modules/customerintelligence/prompt_templates/recommendation_follow_up.md`;
- `back/internal/modules/customerintelligence/prompt_templates/recommendation_offer.md`;
- `back/internal/modules/customerintelligence/prompt_templates/recommendation_important_dates.md`;
- `back/internal/modules/customerintelligence/prompt_templates/source_suggest.md`;
- `back/internal/modules/customerintelligence/prompt_templates/portfolio_opportunity.md`;
- `back/internal/modules/customerintelligence/prompt_templates/media_image_analysis.md`;
- `back/internal/modules/customerintelligence/prompt_templates/media_document_analysis.md`;
- `back/internal/modules/customerintelligence/prompt_templates/quality_review.md`;
- testes correspondentes;
- AGENT do módulo.

### CI06-BE-CONTEXT-01

**Escrita permitida:**

- `back/internal/modules/customerintelligence/context_builder.go`;
- `back/internal/modules/customerintelligence/context_budget.go`;
- `back/internal/modules/customerintelligence/context_resolvers.go`;
- testes correspondentes;
- AGENT do módulo.

### CI06-BE-RUNTIME-01

**Escrita permitida:**

- `back/internal/modules/customerintelligence/model_runtime.go`;
- `back/internal/modules/customerintelligence/service_runtime.go`;
- `back/internal/modules/customerintelligence/service_conversation_pipeline.go`;
- `back/internal/modules/customerintelligence/store_runtime.go`;
- `back/internal/modules/customerintelligence/processors/**`;
- testes correspondentes;
- AGENT do módulo.

### CI06-BE-MODELS-01

**Escrita permitida:**

- `back/internal/modules/customerintelligence/model_agent.go`;
- `back/internal/modules/customerintelligence/service_agent.go`;
- `back/internal/modules/customerintelligence/store_agent.go`;
- `back/internal/modules/customerintelligence/service_credentials.go`;
- `back/internal/modules/customerintelligence/store_credentials.go`;
- `back/internal/modules/customerintelligence/model_profile.go`;
- testes correspondentes;
- AGENT do módulo.

### CI06-BE-TOOLS-01

**Escrita permitida:**

- `back/internal/modules/customerintelligence/tool_registry.go`;
- `back/internal/modules/customerintelligence/tool_gateway.go`;
- `back/internal/modules/customerintelligence/tool_validation.go`;
- `back/internal/modules/customerintelligence/store_tools.go`;
- testes correspondentes;
- AGENT do módulo.

### CI06-BE-KNOWLEDGE-01

**Escrita permitida:**

- `back/internal/modules/customerintelligence/model_knowledge.go`;
- `back/internal/modules/customerintelligence/service_knowledge.go`;
- `back/internal/modules/customerintelligence/store_knowledge.go`;
- testes correspondentes;
- AGENT do módulo.

### CI06-BE-EXEC-NATIVE-01

**Escrita permitida:**

- `back/internal/modules/customerintelligence/executor.go`;
- `back/internal/modules/customerintelligence/executor_native.go`;
- testes correspondentes;
- AGENT do módulo.

### CI06-BE-EXEC-N8N-01

**Escrita permitida:**

- `back/internal/modules/customerintelligence/executor_n8n.go`;
- `back/internal/modules/customerintelligence/http_runtime_gateway.go`;
- testes correspondentes;
- AGENT do módulo.

Não edita workflow. Se contrato exigir mudança de workflow, parar e criar pacote N8N separado
depois de owner/ID aprovados.

### CI06-BE-JOBS-01

**Escrita permitida:**

- `back/internal/modules/customerintelligence/job_runtime.go`;
- `back/internal/modules/customerintelligence/store_runtime_jobs.go`;
- testes correspondentes;
- AGENT do módulo.

### CI06-API-01

**Escrita permitida:**

- `back/internal/modules/customerintelligence/http_prompts.go`;
- `back/internal/modules/customerintelligence/http_pipelines.go`;
- `back/internal/modules/customerintelligence/http_agents.go`;
- `back/internal/modules/customerintelligence/http_runs.go`;
- `back/internal/modules/customerintelligence/http_jobs.go`;
- testes HTTP correspondentes;
- AGENT do módulo.

### CI06-BACKFILL-01

**Escrita permitida:**

- `back/cmd/customer-intelligence-runtime-backfill/main.go`;
- `back/internal/modules/customerintelligence/backfill_ai_runtime.go`;
- `back/internal/modules/customerintelligence/model_legacy_mapping.go`;
- `back/internal/modules/customerintelligence/store_legacy_mapping.go`;
- `back/internal/modules/customerintelligence/service_legacy_mapping.go`;
- `back/internal/modules/customerintelligence/legacy_prompt_splitter.go`;
- `back/internal/modules/customerintelligence/http_legacy_migrations.go`;
- testes correspondentes;
- `docs/customer-intelligence/evidence/CI-06/**`;
- AGENT do módulo.

Responsabilidades fechadas: inventory, canonicalização/hash, mapping dos oito componentes, criação
de drafts, diff mascarado, review/CAS, validate e replay idempotente. Publish/cutover continuam nos
services normais e exigem gates próprios.

### CI06-COMPAT-OMNI-01

**Escrita permitida:**

- adapter em `back/internal/platform/app/customer_intelligence_runtime_adapter.go`;
- compatibility reader/mapping files explicitamente listados em Omnichannel;
- testes/AGENTs correspondentes.

Não altera sender, outbox, providers, FSM ou dispatch ownership.

### CI06-FE-PROMPT-STUDIO-01

Este é o contrato de handoff para `CI08-PROMPTS-04`; não autoriza uma segunda árvore visual.

**Escrita permitida apenas quando CI-08 liberar shell/gates:**

- `web/app/pages/inteligencia-clientes/prompts.vue`;
- `web/app/components/customer-intelligence/prompts/PromptStudio.vue`;
- `web/app/components/customer-intelligence/prompts/PromptProcessList.vue`;
- `web/app/components/customer-intelligence/prompts/PromptEditor.vue`;
- `web/app/components/customer-intelligence/prompts/PromptLayersPanel.vue`;
- `web/app/components/customer-intelligence/prompts/PromptVersionsPanel.vue`;
- `web/app/components/customer-intelligence/prompts/PromptEvaluationPanel.vue`;
- `web/app/components/customer-intelligence/prompts/PromptRolloutPanel.vue`;
- `web/app/components/customer-intelligence/prompts/PromptLegacyMigrationPanel.vue`;
- `web/app/components/customer-intelligence/prompts/PromptLegacyMappingDiff.vue`;
- `web/app/domain/customer-intelligence/prompt-api.ts`;
- `web/app/domain/customer-intelligence/prompt-types.ts`;
- `web/app/composables/customer-intelligence/usePromptStudio.ts`;
- testes correspondentes.

Compatibilidade/reuso dos componentes Omnichannel existentes usa somente a allowlist nominal de
`CI08-PROMPTS-04`; os dois componentes de migração acima são handoff solicitado e só entram em
execução quando CI-08 reconciliar os mesmos nomes na sua allowlist. CI-06 não apaga o fluxo legado.

### CI06-QA-01

**Escrita permitida:** testes dentro das áreas acima e evidências
`docs/customer-intelligence/evidence/CI-06/**`.

### CI06-CUTOVER-01

**Escrita permitida:** writer-state/mapping/feature flags, adapters/fachadas e evidências
nominalmente listados. Não inclui `REMOVE`.

### CI06-N8N-01 — bloqueado

Só pode existir após owner/ID/path aprovados. Allowlist futura deve conter exatamente um export
Customer Intelligence e testes/manifest correspondentes. Nunca inclui outros workflows.

### Sempre proibido

- `back/internal/modules/socialpublishing/**`;
- `docs/social-publishing/**`;
- `automation/workflow-whatsapp.json`, WAHA;
- workflows Calendar/Operação;
- `workflow-omnichannel-brain.json` sem pacote/owner explícitos;
- provider/sender/outbox de canal;
- migrations aplicadas;
- secret/env/volume/payload real.

## 25. Testes e comandos

Backend:

```powershell
cd back
go test ./internal/modules/customerintelligence/...
go test -race ./internal/modules/customerintelligence/...
go test ./internal/platform/llm/... ./internal/platform/app/...
go test ./internal/modules/omnichannel/...
go test ./...
golangci-lint run ./...
```

Frontend, quando aplicável:

```powershell
npm --prefix web run test
npm --prefix web run lint
npm --prefix web run build
```

Test matrix:

- cada process key resolve prompt distinto;
- pipeline resolve versão/binding publicados e somente process keys allowlisted;
- triage e reply geram `ProcessResult.v1`/runs separados;
- triage intermediária nunca vira mensagem, handoff ou `no_reply` por tradução;
- close proposal preserva resposta final e chega como `closure` na decisão composta;
- ordem/fingerprint das camadas;
- placeholder/type/missing behavior;
- prompt injection em message/ERP/document/knowledge;
- schema/enum/ref invalid;
- binding fallback sem cross-client;
- canary bucket estável;
- draft não afeta active;
- publish imutável;
- rollback preserva histórico;
- kill switch/cancelamento;
- timeout/provider/schema retry;
- tool/source intersection;
- write tool denied no simulate;
- segredo ausente de response/log/n8n;
- native × n8n contract;
- headless sem UI/chat;
- dispatch conversation continua no Omnichannel;
- stale result não produz efeito;
- target `ai_agent_versions.status` rejeita estado de migração;
- estado funcional do target e `migration_state` evoluem independentemente;
- os oito componentes legados terminam em `split_map` ou `unmapped_paths`;
- identity/context geram versões distintas por triage/reply;
- goal/guardrails produzem split determinístico com `transform_version`;
- layer ausente materializa o default da versão exata do builder legado;
- chave desconhecida/schema incompatível/unmapped bloqueiam validate/publish;
- mesmo source hash/transform/idempotency key retorna os mesmos targets/hashes;
- source ou transform alterado cria lineage/drafts novos sem mutar target anterior;
- import parcial falha e retry não duplica;
- target hash divergente bloqueia shadow/cutover;
- backfill não altera active version, published binding, writer ou credential;
- legacy mapping/backfill/checksum tenant-safe;
- old/new API writer state;
- tenant negativo em todas as rotas;
- budget/truncation/omission codes;
- load/concurrency/cost cap.

Browser Prompt Studio:

- papéis view/manage/publish/platform;
- loading/vazio/erro;
- dirty draft/ETag conflict;
- diff/clone/validate/test/simulate;
- publish confirmation;
- shadow/canary/rollback/stop;
- estado writer × estado da migração sem colapsar enums;
- inventário dos oito componentes, diff triage/reply e modos de transformação;
- unmapped bloqueia ações e descarte exige justificativa auditada;
- retry/reimport idempotente e conflito de `mapping_revision`;
- diff/preview sem segredo, PII ou conversa real;
- credential write-only;
- troca de conta/cliente;
- mobile/tema/teclado.

## 26. Rollout

1. schemas/tabelas e módulo desabilitado;
2. registry/process definitions/config versions e native executor com fixtures;
3. importar mappings legados sem writer novo;
4. headless em dados sintéticos;
5. profile/memory shadow por cliente;
6. Prompt Studio somente leitura/teste;
7. separar prompts de conversa e rodar shadow;
8. canary de processo não operacional;
9. CI-07 integra conversa mantendo sender/lease;
10. mover writer de prompts/agentes/pipelines por cliente/processo;
11. legacy UI vira fachada/read-only;
12. n8n genérico somente após ownership.

Gates quantitativos e janela são CI-10. Sem safety/eval/rollback, não promover.

## 27. Rollback

- prompt: repontar binding baseline;
- model: repontar model profile version;
- executor: feature flag `native|n8n` por process/account;
- runtime: retornar resolver legado enquanto writer ainda legado;
- após writer novo: manter writer novo e fachada antiga;
- job: pausar claims novas, preservar runs;
- conversa: nunca trocar sender; Omnichannel continua fail-open humano;
- não reprocessar dispatch/evento deduplicado;
- não apagar dados/tabelas como rollback;
- não reativar writers simultâneos.

## 28. Critérios de aceite

- [ ] Cada processo resolve prompt publicado próprio.
- [ ] Pipeline versionado compõe resultados intermediários sem mega-prompt.
- [ ] Triagem isolada não é aceita como efeito operacional.
- [ ] Auto-close preserva proposta, mensagem final e revalidação `SystemTryAutoClose`.
- [ ] Painel permite editar, validar, testar, simular, publicar e reverter.
- [ ] Prompt governa semântica sem alterar invariantes.
- [ ] Draft não muda produção.
- [ ] Published é imutável.
- [ ] Binding registra todas as camadas/versions/model/sources/tools/schema.
- [ ] Variável inválida bloqueia publish.
- [ ] Contexto é autorizado, paginado, token-budgeted e auditável.
- [ ] Runtime opera headless sem inbox/painel.
- [ ] Native/n8n obedecem o mesmo contrato.
- [ ] n8n não guarda segredo, memória nem envia.
- [ ] Tool/source inválida é rejeitada.
- [ ] Resultado atrasado/cancelado não produz efeito.
- [ ] Legacy prompt/version/UI são preservados até cutover.
- [ ] Versões importadas usam somente `draft|validated|published|archived`.
- [ ] Lineage possui source/target hashes, transform version, idempotência e cobertura das oito camadas.
- [ ] Split triage/reply é revisável no painel e não publica com path não mapeado.
- [ ] Não há dual-write permanente.
- [ ] Chat dispatch/lease continuam Omnichannel.
- [ ] Logs/runs não expõem prompt bruto, chave ou PII desnecessária.

## 29. Stop conditions

Parar quando:

- process key/schema/camadas de CI-00 não estiverem reconciliados;
- CI-04/05 não fornecerem bank/source contracts estáveis;
- publish puder ocorrer sem eval/rollback;
- runtime precisar aceitar URL/SQL/tool/source livre;
- prompt precisar alterar tenant/RBAC/FSM/consentimento/sender;
- segredo precisar chegar em log/front/workflow;
- context snapshot não tiver crypto/TTL;
- implementação criar segunda fila de conversa;
- migration quebrar FK de `messaging.ai_dispatches`;
- writer antigo e novo precisarem ficar ativos juntos;
- backfill não conseguir preservar IDs/hashes;
- algum dos oito componentes não puder ser mapeado ou justificado pelo painel;
- import tentar gravar estado de migração no `status` da versão-alvo;
- replay idempotente produzir target/hash diferente;
- workflow owner/ID não estiver aprovado;
- teste tenant/safety falhar;
- arquivo do usuário fora da allowlist precisaria ser sobrescrito;
- tarefa tocar social-publishing ou outro workflow.

## 30. Handoff obrigatório

- baseline/worktree e migrations atuais;
- estado `legacy|shadow|new` por cliente/agente/processo;
- mappings/backfill/checksums/órfãos;
- process keys/schemas/variables efetivos;
- versions/bindings/model/tool/source/knowledge usados;
- eval suite/resultados/custo/latência;
- native/n8n paridade;
- secrets testados como write-only;
- tenant/safety negative tests;
- rollout/bucket/stop/rollback;
- consumidores legados restantes;
- prova de que dispatch/outbox/sender permanecem Omnichannel;
- confirmação de nenhum workflow não autorizado, secret, volume ou social-publishing alterado.
