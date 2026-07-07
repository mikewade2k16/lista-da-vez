# OBS-03 — Uptime externo no /healthz (UptimeRobot)

> Spec de implementação (runbook puro, zero código) · Prioridade **P1** · Esforço **S** · Impacto **alto**
> Origem: AC-16 (pendência declarada) · roadmap `observabilidade-n8n` → task `obs-03-uptime-externo-healthz`

## 1. Contexto

**O achado:** toda a monitoração atual roda DENTRO da VPS (check-vps.sh no cron + healthcheck do
compose). Se o host inteiro cair (rede, kernel, disco cheio travando o docker), ninguém é avisado —
exatamente o cenário em que o aviso mais importa. O esboço do monitor externo já está em
`docs/DEPLOY_VPS.md → Monitoração § 1` e nunca foi executado.

Fatos que o runbook usa:
- `GET https://omni.crowvisuals.com.br/healthz` devolve `200` com banco ok e `503` (`db:"unreachable"`)
  com banco fora (AC-16) — um monitor de STATUS simples cobre api E banco.
- O painel (`https://omni.crowvisuals.com.br/`) é servido pelo web via Caddy — monitor separado pega
  o caso "api ok, web caído".

## 2. Objetivo e não-objetivos

**Objetivo:** dois monitores externos (healthz + painel) com alerta em e-mail e Telegram, testados
com um downtime simulado, e o registro dos monitores na doc.

**Não-objetivos:** status page pública; monitor de staging; métricas/latência histórica (fica para
OBS-04/05); integração com o n8n (o UptimeRobot alerta direto — é a rede de segurança de FORA, não
deve depender da nossa stack).

## 3. Regras de execução

- Zero código/deploy. Conta criada/possuída PELO DONO (não criar conta em nome dele sem ok; se ele
  preferir BetterStack, os passos são equivalentes).
- NUNCA registrar tokens/API keys do UptimeRobot no repo — só nomes/URLs dos monitores.

## 4. Mudanças (passo a passo)

### 4.1 Criar os monitores (uptimerobot.com, plano free)

| Monitor | Tipo | URL | Intervalo | Confirmação |
|---|---|---|---|---|
| `omni-api-healthz` | HTTP(s) | `https://omni.crowvisuals.com.br/healthz` | 5 min | esperar status 200 (503/timeout = down) |
| `omni-painel` | HTTP(s) | `https://omni.crowvisuals.com.br/` | 5 min | status 200 |

### 4.2 Alert contacts

- E-mail do operador (o dono informa) — confirmar o opt-in que o UptimeRobot manda.
- Telegram: contato do tipo Telegram (bot do UptimeRobot) OU webhook para o ntfy topic já usado
  pelo check-vps (`ALERT_NTFY_URL`) — escolher UM para não duplicar ruído; recomendação: Telegram.

### 4.3 Teste real (obrigatório)

1. Pausar o monitor `omni-painel` → despausar (sanity da conta).
2. Downtime controlado de ~2min COM O DONO JUNTO: na VPS,
   `docker compose --env-file .env.production -f docker-compose.prod.yml stop web` → aguardar o
   alerta DOWN chegar (email/telegram) → `up -d web` → aguardar o UP. NUNCA parar postgres/api/Caddy
   para esse teste.
3. Registrar horário DOWN/UP recebidos.

### 4.4 EDITAR `docs/DEPLOY_VPS.md → Monitoração § 1`

Marcar como EXECUTADO (data), colar a tabela do 4.1 e o resultado do teste 4.3. Acrescentar: "o
alerta externo é independente da stack — se healthz E UptimeRobot ficarem mudos ao mesmo tempo,
suspeitar do DNS/Caddy, não do app".

## 5. Critérios de aceite

1. Dois monitores ativos com 5 min de intervalo e 2 contatos de alerta.
2. Teste 4.3 executado com alerta DOWN e UP recebidos (evidência = print/horários na doc).
3. `docs/DEPLOY_VPS.md` atualizado; nenhum segredo no repo.
4. 24h depois: uptime > 0 registrado sem falso-positivo (se houver falso-positivo, subir o
   confirmation time do monitor para 2 checks).

## 6. Validação

O próprio 4.3. Nada a compilar.

## 7. Notas de Deploy

Nenhuma migration/env/rebuild. Serviço externo independente da VPS (é o ponto).

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `docs/DEPLOY_VPS.md` | editar (§ Monitoração 1 → executado + tabela) |

**Conflitos potenciais:** nenhum. Complementa OBS-01/02 (dentro→fora); não depende deles.
