# E5 — handoff e operação humana

**Status:** `E5-DB-02 + E5-BE-03/04/05 + policies determinísticas APLICADOS localmente; FE de assumir/liberar + detalhe de handoff/SLA + transferência de fila + editor de policies entregue; calendário comercial, outbox do aviso ao cliente e QA Evolution pendentes`

**Resultado:** a IA transfere uma conversa com resumo/campos/evidências para a fila correta, um
humano assume sem corrida, e SLA/estado ficam visíveis e auditáveis.

## 1. Decisões fechadas

- a máquina de estados existente continua única; não criar booleanos `ai_active`, `assigned` etc.;
- IA sugere destino; routing engine valida regra/fila ativa/membership;
- handoff gera snapshot imutável do que o humano precisa, sem exigir leitura do output bruto;
- assumir conversa cancela dispatch IA antes de permitir mensagem humana;
- SLA é calculado no backend e persistido em eventos; front só apresenta;
- distribuição automática inicial é determinística e configurável, sem LLM.

## 2. Banco

Criar `messaging.handoffs`:

| Campo | Regra |
|---|---|
| `id`, `account_id`, `conversation_id` | escopo/FK |
| `ai_run_id`, `routing_decision_id` | evidência opcional |
| `reason_code` | enum fechado: requested, low_confidence, max_turns, tool_failed, policy, error |
| `summary` | texto limitado e mascarado conforme acesso |
| `collected_fields` | snapshot apenas de defs permitidas |
| `source_state`, `target_queue_id` | contexto da transição |
| `status` | `requested`, `queued`, `accepted`, `cancelled`, `closed` |
| `requested_at`, `queued_at`, `accepted_at`, `closed_at` | métricas |
| `accepted_by_user_id` | atendente da conta/fila |

No máximo um handoff aberto por conversa (unique parcial). Criar `messaging.queue_sla_policies`
por conta/fila com `first_response_seconds`, `resolution_seconds`, horário/calendário aplicável e
enabled. Criar `messaging.sla_events` (`started`, `warning`, `breached`, `paused`, `resumed`,
`satisfied`) com idempotency key. Se já houver scheduler/calendário operacional equivalente,
referenciá-lo; não copiar feriados para JSON local.

Criar `messaging.handoff_policies` com prioridade estável, enabled, condições fechadas (setor,
horário UTC, confidence, intent, lifecycle do cliente, tag e risco de SLA), target/fallback e
`customer_notice_template`. O motor avalia regras determinísticas sob lock e registra `policy_id`
mais `policy_snapshot` no handoff; modelo não cria condição nem template. A janela comercial com
feriados/calendário de conta permanece no pacote seguinte, sem duplicar calendário em JSON.

## 3. Service e concorrência

### Solicitar handoff

1. lock da conversa;
2. validar state AIAllowed e generation do dispatch;
3. validar/sanitizar reason/summary/campos;
4. executar routing engine; fallback para fila default/unrouted conforme domínio;
5. criar handoff idempotente e transicionar para `queued`;
6. cancelar dispatches IA abertos;
7. iniciar SLA e publicar realtime após commit.

Se `customer_notice_template` estiver configurado e a janela/capability permitir, produzir uma
mensagem pela outbox na mesma decisão idempotente; sem template/capability, registrar que o aviso
foi omitido, nunca enviar texto proibido.

### Assumir

Endpoint `POST /v1/omnichannel/conversations/{id}/take` com idempotency key:

- lock; validar membership/permission e conversa ainda disponível;
- atribuir `assigned_user_id`, transicionar `human.assign`, aceitar handoff e satisfazer first SLA;
- dois atendentes concorrendo: exatamente um 200; outro 409 com estado atual;
- qualquer resultado IA posterior é no-op.

### Transferir/liberar/encerrar

- transferir valida queue/usuário no tenant, cria routing decision/manual audit e reinicia SLA
  conforme policy;
- liberar remove responsável e volta à fila sem reativar IA automaticamente;
- pending pausa apenas SLAs definidos como pausáveis;
- fechar exige reason configurado quando aplicável; reabrir cria novo ciclo, não reescreve o anterior.

## 4. APIs e realtime

Adicionar/estabilizar:

- `GET /v1/omnichannel/queues/{id}/workload`;
- `POST /conversations/{id}/take`;
- `POST /conversations/{id}/release`;
- `GET /conversations/{id}/handoffs`;
- `GET /conversations/{id}/sla`;
- CRUD de SLA em `/settings/queues/{id}/sla`.

