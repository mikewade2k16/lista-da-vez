# E2 — cérebro n8n v2

**Status:** `CONTRACT-01 + DB-02 + BE-03 + FE-06 IMPLEMENTADOS localmente; BE-05/N8N-04 e gateway seguro implementados em modo opt-in; importação/ativação e QA externo ainda pendentes`

**Resultado:** mensagens consecutivas são agrupadas de forma durável, o workflow próprio produz
uma decisão `brain.result.v2` validada no Go e a policy decide responder, perguntar ou transferir.

## 1. Divisão de responsabilidade

| Go/PostgreSQL | n8n |
|---|---|
| decide se IA pode rodar | agrupa semanticamente o contexto recebido |
| cria/cancela dispatch durável | chama o modelo configurado |
| fornece histórico/CRM/config permitido | monta prompt e contexto |
| valida schema e autorização | produz resposta estruturada |
| aplica campos, estado, fila e outbox | nunca envia ao canal |
| registra run/custo/auditoria | devolve uso/latência/erro normalizado |

## 2. Banco e configuração

Adicionar à versão/agente, somente se ainda não houver equivalente:

- `debounce_ms` (default 2500, faixa 500–15000);
- `max_context_messages` (default 30, faixa 1–100);
- `max_ai_turns` (default 6, faixa 1–20);
- `min_confidence` (default 0.65);
- `handoff_on_error` e `handoff_on_limit`;
- `workflow_contract_version` fixo em `brain.v2` na versão publicada.

Criar `messaging.ai_dispatches`:

| Campo | Regra |
|---|---|
| `id`, `account_id`, `conversation_id`, `agent_version_id` | identidade e escopo |
| `generation` | inteiro crescente por conversa; invalida resultado atrasado |
| `status` | `buffering`, `queued`, `processing`, `completed`, `cancelled`, `failed` |
| `message_ids` | `uuid[]` não vazio, apenas mensagens inbound da mesma conversa/conta |
| `run_after`, `locked_at`, `completed_at` | controle durável |
| `idempotency_key` | unique por conta; derivada de conversa+generation |
| `result_run_id`, `last_error` | auditoria sem PII |
| `created_at`, `updated_at` | `timestamptz` |

Índice hot path: `(status, run_after)` parcial; unique de no máximo um dispatch
`buffering|queued|processing` por conversa. O append de nova mensagem bloqueia a conversa,
incrementa generation, agrega ID e reposiciona `run_after`. Não usar timer em memória.

O pacote `E2-DB-02` está aplicado localmente em
`back/internal/platform/database/migrations/0216_messaging_ai_dispatches.sql`. Ele adiciona a
configuração versionada do agente, FKs tenant-safe e a tabela/indexes de dispatch. A migration foi
executada no banco do Compose após dump de segurança; a imagem atual do api foi construída e o
serviço api foi recriado isoladamente. WAHA, Evolution, n8n e os demais serviços não foram
recriados/resetados.

O núcleo de `E2-BE-03` está em `ai_dispatch.go`, `store_ai_dispatch.go` e
`ai_dispatch_job.go`. Ele mantém a
ordem de locks conversa → dispatch, deduplica `message_id`, incrementa a generation somente para
mensagem nova, grava o job genérico na mesma transação, usa o worker de outbox, respeita debounce
configurável, stale-generation, takeover e retry. O caminho nativo continua como rollback; o
executor n8n ainda não é chamado.

`E2-BE-05` está parcialmente preparado em `ai_policy.go`: o decoder é strict, valida o envelope v2, enums,
limites e uso, e a policy transforma `continue_ai|handoff|no_reply` em uma intenção sem escrever
fila/estado. No caminho nativo, `max_ai_turns`, `min_confidence`, `handoff_on_error` e
`handoff_on_limit` já são aplicados antes/depois da chamada; `no_reply` conclui o dispatch sem
enviar nem rotear. A integração do resultado `brain.result.v2` com o executor n8n e a auditoria
transacional completa continuam no pacote BE-05/N8N-04.

## 3. Contrato `brain.request.v2`

Go envia ao n8n por endpoint interno assinado:

