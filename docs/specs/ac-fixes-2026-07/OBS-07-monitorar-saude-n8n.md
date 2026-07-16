# OBS-07 — Sonda de saúde do n8n (container no ar + workflows críticos ativos)

> Spec de implementação · Prioridade **P1** · Esforço **S** · Impacto **médio-alto**
> Origem: ideia do dono 2026-07-13 (evolução do AC-16 / grupo observabilidade-n8n) · roadmap `observabilidade-n8n` → task `obs-07-monitorar-saude-n8n`
> **DEPENDE do contrato de payload de [OBS-01/OBS-02]** (`{host,key,msg,severity,ts}`). Deve subir DEPOIS de OBS-01 (o `send_alert` com canal n8n) e idealmente com OBS-02 (fan-out) no ar.

## 1. Contexto

**O achado:** o n8n é infraestrutura crítica silenciosa. Hoje:
- O `deploy-pull.ps1` (quando `-DeployAutomation`) sobe os containers do profile automation e, ao final, **reativa uma lista fixa de workflows** (`calendaromni0001 calendarchat0001 calendartrans001 omnichatmvp00001`, ver `scripts/deploy/deploy-pull.ps1:246`). Ou seja: o deploy garante que eles estão ativos **no instante do deploy**.
- **Depois disso, nada vigia.** Se o container n8n cair, ou se um workflow crítico for despausado (manual, erro de import, restart parcial), o Calendário/Omni Chat param de responder e **ninguém é avisado** — o `check-vps.sh` só olha api/web/disco/RAM, não o n8n em si (`scripts/monitoring/check-vps.sh:55-92`, 6 checks, nenhum de n8n).
- O incidente do bot pausado já mordeu antes: o prompt "não mudava" porque o AI Agent estava parado (memória `project_vps_n8n_prompt_law`) — exatamente a classe de problema que esta sonda pega.

**Evidências (código lido em 2026-07-13):**
- `scripts/monitoring/check-vps.sh:94` — `exit 0` final; a sonda nova entra ANTES dele como check #8 (o #7/#7b são do OBS-01: backup/drill).
- `scripts/deploy/deploy-pull.ps1:229-230` — comando canônico que lê os workflows COM o estado `active` real do banco do n8n (respeita o WAL do SQLite, ao contrário de `docker cp database.sqlite`):
  ```
  n8n export:workflow --all --output=/tmp/... ; node -e "...w.active===true..."
  ```
- `scripts/deploy/deploy-pull.ps1:246` — a lista canônica dos workflows que prod exige ativos.
- Container n8n em prod: nome do serviço compose `n8n` (no dev o container é `omni-n8n-1`); publica no host em loopback `127.0.0.1:${AUTOMATION_N8N_PORT:-15680}:5678` (`docker-compose.prod.yml`; **CONFERIR na VPS: `grep AUTOMATION_N8N_PORT .env.production`**, mesma pegadinha do OBS-01).

**Fonte da verdade dos IDs** (id → nome, levantado do banco em 2026-07-13):
`calendaromni0001`=Calendar Omni · `calendarchat0001`=Calendar Chat · `calendartrans001`=Calendar Transcribe · `omnichatmvp00001`=Omni Chat. (O `lzhb5JjN5kdcVuRR`=Whatsapp NÃO entra na lista crítica: hoje não é core de prod — mantém-se fora, igual à lista do deploy.)

## 2. Objetivo e não-objetivos

**Objetivo (escopo fechado):**
1. Adicionar ao `check-vps.sh` um **check #8 "n8n"** que:
   a. confere se o container n8n está **rodando e healthy** (via `docker compose ... ps` do profile automation, no `remotePath` da VPS);
   b. se está no ar, lista os workflows e confere que **todos os IDs críticos estão `active=true`**;
   c. dispara `send_alert n8n "<detalhe>" critical` quando o container está fora, e `warning` quando o container está no ar mas ≥1 workflow crítico está inativo (degradação parcial ≠ queda total).
2. Reusar **exatamente** o mecanismo de leitura do deploy (`n8n export:workflow --all` + parse do `active`) — sem inventar outra forma nem depender de `docker cp` do SQLite.
3. Zero credencial/segredo novo: a sonda roda no host da VPS como o `deploy` (já tem acesso ao docker), não precisa de token n8n para *ler* o estado.

