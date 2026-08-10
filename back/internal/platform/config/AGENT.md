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

## Variaveis de ambiente notaveis

- `AUTH_PRINCIPAL_CACHE_TTL` (duration, default `30s`) — TTL do cache de Principal autenticado (AC-01). `0s` desliga o cache e restaura o comportamento legado (1 rajada de queries por request), sem rebuild. Consumida em `platform/app/principal_cache_wiring.go`. Documentada comentada nos `.env*.example`.
- `DATABASE_APP_URL` (AC-04) — URL de RUNTIME da api com a role least-privilege `omni_app` (sem DDL). `OpenAppPool` a usa; sem ela cai no fallback `DATABASE_URL` (dev). `Validate()` em `APP_ENV=production` aborta o boot se estiver vazia — a api nunca deve conectar como o superuser em prod. O binario `migrate` continua com `DATABASE_URL` (privilegiada). Runbook em `docs/MULTITENANT_COMPLETION_PLAN.md` (Notas de Deploy AC-04).

- `R2_*`: conexao privada do modulo `storage`. `R2_ENABLED=false` preserva o boot sem
  credenciais; quando `true`, o Build exige account, bucket, access key, secret e timeout positivo.
  Os limites fail-closed nao sao env: vivem no singleton autoritativo `storage.settings` e sao
  editados por `platform_admin`. Segredos nunca entram no banco nem sao expostos pela API.
  `R2_UPLOAD_TIMEOUT` e separado do timeout curto das demais chamadas. O Bearer
  `R2_ANALYTICS_API_TOKEN` e diferente da Access Key/Secret S3 e habilita metricas account-wide.
  `R2_ALLOW_NONEMPTY_BUCKET_INITIALIZATION=false` mantem o bootstrap fail-closed; `true` deve ser
  usado somente quando uma nova instalacao precisa adotar explicitamente um bucket dedicado ja
  populado. A flag nao altera nem regrava os objetos existentes.

## Feature flags ativas

| Flag | Tipo | Default | Propos
ito |
|---|---|---|---|
| `CORE_V2_ENABLED` | bool | `false` | Ativa endpoints `/v2/*` e schema `core` novo da reestruturacao multi-tenant (branch `refactor/multi-tenant-core`). Ver `docs/CONTRACT_FREEZE.md` e `docs/SCHEMA_TARGET.md`. **Manter `false` em producao** ate Fase 4 atingir paridade com o produto atual. |

## Onde a flag e consumida

- `back/internal/platform/app/app.go` — log de boot informativo, exposicao em `GET /healthz` (`coreV2Enabled`).
- Codigos da Fase 1 em diante (a serem adicionados) usarao `cfg.CoreV2Enabled` para gatear handlers/services novos.
