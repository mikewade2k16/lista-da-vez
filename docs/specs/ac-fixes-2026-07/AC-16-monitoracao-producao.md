# AC-16 — Monitoração mínima de produção (uptime + alertas de host + healthz com DB + log rotation)

> Spec de implementação. Prioridade **P1**, impacto alto, esforço M.
> Fonte canônica do achado: `fatos.json → achados_canonicos.AC-16`.

---

## 1. Contexto

**Achado (AC-16):** zero monitoração de produção. Evidências verificadas em 2026-07-02:

- `fatos.json → infra.observabilidade`: "logs slog p/ stdout + docker logs; healthz + smoke
  no deploy; SEM métricas (Prometheus/Grafana), SEM alertas, SEM uptime monitor, SEM
  agregação de logs".
- `back/internal/platform/app/app.go:237-262` — o handler `GET /healthz` devolve payload
  **estático** (service, status "ok", lista fixa de módulos, tenantMode, coreV2Enabled).
  **Não faz ping no banco**: a API responde `200` mesmo com o Postgres morto (o pool pgx é
  lazy). O healthcheck do compose (`docker-compose.prod.yml:116-125`, `curl --fail /healthz`)
  e o smoke do deploy (`.github/workflows/deploy-vps.yml:170-173`, exige `200`) herdam essa
  cegueira.
- `docker-compose.prod.yml` — **nenhum** serviço tem bloco `logging:` (grep
  `logging|max-size|json-file` = 0 matches). Default do Docker = driver `json-file` **sem
  limite** → o log de cada container cresce até encher o disco da VPS.
- VPS única (`85.31.62.33`, Ubuntu 24.04, ~6GB RAM) **compartilhada** com a stack
  `omnichannel-mvp` (cujo Caddy roteia o painel — nunca parar) e com stacks órfãs que já
  entraram em crash-loop consumindo 158% de CPU do dockerd (AC-02) — ou seja, incidentes
  reais já aconteceram e ninguém foi avisado.

**Por que importa:** hoje a única forma de descobrir que o painel caiu é um cliente
reclamar. Disco cheio por log sem rotation derruba o Postgres junto. Numa VPS de 6GB
dividida, o mínimo viável é: (1) alguém de fora vigiando o `/healthz`, (2) um script leve no
cron do host alertando RAM/disco/CPU/containers doentes, (3) um `/healthz` que diga a
verdade sobre o banco, (4) rotation de log. **Nada de serviço 24/7 pesado** (Prometheus,
Grafana, cAdvisor ficam para o estágio 2 — fora de escopo).

---

## 2. Objetivo e não-objetivos

### Objetivo (escopo fechado)

1. **Uptime externo** — runbook de configuração do UptimeRobot (free) apontando para
   `https://omni.crowvisuals.com.br/healthz`. Só documentação, zero código.
2. **Alertas de host** — novo `scripts/monitoring/check-vps.sh` (bash, roda no **host** da
   VPS via cron a cada 5 min): disco, RAM disponível, load por core, containers `unhealthy`,
   containers em `restarting` (crash-loop) e `GET 127.0.0.1:18080/healthz`. Alerta via
   ntfy.sh e/ou Telegram (credencial em env file na VPS, **nunca no repo**), com cooldown
   anti-spam de 1h por check.
3. **`/healthz` com ping de banco** — enriquecer o handler em `app.go` com `pool.Ping`
   (timeout 2s): `200 + "db":"ok"` ou `503 + "db":"unreachable"`. Sem expor erro/stack.
4. **Log rotation** — bloco `logging:` (driver `json-file`, `max-size`/`max-file`) nos 6
   serviços do `docker-compose.prod.yml` via YAML anchor.
5. **Documentação** — nova seção `## Monitoração` no `docs/DEPLOY_VPS.md` (runbook completo:
   UptimeRobot + instalação do script + crontab + env file) e AGENT.md dos módulos tocados.

