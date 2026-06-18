# Plano — Ajustes Módulo Fila / Página Operação

> Documento canônico desta entrega. Espelhado em `roadmap-data.ts` (grupo `fila-operacao`,
> fase `operacao-ajustes`) e nos `AGENT.md` dos módulos tocados.
> Status: **CÓDIGO LOCAL CONCLUÍDO (2026-06-16)** — 4 lanes fechadas inline pelo orquestrador
> (os subagentes em background travaram num gate de validação). Gates locais OK: Go build/vet/gofmt +
> go test (operations/reports); web eslint 0 erros + vue-tsc sem erro novo nos arquivos tocados +
> vitest 46/46. **Falta o usuário:** aplicar migration `0159` + `docker compose up -d --build api` +
> validar no browser. `TestAllMigrationsApply`/`govulncheck` rodam no CI (banco limpo).
> Criado em 2026-06-16.

## Contexto

Quatro ajustes na página **Operação** (`web/app/pages/operacao/index.vue` + `web/app/components/operation/*`)
e no módulo Go `queue/operations` + `queue/reports`. Decisões de produto já tomadas com o usuário:

- **Item 1:** ao filtrar UMA loja no modo "Todas as lojas", aquela loja vira contexto operável; "Todas as lojas" segue só leitura.
- **Item 4:** métricas de pausa numa seção **"Pausas"** dentro de Relatórios.

---

## Item 1 — Controle operacional em loja individual (multi-loja)

**Sintoma:** quem tem acesso a todas as lojas perdeu os botões de iniciar/encerrar/pausar atendimento.

**Causa raiz (confirmada no código):**
- `operacao/index.vue:29-35` força `scopeMode='all'` para `canSeeIntegrated` (usuário multi-loja).
- `OperationWorkspace.vue:45` → `showIntegratedView = canSeeIntegrated && scopeMode==='all'`, passado como
  `integrated-mode` para os filhos.
- Os botões são gateados por `!integratedMode`:
  - `OperationQueueColumns.vue:483, 529, 543, 553` (atender primeiro / na vez / fora da vez).
  - `OperationActiveServiceCard.vue:311` (`v-if="!readOnly && !integratedMode"` → parar/encerrar).
  - `OperationConsultantStrip.vue` só renderiza com `canOperate`, mas as ações por loja dependem do `integratedMode`.
- O filtro de loja (`OperationScopeBar.vue` → `integratedStoreId`) só filtra a visão agregada; **não** habilita operação.

**Decisão:** quando `scopeMode==='all'` E `integratedStoreId` aponta para UMA loja com snapshot real carregado,
tratar como contexto single-store operável daquela loja. "Todas as lojas" (sem filtro) continua agregado/leitura.

**Mudanças (frontend apenas — sem backend; mutações já validam `store_id` contra o Principal):**
1. `operacao/index.vue`: ao selecionar uma loja no filtro (e em updates realtime dessa loja), chamar
   `operationsStore.refreshOperationSnapshot(integratedStoreId)` para carregar o snapshot REAL da loja
   (mecanismo já existe — `stores/operations.ts:200`, hidrata `state.storeSnapshots[storeId]`).
2. `OperationWorkspace.vue`:
   - Novo computed `operableStoreId` = `scopeMode==='all' && integratedStoreId && hasTrustedScopedSnapshot(integratedStoreId)`.
   - Quando operável: montar `displayState` a partir do snapshot escopado dessa loja (não do `overview`),
     e passar `integrated-mode = false` + `read-only = !canOperate` aos filhos.
   - Garantir que `activeStoreId`/`finishModalServiceId` resolvam para a loja operada (o modal e as ações
     já passam `service.storeId`/`employee.storeId`; validar draft-key `storeId:serviceId`).
3. Filhos (`OperationQueueColumns`, `OperationActiveServiceCard`, `OperationConsultantStrip`): **sem mudança** —
   já reagem aos props `integrated-mode`/`read-only`.

**Risco principal:** o `OperationFinishModal` lê `props.state.activeStoreId` para draft-key. Validar que,
operando uma loja filtrada, o draft e o `finishService` usam a `storeId` do serviço (não a loja ativa do login).

**Verificável:** usuário com acesso a N lojas, em "Todas as lojas" → leitura; ao escolher 1 loja no filtro →
inicia/encerra/para/pausa/retoma como um operador comum, com timers/serviceId reais daquela loja.

---

## Item 2 — Remover o ID do topo do modal de encerrar

**Causa:** `OperationFinishModal.vue:157-159` →
`{{ serviceDisplayName(service) }} | ID {{ service.serviceId }}`.

**Mudança:** subtítulo passa a mostrar só `{{ serviceDisplayName(service) }}` (nome do consultor). Remover `| ID ...`.
O board card e o modal de parar/cancelar (`OperationQueueColumns.vue:636`) já mostram só o nome — consistente
(regra "modal e board card espelhados").

