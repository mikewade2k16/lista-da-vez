# Panel Embed Contract

## Objetivo

Este documento explica o que um painel host precisa fornecer para embutir a interface Omni dentro de outro sistema que ja tenha:

- auth proprio
- usuarios proprios
- clientes/tenants proprios
- lojas proprias

## Modos de integracao

### 1. Painel Omni completo

Usar `web + back` do proprio projeto.

### 2. Frontend Omni em outro backend

O host reaproveita o `web/` e fornece endpoints equivalentes.

### 3. Somente modulo de operacao

O host embute apenas as telas e stores da operacao, com um subconjunto de endpoints.

## Contrato minimo para embutir a operacao

Para usar a tela `/operacao` com a experiencia atual, o host precisa fornecer:

### Sessao/contexto

- `POST /v1/auth/login` ou fluxo equivalente de sessao
- `GET /v1/me/context`

O contexto precisa devolver:

- usuario autenticado
- papel mapeado para os papeis Omni
- `tenantId`
- `storeIds[]`
- `activeStoreId`
- lista de lojas acessiveis

### Operacao

- `GET /v1/operations/snapshot`
- `GET /v1/operations/overview` se quiser modo integrado multi-loja
- `POST /v1/operations/queue`
- `POST /v1/operations/pause`
- `POST /v1/operations/resume`
- `POST /v1/operations/assign-task`
- `POST /v1/operations/start`
- `POST /v1/operations/finish`

### Realtime

- `GET /v1/realtime/operations`
- `GET /v1/realtime/context` se quiser sincronizacao administrativa entre instancias

### Configuracao da loja

Se quiser o modal/fluxo completo atual:

- `GET /v1/consultants`
- `GET /v1/settings`

## Contrato minimo para embutir o painel administrativo completo

Além do bloco acima, o host precisa fornecer:

- `GET /v1/stores`
- `POST /v1/stores`
- `PATCH /v1/stores/{id}`
- `POST /v1/stores/{id}/archive`
- `POST /v1/stores/{id}/restore`
- `DELETE /v1/stores/{id}`
- `GET /v1/users`
- mutacoes de usuarios/acessos
- `GET /v1/reports/*`
- `GET /v1/analytics/*`

## Roles esperadas pela UI

O frontend trabalha com estes papeis normalizados:

- `consultant`
- `store_terminal`
- `manager`
- `marketing`
- `owner`
- `platform_admin`

Se o host usar outros nomes, ele deve mapear isso no endpoint de contexto.

## Regra de acoplamento do frontend

O frontend nao deve depender do banco ou do auth concreto do projeto.

Ele deve depender apenas de:

- sessao/token/cookie valido
- contexto autenticado no shape esperado
- endpoints HTTP equivalentes
- websocket equivalente quando o caso pedir realtime

## Referencias

- `AGENTS.md`
- `app/pages/operacao/operations.md`
- `../back/CORE_MODULES_PORTABILITY.md`
