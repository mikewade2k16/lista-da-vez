# Módulo `observability` — painel de observabilidade e coletor do front

> Backlog e estado da observabilidade (o que foi feito e o que falta): `observability/AGENT.md`.

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

Saúde do sistema (fase 6; migration 0330; contratos em `observability/SPEC.md`):

| rota | uso |
|---|---|
| `GET /health` | status por área (api, database, containers, pages, workers, security, deploy), issues (máx. 15, com link de aba), erros 1 h/24 h, online (id e nome, sem e-mail), última amostra de sistema, resumo de alertas e segurança |
| `GET /security` | `obs.security_checks` + `obs.findings` |
| `GET /system?hours=` | séries `api`/`agent` de `obs.system_samples` (hours 1..72, padrão 6; até ~360 pontos) e última amostra de cada container |
| `GET /alerts?status=open\|resolved\|all` | `obs.alerts` (padrão `open`) |
| `GET /deploys?limit=` | `obs.deploys` (limit 1..100, padrão 30; 400 `invalid_limit`): `items` com veredito `ok\|slow\|failed` e `stats` (mediana/p90 dos últimos 10 sucessos do mesmo comando do deploy mais recente; lento = > 1,5 × mediana com ao menos 3 referências) |
| `POST /alerts/test` | e-mail de teste (204; 409 `alerts_not_configured` sem SMTP ou destinatários; 502 se o envio falhar) |

`POST /v1/obs/collect` exige login. Aceita corpo de até 64 KB e 100 eventos, e
responde 202. Eventos inválidos são descartados um a um.

## Arquivos

- `http.go`: validação na borda (ids por regex, módulo/filtro).
- `service.go`: percentis, ordenação, regras de produção e fila.
- `compare.go`: diff entre execuções; métrica maior = pior. Regressão exige mais de
  20% de variação e delta mínimo por métrica.
- `store_health.go` / `store_health_panel.go`, `service_health.go`, `http_health.go`: saúde,
  segurança, sistema e alertas. `buildHealth` e as regras são funções puras (testadas sem
  banco); "unknown" = sem dado, nunca "ok". Segurança fica `warn` enquanto houver check
  `not_tested` ("N de M ataques testados").
- `store_deploys.go` / `service_deploys.go`: histórico de deploys (gravado por `scripts/deploy`,
  ver `deploy-record.mjs`). Veredito, estatísticas, `deployArea` e `deployIssue` são puros
  (testados). Área `deploy`: sem registro = unknown; último ok = ok; lento = warn; falhou =
  critical nas primeiras 24 h (depois warn) e issue high (falhou) ou medium (lento).
- `alerts.go` (laço e sincronização com `obs.alerts`), `alerts_rules.go` (regras puras da
  tabela do SPEC), `alerts_notify.go` (anti-spam e texto do e-mail).
- `store.go` / `store_runs.go`: SQL somente no schema `obs`, sempre com
  `WithoutTracing`.

O worker que consome `obs.run_requests` roda no host (`npm run obs:worker`), não na
API: o runner precisa do Chrome e do repositório.

## Motor de alertas

Goroutine iniciada em `platform/app` só com `OBSERVABILITY_ENABLED` ligado e
`OBS_ALERTS_ENABLED` (padrão true). Primeira rodada 30 s após o boot, depois a cada
`OBS_ALERT_INTERVAL` (60s); cada rodada tem timeout de 30 s e `recover`. Se qualquer
consulta de métrica falha, a rodada é descartada (nada é resolvido com dado parcial).
Um alerta aberto por fingerprint (`regra` ou `regra:alvo`); condição que some resolve o
alerta. Estado de notificação interno fica em `detail._notify` (omitido da API).

Notificação por e-mail (um e-mail agrupado por rodada): ao abrir, ao subir de severidade
(comparada com a severidade já notificada), a cada `OBS_ALERT_RENOTIFY_MINUTES` (60) e ao
resolver (só se o alerta chegou a ser notificado). Falha de envio: `last_notify_error` e
nova tentativa só após 5 min. Sem SMTP ou sem destinatários: avalia e grava, não envia e
marca `last_notify_error = "email nao configurado"`. O corpo só tem ids, rotas e contagens.

| env | padrão | uso |
|---|---|---|
| `OBS_ALERT_EMAILS` | vazio | destinatários, separados por vírgula (a API só devolve a contagem) |
| `OBS_ALERT_RENOTIFY_MINUTES` | 60 | intervalo mínimo de re-notificação |
| `OBS_ALERT_INTERVAL` | 60s | período do avaliador |
| `OBS_ALERTS_ENABLED` | true | liga o laço |

SMTP: mesmos `SMTP_*` do auth, via `platform/mailer`. Link do e-mail: `WEB_APP_URL` +
`/manage/observabilidade`.

Limites conhecidos: assume uma única réplica da API (várias réplicas duplicariam e-mails);
`api_5xx_rate`, `route_errors` e `slow_route` leem `obs.http_requests`, que respeita
`OBS_SAMPLE_RATE` (5xx e lentas sempre gravam; com taxa < 1 o total de requisições é
subestimado e a taxa de 5xx, superestimada). `container_down` trata qualquer container
fora de `running` como crítico, inclusive jobs one-shot que terminam em `exited`.

## Ajustes de alertas (containers, pool, tráfego real)
- `container_down` só para restart_policy always|unless-stopped|on-failure parado, ou health unhealthy; policy `no`/vazia parada não alerta e não conta como "fora" na área containers (aparece como "parados de propósito"). `latestContainers` traz `restart_policy`/`exit_code` e só considera amostras dos últimos 10 min.
- `pool_saturation`: utilização >= 0.9 (3 amostras), canceledAcquireDelta > 0 (2) ou espera média > 50 ms por acquire (3). `emptyAcquireDelta` não é sinal de saturação.
- Consultas de erros/alertas em `obs.http_requests` filtram `source = 'live'`. O agregado `obs.http_route_hourly` não tem coluna source e inclui o tráfego do runner (usado só em latência/painéis, não em alertas).
