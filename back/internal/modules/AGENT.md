# AGENT

## Escopo

Estas instrucoes valem para os modulos em `back/internal/modules/`.

## O que e um modulo aqui

Modulo e uma unidade de negocio do backend, por exemplo:

- `auth`
- `access`
- `tenants`
- `stores`
- `consultants`
- `settings`
- `operations`
- `reports`
- `analytics`
- `users`

Cada modulo deve poder crescer sem transformar `internal/platform` em lugar de regra de produto.

## Regra de desenho

- cada modulo deve ter sua propria pasta
- cada modulo deve ter seu proprio `AGENT.md`
- cada modulo deve expor contratos pequenos para o resto da aplicacao
- handlers HTTP do modulo devem conversar com services do proprio modulo

## Regra de dependencia

- modulo pode usar `internal/platform/*`
- modulo nao deve acessar arquivos internos de outro modulo diretamente
- quando precisar conversar com outro modulo, usar interface pequena no service consumidor

## Regra de escopo

Como este produto e multi-tenant:

- modulos operacionais devem nascer tenant-aware
- tudo que for por loja deve considerar tambem `store_id`
- autorizacao deve acontecer antes da regra sensivel

## Regra de rollout

Preferir esta sequencia para cada modulo:

1. modelagem
2. service
3. handler HTTP
4. repositorio em memoria ou mock tecnico
5. repositorio PostgreSQL
6. eventos/websocket, se fizer sentido

## Modulos atuais

- `auth`: identidade, token, perfil, convite, reset e middleware
- `access`: catalogo de permissoes, grants por papel e overrides por usuario
- `tenants`: clientes/grupos acessiveis e ciclo administrativo basico
- `stores`: lojas acessiveis e ciclo administrativo basico
- `consultants`: roster operacional por loja e vinculo com conta real
- `settings`: configuracao operacional por loja, catalogos e produtos
- `operations`: fila, atendimento, status corrente, historico e overview integrado
- `realtime`: WebSocket de invalidacao operacional por loja e contexto administrativo por tenant
- `reports`: relatorios server-side sobre historico operacional
- `analytics`: agregados para ranking, dados e inteligencia
- `users`: administracao de contas, papeis, escopo, convite e reset administrativo

`campaigns` ainda nao existe como modulo Go em `back/internal/modules`; enquanto isso, seus dados aparecem como campos/filtros em historico, relatorios e analytics.
