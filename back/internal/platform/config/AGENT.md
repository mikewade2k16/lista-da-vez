# AGENT

## Escopo

Estas instrucoes valem para `back/internal/platform/config/`.

## Papel desta pasta

- carregar configuracao da aplicacao a partir de variaveis de ambiente
- expor `Config` consumido pelo bootstrap em `platform/app/`
- definir defaults seguros para ambientes locais sem `.env`

## Regras

- Nao colocar regra de negocio aqui. Apenas leitura, parsing e validacao leve.
- Toda nova variavel de ambiente deve aparecer tambem em `back/.env.example` com comentario explicando o impacto.
- Defaults: producao deve preferir falhar cedo (env obrigatoria) quando o valor nao tiver fallback seguro. Dev/local pode usar valores razoaveis (ex: `APP_ADDR=:8080`).
- Guards de producao vao no metodo `(Config).Validate()`. Em dev/docker ele e no-op; em `APP_ENV=production` aborta o boot se algum default inseguro escapou. Adicionado em 2026-05-21 (Fase 8.6): aborta com `AUTH_TOKEN_SECRET` em branco ou igual ao default de dev, e com `AUTH_BCRYPT_COST < 10`. `cmd/api/main.go` chama `cfg.Validate()` logo apos `config.Load()` e faz `os.Exit(1)` se falhar.

## Feature flags ativas

| Flag | Tipo | Default | Propos
ito |
|---|---|---|---|
| `CORE_V2_ENABLED` | bool | `false` | Ativa endpoints `/v2/*` e schema `core` novo da reestruturacao multi-tenant (branch `refactor/multi-tenant-core`). Ver `docs/CONTRACT_FREEZE.md` e `docs/SCHEMA_TARGET.md`. **Manter `false` em producao** ate Fase 4 atingir paridade com o produto atual. |

## Onde a flag e consumida

- `back/internal/platform/app/app.go` — log de boot informativo, exposicao em `GET /healthz` (`coreV2Enabled`).
- Codigos da Fase 1 em diante (a serem adicionados) usarao `cfg.CoreV2Enabled` para gatear handlers/services novos.
