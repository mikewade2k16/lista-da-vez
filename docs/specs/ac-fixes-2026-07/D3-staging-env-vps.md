# D3 — Criar o `.env.staging` real na VPS (staging sob demanda)

> Spec de implementação · Prioridade **P2** · Esforço **S** · Impacto **médio**
> Origem: plano `docs/deploy/REGISTRY_STAGING_DEPLOY_PLAN.md` fase D3 · roadmap `deploy-registry-staging` → task `dep-d3-staging-env`

## 1. Contexto

**O achado:** o pipeline de staging está pronto do lado do repo (`.env.staging.example` completo,
`scripts/deploy/staging-up.ps1`/`staging-down.ps1`/`promote.ps1`, `deploy-pull.ps1 -Environment
staging`), mas **o `.env.staging` real nunca foi criado na VPS** — o deploy aborta em
`deploy-pull.ps1:123-126` ("ERRO: .env.staging nao existe"). Sem staging, todo ensaio de release
acontece direto em produção (vide incidente AC-04 de 03/07).

Evidências:
- `.env.staging.example` (166 linhas) — todas as variáveis com placeholders `TROQUE-...`.
- `scripts/deploy/deploy-pull.ps1:118-126` — staging = env file `.env.staging`, path remoto
  `/home/deploy/lista-atendimento-staging`, exige o arquivo na VPS.
- `docs/deploy/REGISTRY_STAGING_DEPLOY_PLAN.md` §4.3/§6 — isolamento por
  `COMPOSE_PROJECT_NAME=omni-staging` (volumes/rede próprios) e decisão default de seed =
  **bootstrap limpo de teste, SEM PII real**.

## 2. Objetivo e não-objetivos

**Objetivo:** deixar o ciclo `staging-up → smoke → staging-down` funcionando na VPS, 100% isolado
de prod (volumes `omni-staging_*`, portas 18082/13005, aliases `lista-staging-*`).

**Não-objetivos (FORA):**
- NÃO configurar DNS/Caddy do domínio `preview.*` (staging é acessado via porta local/SSH tunnel
  nesta fase; o bloco Caddy é passo posterior do plano — registrar como pendência).
- NÃO restaurar dump de prod no staging (seed = bootstrap limpo; dump sanitizado é evolução futura).
- NÃO mexer em NENHUM arquivo/volume de prod.

## 3. Regras de execução (obrigatórias)

- NENHUM comando git. Ações na VPS via SSH (`deploy@85.31.62.33`, chave `gh_actions_omnichannel_vps`).
- Segredos do staging: gerar NOVOS (`openssl rand -hex 32`); NUNCA copiar os de prod; NUNCA inventar
  senha de usuário — o `BOOTSTRAP_OWNER_PASSWORD` de teste é definido PELO DONO (perguntar).
- A tag de imagem usada precisa existir no GHCR (CI verde para o SHA, ou tag `local-*` já publicada).
- Nunca parar o Caddy da omnichannel-mvp.

## 4. Mudanças (passo a passo)

### 4.1 Preparar diretório e env na VPS

```bash
ssh -i ~/.ssh/gh_actions_omnichannel_vps deploy@85.31.62.33
mkdir -p /home/deploy/lista-atendimento-staging/backups
# copiar o example a partir do repo local (da SUA máquina):
#   scp -i ~/.ssh/gh_actions_omnichannel_vps .env.staging.example deploy@85.31.62.33:/home/deploy/lista-atendimento-staging/.env.staging
```

Editar `/home/deploy/lista-atendimento-staging/.env.staging` preenchendo (checklist):

| Variável | Valor |
|---|---|
| `COMPOSE_PROJECT_NAME` | `omni-staging` (já vem do example — NÃO mudar) |
| `POSTGRES_PASSWORD` | `openssl rand -hex 32` (novo) |
| `APP_DB_ROLE` / `APP_DB_ROLE_PASSWORD` | `omni_app` / `openssl rand -hex 24` (ALFANUMÉRICA; novo) |
| `AUTH_TOKEN_SECRET` | `openssl rand -hex 32` (novo) |
| `AUTH_CONSULTANT_DEFAULT_PASSWORD` | definido pelo dono |
| `API_PORT` / `WEB_PORT` | `18082` / `13005` (conferir colisão: `docker ps --format '{{.Ports}}' \| grep -E '18082\|13005'`) |
| `IMAGE_TAG` | deixar vazio (o deploy grava) |
| demais `TROQUE-...` | valores próprios de staging |

`chmod 600 .env.staging`.