### Não-objetivos (explicitamente FORA)

- **Prometheus / Grafana / cAdvisor / node_exporter / Netdata / Loki** — estágio 2; fica só
  como referência de 3 linhas no fim da seção de doc. Não adicionar NENHUM serviço novo ao
  compose (a VPS de 6GB não comporta mais um daemon 24/7 agora).
- Endpoint `/metrics` na API Go, APM, tracing — fora.
- Agregação/centralização de logs — fora (rotation local basta neste estágio).
- **Não mexer** no `docker-compose.yml` (dev) — healthchecks e mem_limits são do **AC-11**;
  este AC só adiciona `logging:` no compose **de prod**.
- **Não mexer** no Caddy da stack `omnichannel-mvp` nem nas stacks órfãs (AC-02).
- **Não tocar** em `web/` nem em `roadmap-data.ts` (sincronização do roadmap é do
  coordenador da sessão multi-agente — evita conflito).
- Não criar cron de backup (isso é AC-05).

---

## Regras de execução (OBRIGATÓRIAS para o implementador)

- **NENHUM comando git** (sessão multi-agente — só o usuário roda git).
- **NÃO rodar npm/build/generate do web** sem aprovação do usuário. Validação do back:
  `docker compose up -d --build api` **PODE e DEVE** rodar (back/ muda nesta spec).
- Máx **450 linhas** por arquivo novo/refatorado (o script novo tem ~130).
- Não remover funcionalidade existente; o payload atual do `/healthz` (service, status,
  modules, tenantMode, coreV2Enabled) **permanece** — só ganha o campo `db` e o status
  condicional.
- Zero mock/legado novo; nada de credencial no repo (webhook/token ficam em env file na VPS).
- Go: sem lib uuid externa; sem dependência nova (usa `pool.Ping` do pgxpool já importado).
- Portas fixas: api 9091 (host dev), web 3003, postgres 5432 — não mudar. Na VPS a api é
  `127.0.0.1:18080`.
- NUNCA sobrescrever password_hash/dados de usuário (esta spec não toca banco).
- Atualizar o AGENT.md dos módulos tocados ao final.
- Sem migrations nesta spec (não tocar em `back/internal/platform/database/migrations/`).

---

## 3. Mudanças

### 3.1 `/healthz` com ping de banco — `back/internal/platform/app/app.go`

O handler atual (linhas 237-262) está dentro de `BuildHTTPHandler(cfg config.Config,
logger *slog.Logger, pool *pgxpool.Pool)` (linha 47) — `pool` e `logger` já estão
disponíveis na closure. **Assinatura de nada muda.**

Código atual (referência):

```go
mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"service": cfg.AppName,
		"status":  "ok",
		"modules": []string{ /* ... lista fixa ... */ },
		"tenantMode":    "owner-is-client",
		"coreV2Enabled": cfg.CoreV2Enabled,
	})
})
```

Substituir por (manter a lista `modules` EXATAMENTE como está hoje):

```go
mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
	pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	status := http.StatusOK
	overall := "ok"
	dbStatus := "ok"
	if err := pool.Ping(pingCtx); err != nil {
		status = http.StatusServiceUnavailable
		overall = "degraded"
		dbStatus = "unreachable"
		logger.Warn("healthz_db_ping_failed", "error", err)
	}

	httpapi.WriteJSON(w, status, map[string]any{
		"service": cfg.AppName,
		"status":  overall,
		"db":      dbStatus,
		"modules": []string{ /* ... mesma lista fixa de hoje, NÃO alterar ... */ },
		"tenantMode":    "owner-is-client",
		"coreV2Enabled": cfg.CoreV2Enabled,
	})
})
```

Decisões já tomadas (não reabrir):

