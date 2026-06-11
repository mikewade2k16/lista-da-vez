# Arquitetura Agência → Clientes (tenants) — mudança a fazer

> Status: **PLANEJADO** (não implementado). Documento canônico desta mudança.
> Origem: 2026-06-10, ao corrigir o "dono" do board de Tasks. Descobriu-se que o modelo
> agência/cliente ainda não está montado no sistema. Ver também [LEGADO.md](LEGADO.md) §4 e a
> memória `project_tasks_client_source`.

---

## 1. Modelo-alvo (definido pelo dono do produto)

- **Crow = a agência** (o time dev/dono do projeto). É o nível mais alto.
- **Clientes = tenants** (`core.accounts`): Pérola, AM Malls, UNO, Dr Antonio Tavares, Duby,
  Juliana Oliveira, Cléo Moraes, Mostarda… Todos os clientes **pertencem à agência Crow**.
- **Módulos são da agência**; cada cliente usa os que contratou. Ex.: o módulo **Fila** hoje só a
  **Pérola** usa (operação real: 46 usuários, terminais, consultoras). A agência enxerga o módulo
  de um cliente como enxergaria o de outro.
- **Login de agência** acessa (ou não) os módulos de cada cliente **conforme o nível** do usuário
  dentro da agência. Um usuário de cliente só vê o próprio tenant.
- O board **Tasks** é da **agência** (board geral, com clientes como *atributo* da task). Nenhum
  cliente é "dono" das tasks — o dono é a agência.

```
Organização "Crow Visuals" (agência)
├── conta-agência "Crow"        → dona do board geral Tasks (clientes = client_account_id)
├── conta-cliente "Pérola"      → módulo Fila (operação real)
├── conta-cliente "AM Malls"
├── conta-cliente "UNO"
└── … demais clientes
```

## 2. Estado atual (o que está errado)

- `core.organizations` e `core.organization_users` **existem mas estão VAZIAS** (0 orgs). Não há
  agência "Crow" no banco.
- As 11 contas estão **soltas** (`organization_id = NULL`). Não há hierarquia agência→cliente.
- O board geral **Tasks** (`9d40be47-…`, 247 tasks) está **dentro da conta-cliente Pérola**
  (`aaaaaaaa-…`), que é a conta de trabalho ativa do dev. Por isso:
  - 111 tasks têm `client_account_id = aaaa` = a própria conta dona → **"dono == cliente"**.
  - A conta `aaaa` está **sobrecarregada**: é a operação Pérola (Fila) **E** o workspace de tasks
    da agência ao mesmo tempo.
- O **switcher de conta** (`web/layers/core/components/CoreAccountSwitcher.vue`, account store v2)
  **NÃO está ligado** ao contexto que o módulo Tasks lê (`auth.activeTenantId`, legado). Então hoje
  não dá para a agência alternar a conta ativa do Tasks entre Crow e clientes.

### Incidente (aprendizado — 2026-06-10)
Tentou-se mover o board para uma conta Crow via SQL **antes** de ligar a troca de conta. Resultado:
o board sumiu da visão do dev (conta ativa continuou `aaaa`) e o front auto-criou um board vazio.
**Revertido do backup** (`/tmp/tasks_mig_backup_20260610.csv`). **Regra: não mover dado de tenant
antes de o switcher estar ligado ao contexto do módulo.**

## 3. Plano em estágios (a ordem é de segurança)

> Cada estágio é verificável e não quebra acesso. Dado de tenant só se move no Estágio 4.

1. **Ligar a troca de conta ao contexto do Tasks.** Unificar/ponte entre o account store v2
   (`CoreAccountSwitcher` / `core/stores/account.ts`) e o `auth.activeTenantId` (legado) que
   `tasks/stores/tasks.ts` e `useTimeTracking.ts` usam no header `X-Account-Id`. Verificável: o dev
   troca a conta ativa e o board do Tasks recarrega na conta escolhida.
2. **Criar a organização "Crow Visuals"** em `core.organizations` e **vincular** todas as contas-
   cliente (`organization_id = Crow`). A conta-agência "Crow" (`80caf5d5`) também entra na org.
3. **Acesso da agência por nível.** Definir como um login de agência acessa as contas-cliente
   (membership cross-account em `core.account_users` / `core.organization_users` + permissões por
   nível). platform_admin acessa todas; níveis menores, um subconjunto.
4. **Migrar o board Tasks** da conta `aaaa` (Pérola) para a **conta-agência Crow** (`80caf5d5`),
   incluindo `tasks.boards/tasks/task_time_entries/audit_log` (as 4 tabelas tasks.* com
   `account_id`). **Só depois do Estágio 1**, com o switcher funcionando. Atenção à FK composta
   `tasks_tasks_board_account_fk (board_id, account_id)` → deferir na transação e restaurar.
5. **Limpar o `client_account_id`.** Pérola passa a ser cliente de verdade (conta distinta do dono
   do board). Tasks que apontavam para a própria conta-agência viram `client_account_id = null`
   (internas) ou para o cliente correto.

## 4. Notas de Deploy / dados

- Estágios 2 e 3 envolvem **dados/seed em `core.organizations` + `organization_users` + vínculo de
  `organization_id`** nas contas — portar para **migration idempotente** antes da VPS (não rodar só
  manual local). Ver [feedback] script manual deve virar migration.
- Estágio 4 é **movimentação de dados** (account_id de board/tasks/entries/audit) — exige backup +
  checagem + a FK composta deferível. Reversível via backup.
- Estágio 1 pode exigir mudança no **auth** (como `activeTenantId` é resolvido/trocado) → rebuild da
  api se mexer no Go (`docker compose up -d --build api`).

## 5. Fora de escopo (intencional)
- **Performance do board** (virtualização/render dos 247 cards) é **independente** desta arquitetura
  e está sendo feita à parte. Mover de conta não muda o custo de render.