**Nota AC-04b:** se a imagem que vai subir já contém o AC-04b, a role `omni_app` do staging nasce
sozinha no primeiro boot. Se NÃO contém, rodar antes o passo 4 do runbook AC-04 (create-app-role.sql
no postgres do staging — que só existe após o primeiro `up` do serviço postgres; nesse caso: subir
só o postgres, criar a role, depois subir api/web).

### 4.2 Primeira subida (da sua máquina, PowerShell)

```powershell
# tag: um SHA que o build-images.yml já publicou (gh run list --workflow=build-images.yml)
powershell -ExecutionPolicy Bypass -File scripts/deploy/staging-up.ps1 -Tag sha-<40hex>
```

O script faz: scp do compose, grava `IMAGE_TAG`, `pull` + `up -d --no-build`, smoke público.
Como o domínio `preview.*` não tem Caddy ainda, o smoke público pode falhar — validar pelas portas
locais (passo 4.4) e usar `-SkipSmokeTests` se o script oferecer.

### 4.3 Bootstrap do owner de teste (SEM PII real)

```bash
cd /home/deploy/lista-atendimento-staging
docker compose --env-file .env.staging -f docker-compose.prod.yml run --rm \
  -e BOOTSTRAP_TENANT_SLUG=staging-teste -e BOOTSTRAP_TENANT_NAME='Staging Teste' \
  -e BOOTSTRAP_STORE_CODE=STG01 -e BOOTSTRAP_STORE_NAME='Loja Staging' -e BOOTSTRAP_STORE_CITY='Aracaju' \
  -e BOOTSTRAP_OWNER_NAME='Owner Staging' -e BOOTSTRAP_OWNER_EMAIL='staging@teste.local' \
  -e BOOTSTRAP_OWNER_PASSWORD='<definida pelo dono>' \
  api sh -lc 'migrate bootstrap-owner'
```

### 4.4 Smoke pelas portas locais + descida

```bash
curl -fsS http://127.0.0.1:18082/healthz          # api staging => 200
curl -fsS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:13005/   # web => 200
docker volume ls | grep omni-staging               # volumes namespaced existem
docker volume ls | grep -v staging | grep omni     # volumes de prod INTOCADOS (listaatendimento_*)
```

Depois: `powershell -File scripts/deploy/staging-down.ps1` (da sua máquina) e conferir que só os
containers `omni-staging-*` desceram.

### 4.5 EDITAR docs

- `docs/deploy/REGISTRY_STAGING_DEPLOY_PLAN.md`: marcar D3 como EXECUTADO (data), registrar a
  pendência "bloco Caddy do preview.* (DNS + basic_auth)" como próximo passo do staging.
- `docs/DEPLOY_VPS.md`: em Atalhos úteis, nota "staging exige `.env.staging` na VPS — D3 feito em <data>".

## 5. Critérios de aceite

1. `.env.staging` existe na VPS com chmod 600 e ZERO valores iguais aos de prod
   (`diff <(grep -E 'PASSWORD|SECRET' .env.staging) <(grep -E 'PASSWORD|SECRET' ../lista-atendimento/.env.production)` sem linhas comuns).
2. Ciclo completo up → healthz 200 (18082) + web 200 (13005) → login do owner de teste → down, sem erro.
3. `docker volume ls`: volumes `omni-staging_*` criados; nenhum volume de prod novo/alterado.
4. Prod continuou de pé o tempo todo (`curl -I https://omni.crowvisuals.com.br/healthz` = 200 antes/depois).
5. `migrate up` do staging aplicou as migrations no banco PRÓPRIO (log `migration_up_ok` no api staging).

## 6. Validação

Os passos 4.2–4.4 são a validação (execução real). Registrar as saídas (healthz, volume ls) na
seção "Estado" do README da rodada.

## 7. Notas de Deploy

- Nenhuma migration/env de PROD. Staging: env file novo SÓ na VPS (nunca no repo), volumes próprios.
- Custo de RAM: staging sobe SOB DEMANDA (subir → testar → descer; não deixar 24/7 na VPS de 7,8GB).
- Rollback: `staging-down.ps1` + (se quiser zerar) `docker volume rm $(docker volume ls -q | grep omni-staging)`.

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `/home/deploy/lista-atendimento-staging/.env.staging` (VPS) | criar (fora do repo) |
| `docs/deploy/REGISTRY_STAGING_DEPLOY_PLAN.md` | editar (D3 executado) |
| `docs/DEPLOY_VPS.md` | editar (nota) |

**Conflitos potenciais:** nenhum. Dependência favorável: AC-04b elimina o passo manual da role.