**Verificável:** modal de encerrar mostra só o nome do consultor; nenhum ID/serviceId visível.

---

## Item 3 — Justificativa só aparece ao tentar avançar

**Sintoma:** no passo Cliente (reserva/não-compra), os campos de justificativa de "campo não preenchido"
aparecem de imediato, antes do usuário tentar avançar.

**Causa raiz:** `useFinishModalController.js` expõe `step1MissingJustifications` (linha 1059) e
`step2MissingJustifications` (1107) como computeds reativos — qualquer campo vazio com
`requireXJustification=true` entra na lista na hora. Renderizados sempre em:
- `FinishStepProduct.vue:262` `<section v-if="step1MissingJustifications.length">`
- `FinishStepNotes.vue:278` `<section v-if="step2MissingJustifications.length">`
(+ badges de qualidade em `FinishStepProduct.vue:335` e `FinishStepNotes.vue:402`).

**Decisão:** revelar as justificativas só após a tentativa de avançar/concluir. Aplicar nos DOIS passos (consistência).

**Mudanças (`useFinishModalController.js` + 2 step components + modal):**
1. Novo ref `justificationsRevealed = ref(false)`.
2. Em `goToStep2()` (1535): se há `step1MissingJustifications` inválidas, setar `justificationsRevealed=true`
   antes de `validateFieldJustifications`/alerta (o campo aparece para o usuário preencher e tentar de novo).
3. Em `submitForm()` (1587): idem, considerando step1+step2.
4. Resetar `justificationsRevealed=false` em `resetForm()`, `clearCurrentDraft()` e no `watch(form.outcome)`.
5. Expor `justificationsRevealed` no `return`; `OperationFinishModal.vue` passa `:justifications-revealed`
   para `FinishStepProduct` e `FinishStepNotes`.
6. Nos step components, gatear as seções/badges:
   `v-if="justificationsRevealed && stepXMissingJustifications.length"`.

**Verificável:** ao entrar no passo Cliente com campos vazios, NENHUMA justificativa aparece; ao clicar
"Avançar"/"Concluir" sem preencher, aí sim a justificativa surge (e o alerta orienta). Preenchendo o dado,
a justificativa some.

---

## Item 4 — Persistir motivo/kind das pausas + métrica em Relatórios

**Estado real (corrige a percepção "não salva no banco"):** a DURAÇÃO de cada pausa já é gravada —
toda transição de status emite uma linha append-only em `queue.operation_status_sessions`
(`status='paused'`, `started_at`, `ended_at`, `duration_ms`) via `applyStatusTransitions`
(`operations/snapshot.go:204`). **Falta:** (a) o MOTIVO e o KIND da pausa (vivem só em
`operation_paused_consultants`, apagado no resume) e (b) qualquer relatório que leia isso.

### 4a — Persistência (migration + backend, exige rebuild da api)
1. **Migration `0159_pause_session_metrics.sql`** (idempotente, schema-qualificada):
   - `alter table queue.operation_status_sessions add column if not exists reason text;`
   - `alter table queue.operation_status_sessions add column if not exists kind text;`
   - `create or replace view public.operation_status_sessions as select * from queue.operation_status_sessions;`
     (a view com `select *` só pega as colunas novas se for recriada).
2. **Model** (`operations/model.go`): `ConsultantSession` ganha `Reason string` + `Kind string`.
3. **Transição** (`operations/snapshot.go:204` `applyStatusTransitions`): receber a lista de pausados ANTES
   da transição (`[]PausedStateItem`); ao fechar uma sessão cujo `previous.Status == statusPaused`, anexar
   `reason`/`kind` do item pausado correspondente. Threadear o argumento novo em todos os call sites
   (`service_pause.go`, `service.go`). `buildSnapshotView` NÃO cria sessão (só ajusta `currentStatus`), então
   não precisa.
4. **Persist** (`operations/store_postgres.go`): `appendSessions` (740) insere `reason, kind`; `loadSessions`
   (462) seleciona `reason, kind` com scan `*string` (nullable, regra Go).

### 4b — Relatório de Pausas (endpoint + UI)
1. **Backend reports** (`queue/reports`): novo `GET /v1/reports/pauses` (`http.go`, padrão `middleware.RequireAuth`,
   mesmo escopo dos outros reports). Agrega `operation_status_sessions where status='paused'` join
   `queue.consultants` (nome), filtrado por `storeId`/`tenantId`/`dateFrom`/`dateTo`/`consultantIds`.
   Retorna por consultor: nº de pausas, duração total/média, e breakdown por motivo e por hora do dia.
