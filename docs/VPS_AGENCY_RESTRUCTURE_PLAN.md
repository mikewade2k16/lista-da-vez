# Plano — Reestruturação da agência na VPS (preservando a Pérola)

> Status: **PLANO. NADA EXECUTADO.** Aguarda aprovação + respostas das "Decisões em aberto".
> Criado em 2026-06-16. Fonte do diagnóstico: auditoria read-only da VPS (ver [[project_vps_real_topology]] / [[project_vps_platform_admin_gap]] na memória).

## 1. Por que este plano existe

O painel na VPS (`omni.crowvisuals.com.br`) mostra **uma só conta** no switcher, enquanto o
local mostra a árvore **agência → clientes**. A causa NÃO é código nem deploy — é a forma
como os dados estão na produção.

### Estado real auditado

| | LOCAL (dev sandbox) | VPS (produção) |
|---|---|---|
| Contas | 11 (`crow` agência + Pérola + AM Malls, Cléo, Dr Antonio, Duby, Juliana, Mostarda + 3 teste) | **1** (`tenant-demo`) |
| Usuários | 67 | **35 reais** |
| Lojas | 6 | **8 reais** |
| ERP (`erp_item_raw`) | 1.551.218 | **1.559.991** |

**A conta `tenant-demo` da VPS É a produção real da Pérola** (nome errado):
- Lojas: `Perola Garcia`, `Perola Jardins`, `Perola Riomar`, `Perola Treze`, terminais
  (garcia/jardins/riomar/treze), `Loja 184`, `Loja Teste`.
- 35 usuários reais (funcionárias com emails gmail/hotmail) + 3 platform_admins
  (`mikewade2k16@gmail.com`, `maykell072009@gmail.com`, `tonyw.wright@outlook.com`).
- 1,56 M linhas de ERP.

### Regra que este plano respeita

- ❌ **NUNCA** copiar o banco local por cima da VPS — apagaria a produção da Pérola e
  sobrescreveria senhas de quem está trabalhando agora.
- ✅ Toda escrita é **aditiva ou rename** (sem DELETE de dados da Pérola), **idempotente**,
  em **transação**, com **backup antes** e **sem tocar em `password_hash`**.

## 2. Estado-alvo (igual ao local, mas preservando a Pérola)

Organização `crow-visuals` (já existe na VPS) como topo. Abaixo dela:

- Conta-**agência** `crow` / "Crow Visuals" (`is_agency=true`) — casa dos platform_admins.
- Conta-**cliente** `perola` / "Pérola" (`is_agency=false`) ← **é a atual `tenant-demo`,
  só renomeada; mantém TODAS as 8 lojas, 35 usuários e o ERP.**