- **Erro nunca vai no corpo da resposta** (endpoint é público via Caddy) — só `slog.Warn`.
- Timeout de ping = **2s** (menor que o `timeout: 5s` do healthcheck do compose).
- `503` quando o banco cai é **intencional e desejado**: o healthcheck do compose
  (`curl --fail`) passa a marcar o container `unhealthy` (que o `check-vps.sh` do item 3.3
  detecta) e o smoke do deploy falha alto se subir com banco morto. `restart:
  unless-stopped` NÃO reinicia container unhealthy — sem risco de restart-loop novo.
- Custo: UptimeRobot (5 min) + healthcheck do compose (15s) ≈ 5 pings/min — desprezível;
  `/healthz` já passa pelo rate limit global (60 req/min/IP).
- Os imports `context` e `time` já existem em `app.go` (linhas 4 e 8) — nenhum import novo.
- **Sem teste unitário novo**: `app_test.go` não cobre healthz hoje e o ping exige pool
  real; a validação é a da seção 5.

### 3.2 Log rotation — `docker-compose.prod.yml`

Adicionar no topo do arquivo (entre `name:` e `services:`) o extension field com anchor:

```yaml
# Log rotation: sem isso o json-file default cresce sem limite e enche o disco
# da VPS (~6GB compartilhada). Mudanca de logging so aplica ao RECRIAR o
# container (up -d --force-recreate) — ver docs/DEPLOY_VPS.md secao Monitoracao.
x-logging: &default-logging
  driver: json-file
  options:
    max-size: "10m"
    max-file: "3"
```

E aplicar em **cada um dos 6 serviços**:

- `postgres`, `web`, `redis`, `waha`, `n8n` → adicionar `logging: *default-logging`
  (mesma indentação de `restart:`).
- `api` → bloco próprio maior (o slog loga cada request; 30MB seria janela curta demais
  para depurar):

```yaml
    logging:
      driver: json-file
      options:
        max-size: "20m"
        max-file: "5"
```

Teto total de log em disco: 5×30MB + 100MB = **~250MB** (hoje: ilimitado).

**Coordenação com AC-11:** AC-11 também edita `docker-compose.prod.yml` (mem_limits +
healthchecks). Ordem recomendada: **AC-11 primeiro, AC-16 depois** (ou o MESMO agente
implementa os dois). Se AC-11 já tiver adicionado blocos aos serviços, apenas acrescentar a
chave `logging:` sem tocar no resto.

### 3.3 Novo `scripts/monitoring/check-vps.sh`

Criar o diretório `scripts/monitoring/` e o arquivo abaixo **integralmente** (conteúdo
fechado, não improvisar). Roda no **host** da VPS (user `deploy`, que já está no grupo
docker), via cron a cada 5 min. Quiet por padrão: só imprime quando alerta (cron não gera
lixo de log).

