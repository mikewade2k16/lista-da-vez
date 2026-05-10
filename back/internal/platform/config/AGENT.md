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

## Feature flags ativas

| Flag | Tipo | Default | Propos
ito |
|---|---|---|---|
| `CORE_V2_ENABLED` | bool | `false` | Ativa endpoints `/v2/*` e schema `core` novo da reestruturacao multi-tenant (branch `refactor/multi-tenant-core`). Ver `docs/CONTRACT_FREEZE.md` e `docs/SCHEMA_TARGET.md`. **Manter `false` em producao** ate Fase 4 atingir paridade com o produto atual. |

## Onde a flag e consumida

- `back/internal/platform/app/app.go` — log de boot informativo, exposicao em `GET /healthz` (`coreV2Enabled`).
- Codigos da Fase 1 em diante (a serem adicionados) usarao `cfg.CoreV2Enabled` para gatear handlers/services novos.