**Não-objetivos (FORA):**
- NÃO criar o workflow de fan-out (é OBS-02) — OBS-07 só chama `send_alert`, que já roteia (OBS-01).
- NÃO monitorar execuções/erros internos de cada workflow (fila de execuções, últimos erros) — isso é evolução futura (encostar em OBS-04/anomalia).
- NÃO tentar **reativar** automaticamente o workflow caído (auto-heal do n8n é decisão separada; a sonda só ALERTA). Registrar como follow-up.
- NÃO incluir o Whatsapp na lista crítica enquanto ele não for core de prod (espelha a lista do deploy-pull.ps1:246).
- NÃO tocar nos 6 checks existentes além de já estarem com severidade (isso é do OBS-01).

## 3. Regras de execução (obrigatórias)

- **NENHUM comando git** (o dono commita). Editar o script no repo; instalação na VPS é `scp` + teste (runbook §6), como OBS-01.
- Bash POSIX-ish, estilo do arquivo atual (`set -u`), roda em Ubuntu 24.04.
- **Nunca inventar credencial** ([[feedback_credentials]]): a leitura do estado do n8n é via `docker compose exec` (o `deploy` já tem docker); não pedir token.
- **Não supor a porta/nome**: conferir `AUTOMATION_N8N_PORT` e o nome do serviço no `.env.production`/compose da VPS antes (a spec assume serviço `n8n`, mas o runbook manda checar).
- A lista de IDs críticos deve ficar **num único ponto** do script (variável no topo do check), com comentário apontando que é a MESMA de `deploy-pull.ps1:246` — se uma mudar, a outra muda junto (contrato).
- Atualizar `scripts/monitoring/AGENT.md` + `docs/DEPLOY_VPS.md → Monitoração`.

## 4. Mudanças (passo a passo)

### 4.1 EDITAR `scripts/monitoring/check-vps.sh` — header de config

Acrescentar às linhas de exemplo do header (junto das do OBS-01):

```bash
#   N8N_COMPOSE_DIR=/home/deploy/lista-atendimento         (onde vive o docker-compose.prod.yml + .env.production)
#   N8N_ENV_FILE=.env.production                            (para `docker compose --env-file`)
#   N8N_CRITICAL_IDS="calendaromni0001 calendarchat0001 calendartrans001 omnichatmvp00001"  (default; = deploy-pull.ps1)
```

### 4.2 EDITAR `scripts/monitoring/check-vps.sh` — check #8, ANTES do `exit 0` final (linha 94)

```bash
# 8) Saude do n8n: container no ar + workflows criticos ATIVOS.
#    A lista N8N_CRITICAL_IDS e a MESMA de scripts/deploy/deploy-pull.ps1:246
#    (contrato: mudou uma, muda a outra). Le o estado real via `n8n export:workflow`
#    (respeita o WAL do SQLite; `docker cp database.sqlite` NAO leva writes recentes).
N8N_COMPOSE_DIR="${N8N_COMPOSE_DIR:-/home/deploy/lista-atendimento}"
N8N_ENV_FILE="${N8N_ENV_FILE:-.env.production}"
N8N_CRITICAL_IDS="${N8N_CRITICAL_IDS:-calendaromni0001 calendarchat0001 calendartrans001 omnichatmvp00001}"
n8n_compose="docker compose --env-file $N8N_ENV_FILE -f docker-compose.prod.yml --profile automation"

if [ -d "$N8N_COMPOSE_DIR" ]; then
  # 8a) container rodando?
  n8n_state=$(cd "$N8N_COMPOSE_DIR" && $n8n_compose ps -q n8n 2>/dev/null)
  if [ -z "$n8n_state" ]; then
    send_alert n8n "Container n8n NAO esta rodando (profile automation). Calendario/Omni Chat parados. Checar: cd $N8N_COMPOSE_DIR && $n8n_compose ps n8n; logs n8n." critical
  else
    n8n_health=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$n8n_state" 2>/dev/null || echo unknown)
    if [ "$n8n_health" = "unhealthy" ]; then
      send_alert n8n "Container n8n UNHEALTHY. Checar: docker logs $n8n_state --tail=50." critical
    else
      # 8b) workflows criticos ativos? (so checa se o container respondeu)
      inactive=$(cd "$N8N_COMPOSE_DIR" && $n8n_compose exec -T n8n sh -lc \
        "n8n export:workflow --all --output=/tmp/obs7_wf.json >/dev/null 2>&1 && node -e '
          const fs=require(\"fs\");
          const want=process.env.WANT.split(/\s+/).filter(Boolean);
          let list=[]; try{const r=JSON.parse(fs.readFileSync(\"/tmp/obs7_wf.json\",\"utf8\"));list=Array.isArray(r)?r:(r.data||[]);}catch(e){process.exit(3);}
          const byId=Object.fromEntries(list.map(w=>[String(w.id),w]));
          const bad=want.filter(id=>!byId[id] || byId[id].active!==true);
          process.stdout.write(bad.join(\" \"));
        '" WANT="$N8N_CRITICAL_IDS" 2>/dev/null || echo "__EXPORT_FAIL__")
      if [ "$inactive" = "__EXPORT_FAIL__" ]; then
        send_alert n8n "Nao consegui ler os workflows do n8n (export falhou). Container up mas CLI/Node com problema? Checar: $n8n_compose exec n8n n8n export:workflow --all." warning
      elif [ -n "$inactive" ]; then
        send_alert n8n "Workflow(s) critico(s) INATIVO(s) no n8n: ${inactive}. Reativar: $n8n_compose exec n8n n8n update:workflow --id=<id> --active=true (ou re-deploy com -DeployAutomation)." warning
      fi
    fi
  fi
fi
```