Eventos versionados: `handoff.requested`, `conversation.assigned`, `conversation.transferred`,
`sla.updated`. Payload mínimo, tenant-scoped, sem output bruto do modelo.

Notificação do responsável/fila usa o serviço interno de notificações após commit e chave
idempotente. Presença vem do heartbeat/realtime compartilhado, é efêmera e pode ser `unknown`; não
persistir “online” como verdade duradoura nem rotear exclusivamente por presença sem fallback.

## 5. Frontend

- sidebar oferece visões “Minhas”, “Não atribuídas”, “Minha fila” e “SLA em risco”, respeitando
  gate do backend;
- card mostra fila, responsável, origem, prioridade e relógio SLA;
- cabeçalho permite assumir, liberar, transferir, pending e encerrar conforme capability;
- detalhes exibem resumo do handoff, campos coletados, origem e motivo;
- atualização concorrente 409 reconcilia a conversa e explica que outro atendente assumiu;
- badge “IA pausada/humano assumiu” é autoritativo, não inferido localmente;
- ações perigosas têm confirmação e foco acessível.

## 6. Pacotes atômicos

| Pacote | Entrega |
|---|---|
| `E5-CONTRACT-01` | matriz de transições/eventos e contrato do snapshot |
| `E5-DB-02` | handoffs, SLA policies/events |
| `E5-BE-03` | request handoff e cancelamento IA |
| `E5-BE-04` | take/release/transfer concorrentes |
| `E5-BE-05` | scheduler/eventos SLA |
| `E5-FE-06` | filas, ações e resumo/SLA |
| `E5-QA-07` | teste de corrida e operação end-to-end |

## 7. Aceite

- handoff por max turns entra na fila com resumo/campos e uma routing decision;
- fila sugerida inexistente cai no fallback determinístico;
- duas tomadas simultâneas produzem um único responsável;
- humano envia enquanto IA termina: somente mensagem humana sai;
- transferir para fila de outra conta retorna 404 e não muda estado;
- SLA inicia, alerta, viola, pausa/retoma e satisfaz sem duplicar eventos após restart;
- policy por horário/cliente/confiança é reproduzível e o aviso ao cliente respeita capability;
- responsável recebe uma notificação idempotente e presença indisponível não perde a conversa;
- workload bate com consultas de banco por tenant/fila;
- front reflete transições por realtime e reconcilia perda de evento;
- nenhum workflow envia ao canal.

## Implementação local atual

- Migration `0220_messaging_handoff_operational.sql`: `handoffs`, `queue_sla_policies`,
  `sla_events`, `handoff_policies` e ampliação da auditoria existente.
- `POST /v1/omnichannel/conversations/{id}/handoff` cria snapshot tenant-scoped, aplica fila
  sugerida/default e invalida dispatch/outbox de IA na mesma transação.
- `POST /v1/omnichannel/conversations/{id}/take` usa lock da conversa; o primeiro atendente vence,
  o concorrente recebe `409`, e resultado atrasado da IA vira no-op. Leituras de handoff e SLA
  estão disponíveis em `/handoffs` e `/sla`.
- `POST /v1/omnichannel/conversations/{id}/release` reutiliza a FSM (`human.unassign`) e não
  reativa IA automaticamente; a transferência de fila continua no endpoint canônico `PATCH
  /conversations/{id}/queue`.
- O cabeçalho do inbox agora expõe `Assumir` para conversas sem responsável e `Liberar` para
  o atendente atual; ambas as ações usam o backend autoritativo, loading, erro acionável e
  reconciliação da conversa. O botão não envia mensagens nem altera estado local por conta
  própria.
- O detalhe da conversa lê `/handoffs` e `/sla` somente após trocar a conversa, exibe motivo,
  resumo, nomes dos campos coletados e os últimos eventos SLA; transferência usa
  `PATCH /conversations/{id}/queue`, reidrata a projeção da conversa e não cria estado local
  concorrente. Falha de permissão para listar filas não fabrica opções de destino.
- A migration `0221_messaging_handoff_policy_snapshot.sql` adiciona `policy_id` e `policy_snapshot`
  ao handoff. O backend expõe CRUD em `/settings/handoff-policies`, avalia condições fechadas sob
  lock e a aba `Handoff` do drawer permite configurar prioridade, filas, condições e template sem
  expor credenciais ou permitir envio direto.
- Fixture descartável `TestTakeConversationE5` comprovou a corrida e o cancelamento de IA;
  migration completa passou em banco limpo. O scheduler básico cria eventos `started`, `warning`
  e `breached` com chaves idempotentes; ainda faltam calendário de horário comercial, outbox do
  aviso ao cliente e smoke real com Evolution.
