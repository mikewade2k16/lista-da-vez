# scripts/monitoring — Monitoracao minima do host da VPS (AC-16)

Escopo: monitoracao leve do **host** da VPS Omni (85.31.62.33, Ubuntu, ~6GB, compartilhada
com a stack omnichannel-mvp). Nada de daemon 24/7 pesado — Prometheus/Grafana/cAdvisor sao
estagio 2 (fora de escopo; ver referencia no fim de `docs/DEPLOY_VPS.md → Monitoracao`).

## check-vps.sh

Roda no host (user `deploy`, ja no grupo docker) via cron a cada 5 min. Quiet por padrao:
so imprime quando ha alerta/erro (o cron nao gera lixo de log). `exit 0` sempre — nunca
quebra o cron. 6 checks:

1. **Disco** da particao raiz (`DISK_USAGE_MAX`, default 85%).
2. **RAM** disponivel `MemAvailable` (`MEM_AVAILABLE_MIN_PCT`, default 10%).
3. **Load 1min por core** (`LOAD_PER_CORE_MAX`, default 2 x nproc).
4. **Containers `unhealthy`** (`docker ps --filter health=unhealthy`; inclui a api quando
   `/healthz` devolve 503).
5. **Containers em `restarting`** (crash-loop).
6. **`GET 127.0.0.1:18080/healthz`** (porta local, nao depende do Caddy compartilhado):
   200=ok, 503=banco fora, 000=api fora.

## Config (na VPS, NUNCA no repo)

`/home/deploy/.omni-monitoring.env` (chmod 600): `ALERT_NTFY_URL` e/ou
`ALERT_TELEGRAM_BOT_TOKEN` + `ALERT_TELEGRAM_CHAT_ID`, alem dos thresholds acima. Sem env
file, o script so ecoa no stdout (nao falha). **Nenhuma credencial vive no repositorio.**

Cooldown de 1h por chave via timestamps em `~/.omni-monitoring-state/` — condicao
persistente re-alerta a cada hora, nao a cada 5 min.

Runbook de instalacao (scp + crontab + env file + UptimeRobot): `docs/DEPLOY_VPS.md →
secao Monitoracao`.
