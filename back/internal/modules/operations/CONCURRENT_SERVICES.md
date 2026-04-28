# Concurrent Services (Atendimentos Paralelos)

## Overview

Permite que um consultor atenda múltiplos clientes simultâneos. O limite é configurável por loja.

**Status:** Em desenvolvimento — Fase 1: Banco + Backend

---

## Conceitos

### Atendimento Normal (Queue)
- Consultor entra na fila (`waitingList`)
- Quando "chega a vez", sai da fila e entra em `service` (`activeServices`)
- Um cliente por vez
- Ao encerrar, volta pra fila

### Atendimento Paralelo (Novo)
- Consultor já está em `service` (atendendo 1 cliente)
- Clica botão "+ Iniciar outro atendimento"
- **Não** sai da fila normal (já está em service)
- Cria novo `ServiceID`, novo cronômetro
- Consultor continua em status `service` (não há mudança de status)
- Apenas quando encerrar o **ÚLTIMO** atendimento paralelo volta à fila

### Limit Policy
- **Limite por loja:** `max_concurrent_services` (ex.: 10 atendentes simultâneos máximo na loja)
- **Limite por consultor:** `max_concurrent_services_per_consultant` (ex.: 2 clientes simultâneos por consultor)
- Ambos limitam. Ex.: loja com limite 10, consultor com limite 2 → máximo 2 paralelos daquele consultor.

### Paralelismo Metadata
Cada atendimento paralelo registra:
- `parallel_group_id` — ID compartilhado entre atendimentos sobrepostos do mesmo consultor
- `parallel_start_index` — Ordem: 1º paralelo = 1, 2º = 2, etc
- `sibling_service_ids` — IDs dos outros atendimentos paralelos simultâneos
- `start_offset_ms` — Tempo decorrido desde o 1º atendimento do grupo

**Exemplo:**
```
Consultor "Daniella" inicia atendimento A às 10:00:00
  → parallel_group_id = "grp_abc123"
  → parallel_start_index = 1
  → sibling_service_ids = []
  → start_offset_ms = 0

Daniella inicia atendimento B às 10:01:23 (1m23s depois)
  → parallel_group_id = "grp_abc123" (mesma)
  → parallel_start_index = 2
  → sibling_service_ids = ["svc_A_id"]
  → start_offset_ms = 83000 (1m23s em ms)

Daniella encerra A às 10:02:00
  → Histórico registra duração real
  → Daniella CONTINUA em service (ainda tem B)

Daniella encerra B às 10:05:00
  → Histórico registra duração real
  → Daniella AGORA volta à fila
```

---

## Data Model Changes

### Tables Modified

#### `operation_active_services`
**PK Change:** `(store_id, consultant_id)` → `(store_id, service_id)`

| Column | Type | Notes |
|--------|------|-------|
| store_id | uuid | |
| consultant_id | uuid | (não unique mais) |
| service_id | text | Agora é parte da PK |
| service_started_at | bigint | |
| queue_joined_at | bigint | (no paralelo: timestamp do click no botão) |
| queue_wait_ms | bigint | (no paralelo: 0) |
| queue_position_at_start | int | (no paralelo: NULL) |
| start_mode | text | Agora aceita `'parallel'` |
| skipped_people_json | jsonb | (no paralelo: `[]`) |
| **parallel_group_id** | **text** | **NEW** |
| **parallel_start_index** | **int** | **NEW** |
| **sibling_service_ids_json** | **jsonb** | **NEW** |
| **start_offset_ms** | **bigint** | **NEW** |

#### `operation_service_history`
Adicionadas as 4 colunas de paralelismo (mesmo nomes).

#### `store_operation_settings`
Adiciona: `max_concurrent_services_per_consultant integer not null default 1`

---

## Business Rules

1. **Ao iniciar atendimento normal (fila):**
   - Requer: há gente na fila, consultor não está em service
   - Valida: `activeServices.count < max_concurrent_services` (por loja)
   - Cria: novo `ActiveService`, transita status `queue` → `service`

2. **Ao iniciar atendimento paralelo (novo botão):**
   - Requer: consultor já está em `service`, há limite disponível
   - Valida: 
     - `activeServices.filter(consultantID == $id).count < max_concurrent_services_per_consultant`
     - `activeServices.count < max_concurrent_services` (por loja ainda conta)
   - Cria: novo `ActiveService` com `start_mode='parallel'`, **NÃO** muda status (já está `service`)
   - Calcula: `parallel_group_id` (novo UUID ou baseado em timestamp+ID)

3. **Ao encerrar atendimento:**
   - Localiza por `ServiceID` (não mais por `PersonID` único)
   - Remove da lista `activeServices`
   - Se `activeServices.filter(consultantID == $id).count == 0` (não há mais paralelos):
     - Transita: `service` → `queue`, devolve à fila
   - Caso contrário:
     - Status **permanece** `service`

