# E6 — tools, automações e conhecimento

**Status:** `DONE (local) — E6-AUDIT-01, E6-DB-02, E6-BE-03, E6-BE-04, E6-N8N-05, E6-FE-06 e E6-QA-07 concluídos em 2026-07-21.`

O fechamento é local e não autoriza importação/ativação no runtime n8n nem deploy. Adapters
corporativos sem contrato estável permanecem deliberadamente ausentes: binding sem handler Go
falha fechado e auditado.

**Resultado:** a IA consulta somente ferramentas e bases autorizadas para a conta, com argumentos
validados, timeout, auditoria, custo e resposta limitada; nenhum node acessa SQL ou sistemas
internos diretamente.

## 1. Fronteira fechada

- o catálogo de ferramentas corporativas existente é reutilizado; omnichannel cria bindings e
  policies, não uma plataforma paralela;
- n8n solicita uma tool pelo ID lógico; Go autentica, autoriza, executa e retorna;
- ferramenta de leitura não pode virar escrita por prompt;
- ações mutáveis exigem policy própria e, inicialmente, aprovação humana;
- RAG retorna trechos/evidências; o modelo não ganha acesso ao documento inteiro por padrão;
- output de tool é dado não confiável e recebe limite/schema/masking antes do prompt.

## 2. Banco

Após inventário do módulo Tools, criar apenas o que não existir:

O inventário local (`back/internal/modules/tools`) contém apenas QR codes e short-links. Não há
catálogo de operações corporativas reutilizável para execução de IA; por isso `tool_id` permanece
um identificador lógico validado pelo futuro registry, sem FK inventada e sem duplicar o módulo.

### `messaging.ai_tool_bindings`

`id/account_id/agent_id/tool_id/is_enabled/mode/allowed_operations/input_schema/output_schema/
timeout_ms/max_calls_per_dispatch/config/created_at/updated_at`.

- unique `(account_id,agent_id,tool_id)`;
- `mode`: `read`, `propose_write`, `approved_write`;
- `tool_id` referencia identificador canônico do módulo Tools; FK somente se schema estável;
- `config` contém IDs/filtros, nunca credencial.

### `messaging.ai_tool_runs`

`id/account_id/conversation_id/dispatch_id/ai_run_id/binding_id/call_id/status/operation/
input_masked/output_masked/latency_ms/error/created_at/completed_at`.

- unique `(account_id,dispatch_id,call_id)`;
- status fechado: `requested`, `approved`, `denied`, `running`, `completed`, `failed`, `timeout`;
- persistir hashes/shape quando o dado completo for sensível; retenção E9.

### Conhecimento

- `knowledge_bases`: conta, nome, enabled, embedding/search config;
- `knowledge_documents`: origem, título, checksum, status, versão, metadata allowlisted;
- `knowledge_chunks`: document, ordinal, text limitado, token_count, search vector;
- `ai_knowledge_bindings`: agente ↔ base.

P0 usa full-text search do PostgreSQL com índice GIN e ranking reproduzível. `pgvector` só entra
em pacote próprio depois de provar extensão disponível, estratégia de backup e ganho de qualidade;
não adicionar infraestrutura por suposição.

## 3. API interna de execução

`POST /v1/internal/omnichannel/ai/tool-calls` recebe assinatura, timestamp, dispatch ID, generation,
binding ID, call ID, operation e arguments. Go:

1. valida assinatura/replay/dispatch/generation;
2. carrega binding e conta pelo dispatch, nunca pelo body;
3. valida operação e arguments contra schema/tamanho;
4. aplica permission/policy/rate/budget;
5. grava run `requested`;
6. executa por interface do Tools com context timeout;
7. mascara/limita output, grava resultado e devolve schema estável.

Resposta nunca contém stack, token ou conexão. Timeout cancela request e não deixa goroutine.
Para escrita proposta, retorna `approvalRequired=true` e cria uma proposta; não executa.

## 4. RAG

Ingest é job durável: baixar origem permitida, validar tipo/tamanho, extrair, normalizar, quebrar em
chunks determinísticos, checksum e publicar versão somente ao terminar. Nova versão não apaga a
anterior até nenhuma run referenciá-la.

Query exige account/base binding, `topK` limitado, filtro por status/version e devolve:
`documentId`, `title`, `chunkId`, `excerpt`, `score`, `sourceRef`. A resposta da IA registra quais
chunks sustentaram a afirmação. Se score mínimo não for atingido, retorna “sem evidência”; não
força resposta.

Defesas contra prompt injection:

- conteúdo recuperado é delimitado como dado, nunca instrução;
- tool/RAG não pode alterar system prompt ou escolher outra tool;
- HTML/scripts são removidos; URLs/arquivos seguem SSRF e allowlist;
- output não pode expandir permissões nem mudar routing diretamente.

## 5. Frontend

- aba Ferramentas lista catálogo disponível, modo, operações e limites;
- binding usa formulário gerado do schema permitido, sem campo secreto genérico;
- aba Conhecimento cria base, faz upload/origem, mostra status/version/erros e permite desativar;
- simulador mostra chamadas propostas, argumentos mascarados, latência, evidências e approval;
- inbox exibe “consultou X” somente para perfis autorizados, sem vazar payload;
- tela de aprovações separa proposta de ação executada.

## 6. n8n brain