```bash
#!/usr/bin/env bash
# check-vps.sh — monitoracao minima do host da VPS Omni (AC-16).
# Roda via cron (*/5) no host. Quiet: so produz output quando ha alerta/erro.
# Config opcional: /home/deploy/.omni-monitoring.env (chmod 600, NUNCA no repo):
#   ALERT_NTFY_URL=https://ntfy.sh/<topico-secreto>
#   ALERT_TELEGRAM_BOT_TOKEN=123456:ABC...
#   ALERT_TELEGRAM_CHAT_ID=123456789
#   DISK_USAGE_MAX=85  MEM_AVAILABLE_MIN_PCT=10  LOAD_PER_CORE_MAX=2
#   ALERT_COOLDOWN_SECONDS=3600  OMNI_API_PORT=18080
set -u

ENV_FILE="${OMNI_MONITORING_ENV:-/home/deploy/.omni-monitoring.env}"
# shellcheck disable=SC1090
[ -f "$ENV_FILE" ] && . "$ENV_FILE"

DISK_USAGE_MAX="${DISK_USAGE_MAX:-85}"
MEM_AVAILABLE_MIN_PCT="${MEM_AVAILABLE_MIN_PCT:-10}"
LOAD_PER_CORE_MAX="${LOAD_PER_CORE_MAX:-2}"
ALERT_COOLDOWN_SECONDS="${ALERT_COOLDOWN_SECONDS:-3600}"
STATE_DIR="${STATE_DIR:-/home/deploy/.omni-monitoring-state}"
OMNI_API_PORT="${OMNI_API_PORT:-18080}"
HOST_LABEL="${HOST_LABEL:-$(hostname)}"

if ! mkdir -p "$STATE_DIR" 2>/dev/null; then
  STATE_DIR="/tmp/omni-monitoring-state"
  mkdir -p "$STATE_DIR"
fi

# send_alert <chave> <mensagem> — respeita cooldown por chave; envia p/ todos os
# canais configurados; sempre ecoa no stdout (vira log do cron se redirecionado).
send_alert() {
  local key="$1" msg="$2" stamp now last
  stamp="$STATE_DIR/$key.last"
  now=$(date +%s)
  if [ -f "$stamp" ]; then
    last=$(cat "$stamp" 2>/dev/null || echo 0)
    [ $((now - last)) -lt "$ALERT_COOLDOWN_SECONDS" ] && return 0
  fi
  echo "$now" > "$stamp"
  echo "[ALERTA][$HOST_LABEL][$key] $msg"
  if [ -n "${ALERT_TELEGRAM_BOT_TOKEN:-}" ] && [ -n "${ALERT_TELEGRAM_CHAT_ID:-}" ]; then
    curl -sS -m 10 -o /dev/null \
      "https://api.telegram.org/bot${ALERT_TELEGRAM_BOT_TOKEN}/sendMessage" \
      --data-urlencode "chat_id=${ALERT_TELEGRAM_CHAT_ID}" \
      --data-urlencode "text=[Omni VPS ${HOST_LABEL}] ${msg}" \
      || echo "[erro] falha ao enviar alerta telegram (${key})"
  fi
  if [ -n "${ALERT_NTFY_URL:-}" ]; then
    curl -sS -m 10 -o /dev/null -H "Title: Omni VPS ${HOST_LABEL}" \
      -d "${msg}" "$ALERT_NTFY_URL" \
      || echo "[erro] falha ao enviar alerta ntfy (${key})"
  fi
}

# 1) Disco (particao raiz)
disk_pct=$(df -P / | awk 'NR==2 {gsub("%","",$5); print $5}')
if [ "${disk_pct:-0}" -ge "$DISK_USAGE_MAX" ]; then
  send_alert disk "Disco em ${disk_pct}% (limite ${DISK_USAGE_MAX}%). Checar: docker system df; backups antigos em /home/deploy/lista-atendimento/backups."
fi

# 2) RAM disponivel (% de MemAvailable sobre MemTotal)
mem_pct=$(awk '/MemTotal/ {t=$2} /MemAvailable/ {a=$2} END {if (t>0) printf "%d", a*100/t}' /proc/meminfo)
if [ "${mem_pct:-100}" -le "$MEM_AVAILABLE_MIN_PCT" ]; then
  send_alert mem "MemAvailable em ${mem_pct}% (minimo ${MEM_AVAILABLE_MIN_PCT}%). Checar: docker stats --no-stream."
fi

# 3) Load medio 1min por core
cores=$(nproc 2>/dev/null || echo 1)
load1=$(awk '{print $1}' /proc/loadavg)
load_max=$(awk -v c="$cores" -v m="$LOAD_PER_CORE_MAX" 'BEGIN {printf "%.2f", c*m}')
if [ "$(awk -v l="$load1" -v m="$load_max" 'BEGIN {print (l>m) ? 1 : 0}')" = "1" ]; then
  send_alert load "load1=${load1} acima de ${load_max} (${cores} cores x ${LOAD_PER_CORE_MAX}). Checar: top; docker stats."
fi

# 4) Containers unhealthy (inclui a api quando o /healthz devolve 503)
unhealthy=$(docker ps --filter health=unhealthy --format '{{.Names}}' 2>/dev/null | paste -sd' ' -)
if [ -n "${unhealthy:-}" ]; then
  send_alert unhealthy "Containers unhealthy: ${unhealthy}"
fi

# 5) Containers em crash-loop
restarting=$(docker ps --filter status=restarting --format '{{.Names}}' 2>/dev/null | paste -sd' ' -)
if [ -n "${restarting:-}" ]; then
  send_alert restarting "Containers em restart-loop: ${restarting}"
fi

# 6) healthz da api direto na porta local (nao depende do Caddy compartilhado)
api_code=$(curl -sS -m 10 -o /dev/null -w '%{http_code}' \
  "http://127.0.0.1:${OMNI_API_PORT}/healthz" 2>/dev/null || echo 000)
if [ "$api_code" != "200" ]; then
  send_alert healthz "GET 127.0.0.1:${OMNI_API_PORT}/healthz => ${api_code} (200=ok; 503=banco fora; 000=api fora)."
fi

exit 0
```

