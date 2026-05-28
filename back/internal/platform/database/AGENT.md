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

## Comandos uteis

```bash
go run ./cmd/migrate up
go run ./cmd/migrate status
```