```json
{
  "schemaVersion": "brain.request.v2",
  "dispatchId": "uuid",
  "generation": 4,
  "tenant": {"accountId": "uuid", "timezone": "America/Sao_Paulo"},
  "conversation": {"id": "uuid", "state": "ai_active", "channel": "WHATSAPP"},
  "contact": {"id": "uuid", "relationshipStatus": "lead", "tags": [], "origin": {}, "summary": null},
  "messages": [{"id": "uuid", "role": "contact", "type": "TEXT", "text": "..."}],
  "collectedFields": {},
  "requiredFields": [],
  "pendingFields": [],
  "localTime": {"now": "2026-07-20T14:00:00-03:00", "insideBusinessHours": true},
  "agent": {"id": "uuid", "versionId": "uuid", "model": "configurado", "layers": {}},
  "capabilities": {"tools": [], "multimodal": false}
}
```

O request real não envia chave de API; Go resolve credencial e usa um gateway/credencial n8n
tenant-scoped sem materializá-la no export. Se a infraestrutura atual exigir chamada direta pelo
n8n, o Go fornece token efêmero/credential reference, nunca chave persistente no payload/log.

## 4. Contrato `brain.result.v2`

```json
{
  "schemaVersion": "brain.result.v2",
  "dispatchId": "uuid",
  "generation": 4,
  "decision": "continue_ai|handoff|no_reply",
  "reply": {"text": "...", "inReplyToMessageId": null},
  "classification": {"intent": "sales", "confidence": 0.91, "sentiment": "neutral"},
  "extractedFields": {},
  "suggestedRouting": {"departmentSlug": "comercial", "queueSlug": "vendas"},
  "handoff": {"needed": false, "reasonCode": null, "summary": null},
  "usage": {"provider": "...", "model": "...", "promptTokens": 0, "completionTokens": 0},
  "trace": {"toolCalls": [], "warnings": []}
}
```

JSON Schema usa `additionalProperties:false`, enums fechados, limites de tamanho e campos
obrigatórios. Go rejeita versão, dispatch ou generation divergente. `suggestedRouting` nunca grava
fila diretamente; passa pelo motor determinístico existente.

Os schemas congelados desta rodada ficam em
[`../contracts/brain-v2/request.schema.json`](../contracts/brain-v2/request.schema.json) e
[`../contracts/brain-v2/result.schema.json`](../contracts/brain-v2/result.schema.json). As
fixtures anonimizadas de contrato estão em `../contracts/brain-v2/fixtures/`: `request.valid`,
`result.valid-continue` e `result.valid-handoff` devem ser aceitas; os arquivos com prefixo
`invalid-` devem ser rejeitados. Nenhuma fixture contém credencial, número de cliente, URL
privada, `account_id` real ou conteúdo de execução. A validação no Go continua sendo a autoridade
final; o workflow não pode substituir esse gate por validação no n8n.

## 5. Policy após o workflow

Em transação/lock da conversa:

1. confirmar dispatch ainda `processing`, generation atual e estado AIAllowed;
2. gravar `ai_runs` com input mascarado, output, tokens, custo e status;
3. mesclar apenas `collect_field_defs` conhecidos em `extracted_fields`;
4. contar turnos da IA na janela da conversa;
5. aplicar policy:
   - `continue_ai`: confidence suficiente, abaixo de max turns, reply não vazio → outbox;
   - `handoff`: gerar snapshot e rotear;
   - `no_reply`: registrar razão e não enviar; fechamento, se cabível, é outra regra Go e nunca
     efeito direto de texto livre do modelo;
6. marcar dispatch completed e publicar realtime após commit.

Se humano assumir, fechar ou transferir enquanto o n8n roda, marcar dispatch `cancelled`; resultado
tardio vira no-op auditado. Timeout/schema/provider error usa policy configurada, sem loop infinito.
Aplicar cooldown configurável depois de falha/limite e circuit breaker compartilhado quando o n8n
estiver degradado; nesse período o fallback humano continua funcionando. `OMNI_AI_EXECUTOR=native`
permanece rollback temporário e consome o mesmo estado/schema, sem criar segunda configuração.

## 6. Frontend de configuração e inspeção

- painel edita debounce, contexto, max turns, confidence e fallbacks ao criar draft;
- publicar cria versão imutável; rollback reponta versão ativa;
- simulador usa exatamente request/result v2, sem criar conversa ou enviar mensagem;
- timeline técnica mostra dispatch, mensagens agrupadas, decision, versão, latência, custo e motivo
  de cancelamento, com PII mascarada conforme permissão;