Decisões já tomadas:

- **Canal de alerta**: o script suporta ntfy.sh E Telegram; usa o que estiver configurado no
  env file (os dois, se ambos). Nenhum default embutido — sem env file ele só ecoa no
  stdout (não falha).
- **Cooldown 1h por chave** via arquivos de timestamp em `~/.omni-monitoring-state/` —
  condição persistente re-alerta a cada hora, não a cada 5 min.
- `exit 0` sempre — nunca quebra o cron.
- O check 6 usa a porta local `18080` (bind `127.0.0.1` do compose prod) de propósito:
  separa "api fora" (000/timeout) de "Caddy/DNS fora" (que o UptimeRobot pega de fora).

### 3.4 Novo `scripts/monitoring/AGENT.md`

Criar curto (~25 linhas): escopo (monitoração mínima do host da VPS — AC-16), o que o
`check-vps.sh` checa (6 checks), onde fica a config na VPS
(`/home/deploy/.omni-monitoring.env`, chmod 600, fora do repo), cooldown, e ponteiro para o
runbook em `docs/DEPLOY_VPS.md → Monitoração`. Registrar que Prometheus/Grafana é estágio 2.

### 3.5 `docs/DEPLOY_VPS.md` — nova seção `## Monitoração`

Inserir **entre** a seção `## Backup minimo` (linha 252) e `## Procedimentos especiais
(raros)` (linha 261). Conteúdo obrigatório (adaptar redação ao tom do doc, sem acento no
estilo das seções existentes é opcional — o doc mistura):

1. **Visão geral** — 3 camadas: UptimeRobot (de fora), `check-vps.sh` no cron do host
   (de dentro), `/healthz` com ping de banco (a api se auto-reporta; `503` = banco fora).
2. **Uptime externo (UptimeRobot, one-time, sem código)**:
   - criar conta free em uptimerobot.com → Add Monitor → tipo `HTTP(s)`;
   - URL `https://omni.crowvisuals.com.br/healthz`, intervalo 5 min;
   - alert contact = e-mail do operador (opcional: webhook pro mesmo tópico ntfy);
   - como o `/healthz` agora devolve `503` sem banco, o monitor simples de status já cobre
     api E banco — não precisa de keyword monitor.
3. **Instalar o script de alertas (one-time)**:
   ```bash
   # da sua maquina
   scp -i ~/.ssh/gh_actions_omnichannel_vps scripts/monitoring/check-vps.sh \
     deploy@85.31.62.33:/home/deploy/monitoring/check-vps.sh
   ssh -i ~/.ssh/gh_actions_omnichannel_vps deploy@85.31.62.33 \
     "chmod +x /home/deploy/monitoring/check-vps.sh"
   ```
