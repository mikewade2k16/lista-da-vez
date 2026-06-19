# Plano — Usuários e Acessos (operacao/usuarios + manage/users)

Branch: `refactor/multitenant-complete`. Doc canônico desta leva. Espelhado no
roadmap (`web/app/components/roadmap/roadmap-data.ts`, fase `usuarios-acessos-fix`).

## Contexto / diagnóstico

Duas telas mexem com usuário, em sistemas separados:

| Tela | Componente | Sistema | Governa |
|---|---|---|---|
| `/operacao/usuarios` (alias `/usuarios`) | `UsersWorkspace` mode=`queue` → `UsersAccessManager` | módulo `access` (`workspace.*` + overrides por usuário) | o que o **usuário** vê no menu |
| `/manage/users` | `AdminUsersWorkspace` | `core.admin_users` (cross-account) | CRUD global de usuários em `core.users` |
| `/manage/clientes-web` | `ClientsAdminWorkspace` | `core.account_modules` (REAL, DB) | o que o **cliente** contratou (`queue`/`tasks`/`crm`…) |

Achados:

1. **operacao/usuarios não salva módulos.** `saveDetails` só grava overrides se
   `detailAccessReady` for true (exige `roleMatrix` carregada + `userAccess`
   carregado). Se `GET /v1/access/roles` ou `GET /v1/access/users/{id}` falha, o
   `ensureRoleMatrix` engole o erro e o botão grava só os dados básicos mostrando
   "permissões indisponíveis". Causa provável: `GET/PUT /v1/access/users/{id}`
   retorna 404 por escopo de tenant no `FindAccessibleByID`
   (`back/internal/modules/users/store_postgres.go:50`) quando o admin da operação
   tem tenant divergente do usuário-alvo.

2. **Revogação não é imediata.** O backend re-resolve permissões do banco a cada
   request (PrincipalCache NÃO está ligado — `auth/AGENT.md:100`), então o back já
   revoga na hora. O problema é o **front**: a sessão logada monta o menu a partir
   de `auth.permissionKeys` (login) e nunca re-busca. O `access` já publica WS
   `user-overrides-updated` (`access/service.go:143`); falta o front consumir.

3. **manage/users não define senha.** Criação já aceita `temporaryPassword`
   (`admin_users_repository.go:167`). Mas `AdminUpdateUserInput` NÃO tem campo de
   senha (`admin_users_model.go:81`) e a grid não tem ação de senha → impossível
   definir/resetar senha após criar.

4. **"Perfis" sumiu da operação.** `UsersRoleMatrixManager` existe mas só aparece
   quando o modo NÃO é `queue` (`UsersWorkspace.vue:53`). operacao/usuarios usa
   `queue`, então a aba fica escondida.

## Escopo de HOJE (sobe)

### Trilha A — operacao/usuarios (módulo Fila / `access`)
- A1. Corrigir o salvar dos módulos: reproduzir, identificar a chamada `/v1/access/*`
  que falha e corrigir (provável escopo de tenant em `FindAccessibleByID`).
- A2. Tirar o erro silencioso: se o access falhar, mostrar motivo real inline; o
  botão não pode fingir que salvou.
- A3. Revogar ao vivo no front: consumir o WS `user-overrides-updated` → re-buscar
  `/v1/auth/me` → remontar menu/gating, sem deslogar.
- A4. Reexpor "Perfis" (`UsersRoleMatrixManager`) onde combinado.
- A5. AGENT.md de `back/internal/modules/access/` e `web/app/components/users/`.

### Trilha B — manage/users (admin global / `core.admin_users`)
- B1. Senha clara/obrigatória na criação (back já grava).
- B2. Senha pós-criação: `password` em `AdminUpdateUserInput` + hash no service +
  update no repo + ação "Definir/Resetar senha" na grid (gated por platform_admin).
- B3. AGENT.md de `back/internal/modules/core/`.

As trilhas não compartilham arquivos → seguras em paralelo. Sessão multi-agente:
ninguém roda git, só o usuário.

## PENDENTE (complexo — NÃO é hoje)
- Mapa `module_id → workspaces` (ponte entre `core.account_modules` e os
  `workspace.*` do `access`).
- Enforcement: acesso do usuário ⊆ módulos contratados pelo cliente; override só
  pra `platform_admin`. Defesa no service do `access` + UI que esconde/desabilita
  o que o cliente não tem.
