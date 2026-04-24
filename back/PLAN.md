# PLAN

## Objetivo

Plano de implementacao do backend Go alinhado com o frontend Nuxt e com a documentacao atual em `docs/`.

## Premissas tiradas da documentacao atual

- o frontend ja tem quase todo o comportamento operacional pronto e agora a fila principal ja persiste no backend
- auth real e persistencia backend ainda sao gaps oficiais
- o modulo principal e `fila de atendimento`
- realtime por websocket e requisito, mas nao deve entrar antes de auth, tenant e snapshot HTTP
- o backend precisa nascer multi-tenant, mesmo que o primeiro bootstrap use um tenant demo

Principais referencias usadas:

- `docs/BACKLOG.md`
- `docs/NUXT_MIGRATION_BLUEPRINT.md`
- `docs/NUXT_FULL_REFERENCE.md`
- `web/app/pages/operacao/operations.md`

## Resposta direta sobre a integracao com o Nuxt

Sim.

`tenants/stores` e exatamente a camada que faz sentido ligar ao Nuxt antes do websocket.

Ordem recomendada:

1. login e principal autenticado
2. tenant/store context
3. leitura e comandos HTTP da operacao
4. websocket para sincronizacao realtime

Motivo:

- o websocket precisa saber quem e o usuario
- a qual tenant ele pertence
- a quais lojas ele tem acesso
- qual snapshot inicial da loja ele esta vendo

Sem isso, o realtime nasce frouxo e mais dificil de proteger.

## Fase 0

### Base modular do backend

Status: concluido

- bootstrap HTTP em Go
- `internal/platform/*`
- modulo `auth`
- roles iniciais
- principal com `tenant_id`
- middleware de auth e role

## Fase 1

### Fundacao de dados

Status: concluido

Entregas:

- subir PostgreSQL para desenvolvimento
- definir estrategia de migrations
- criar schema inicial para:
  - users
  - tenants
  - stores
  - memberships/roles
- configurar repositorios reais para auth/tenant/store

Observacao:

Sem isso, ate conseguimos fazer smoke de integracao com store em memoria, mas nao e a integracao real que queremos para o Nuxt.

## Fase 2

### Identities, tenants e stores

Status: concluido para leitura de contexto e CRUD basico de lojas

Entregas:

- criar modulo `tenants`
- criar modulo `stores`
- aproveitar memberships reais que ja alimentam o modulo `auth`
- modelar:
  - tenant
  - store
  - user
  - user_tenant_role
  - user_store_role quando necessario
- endpoints iniciais:
  - `GET /v1/me/context`
  - `GET /v1/tenants`
  - `GET /v1/stores`
  - `POST /v1/stores`
  - `PATCH /v1/stores/{id}`

Objetivo no Nuxt:

- substituir o perfil fake por login real
- carregar lojas acessiveis do usuario
- definir loja ativa a partir da API

Status de integracao no Nuxt:

- `auth` ja usa `GET /v1/me/context`
- a loja ativa do runtime local ja respeita o contexto autenticado
- a fonte de verdade da operacao ja passou para a API; o runtime do frontend ficou como camada de compatibilidade e estado efemero de UI

## Fase 3

### Consultores e configuracoes base

Status: concluido para CRUD de consultores + leitura/gravação setorial de settings + integracao real no Nuxt

Entregas:

- modulo `consultants`
- modulo `settings`
- persistir:
  - consultores
  - templates de operacao
  - modal config
  - motivos
  - origens
  - perdas
  - profissoes
  - catalogo de produtos
- endpoints para o Nuxt carregar e editar esses dados
- integracao do Nuxt para hidratar a loja ativa a partir de:
  - `GET /v1/consultants`
  - `GET /v1/settings`
  - `POST /v1/consultants`
  - `PATCH /v1/consultants/{id}`
  - `POST /v1/consultants/{id}/archive`
  - `PUT /v1/settings`
  - `PATCH /v1/settings/operation`
  - `PATCH /v1/settings/modal`
  - `PUT /v1/settings/options/{group}`
  - `PUT /v1/settings/products`

Objetivo no Nuxt:

- tirar catalogos e configuracoes do runtime local
- manter a UI atual, trocando apenas a fonte de dados
- manter `auth` como ponto de bootstrap do contexto remoto por loja

Observacao:

- o `GET /v1/settings` continua entregando um bundle completo para bootstrap de leitura
- as escritas agora acontecem por secao, evitando trafegar e regravar o pacote inteiro a cada mudanca pequena

## Fase 4

### Operacao HTTP

Status: concluido para snapshot, fila, atendimento, pausa, encerramento, historico e sessoes

Entregas:

- modulo `operations`
- snapshot operacional por loja
- comandos HTTP:
  - entrar na fila
  - pausar
  - retomar
  - iniciar atendimento
  - encerrar atendimento
- persistir corretamente:
  - fila
  - atendimento ativo
  - pausas
  - historico
  - sessoes de status

Contrato importante:

Seguir o contrato descrito em `web/app/pages/operacao/operations.md`, especialmente:

- `productsSeen[]`
- `productsClosed[]`
- `visitReasons[]`
- `visitReasonDetails`
- `customerSources[]`
- `customerSourceDetails`
- `lossReasons[]`
- `lossReasonDetails`

Objetivo no Nuxt:

- usar HTTP como fonte de verdade para snapshot + comandos
- manter o runtime do frontend apenas como camada de compatibilidade de tela e estado efemero do modal

Status de integracao no Nuxt:

- `runtime-remote.ts` hidrata `consultants`, `settings` e `operations` da loja ativa
- `operations.ts` envia comandos HTTP reais para o backend
- o historico operacional e reidratado do Postgres
- o modal de encerramento ainda abre com draft local de compatibilidade

## Fase 5

### Realtime com websocket

Status: concluido para eventos operacionais por loja; backlog aberto para broker externo e replay

Entregas:

- canal realtime autenticado por usuario
- escopo por tenant/store
- assinatura por loja
- evento leve `operation.updated` para invalidacao da loja ativa
- bootstrap e fallback mantendo `GET /v1/operations/snapshot` como fonte autoritativa
- revalidacao do snapshot operacional no Nuxt ao receber evento

Backlog desta fase:

- eventos granulares por tipo de acao
- `settings.updated`
- reconciliacao por `version` ou `sequence`
- broker externo para multiplas replicas

Objetivo no Nuxt:

- refletir mudancas em tempo real entre dispositivos sem refresh
- manter HTTP como fallback de snapshot e re-sync

## Fase 6

### Reports, analytics e leitura operacional server-side

Status: em andamento com modulo `reports` em producao para `/relatorios` e modulo `analytics` ativo para `/ranking`, `/dados` e `/inteligencia`

Entregas:

- modulo `reports`
- modulo `analytics`
- mover agregacoes criticas do frontend para backend
- fechar leitura correta de produtos mais fechados usando `productsClosed[]`
- expandir a tela/lista administrativa de ultimos atendimentos com filtros por:
  - loja
  - consultor
  - desfecho
  - periodo
- expor uma visao administrativa para auditar rapidamente:
  - qual loja atendeu
  - qual consultor atendeu
  - quando atendeu
  - o que foi preenchido no fechamento

Objetivo no Nuxt:

- reduzir regra pesada local
- manter telas, trocando calculo local por dados vindos da API
- dar visibilidade operacional e gerencial sem depender de leitura bruta no frontend

Status de integracao no Nuxt:

- `/relatorios` ja consome `GET /v1/reports/overview`, `GET /v1/reports/results` e `GET /v1/reports/recent-services`
- `/ranking` ja consome `GET /v1/analytics/ranking`
- `/dados` ja consome `GET /v1/analytics/data`
- `/inteligencia` ja consome `GET /v1/analytics/intelligence`
- a regra pesada de agregacao saiu do browser dessas telas e foi centralizada no backend

## Fase 7

### Multiloja, usuarios e acessos

Status: em andamento com CRUD real de lojas, gestao de usuarios e onboarding por convite ja funcionando

Progresso recente:

- `users` ja expõe listar, criar, editar, inativar e gerar convite por usuario
- o onboarding inicial ja funciona com:
  - usuario criado sem senha
  - tabela `user_invitations`
  - link `web/auth/convite/:token`
  - aceite do convite definindo a primeira senha e abrindo sessao real
- backlog restante desta fase:
  - entrega do convite por email real
  - onboarding do cliente
  - regras finas de remocao
  - fechar o modelo de identidade operacional por loja

Entregas:

- completar modulo `multistore`
- fechar administracao de lojas com:
  - criar
  - editar
  - arquivar/desativar
  - remover quando permitido pela regra de negocio
- criar modulo `users`
- criar fluxo administrativo para:
  - convidar/criar usuario
  - editar perfil e escopo
  - ativar/desativar acesso
  - vincular usuario a tenant e lojas
  - controlar papel (`consultant`, `store_terminal`, `manager`, `marketing`, `owner`)
- consolidar o modelo operacional de acesso:
  - todo consultor ja nasce como conta real do sistema vinculada ao roster
  - `consultant`, `manager` e `store_terminal` ficam em uma unica loja
  - `store_terminal` opera fila, pausa, inicio e encerramento apenas na propria unidade
  - `owner` e `platform_admin` podem alternar lojas e auditar outras unidades