4. **Configurar o canal de alerta (na VPS, NUNCA no repo)** — criar
   `/home/deploy/.omni-monitoring.env` com `chmod 600`, exemplo com `ALERT_NTFY_URL` (tópico
   secreto/aleatório) e/ou `ALERT_TELEGRAM_BOT_TOKEN` + `ALERT_TELEGRAM_CHAT_ID`.
5. **Crontab (na VPS, user deploy)**:
   ```bash
   ( crontab -l 2>/dev/null; echo '*/5 * * * * /home/deploy/monitoring/check-vps.sh >> /home/deploy/monitoring/check-vps.log 2>&1' ) | crontab -
   ```
   Nota: o script é quiet — o `.log` só cresce quando há alerta/erro.
6. **Log rotation dos containers** — explicar que o compose prod agora tem `logging:`
   json-file com teto (~250MB total) e que a mudança **só aplica ao recriar o container**:
   próximo deploy usar `force_recreate` (input do `deploy-vps.yml`) ou, na VPS,
   `docker compose --env-file .env.production -f docker-compose.prod.yml up -d --force-recreate`.
7. **Teste do alerta** — na VPS: `DISK_USAGE_MAX=1 /home/deploy/monitoring/check-vps.sh`
   deve disparar o alerta de disco no canal configurado (e depois
   `rm /home/deploy/.omni-monitoring-state/disk.last` para rearmar).
8. **Estágio 2 (referência, fora de escopo)** — 3 linhas: quando houver VPS dedicada/maior,
   avaliar node_exporter + cAdvisor + Prometheus/Grafana Cloud free ou Netdata; pré-requisito
   é o AC-11 (limites de memória) aplicado.

### 3.6 `back/internal/platform/app/AGENT.md`

Adicionar 1 bullet na seção "Peças" (ou nova subseção curta): `GET /healthz` agora faz
`pool.Ping` (timeout 2s) e devolve `200 + db:"ok"` ou `503 + db:"unreachable"`; consumidores
que dependem do 200: healthcheck do compose, smoke do deploy, UptimeRobot e
`scripts/monitoring/check-vps.sh` (AC-16).

---

## 4. Critérios de aceite

1. `GET /healthz` com Postgres up → `200`, corpo contém `"status":"ok"`, `"db":"ok"` e TODOS
   os campos que já existiam (`service`, `modules` com as 16 entradas atuais, `tenantMode`,
   `coreV2Enabled`).
2. `GET /healthz` com Postgres parado → `503`, corpo contém `"status":"degraded"`,
   `"db":"unreachable"`; **nenhum** texto de erro/stack no corpo; `slog.Warn
   healthz_db_ping_failed` aparece no log da api.
3. `docker-compose.prod.yml` tem `x-logging` no topo e os 6 serviços (`postgres`, `api`,
   `web`, `redis`, `waha`, `n8n`) com bloco `logging:` (api = 20m/5; demais = anchor 10m/3);
   `docker compose -f docker-compose.prod.yml config` valida sem erro.
4. `scripts/monitoring/check-vps.sh` existe com o conteúdo da seção 3.3, `bash -n` passa,
   tem < 450 linhas e **roda sem env file** sem crashar (apenas stdout).
5. Nenhuma credencial/URL de webhook real em nenhum arquivo do repo.
6. `docs/DEPLOY_VPS.md` tem a seção `## Monitoração` com os 8 itens da seção 3.5.
7. `back/internal/platform/app/AGENT.md` e novo `scripts/monitoring/AGENT.md` atualizados.
8. Nenhum comando git executado; nenhum arquivo fora da lista da seção 7 alterado.

---

## 5. Validação

Back (PODE rodar — back/ mudou):