4. **Ao pausar/desativar consultor:**
   - Bloqueia se há atendimentos ativos (normal ou paralelo)
   - Mensagem: "Consultor com 2 atendimentos em progresso, encerre antes de pausar"

---

## API Changes

### New Endpoint

```
POST /api/operations/services/parallel
{
  "storeId": "uuid",
  "personId": "uuid"
}

Response:
{
  "ok": true,
  "storeId": "uuid",
  "action": "start-parallel",
  "serviceId": "...",
  "savedAt": "2026-04-27T..."
}
```

### Modified Endpoint

```
POST /api/operations/services/finish
{
  "storeId": "uuid",
  "serviceId": "...",  // CHANGED from personId
  "personId": "uuid",  // (kept for backward compat, derived from serviceId)
  "outcome": "compra",
  ...
}
```

---

## Service Layer

### Key Methods

#### `StartParallel(ctx, access, input StartParallelCommandInput) (MutationAck, error)`
```go
type StartParallelCommandInput struct {
  StoreID  string
  PersonID string
}
```
- Localiza consultor em `activeServices`
- Valida limites (por consultor + por loja)
- Cria novo `ActiveServiceState` com metadata de paralelismo
- Não transita status
- Publica evento `start-parallel`

#### `Finish(ctx, access, input FinishCommandInput) (MutationAck, error)` (modificado)
```go
type FinishCommandInput struct {
  StoreID   string
  ServiceID string  // NEW (pode conter PersonID como fallback)
  PersonID  string  // (deprecated, do NOT use)
  // ... resto igual
}
```
- Localiza `ActiveService` por `ServiceID`
- Remove da lista
- Se é último paralelo do consultor: transita `service` → `queue`
- Caso contrário: não transita

#### Status Transitions (modificado)
```go
func applyStatusTransitions(...) {
  // Quando novo paralelo é adicionado:
  // Se consultor já está em 'service', transição é NOOP
  // (não cria nova session)
  
  // Quando último paralelo é encerrado:
  // Se consultor não tem mais atendimentos:
  //   transita service → queue
  //   cria nova session
}
```

---

## Implementation Checklist

### Phase 1: Data Layer ✅
- [x] Migration 0030: refatoração de PK + colunas paralelo
- [x] Migration 0031: add setting per-consultant

### Phase 2: Backend Service 
- [ ] `model.go`: novos campos
- [ ] `service.go`: `StartParallel` method
- [ ] `service.go`: `Finish` refatorado pra `ServiceID`
- [ ] `service.go`: lógica de status simplificada
- [ ] `store_postgres.go`: SELECT/INSERT ajustado
- [ ] `http.go`: novo endpoint + refatoração

### Phase 3: Frontend
- [ ] `OperationActiveServiceCard.vue`: botão paralelo
- [ ] `operation-actions.ts`: `startParallelService` action
- [ ] Modal refatoração `personId` → `serviceId`

### Phase 4: Settings
- [ ] `settings/model.go`: novo campo
- [ ] Settings UI: input 1-5

### Phase 5: Docs + Tests
- [ ] Tests unitários
- [ ] AGENTS.md em módulos tocados
- [ ] Manual test plan

---

## Migration Strategy

### Zero-downtime
1. Deploy migrations (0030, 0031) com feature flag desativada
2. Restartar app (migrations rodadas, campo novo existe)
3. Deploy código backend (lê/escreve novos campos)
4. Deploy frontend (nova UI)
5. Ativar feature flag (admins podem mudar setting)

### Existing Data
- Atendimentos existentes em `activeServices` continuam como estão
- Novos campos (parallelism metadata) iniciam NULL/0
- Nenhuma perda de dados

---

## Testing Strategy

### Unit Tests
- `StartParallel` com/sem limite
- `Finish` último vs. não-último paralelo
- Metadata de paralelismo (offset, group, etc)

### Integration
- Criar 2 atendimentos paralelos, encerrar ambos
- Validar transições de status
- Validar history entries

### Manual
- UI: golden path (iniciar, ver botão, iniciar paralelo, encerrar)
- Limite por consultor bloqueando
- Limite por loja bloqueando
- Toast feedback

---

## Known Issues / TODOs

- [ ] Modal de encerramento: suportar múltiplos abertos ao mesmo tempo?
- [ ] Relatório: construir dashboard "qualidade × paralelismo"
- [ ] Métricas: persistência confirmada, cálculos prontos?
- [ ] Webhooks: novos eventos publicados?

---

## References

- [Parallelism Analytics Plan](../../docs/plan-feedback-5.2.1.md) (futuro relatório)
- Migrations: `0030_active_services_parallel.sql`, `0031_per_consultant_concurrency.sql`