O workflow próprio itera tool calls até quatro rodadas, sempre via API Go. Cada call leva ID
determinístico por dispatch/generation; o endpoint Go de assinaturas (`POST
/v1/internal/omnichannel/ai/tool-call-signatures`) valida o binding e calcula HMAC com o token
cifrado de curta duração, portanto o n8n não precisa de credencial estática nem conhece chave de
provider. Em seguida o workflow chama somente `POST /v1/internal/omnichannel/ai/tool-calls` e
retorna o resultado delimitado como dado não confiável ao modelo. O node nunca contém URL de
ERP/banco, SQL, credencial externa nem envio de canal. Resultado final inclui `trace.toolCalls`
com IDs/status, não payload integral.

## 7. Pacotes atômicos

| Pacote | Entrega |
|---|---|
| `E6-AUDIT-01` | inventário do módulo Tools e contratos reutilizáveis |
| `E6-DB-02` | bindings/runs/knowledge mínimo |
| `E6-BE-03` | executor/policy/auditoria de tools |
| `E6-BE-04` | ingest/search de conhecimento |
| `E6-N8N-05` | loop de tool calls no brain v2 |
| `E6-FE-06` | config, evidências e aprovações |
| `E6-QA-07` | tenant, injection, timeout, retry e carga |

### Implementação local atual

- Migrations `0222_messaging_ai_tools_knowledge.sql` e `0223_messaging_ai_tool_audit_events.sql`
  criaram bindings/runs tenant-scoped, bases,
  documentos, chunks com `tsvector`/GIN e bindings agente↔base. Todas as colunas de entrada/saída
  são JSON limitado e mascarável; nenhuma credencial, SQL ou endpoint de terceiro é persistido.
- Migration `0224_messaging_audit_event_union.sql` recompõe o vocabulário completo de auditoria
  E1–E6; ela impede que a extensão de tools bloqueie eventos existentes de envio, mídia, CRM ou
  handoff.
- A migration completa passou em banco descartável e foi aplicada no PostgreSQL local com grants
  para `omni_app`.
- `E6-BE-03`: o Go expõe `POST /v1/internal/omnichannel/ai/tool-calls`, autenticado por token
  curto cifrado do brain + timestamp/HMAC; valida dispatch/generation, binding do agente, operação,
  schema de argumentos, limite de chamadas e modo de escrita. `call_id` é idempotente, runs são
  mascarados/auditados, timeout usa `context` e nenhum handler recebe credencial. O registry começa
  vazio quando não há adaptador injetado; tool ausente falha fechado, sem resposta inventada.
- `E6-BE-04`: bases, documentos, chunks e bindings têm CRUD tenant-scoped; publicação exige chunks,
  origem não aceita URL com credencial/esquemas perigosos e a busca usa FTS `simple` com ranking,
  `topK`/`minScore` do binding e evidências limitadas (`documentId`, `chunkId`, `sourceRef`, trecho).
- `E6-N8N-05`: `workflow-omnichannel-brain.json` agora possui loop inativo de até quatro tool
  calls, assinatura server-to-server, allowlist de bindings e retorno delimitado. O arquivo foi
  validado localmente e não foi importado/ativado no runtime n8n; nenhum workflow de outro owner
  foi alterado.
- `E6-FE-06`: a aba `Tools e conhecimento` usa client/composable tipados, permite configurar
  bindings sem credenciais, criar/desabilitar bases, cadastrar documentos, editar chunks limitados,
  publicar somente documentos com chunks e vincular bases por agente. Erros/status permanecem
  explícitos. A tela lista propostas mutáveis e evidências mascaradas; aprovar/rejeitar é uma
  transição Go tenant-scoped e o retry assinado é o único caminho de execução.

## Fechamento local 2026-07-21

Implementados no Go e no painel: evidências tenant-scoped mascaradas, propostas mutáveis,
aprovação/rejeição com ator, expiração e retry assinado. As rotas administrativas são
`GET /v1/omnichannel/agents/{id}/tool-runs`, `GET /v1/omnichannel/agents/{id}/tool-approvals`
e `POST .../tool-approvals/{approvalId}/approve|reject`. A migration `0225` cifra os argumentos
originais e audita `AI_TOOL_APPROVAL_REQUESTED|AI_TOOL_APPROVED|AI_TOOL_REJECTED`.

Validações executadas: `go vet ./internal/modules/omnichannel`, `go test
./internal/modules/omnichannel -count=1`, suíte do front com 27 arquivos/158 testes e ESLint
específico dos arquivos da E6. A migration `0225` foi confirmada aplicada no PostgreSQL local e a
API permaneceu saudável após rebuild isolado. Os contratos cobrem tenant, schema/limites,
prompt-injection por conteúdo delimitado, timestamp/HMAC, timeout, idempotência e retry. Não houve
smoke real Evolution, importação n8n ou deploy nesta fase.

## 8. Aceite

- tool não vinculada/disabled é negada e auditada;
- argumentos fora do schema não chegam ao sistema externo;
- retry do n8n retorna o mesmo run, sem repetir escrita/cobrança indevida;
- conta A não consulta catálogo/base/resultado da B;
- documento malicioso não injeta instrução nem executa URL/script;
- ausência de evidência produz fallback honesto;
- timeout respeita deadline e libera recurso;
- ação mutável inicial exige aprovação humana;
- n8n exportado não contém SQL, segredo ou endpoint de terceiro.
