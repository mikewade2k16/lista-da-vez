# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/consultants`.

## Responsabilidade do modulo

O modulo `consultants` cuida do roster administrativo de consultores por loja.

Hoje ele deve responder por:

- listar consultores ativos de uma loja
- criar consultor ja com acesso autenticado vinculado
- atualizar consultor
- arquivar consultor

Ele nao deve cuidar de:

- status operacional em tempo real
- fila
- pausas e atendimentos
- relatorios agregados

## Contrato atual

- `GET /v1/consultants?storeId=...`
- `GET /v1/consultants/orphans?tenantId=...`
- `GET /v1/store-staff?storeId=...`
- `POST /v1/consultants`
- `PATCH /v1/consultants/{id}`
- `POST /v1/consultants/{id}/archive`

`GET /v1/consultants` expoe `nick` a partir de `core.users.nick`, com fallback
para `queue.consultants.name` quando o usuario vinculado ainda nao possui apelido.
Telas densas de escala usam `nick`; o nome completo permanece no contrato para
identificacao e pesquisa.

## `GET /v1/store-staff` (membros de loja que NAO atendem na fila)

Projecao enxuta dos membros das lojas acessiveis que NAO operam a fila
(gerente, terminal/caixa e qualquer papel store-scoped que nao seja consultor).
O front usa esses cards ao lado dos consultores so para exibir quanto recebem
pela meta da loja. Os consultores de fila NAO entram aqui — esses vem de
`/v1/consultants`.

- `storeId` e opcional. Sem `storeId`, lista todas as lojas acessiveis ao
  principal (escopo resolvido a partir do token). Com `storeId`, e filtro
  DENTRO do permitido: membership validada contra o Principal; loja fora do
  escopo retorna `404 store_not_found`.
- Shape de resposta:

  ```json
  {
    "items": [
      {
        "id": "<userId>",
        "name": "<nome>",
        "role": "manager|cashier|...",
        "roleLabel": "Gerente|Caixa|...",
        "storeId": "<id da loja>",
        "storeName": "<nome da loja>"
      }
    ]
  }
  ```

### Fonte de dados (modelo core RBAC)

- membership ativa em `core.account_users`
- papel atribuido em `core.user_role_assignments -> core.roles`
- escopo de loja por usuario em `core.user_module_settings(module_id='queue')`,
  no JSONB `config -> storeIdsByAccount -> <accountId>` (mesmo padrao de
  `stores/scope_queries.go`)
- nome do usuario em `core.users`; nome/escopo da loja em `queue.stores`
- 1 query agregada (`s.id = any($1)`), sem N+1; defesa em profundidade: a query
  so considera lojas ja validadas contra o Principal

### Taxonomia de papeis e normalizacao

Papeis vivem em `core.roles.code` (clonados de `core.role_templates`). Codes do
seed: `queue.owner`, `queue.director`, `queue.marketing`, `queue.manager`,
`queue.consultant`, `queue.store_terminal` (template base `queue.supervisor` ou
`queue.consultant`).

- Consultor de fila (EXCLUIDO da resposta): codes
  `queue.consultant`/`consultant`/`core.member`/`queue.marketing`/`marketing`
  e qualquer role com `cloned_from_template_id = 'queue.consultant'`.
- Normalizacao de `role` (categoria estavel para o front agrupar recebimento):
  `queue.manager`/`manager` -> `manager`; `queue.store_terminal`/`store_terminal`
  -> `cashier`; owner/director/supervisor -> `manager`; roles customizados usam
  o sufixo do code (`queue.caixa` -> `caixa`).
- `roleLabel`: prefere `core.roles.label` (pt-BR no banco); fallback no mapa
  estavel (`manager`->Gerente, `cashier`->Caixa, `assistant/auxiliar`->Auxiliar).

> Limitacao conhecida: o seed atual so define `queue.manager` como papel de loja
> distinto do consultor; nao existe `caixa` nem `auxiliar` pre-semeados. Como o
> RBAC e dinamico, accounts podem criar roles customizados e o endpoint ja os
> normaliza pelo sufixo do code + label do banco.

## Regra nova de identidade

- cada consultor deve ser tambem um usuario real do sistema
- o vinculo e 1:1 por `consultants.user_id`
- o consultor nasce com:
  - email padrao gerado automaticamente por nome + loja
  - senha inicial padrao da politica de rollout
  - `must_change_password = true` para forcar senha pessoal no primeiro acesso
- a conta vinculada usa papel `consultant` no escopo da loja
- ao arquivar o consultor, a conta vinculada tambem deve ser inativada
- a criacao administrativa de consultor por papeis nao-`platform_admin` deve acontecer por este modulo, nao por `users`, para nao nascer conta `consultant` sem roster
- depois de vinculada, a conta de consultor deve ser considerada propriedade deste modulo
- `users` pode listar e resetar senha do consultor, mas nao deve editar escopo, convite, papel nem ciclo de vida dessa conta (excecao: override de `platform_admin`, abaixo)
- ao editar o proprio perfil em `auth`, o nome do consultor deve sincronizar de volta no roster

### `SyncLinkedAccess` (ponte `users` -> roster, o "atalho unificado")

`ProfileSync.SyncLinkedAccess` / `Repository.SyncLinkedAccess(LinkedAccessSyncInput) ([]string, error)` e chamada pelo `users` (Create/Update/Archive) quando `platform_admin` mexe num consultor pela grade de Usuarios. Mantem `queue.consultants` chaveada por `user_id`:

- papel `consultant` + tenant/loja validos e **sem** linha -> **INSERT** de nova linha ativa (`role_label='Atendimento'`, metas 0, cor default); e o unico caminho, alem deste modulo, que cria roster
- papel `consultant` com linha existente -> UPDATE de tenant/loja/nome/initials/ativo (cobre troca de loja)
- papel != `consultant` (ou inativo) -> desativa a linha (`is_active=false`); sem linha, no-op
- consultor inativo sem linha nao gera INSERT (nao ha o que mostrar na fila)

Retorna as **lojas afetadas** (origem + destino numa troca) para o `users` publicar `operation.updated` por loja (Lista da vez ao vivo por WebSocket). Conflito de nome na mesma loja -> `ErrConsultantConflict`.

## Regras de escopo

- leitura: usuarios com `workspace.consultor.view` e tambem operadores administrativos que chegam pelo fluxo de configuracoes
- escrita: `owner` e `platform_admin`

## Observacoes de integracao

- `GET /v1/consultants?storeId=...` alimenta tanto a workspace `consultor` quanto a aba de consultores em `configuracoes`
- a workspace `consultor` nao deve depender de permissao de `ranking` nem de `configuracoes` para montar seu comparativo multi-loja

## Backlog desta frente

- amadurecer reset operacional com auditoria mais detalhada
- avaliar se vale expor no roster algum indicativo visual de conta sem primeiro login concluido

## Seguranca (2026-06-30)

- `Repository.CanAccessTenant(principal, tenantID)` recheca a membership real no banco (`core.account_users` OU `core.organization_users`), org-aware (platform_admin acessa qualquer account ativa). `ListOrphans` e `Update` (consultor orfao) usam isso em vez de confiar no `principal.TenantID` — fecha vazamento cross-tenant e a janela pos-revogacao (token ainda valido apos o usuario sair da account nao pode ler/escrever). Espelha `queue/settings` e `crm/erp`.