- entregar a visao integrada de operacao cross-store para quem tiver escopo multi-loja
- preparar onboarding do cliente dono do tenant para testes reais com acesso parcial

Estado atual:

- `stores` ja expõe CRUD basico real com metas e template padrao por loja
- o frontend `multiloja` ja cria, edita e arquiva lojas via API
- `reports` ja expõe `multistore-overview` para o comparativo gerencial server-side
- `users` ja expõe listar, criar, editar e inativar usuario
- o catalogo de roles ja inclui `store_terminal` para acesso fixo da unidade
- `consultants` agora ja cria o roster com conta `consultant` vinculada 1:1 e senha inicial padrao
- contas com senha temporaria agora usam `users.must_change_password`
- `/perfil` ja limpa a exigencia de troca de senha no primeiro acesso
- `users` ja expõe reset dedicado de senha temporaria
- o frontend `multiloja` ja possui card de `usuarios e acessos`
- ainda falta amadurecer:
  - convite por email
  - onboarding do cliente
  - regras finas de remocao
  - leituras gerenciais adicionais de multiloja 100% server-side

Objetivo no Nuxt:

- parar de depender de usuarios demo para validacao
- permitir rollout controlado por loja e por perfil
- fechar a parte administrativa necessaria para operacao real

### Atualizacao de estado da Fase 7

- o ciclo administrativo de lojas agora cobre:
  - `GET /v1/stores?includeInactive=true`
  - arquivar
  - restaurar
  - remover loja com bloqueio por dependencias operacionais e de acesso
- o frontend `multiloja` ja cria, edita, arquiva, restaura e tenta remover lojas com feedback claro para o admin
- o frontend agora tambem separa:
  - `multiloja` para administracao de lojas e comparativo
  - `usuarios` para administracao de acessos
  - `perfil` para autoatendimento do usuario autenticado
- `users` agora cobre dois fluxos de criacao:
  - convite/onboarding inicial
  - senha inicial definida pelo admin
- o backend agora tambem diferencia:
  - conta individual do consultor
  - conta fixa `store_terminal` da loja
- o cadastro de consultor ja provisiona:
  - email padrao por loja
  - senha inicial padrao
  - inativacao automatica da conta ao arquivar o consultor
- o realtime por tenant ja sincroniza header, multiloja e usuarios entre instancias abertas
- `/operacao` agora tambem oferece:
  - modo `Loja ativa`
  - modo `Todas as lojas` para `owner` e `platform_admin`
  - filtro interno por loja dentro da propria tela
  - cards com identificacao visual da loja em fila, atendimento, pausa/tarefa e disponibilidade
- o backend `operations` agora tambem cobre `POST /v1/operations/assign-task` para tirar alguem da fila por tarefa ou reuniao sem mascarar isso como pausa comum
- a fase continua aberta apenas por itens complementares:
  - seguranca futura por loja/dispositivo para `store_terminal`
  - entrega real de convite por email
  - onboarding do cliente
  - auditoria administrativa adicional

## Fase 8

### Campanhas e multiloja gerencial server-side

Status: depois de reports e administracao de acessos

Entregas:

- modulo `campaigns`
- modulo `multistore`
- mover calculos criticos de campanhas e comparativos de lojas para backend

Objetivo no Nuxt:

- manter telas, trocando calculo local por dados vindos da API
- consolidar leitura gerencial cross-store no servidor

## Fase 9

### Hardening para piloto

Entregas:

- PostgreSQL com migrations
- logs estruturados
- auditoria de comandos
- testes de concorrencia
- smoke realtime
- teste controlado para pelo menos 30 conexoes simultaneas

## Fase transversal

### Plugabilidade do core

Status: iniciado em `operations`

Objetivo:

- deixar claro o que o painel Omni precisa do sistema host para ser embutido em outro produto
- reduzir acoplamento do core com modulos de plataforma como `auth` e `stores`

Passos:

1. `operations` usando contexto proprio e adapters finos
2. `reports` no mesmo padrao
3. `analytics` no mesmo padrao
4. documentacao unica de contratos em `CORE_MODULES_PORTABILITY.md`

## Ordem pratica recomendada para agora

1. modulo `analytics`
2. amadurecer `multiloja`
3. amadurecer `users` e gestao de acessos
4. campanhas server-side
5. eventos de `settings/campaigns` quando esses dominios entrarem em realtime

## O que ainda nao faz sentido fazer

- websocket antes de existir contexto autenticado e loja ativa
- relatorios server-side antes da operacao estar persistindo no backend
- produtos reais antes de existir modulo de settings/catalogo e contrato de store
