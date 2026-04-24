# Core Modules Portability

## Objetivo

Este documento define como reaproveitar o nucleo Omni em outro painel que ja tenha:

- auth proprio
- gestao de usuarios propria
- tenant/cliente proprio
- modulo de lojas proprio

A meta nao e obrigar outro projeto a adotar todo o backend atual, e sim deixar claro o contrato minimo para plugar a fila de atendimento e os modulos derivados.

## O que e o core pluggavel hoje

O bloco mais pronto para reaproveitamento e:

- `operations`
- `realtime`
- `reports`
- `analytics`
- `settings` operacional, quando o host quiser reaproveitar o modal/catalogos

`auth`, `tenants`, `stores` e `users` podem continuar sendo do projeto host, desde que entreguem os contratos minimos descritos abaixo.

## Contrato minimo que o host precisa fornecer

### 1. Identidade e sessao

O host precisa conseguir entregar um contexto de acesso equivalente a:

```go
type AccessContext struct {
    UserID   string
    TenantID string
    Role     string
    StoreIDs []string
}
```

Campos minimos necessarios:

- `user_id`
- `tenant_id`
- `role`
- `store_ids[]`

Roles minimas esperadas pelo nucleo Omni:

- `consultant`
- `store_terminal`
- `manager`
- `marketing`
- `owner`
- `platform_admin`

Se o projeto host usar outros nomes de role, ele deve mapear para esse contrato antes de chamar o core.

### 2. Escopo de lojas acessiveis

O core de operacao nao precisa conhecer o modulo inteiro de lojas do host.

Ele precisa apenas de um provider equivalente a:

```go
type StoreScopeProvider interface {
    ListAccessible(ctx context.Context, access AccessContext, filter StoreScopeFilter) ([]StoreScopeView, error)
}
```

Com view minima:

```go
type StoreScopeView struct {
    ID       string
    TenantID string
    Code     string
    Name     string
    City     string
}
```

Uso atual:

- `operations` usa isso para `overview` integrado
- `reports` e `analytics` tambem precisam disso para validar escopo de leitura
- `realtime` precisa validar se a loja/tenant do socket pertence ao usuario

### 3. Roster de consultores

O core operacional precisa de uma lista de consultores por loja com os campos usados na operacao:

- `id`
- `store_id`
- `name`
- `role`
- `initials`
- `color`
- metas/comissao quando existirem

Hoje isso sai do repositorio de `consultants`, mas outro host pode entregar essa mesma view a partir do proprio cadastro de colaboradores.

### 4. Configuracao operacional

Se o host quiser usar o mesmo modal/fluxo completo do Omni, precisa fornecer por loja:

- `maxConcurrentServices`
- configuracao do modal
- motivos
- origens
- perdas
- profissao
- catalogo de produtos

Se o host so quiser a fila e um fechamento custom, pode substituir esse contrato desde que respeite o `FinishCommandInput` esperado pelo modulo `operations`.

### 5. Persistencia

O host precisa fornecer repositorios equivalentes aos contratos dos modulos.

Para `operations`, o contrato de persistencia continua sendo:

- fila atual
- atendimentos ativos
- pausas/tarefas
- status corrente
- sessoes de status append-only
- historico append-only

Isso pode ficar no mesmo Postgres do host ou em schema separado, desde que o adapter implemente os repositorios esperados.

### 6. Evento e realtime

O core nao exige broker externo para funcionar.

Hoje ele depende apenas de:

```go
type EventPublisher interface {
    PublishOperationEvent(ctx context.Context, event PublishedEvent)
}
```

Se o host nao quiser websocket, pode usar um publisher noop.

Se quiser realtime, precisa publicar:

- invalidacao de operacao por loja
- invalidacao administrativa por tenant

## Modulo por modulo

### `operations`

Ja esta mais preparado para plugabilidade.

O service agora recebe:

- `AccessContext`
- `Repository`
- `StoreScopeProvider`
- `EventPublisher`

Ou seja: ele nao depende mais diretamente do tipo concreto `auth.Principal` no service.

Ponto de adaptacao atual:

- o HTTP adapter do projeto converte `auth.Principal -> operations.AccessContext`
- o app atual converte `stores.Service -> operations.StoreScopeProvider`

Em outro projeto host, basta trocar esses adapters.

### `realtime`

Ainda depende do auth do projeto atual no handshake, mas o contrato real que ele precisa do host e:

- autenticar token/sessao
- devolver um contexto equivalente ao principal
- validar loja acessivel
- validar tenant acessivel

Se o host ja tiver websocket e auth proprios, da para reaproveitar o hub/eventos e trocar apenas o adapter de autenticacao.

### `reports`

Hoje ainda usa `auth.Principal` diretamente no service.

Dependencias reais:

- contexto de acesso
- finder/listador de lojas acessiveis
- leitura do historico operacional

Proximo passo recomendado:

- alinhar `reports` ao mesmo contrato `AccessContext + StoreScopeProvider` ja adotado em `operations`

### `analytics`

Hoje ainda usa `auth.Principal` diretamente no service.

Dependencias reais:

- contexto de acesso
- validacao de loja acessivel
- snapshot operacional
- roster
- settings por loja

Proximo passo recomendado:

- alinhar `analytics` ao mesmo contrato `AccessContext + StoreScopeProvider`

### `settings`

Pode ser reaproveitado se o host quiser o mesmo formulario/modal/configuracao operacional.

Dependencias reais:

- contexto de acesso com permissao administrativa
- loja alvo
- persistencia de configuracao por loja

Se o host ja tiver configurador proprio, ele pode substituir esse modulo e continuar chamando apenas `operations`.

## Como plugar em outro painel

### Caminho 1: embutir tudo

Usar:

- frontend `web`
- backend `back`
- auth/usuarios/lojas do proprio Omni

E o caminho mais simples.

### Caminho 2: embutir o core operacional em outro backend

O projeto host fornece:

- auth proprio
- usuarios/lojas proprios
- adapters para `AccessContext`
- adapters para `StoreScopeProvider`
- adapters de repositorio

E reaproveita:

- `operations`
- `realtime`
- `reports`
- `analytics`

### Caminho 3: embutir apenas o frontend Omni em outro painel

O projeto host precisa expor endpoints equivalentes para:

- `/v1/me/context`
- `/v1/operations/*`
- `/v1/realtime/operations`
- `/v1/reports/*`
- `/v1/analytics/*`

E, se quiser a area administrativa completa:

- `/v1/stores`
- `/v1/users`
- `/v1/consultants`
- `/v1/settings`

## Regra de ouro

Quando um modulo do core depender de outro modulo externo, a dependencia deve ser descrita por:

1. contexto minimo que ele precisa
2. interface minima que ele consome
3. shape minimo de dados que ele espera

Nunca por acoplamento implicito ao projeto inteiro.

## Proximo passo recomendado

Depois de `operations`, os proximos modulos a receber o mesmo tratamento explicito de plugabilidade devem ser:

1. `reports`
2. `analytics`
3. `settings`