- manage/users como lugar canônico de criar usuário + editar módulos + perfis;
  operacao/usuarios fica como visão-fila (submenu da Fila).

## Resultado (implementado nesta sessao)

Diagnostico confirmado AO VIVO (login platform_admin, API :9091):

- **Salvar modulos no backend FUNCIONA.** Round-trip GET/PUT `/v1/access/users/{id}/overrides`
  retorna 200 e persiste (testado: deny `workspace.operacao.view` gravou e reverteu).
  Logo NAO ha bug de escopo no `access`/`FindAccessibleByID` para platform_admin —
  nao mexemos no backend do access (evitar regressao).
- **Revogacao ao vivo JA estava cabeada.** O PUT publica evento de contexto `access`;
  `useContextRealtime` (montado em `layouts/dashboard.vue`) trata `context.updated`
  resource `access` -> `auth.fetchContext()` (re-busca `/v1/me/context`, atualiza
  `permissionKeys` -> menu reativo) + `accessControl.refreshRealtimeState()`. Backend
  re-resolve permissoes do banco a cada request (sem cache de principal). Decisao do
  usuario (re-buscar ao vivo) ja e o comportamento existente — verificado.

O que de fato foi corrigido:

- **Trilha A** (front, `web/`, sem rebuild de back):
  - `useUsersAccessManager.js` `saveDetails`: a validacao de nome/loja so roda quando
    os dados basicos mudaram (`basicChanged`). Mexer SO nos modulos vai direto ao PUT
    de overrides — antes um store_terminal sem loja vinculada travava o save inteiro.
  - Erro honesto: quando os modulos nao podem ser salvos, mostra o motivo REAL
    (`detailAccessError`) em vez de "indisponivel".
  - Aba "Perfis e padroes" reexposta em `/operacao/usuarios` gated por
    `canManageRoleDefaults` (platform_admin).
- **Trilha B** (back core.admin_users + front admin):
  - PATCH `/v1/admin/users/{id}` aceita `password` (ausente/vazio = nao toca no hash).
  - Front: acao "Definir/Resetar senha" por linha + modal; senha validada na criacao.
  - **B1 — descoberta importante:** criar usuario SEM cliente/agencia/papel gerava um
    usuario que **nao logava** (login 500 — "nao serve de nada"). Confirmado ao vivo.
    Fix: o modal exige cliente (com papel), agencia (com cargo) OU flag platform admin.

- **Agencia (decisao do usuario "acesso de agencia nos clientes"):** usuario de agencia
  agora **loga**. Ao criar com `organizationId`, o repo tambem matricula o user na
  **conta-agencia** (`core.accounts` is_agency=true da org) com papel conforme o cargo:
  `agency_owner -> owner` (acesso total), `agency_member -> director` (limitado). O
  switcher org-aware (AGENCY_TENANT_ARCHITECTURE) abre os clientes da agencia. O modal
  ganhou o select "Cargo na agencia". Testado ao vivo: agency_member -> login 200 role
  director; agency_owner -> login 200 role owner.
- **Login 500 -> 4xx limpo:** `ErrInvalidRoleScope` no login agora retorna
  403 `user_no_role` (em vez de 500). Testado: usuario sem vinculo -> 403 user_no_role.

## Pendencias / gaps conhecidos (nao resolvidos hoje)
- Cargos de agencia sao 2 (owner/member). Granularidade maior (cargos custom, acesso
  por cliente/modulo dentro da agencia) fica para depois.
- `web/app/components/admin/AdminUsersWorkspace.vue` passou de ~500 linhas (673) —
  candidato a extrair os modais (criar/senha) em subcomponentes.
- A regra de coerencia conta->usuario (mapa `module_id->workspaces` + enforcement)
  segue como complexo (nao hoje).

## Notas de Deploy
- Trilha B muda `back/` (model/service/repo de core.admin_users) → exige
  `docker compose up -d --build api`. Sem migration nova nesta leva.
- Trilha A é só `web/` (sem mudanca em `back/`). O front de dev recarrega sozinho;
  nao precisa rebuild para a Trilha A.
- O reset de senha (PATCH com `password`) so funciona DEPOIS do rebuild da api
  (o binario em execucao ainda e o antigo, sem o campo).