- Contas-cliente novas e **vazias** (a popular conforme cada cliente entra): AM Malls,
  Cléo Moraes, Dr Antonio Tavares, Duby, Juliana Oliveira, Mostarda. *(ver Decisão #1)*

## 3. Decisões (RESPONDIDAS — 2026-06-16, dono do produto)

1. **Contas-cliente:** criar **TODA a lista do local**, **vazias** — AM Malls, Cléo Moraes,
   Dr Antonio Tavares, Duby, Juliana Oliveira, Mostarda. (sem migrar dados; populam pelo painel)
2. **Rename `tenant-demo`→`perola`:** **OK.** Grep confirmou que o slug só aparece em docs, no
   seed `0002_seed_demo_auth.sql` (data-seed, **pulado em prod** por `SkipDataSeeds`) e em testes
   — **nenhum código runtime depende dele.** `.env` tem `ERP_BOOTSTRAP_TENANT_SLUG` vazio, então
   o bootstrap do ERP não quebra (dá skip com múltiplos tenants; a loja 184 já existe). Seguro.
3. **Módulos da Pérola:** só **`queue` (Fila)** por enquanto (+ `core`, obrigatório). O resto o
   dono habilita pelo painel. ⚠️ Bloco separado e opcional (esconde ERP/CRM/Tasks/etc das telas
   da Pérola até reativar; **os dados continuam intactos**).
4. **Lojas `Loja Teste` / `Loja 184`:** **manter** — o dono ajusta pelo painel.

Identificadores resolvidos: `core.modules.id` é o código do módulo. Fila = `queue`. Módulo
obrigatório = `core` (`is_core=true`). Org topo já existe: `core.organizations` slug `crow-visuals`.

## 4. Passos da migração (ordenados — NÃO rodar até aprovar)

> Cada passo é um bloco SQL idempotente, em transação, precedido por backup. Os UUIDs são
> resolvidos por slug (data-driven), nunca hardcoded.

**Passo 0 — Backup completo da VPS** (não só as tabelas core; é mudança estrutural):
```bash
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  sh -lc 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' \
  > backups/pre_restructure_$(date +%Y%m%d_%H%M%S).dump
```

**Passo 1 — Renomear a conta da Pérola** (mantém `is_agency` por ora p/ não trancar o Mike):
```sql
update core.accounts
   set name = 'Pérola', slug = 'perola', updated_at = now()
 where lower(slug) = 'tenant-demo';
```

**Passo 2 — Criar a conta-agência `crow`** (vinculada à org crow-visuals):
```sql
insert into core.accounts (slug, name, is_agency, organization_id)
select 'crow', 'Crow Visuals', true,
       (select id from core.organizations where lower(slug)='crow-visuals')
where not exists (select 1 from core.accounts where lower(slug)='crow');
```

**Passo 3 — Platform_admins viram membros da `crow` + todos os módulos na `crow`:**
```sql
insert into core.account_users (account_id, user_id, is_active)
 select a.id, u.id, true
   from core.accounts a cross join core.users u
  where lower(a.slug)='crow' and u.is_platform_admin=true and u.is_active=true
 on conflict (account_id, user_id) do update set is_active=true;

insert into core.account_modules (account_id, module_id, enabled)
 select a.id, m.id, true from core.accounts a cross join core.modules m
  where lower(a.slug)='crow'
 on conflict (account_id, module_id) do update set enabled=true;
```

**Passo 4 — Acertar a identidade da Pérola** (só DEPOIS que o Mike já é membro da `crow`,
senão a conta ativa dele cairia numa conta sem agência):
```sql
-- Pérola passa a ser CLIENTE (não agência)
update core.accounts set is_agency=false, updated_at=now() where lower(slug)='perola';
-- remover as memberships de platform_admin que o quick-win pôs na Pérola
delete from core.account_users au
 using core.accounts a, core.users u
 where au.account_id=a.id and au.user_id=u.id
   and lower(a.slug)='perola' and u.is_platform_admin=true;
-- (Decisão #3) reverter módulos da Pérola pro conjunto contratado — definir lista
```
> ⚠️ Verificar a ordem de resolução de conta ativa (`ListAccountsForUser` membership-first):
> garantir que, com o Mike membro de `crow` (agência) e não mais da Pérola, a conta ativa
> default seja a `crow`. Validar no `/v1/me/context` antes de fechar.

**Passo 5 — Criar as contas-cliente vazias** (conforme Decisão #1), ex.:
```sql
insert into core.accounts (slug, name, is_agency, organization_id)
select v.slug, v.name, false, (select id from core.organizations where lower(slug)='crow-visuals')
from (values ('am-malls','AM Malls'), ('cleo-moraes','Cléo Moraes'),
             ('dr-antonio-tavares','Dr Antonio Tavares'), ('duby','Duby'),
             ('juliana-oliveira','Juliana Oliveira'), ('mostarda','Mostarda')) as v(slug,name)
where not exists (select 1 from core.accounts a where lower(a.slug)=v.slug);
-- habilitar módulos contratados de cada cliente (definir) via core.account_modules
```

**Passo 6 — Validação:**
- `select slug, name, is_agency from core.accounts order by is_agency desc, name;`
- Conferir Pérola intacta: contagem de `queue.stores`, `core.users` membros, `erp_item_raw`
  iguais ao pré-migração.
- Login do Mike → switcher mostra `Plataforma (dev)` + org `Crow Visuals` + lista de clientes.
- Login de uma funcionária da Pérola → tudo normal (nada mudou pra ela).

## 5. Rollback

Restaurar o dump do Passo 0:
```bash
docker compose ... exec -T postgres sh -lc 'dropdb -U "$POSTGRES_USER" "$POSTGRES_DB" && createdb -U "$POSTGRES_USER" "$POSTGRES_DB"'
docker compose ... exec -T postgres pg_restore -U <user> -d <db> --no-owner /caminho/pre_restructure_*.dump
```
> Só é seguro enquanto nenhuma funcionária tiver gravado dados novos depois do backup.
> Por isso: executar em **janela de baixo uso** e com o backup imediatamente antes.

## 6. Notas de deploy / execução

- Nenhuma migration nova de código é necessária (a coluna `is_agency` já existe — 0158).
- Tudo é dado; **não precisa rebuild** de api/web.
- Escrita em prod é **o Mike** que roda (ver [[feedback_local_only]]); este doc entrega os
  blocos prontos. A IA não executa escrita em prod sozinha.