> Nota de implementação: se `docker compose --profile automation ps` for pesado/lento no cron a cada 5min, o implementador pode trocar 8a por um `docker ps --filter` mais barato pelo nome do container — mas mantendo o `--env-file` para o 8b (o `exec` precisa resolver o serviço). Decidir na implementação medindo o tempo real na VPS.

### 4.3 EDITAR docs

- `scripts/monitoring/AGENT.md`: documentar o check #8 (o que alerta, severidades critical/warning, a lista de IDs críticos como contrato com o deploy, envs `N8N_*`).
- `docs/DEPLOY_VPS.md → Monitoração`: as envs novas no `.omni-monitoring.env` (na prática só se o path/porta divergir do default) + a observação de que a sonda roda como `deploy` (acesso docker).

## 5. Critérios de aceite

1. `bash -n scripts/monitoring/check-vps.sh` limpo.
2. Com `N8N_COMPOSE_DIR` ausente/inexistente → check #8 é NO-OP (não quebra a sonda em hosts sem automation).
3. Container n8n parado (`$n8n_compose stop n8n`) → `send_alert n8n ... critical` com a mensagem de container fora.
4. Container up + 1 workflow crítico despausado (`... update:workflow --id=calendarchat0001 --active=false`) → `send_alert n8n ... warning` listando **exatamente** `calendarchat0001`. Reativar → próxima execução não alerta.
5. Container up + todos ativos → check #8 silencioso.
6. Cooldown por chave `n8n` respeitado (2ª execução <1h não re-alerta).
7. A lista de IDs no script é idêntica à de `deploy-pull.ps1:246` (grep de conferência).

## 6. Validação

Local (dev, container `omni-n8n-1`) — provar o parse de `active` sem a infra de prod:

```bash
# lista atual + estados (deve casar com os IDs críticos):
docker exec omni-n8n-1 sh -lc "n8n export:workflow --all --output=/tmp/w.json >/dev/null 2>&1 && node -e 'const r=require(\"/tmp/w.json\");const l=Array.isArray(r)?r:(r.data||[]);for(const w of l)console.log(w.id,w.active)'"
bash -n scripts/monitoring/check-vps.sh
```

Na VPS (dono roda): scp do script, `grep AUTOMATION_N8N_PORT .env.production` + confirmar nome do serviço, rodar 1x manual e reproduzir os cenários 3/4/5.

## 7. Notas de Deploy

- **Migrations:** nenhuma. **Rebuild:** nenhum (é host-side/script). **Env vars:** opcionais no `.omni-monitoring.env` (só se path/porta divergirem do default).
- **Ordem:** subir DEPOIS de OBS-01 (o `send_alert` precisa já existir; sem OBS-01 esta spec não tem para onde rotear). Funciona antes de OBS-02, mas aí o alerta só cai nos canais diretos.
- **Rollback:** remover o bloco do check #8 (ou setar `N8N_COMPOSE_DIR` para um path inexistente) desliga a sonda.

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `scripts/monitoring/check-vps.sh` | editar (check #8 n8n, antes do exit 0) |
| `scripts/monitoring/AGENT.md` | editar |
| `docs/DEPLOY_VPS.md` | editar (§ Monitoração) |

**Conflitos potenciais:** OBS-01 (usa o `send_alert` novo — esta spec ASSUME OBS-01 já aplicado; se OBS-01 ainda não entrou, o `send_alert` atual sem severidade ainda funciona, só ignora o 3º arg). Contrato de IDs com `deploy-pull.ps1:246`. **Follow-up registrado:** auto-reativação do workflow caído (fora de escopo aqui) e vigilância de execuções com erro (encosta em OBS-04).