2. **Contrato (fixo aqui para D rodar em paralelo com C):**
   ```jsonc
   GET /v1/reports/pauses?storeId&dateFrom&dateTo&consultantIds
   {
     "storeId": "...",
     "filters": { ... },
     "summary": { "totalPauses": 0, "totalDurationMs": 0, "averageDurationMs": 0, "distinctConsultants": 0 },
     "byConsultant": [
       { "consultantId": "...", "consultantName": "...", "pauseCount": 0,
         "totalDurationMs": 0, "averageDurationMs": 0,
         "byReason": [ { "reason": "Almoço", "count": 0, "totalDurationMs": 0 } ] }
     ],
     "byReason": [ { "reason": "Almoço", "count": 0, "totalDurationMs": 0 } ],
     "byHour": [ { "hour": "13", "count": 0, "totalDurationMs": 0 } ],
     "rows": [
       { "consultantId": "...", "consultantName": "...", "reason": "Almoço", "kind": "pause",
         "startedAt": 0, "endedAt": 0, "durationMs": 0 }
     ]
   }
   ```
   (`kind` distingue pausa `pause` de deslocamento `assignment`; o relatório de pausas foca `kind='pause'`,
   mas expõe os dois.)
3. **Frontend** (`web/app/pages/relatorios.vue` + store/composable de reports): nova seção/aba **"Pausas"**
   consumindo o endpoint — cartões de resumo (qtd, duração total/média), tabela por consultor, gráfico por
   motivo e por hora. Respeitar design system (tokens), sem hex.

**Verificável:** pausar e retomar um consultor → sessão gravada com `reason`/`kind`; a aba Pausas em Relatórios
mostra a pausa (consultor, motivo, horário, duração) e os agregados por motivo/hora.

---

## Divisão em subagentes (Opus) — paralelizável

Sem sobreposição de arquivos → os quatro podem rodar em paralelo. Contrato do endpoint de pausas fixado
acima desbloqueia C×D em paralelo.

| Agente | Lane | Arquivos principais | Dep. |
|---|---|---|---|
| **A** | Front — controle multi-loja (Item 1) | `operacao/index.vue`, `OperationWorkspace.vue`, (leitura) `stores/operations.ts` | nenhuma |
| **B** | Front — modal encerrar (Itens 2+3) | `OperationFinishModal.vue`, `useFinishModalController.js`, `FinishStepProduct.vue`, `FinishStepNotes.vue` | nenhuma |
| **C** | Back — pausas: persistência + endpoint (Item 4a+4b-back) | migration `0159`, `operations/{model,snapshot,store_postgres,service,service_pause}.go`, `reports/{model,service,store_postgres,http}.go`, AGENT.md | nenhuma |
| **D** | Front — Relatórios "Pausas" (Item 4b-front) | `relatorios.vue` + store/composable reports | contrato de C (fixado) |

**Regras para os agentes (memória do projeto):**
- Trabalhar SÓ localmente; nenhum agente roda git (só o usuário). Sessão multi-agente: nenhum `git`.
- Validar local (build/lint/type-check + teste de browser no que for UI).
- C mexe em Go → exige `docker compose up -d --build api` + aplicar migration `0159` (comandos para o USUÁRIO rodar).
- **roadmap-data.ts e este plano são atualizados pelo orquestrador** (não pelos agentes) para evitar conflito
  de edição concorrente; cada agente atualiza só o(s) `AGENT.md` do seu módulo.
- Sem emojis, sem `Co-Authored-By`, arquivos ≤450 linhas, tokens de cor do design system.

## Notas de Deploy
- **Migration nova:** `0159_pause_session_metrics.sql` (ordem: aplicar antes de subir a api nova).
- **Rebuild obrigatório da api** (mudança em Go): `docker compose up -d --build api`.
- Sem env var nova. Sem mudança de porta.

### Ajuste 2026-06-16 (follow-up Item 3) — dados do cliente: "preenche OU justifica" (FRONT-ONLY)
- Decisão: validação simples no FRONT, independente de config de tenant (a tentativa de mudar o default no
  backend foi REVERTIDA — não era o lugar e não pegava tenants com config salva).
- `useFinishModalController.js`: nome/telefone/e-mail deixam de ter bloqueio duro "obrigatório" no `submitForm`;
  em `step2MissingJustifications`, esses 3 passam a exigir justificativa sempre que estiverem **vazios** na
  finalização (`requiresInput: showXField && !hasValue`). Resultado: ao finalizar sem o dado, em vez de só
  cobrar, aparece o input de justificativa (mín. configurado, default 20) e ele precisa ser preenchido.
- Reveal continua por passo (`step1JustificationsRevealed`/`step2JustificationsRevealed`), na tentativa de
  avançar/finalizar. Demais inputs/regras de justificativa inalterados.
- Só front: **não exige rebuild da api** para este item (basta recarregar o web).