- badge no inbox distingue “IA analisando”, “aguardando cliente” e “transferindo”.

## 7. Workflow n8n

Editar apenas `workflow-omnichannel-brain.json` (`omnibrain0000001`). Nodes mínimos:

1. trigger interno autenticado;
2. validação de envelope/versão;
3. normalização do contexto;
4. chamada ao modelo configurado;
5. parse/repair único de JSON;
6. montagem de `brain.result.v2`;
7. resposta HTTP; erro normalizado.

Sem Evolution/Meta/Instagram nodes, sem SQL, sem credencial exportada e sem Wait usado como fonte
do debounce (o relógio durável é do Go/PostgreSQL). O export aceito possui `pinData={}`,
`staticData=null`, save de execution data desligado e nenhuma amostra com PII.

## 8. Pacotes atômicos

| Pacote | Resultado | Owner |
|---|---|---|
| `E2-CONTRACT-01` | schemas JSON + fixtures válidas/inválidas | arquitetura |
| `E2-DB-02` | config e `ai_dispatches` | banco |
| `E2-BE-03` | agregação/cancelamento/worker | backend |
| `E2-N8N-04` | workflow v2 sem side effect | n8n |
| `E2-BE-05` | validação/policy/outbox/auditoria | backend |
| `E2-FE-06` | configuração, status e inspeção | frontend |
| `E2-QA-07` | concorrência, shadow e end-to-end | revisor |

### Estado desta preparação

O arquivo próprio `automation/export/workflow-omnichannel-brain.json` foi preparado para o
contrato v2, permanece inativo e não foi importado no n8n. Ele aceita o request no corpo principal
(e a forma aninhada apenas para compatibilidade), exige `X-Omni-Internal-Token` comparado com
`OMNI_N8N_INTERNAL_TOKEN`, rejeita campos de credencial, chama somente
`OMNI_LLM_GATEWAY_URL` e valida `brain.result.v2` antes de responder. Não possui nodes/URLs de
Evolution, WAHA, Meta ou Instagram, não acessa SQL e não envia mensagem. O gateway interno foi
implementado no Go com token cifrado curto; portanto o modo n8n pode ser habilitado somente quando
as envs internas estiverem preenchidas e o workflow próprio tiver passado por importação/ativação
controlada. O E2-INT local agora cobre o caminho nativo e o adaptador `brain.request.v2`/
`brain.result.v2`; timeline/badge de inspeção e gates externos do QA-07 continuam pendentes.
Quando `OMNI_AI_EXECUTOR=n8n`, `OMNI_N8N_BRAIN_WEBHOOK_URL` e `OMNI_N8N_INTERNAL_TOKEN` estão
configurados, o boot registra o executor brain.v2 e a rota interna
`POST /v1/runtime/omnichannel/llm-gateway`. A chave do provider é selada no Go com o
`secretbox` por token curto e cifrado; o n8n recebe somente o token opaco e prompts/contexto.
Sem essa configuração, o boot continua no executor nativo (rollback seguro) e registra o bloqueio.
O workflow segue inativo e não foi importado/ativado no runtime.

O inbox agora recebe `aiStatus` como projeção read-only do estado Go (`analyzing`,
`transferring`, `awaiting_client`, `human` ou `closed`) e exibe os badges correspondentes;
os filtros legados `OPEN/PENDING/CLOSED` continuam sendo derivados separadamente.

O gateway valida expiração, dispatch/generation, provider/model e executa a chamada provider no Go
sem devolver a chave. O adaptador n8n valida `brain.result.v2`, rejeita resultado de outra geração e
converte o shape legado de triagem apenas dentro do workflow para manter versões antigas do painel
compatíveis durante a migração.

## 9. Critérios de aceite

- três mensagens em 2,5s geram um dispatch e uma chamada ao modelo;
- mensagem após o claim cria nova generation, sem perder nenhuma entrada;
- retry do job/n8n não duplica resposta/outbox/run cobrado;
- humano assume durante chamada: nenhuma resposta da IA é enviada;
- schema inválido tenta repair uma vez e depois segue fallback configurado;
- sugestão de fila inexistente não é aplicada;
- max turns transfere ou silencia conforme config;
- conta A nunca usa agente/chave/contexto da conta B;
- shadow grava decisão/custo, mas outbox não recebe envio;
- diff dos workflows não próprios é vazio.
