# AGENT

## Escopo

Estas instrucoes valem para o codigo tecnico de banco em `back/internal/platform/database/`.

## Papel desta pasta

- abrir pool PostgreSQL
- carregar e aplicar migrations
- servir como infraestrutura compartilhada para os modulos

## Regra principal

Esta pasta nao deve conter regra de negocio do produto.

Ela so cuida de:

- conexao
- transacao
- migrations
- utilitarios tecnicos de persistencia

Se a mudanca for de modelagem funcional, ela precisa refletir tambem em:

- `back/database/AGENT.md`
- `back/database/ERD.md`
- modulo de negocio correspondente

## Regra de eficiencia

- esta camada deve favorecer operacoes SQL pequenas, previsiveis e idempotentes
- evitar apoiar implementacoes que regravem secoes inteiras quando a mudanca e de um unico item
- quando o modulo precisar mutacao granular, a infraestrutura deve facilitar `upsert`, `delete` e `touch` focados no recurso alterado
- a infraestrutura tambem deve favorecer contratos HTTP que omitam campos opcionais/default, reduzindo serializacao, I/O e escrita desnecessaria
- quando houver upload de arquivo de usuario, a infraestrutura deve manter no banco apenas o metadado necessario
  - exemplo atual: `users.avatar_path`
  - o binario fica em storage/volume fora do PostgreSQL
- quando houver onboarding por convite, a infraestrutura deve favorecer:
  - token armazenado em hash
  - expiracao previsivel
  - revogacao simples de convites pendentes
  - leitura segura do estado de onboarding sem precisar devolver o token persistido
- quando houver papeis de escopo de loja:
  - a infraestrutura deve facilitar validacao de loja unica por usuario
  - o schema deve permitir diferenciar conta individual (`consultant`) de conta fixa da unidade (`store_terminal`)
- para leituras analiticas, a infraestrutura deve facilitar:
  - indices por `store_id` + tempo
  - filtros previsiveis por colunas mais consultadas
  - uso consciente de `jsonb` quando o dado estruturado fizer parte do filtro ou da agregacao
- para a operacao integrada multi-loja:
  - a infraestrutura deve permitir leitura previsivel do estado corrente por varias lojas sem duplicar logica de montagem
  - a coluna `kind` em `operation_paused_consultants` deve permanecer tratada como campo funcional do estado corrente
  - a tabela `store_setting_options` deve aceitar os `kind` funcionais atuais:
    - `visit_reason`
    - `customer_source`
    - `pause_reason`
    - `queue_jump_reason`
    - `loss_reason`
    - `profession`

## Fonte de verdade humana do banco

Consultar primeiro:

- `back/database/AGENT.md`
- `back/database/ERD.md`

## Convencao de nomeacao de migrations (Fase 6.6 do PLANO_REFATORACAO)

Padrao oficial **a partir de 2026-05-18**:

```
NNNN_slug_descritivo.sql
```

Onde:

- `NNNN`: 4 digitos zero-padded, ordem cronologica estrita
- `slug_descritivo`: kebab/snake case curto descrevendo o que a migration faz
- Extensao `.sql` obrigatoria

Regras inegociaveis:

1. **Nunca usar sufixo alfabetico** (`0015a_*.sql`). Causa ambiguidade lexicografica.
2. **Nunca criar 2 migrations com o mesmo NNNN**. O migrator usa o nome de arquivo como chave em `schema_migrations.version` — colisoes funcionam por sorte da ordenacao lexicografica do `slug`.
3. **Sempre incrementar a partir do maior numero existente**. Antes de criar `NNNN_*.sql`, listar `back/internal/platform/database/migrations/` e usar `MAX(NNNN) + 1`.
4. **Migrations sao imutaveis depois de comitadas**. Para mudar comportamento, criar nova migration.

### Colisoes legadas (dividia congelada — NAO renomear)

As 4 colisoes abaixo ficam congeladas porque renomear arquivos quebra o migrator (`schema_migrations.version = filename` no banco; renomear faz a migration ser tratada como nova e tenta reaplicar):

