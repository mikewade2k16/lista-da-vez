# scripts/monitoring — Monitoracao minima do host da VPS (AC-16)

Escopo: monitoracao leve do **host** da VPS Omni (85.31.62.33, Ubuntu, ~6GB, compartilhada
com a stack omnichannel-mvp). Nada de daemon 24/7 pesado — Prometheus/Grafana/cAdvisor sao
estagio 2 (fora de escopo; ver referencia no fim de `docs/DEPLOY_VPS.md → Monitoracao`).

## check-vps.sh

Roda no host (user `deploy`, ja no grupo docker) via cron a cada 5 min. Quiet por padrao:
so imprime quando ha alerta/erro (o cron nao gera lixo de log). `exit 0` sempre — nunca
quebra o cron. 8 checks:

1. **Disco** da particao raiz (`DISK_USAGE_MAX`, default 85%).
2. **RAM** disponivel `MemAvailable` (`MEM_AVAILABLE_MIN_PCT`, default 10%).
3. **Load 1min por core** (`LOAD_PER_CORE_MAX`, default 2 x nproc).
4. **Containers `unhealthy`** (`docker ps --filter health=unhealthy`; inclui a api quando
   `/healthz` devolve 503).
5. **Containers em `restarting`** (crash-loop).
6. **`GET 127.0.0.1:18080/healthz`** (porta local, nao depende do Caddy compartilhado):
   200=ok, 503=banco fora, 000=api fora.
7. **Saude da WAHA**: container/health, sessao `default=WORKING` e incremento de
   `RestartCount`. O healthcheck do compose executa a recuperacao; esta sonda somente alerta.
   `WAHA_SESSION`, `WAHA_PORT` e `WAHA_EXPECT_WORKING=0|1` permitem ajustar/desativar a
   expectativa de sessao conectada sem editar o script.
8. **Saude do n8n** (OBS-07): container do profile automation no ar + workflows criticos
   `active=true`. NO-OP se `N8N_COMPOSE_DIR` (default `/home/deploy/lista-atendimento`) nao
   existir — nao quebra a sonda em hosts sem automation.
   - `critical`: container n8n fora (`ps -q n8n` vazio) ou `unhealthy` — Calendario/Omni Chat parados.
   - `warning`: container no ar mas >=1 workflow critico despausado (degradacao parcial), ou o
     export falhou (CLI/Node com problema).
   - Le o estado real via `n8n export:workflow --all` + parse do `active` (respeita o WAL do
     SQLite; `docker cp database.sqlite` nao leva writes recentes) — MESMO mecanismo do deploy.
   - **Contrato de IDs:** `N8N_CRITICAL_IDS` (default `calendaromni0001 calendarchat0001
     calendartrans001 omnichatmvp00001`) e a MESMA lista de `scripts/deploy/deploy-pull.ps1:246`.
     Mudou uma, muda a outra. Whatsapp fica de fora (nao e core de prod, igual ao deploy).
   - Roda como `deploy` (ja tem acesso docker) — **sem token/credencial n8n** para so *ler* o estado.
   - Envs opcionais (so se path/porta divergirem do default): `N8N_COMPOSE_DIR`, `N8N_ENV_FILE`
     (default `.env.production`), `N8N_CRITICAL_IDS`.

## Config (na VPS, NUNCA no repo)

`/home/deploy/.omni-monitoring.env` (chmod 600): `ALERT_NTFY_URL` e/ou
`ALERT_TELEGRAM_BOT_TOKEN` + `ALERT_TELEGRAM_CHAT_ID`, alem dos thresholds acima. Sem env
file, o script so ecoa no stdout (nao falha). **Nenhuma credencial vive no repositorio.**

Cooldown de 1h por chave via timestamps em `~/.omni-monitoring-state/` — condicao
persistente re-alerta a cada hora, nao a cada 5 min.

Runbook de instalacao (scp + crontab + env file + UptimeRobot): `docs/DEPLOY_VPS.md →
secao Monitoracao`.