```bash
cd c:/Users/Mike/Documents/Projects/fila-atendimento
docker compose up -d --build api

# banco up -> 200 + db ok (porta host dev = 9091)
curl -s -o - -w '\nHTTP %{http_code}\n' http://localhost:9091/healthz

# banco down -> 503 + db unreachable (e o log da api mostra healthz_db_ping_failed)
docker compose stop postgres
curl -s -o - -w '\nHTTP %{http_code}\n' http://localhost:9091/healthz
docker compose logs --tail=20 api
docker compose start postgres
curl -s -o /dev/null -w 'HTTP %{http_code}\n' http://localhost:9091/healthz   # volta a 200
```

Compose prod (só sintaxe/merge — não sobe nada):

```bash
docker compose --env-file .env.docker -f docker-compose.prod.yml config --quiet
# (se faltar env obrigatoria no .env.docker, exportar dummies so p/ o config:
#  POSTGRES_DB=x POSTGRES_USER=x POSTGRES_PASSWORD=x AUTH_TOKEN_SECRET=x ... )
docker compose --env-file .env.docker -f docker-compose.prod.yml config | grep -A4 'max-size'
```

Script (no Git Bash local, sem tocar na VPS):

```bash
bash -n scripts/monitoring/check-vps.sh
STATE_DIR=/tmp/omni-mon-test OMNI_API_PORT=9091 bash scripts/monitoring/check-vps.sh; echo "exit=$?"
# esperado: exit=0; com a api local up e recursos ok, nenhum output (quiet)
DISK_USAGE_MAX=1 STATE_DIR=/tmp/omni-mon-test2 bash scripts/monitoring/check-vps.sh
# esperado: linha "[ALERTA][...][disk] ..." no stdout (sem canal configurado, so ecoa)
```

Go vet/test (parte do gate normal): `cd back && go vet ./... && go test ./internal/platform/app/...`

Passos na VPS (UptimeRobot, scp do script, crontab, force_recreate) são **do usuário**,
seguindo o runbook novo — o implementador NÃO acessa a VPS.

---

## 6. Notas de Deploy

- **Migrations:** nenhuma.
- **Env vars da aplicação:** nenhuma. (As envs do script — `ALERT_NTFY_URL` etc. — vivem em
  `/home/deploy/.omni-monitoring.env` na VPS, criadas manualmente pelo usuário, chmod 600.)
- **Rebuild:** api precisa de imagem nova (mudou `app.go`) → deploy normal
  (`npm run deploy:fast:prod` ou CI), rodado pelo usuário.
- **Ordem de aplicação (pelo usuário):**
  1. (se AC-11 estiver na fila) aplicar AC-11 antes — mesmo arquivo compose;
  2. deploy da imagem nova da api;
  3. na VPS: `up -d --force-recreate` (ou input `force_recreate` do deploy-vps.yml) para o
     `logging:` valer nos containers existentes;
  4. instalar `check-vps.sh` + env file + crontab (runbook seção Monitoração);
  5. configurar o monitor no UptimeRobot;
  6. testar: parar postgres por 30s em staging OU rodar `DISK_USAGE_MAX=1 ...` e conferir a
     notificação chegando.

---

## 7. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `back/internal/platform/app/app.go` | editar (handler `GET /healthz`, linhas ~237-262) |
| `back/internal/platform/app/AGENT.md` | editar (bullet do healthz) |
| `docker-compose.prod.yml` | editar (`x-logging` + `logging:` nos 6 serviços) |
| `scripts/monitoring/check-vps.sh` | **criar** |
| `scripts/monitoring/AGENT.md` | **criar** |
| `docs/DEPLOY_VPS.md` | editar (nova seção `## Monitoração` entre "Backup minimo" e "Procedimentos especiais") |

Conflitos potenciais com outros ACs: **AC-11** (mesmo `docker-compose.prod.yml` — aplicar
AC-11 primeiro ou mesmo agente); **AC-01** (também edita `app.go`, em região diferente —
wiring do PrincipalCache no topo do `BuildHTTPHandler`); **AC-05** (também deve adicionar
runbook de cron/backup no `DEPLOY_VPS.md` — seções distintas, merge trivial).