| Prefixo | Arquivos | Ordem real (lex) |
|---|---|---|
| `0015` | `0015_mvp_access_foundation.sql`, `0015a_access_foundation_schema.sql` | `0015_*` aplica antes de `0015a_*` |
| `0019` | `0019_user_password_resets.sql`, `0019_workspace_access_matrix.sql` | `_user` antes de `_workspace` |
| `0031` | `0031_active_service_stop_state.sql`, `0031_per_consultant_concurrency.sql` | `_active` antes de `_per` |
| `0039` | `0039_erp_store_184_production_bootstrap.sql`, `0039_finish_modal_purchase_code.sql` | `_erp` antes de `_finish` |

Bancos em producao e dev ja tem essas migrations gravadas com o nome exato. Trocar nome = bug imediato.

## Dois Postgres em desenvolvimento local (ARMADILHA)

Em desenvolvimento local existem **dois** servidores PostgreSQL rodando simultaneamente:

| Servidor | Porta host | Uso |
|---|---|---|
| `omni-postgres-1` (container Docker) | **5433** | banco real do produto — apps conectam aqui |
| PostgreSQL nativo Windows | 5432 | banco legado/dev antigo — **NÃO usar pra migrations do projeto** |

O `back/.env` aponta para `localhost:5433` (correto). Se por qualquer motivo mudar para 5432, o `migrate up` vai aplicar migrations num banco errado sem aviso.

Antes de rodar qualquer `go run ./cmd/migrate`, confirmar:
```bash
echo $DATABASE_URL   # deve conter :5433
```

## Comandos uteis

```bash
# A partir do diretório back/
export DATABASE_URL="postgres://omni:omni_dev@localhost:5433/omni?sslmode=disable"
go run ./cmd/migrate up
go run ./cmd/migrate status
```

## Role de runtime least-privilege (AC-04)

Dois pools, duas roles (defesa em profundidade + pre-requisito do RLS):

- **`api`** conecta via `OpenAppPool` / `DATABASE_APP_URL` com a role `omni_app`
  (`NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS NOREPLICATION`, sem `CREATE`
  em schema algum). So DML — nao roda DDL. Fallback para `DATABASE_URL` quando
  `DATABASE_APP_URL` esta vazia (dev local fora do docker); em `APP_ENV=production`
  o `Validate()` da config exige `DATABASE_APP_URL` e aborta o boot sem ela.
- **`migrate`** continua com `OpenPool` / `DATABASE_URL` (role privilegiada `omni`):
  roda o DDL das migrations e o bootstrap.
- Os GRANTs de runtime da `omni_app` (USAGE em schemas; SELECT/INSERT/UPDATE/DELETE
  em tabelas/views; USAGE/SELECT/UPDATE em sequences; `ALTER DEFAULT PRIVILEGES`
  global para objetos futuros) sao **auto-sincronizados** por `SyncAppRoleGrants`
  (`app_role_grants.go`) a cada `migrate up` — cobre schemas/tabelas novos sem
  passo manual. A lista de schemas vem de `pg_namespace` (nada hardcoded).
- **AC-04b — auto-provisao da role (IMPLEMENTADO):** a **existencia + senha** da role
  agora tambem se auto-curam. O `migrate up` chama `EnsureAppRole` (`app_role_ensure.go`)
  ANTES de `SyncAppRoleGrants`: extrai nome+senha de `DATABASE_APP_URL` e, como a role
  privilegiada que ja e, `CREATE ROLE ... LOGIN` (se ausente) + `ALTER ROLE ... PASSWORD`
  (converge senha/atributos least-privilege sempre — cura rotacao) + `GRANT CONNECT`,
  tudo idempotente e numa tx com `log_statement=none` para a senha nao vazar no log do
  Postgres. **Fail-fast:** em `APP_ENV=production`, se a role for pulada por `empty_url`
  ou `empty_password`, o `migrate` sai com `os.Exit(1)` e loga `app_role_ensure_failed` —
  em vez do crash-loop opaco `28P01` da api. Em dev sem role dedicada (app e migrate na
  mesma role), pula com `same_role` e segue. Log de sucesso: `app_role_ensure_ok created=<bool>`.
  Com isso o deploy AC-04 vira self-healing: nenhum ambiente novo (prod/staging/dev com
  volume limpo) cai no incidente de 2026-07-03. Contrato de `SyncAppRoleGrants` intacto
  (segue como defesa em profundidade). Runbook/causa raiz em `docs/DEPLOY_VPS.md` e
  `docs/MULTITENANT_COMPLETION_PLAN.md` (AC-04).
