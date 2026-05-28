# AGENT — platform/events

## Escopo

Pacote `back/internal/platform/events/`. Event bus in-process para comunicacao
assincrona entre modulos da plataforma multi-tenant.

Branch: `refactor/multi-tenant-core`. Plano mestre secao D.

## Por que existe

Modulos satelites (queue, finance, omni, ...) precisam reagir a eventos uns
dos outros sem se acoplar diretamente. Exemplos:

- `queue.service_finished` → `finance` cria comissao do consultor.
- `finance.invoice_paid` → `tasks` fecha tarefa de cobranca vinculada.
- `account.modules.changed` → `httpapi.AccountModulesGuard` invalida cache.

Manter um bus separado (em vez de chamadas diretas) permite que cada modulo
nao saiba quem escuta.

## Quando NAO usar

- **Leitura sincrona** (Finance pergunta nome de contato): use interface no
  `Dependencies` (ex: `Resolver`). Bus nao e RPC.
- **Mesmo modulo**: handler nao publica evento do mesmo modulo (deve ser
  sincrono). Reviewer rejeita PR que faca isso.

## Convencoes obrigatorias

- **Topico**: `<module>.<entity>.<verb_past>`. Ex: `queue.service_finished`,
  `finance.invoice_paid`. Sem espacos, lowercase.
- **AccountID**: obrigatorio em todo evento que dependa de tenancy (quase
  todos). Vazio so em eventos plataforma-wide raros.
- **CausationID + CorrelationID**: bus calcula `Depth` automaticamente para
  detectar loops. Ao publicar dentro de um handler, copiar `CorrelationID`
  do evento que voce recebeu para preservar a cadeia.
- **MaxEventDepth = 10**: bus rejeita evento com profundidade maior. Indicio
  de loop entre handlers.

## API resumida

```go
bus := events.NewInMemoryBus(logger)

// Publicar
bus.Publish(ctx, events.Event{
    Topic:     "queue.service_finished",
    AccountID: principal.AccountID,
    Payload:   map[string]any{"serviceId": "...", "consultantId": "..."},
    // ID, OccurredAt, CorrelationID gerados se vazios.
})

// Consumir
sub := bus.Subscribe("queue.service_finished", func(ctx context.Context, e events.Event) error {
    // ...
    return nil
})
defer sub.Unsubscribe()
```

## Implementacao atual: InMemoryBus

- Despacho **sincrono**. Erros de handler sao logados (nao param o despacho
  dos demais handlers do topico).
- Sem persistencia. Se o processo cair entre Publish e Subscribe processando,
  o evento e perdido. Para handlers criticos (ex: outbox de cobranca), criar
  tabela `core.event_outbox` (planejada — nao implementada ainda).

## Caminho para broker externo

A interface `Bus` e identica em forma a NATS/RabbitMQ/Kafka. Substituir o
`InMemoryBus` por adapter externo nao exige mudanca em quem publica/consome.
Quando algum modulo for extraido para microservico, fazer essa troca aqui.

## Sem testes ainda

Pacote criado na Fase 2; testes serao adicionados quando aparecer o primeiro
handler nao trivial (provavel Fase 6 com finance/queue integrando).

## Quando atualizar este AGENT.md

- Quando adicionar `event_outbox` (persistencia para handlers criticos).
- Quando trocar para broker externo.
- Quando adicionar wildcard subscribe ou recursos similares.
