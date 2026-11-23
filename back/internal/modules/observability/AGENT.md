# Módulo `observability` — painel de observabilidade e coletor do front

Leitura do schema `obs` para a página `/manage/observabilidade` e ingestão dos eventos
do navegador. A instrumentação fica em `internal/platform/observability`. O runner
(`npm run obs:run`) e o inventário ficam em `observability/` na raiz. Contratos:
`observability/SPEC.md`.

## Rotas

Todas em `/v1/admin/observability`, exigem `RequireRoles(platform_admin)`: a
telemetria é global e atravessa tenants. Envelope `{ "data": ... }`. Parâmetro `days`
de 1 a 90 (padrão 7).

| rota | uso |
|---|---|
| `GET /overview` | números ao vivo, nota/componentes/cobertura por módulo da última execução, tendência das notas |
| `GET /endpoints?module=&sort=p95\|errors\|calls\|queries` | p50/p95/p99 pelo histograma, taxa de erro, queries/req |
| `GET /db` | lentas, suspeitas de N+1, top por tempo total, checagens do último run |
| `GET /pages` | Web Vitals p75 por página (ao vivo) e resultado do último run |
| `GET /errors` | erros agrupados (API 5xx e front) com correlation IDs |
| `GET /trace/{correlationId}` | requisição → queries lentas/N+1 → externas → eventos do front |
| `GET /runs`, `GET /runs/{id}`, `GET /runs/compare?a=&b=` | histórico (notas e cobertura resumidas em números), detalhe completo e comparação antes × depois |
| `GET /run-requests`, `POST /run-requests` | fila do botão "rodar runner" (403 `runner_disabled_in_production` em produção; 429 com 5 pendentes) |

`POST /v1/obs/collect` exige login. Aceita corpo de até 64 KB e 100 eventos, e
responde 202. Eventos inválidos são descartados um a um.

## Arquivos

- `http.go`: validação na borda (ids por regex, módulo/filtro).
- `service.go`: percentis, ordenação, regras de produção e fila.
- `compare.go`: diff entre execuções; métrica maior = pior. Regressão exige mais de
  20% de variação e delta mínimo por métrica.
- `store.go` / `store_runs.go`: SQL somente no schema `obs`, sempre com
  `WithoutTracing`.

O worker que consome `obs.run_requests` roda no host (`npm run obs:worker`), não na
API: o runner precisa do Chrome e do repositório.