- Script canonico de criacao da role: `scripts/db/create-app-role.sql` (idempotente,
  cluster-level, senha via `-v pw`). Dev: `scripts/db/postgres-init/10-app-role.sh`
  roda no init do volume (docker-entrypoint-initdb.d) e e reutilizavel em volume
  existente via `docker compose exec -T postgres sh /docker-entrypoint-initdb.d/10-app-role.sh`.
- Testes de integracao (skip sem `TEST_DATABASE_URL`): `app_role_grants_test.go` prova
  DML permitido (`select core.users`) e DDL negado (`create table`/`create schema`
  retornam SQLSTATE 42501); `app_role_ensure_test.go` cobre criacao, idempotencia,
  rotacao de senha, os tres skips (`empty_url`/`same_role`/`empty_password`, sem tocar
  DDL) e a rejeicao de nome de role invalido.

## Notas recentes de schema

- `0236_messaging_contact_intelligence.sql` cria a memória estruturada tenant-safe por contato.
  A tabela guarda somente resumo/fatos/preferências derivados e métricas operacionais; não guarda
  histórico bruto, prompt ou segredo. O upsert deve permanecer condicionado à lease válida da
  conversa no mesmo statement.

- `0232_messaging_ai_unlimited_turns.sql` define `max_ai_turns=0` como sem limite, amplia o check
  para `0..100` e altera somente o default para novas versões. Versões publicadas permanecem
  imutáveis e devem ser atualizadas pelo endpoint de configuração, que cria nova versão.
- `0231_messaging_contact_suppressions.sql` cria a supressão lógica tenant-safe de contatos e o
  cutoff opcional de histórico. Também cria `omnichannel.conversations.privacy.manage` e concede
  override explícito apenas ao usuário autorizado; não ampliar esse grant por papel ou por bypass
  de `platform_admin` sem decisão de produto.
- `0228_messaging_automation_profiles.sql` cria o vínculo tenant-safe cliente↔número↔agente
  do MVP de automação. Um número não pode atender dois clientes; a policy de encerramento nasce
  conservadora e a lease `conversations.ai_generation` continua obrigatória no Go. O arquivo é
  apenas DDL local até aplicação explícita no banco correto; não aplicar automaticamente.
- `0229_messaging_ai_close_evaluations.sql` registra toda proposta de fechamento da IA, aceita
  ou bloqueada, incluindo snapshot dos gates e da geração. Não contém prompt ou mensagem; o
  fechamento continua sendo aplicado pelo Go sob lock da conversa.
- `0234_messaging_ai_credentials_and_roles.sql` cria o cofre nomeado account-scoped de IA,
  vincula a credencial de resposta/analise às versões e análises e adiciona `video_summary`.
  A importação de keyrings legados é feita pelo service após a migration, nunca pelo SQL.
- `0215_messaging_delivery_reconciliation.sql` adiciona metadados canonicos de ACK a
  `messaging.webhook_events`, com constraint de completude/vocabulario seguro e indice de replay
  por conta+provider+instancia+mensagem ordenado por timestamp+UUID. Tambem adiciona o GIN
  `pg_trgm` que corresponde a `lower(messaging.messages.content) LIKE '%...%'`; a extensao ja
  existe desde 0034. A migration e aditiva e nao deve ser renumerada nem incorporada a 0213/0214.
- `0213_messaging_message_delivery.sql` amplia o E1 omnichannel com lease `ai_generation`,
  reply local/externo, origem fechada, ACKs monotonicos e indices de dedupe/cursor. E aditiva;
  nunca editar depois de aplicada e nunca guardar arquivo de midia ou segredo nas novas colunas.
- `0129_site_tracking_events.sql` adiciona `site.tracking_events` e amplia
  `site.webhook_sources.entity_type` para `tracking`. O receptor usa HMAC
  com timestamp e idempotencia por `source_id + source_event_id`.
