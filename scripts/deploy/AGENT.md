# AGENTS

## Escopo

Estas instrucoes valem para `scripts/deploy`.

## Objetivo

Os scripts desta pasta sao a camada de entrada para deploy de producao deste repositorio.
Eles existem para reduzir erro manual e padronizar o fluxo real de deploy para a VPS auditada.

## Modelo de deploy atual — registry (GHCR) + pull (sem build na VPS)

Fonte de verdade: `docs/deploy/REGISTRY_STAGING_DEPLOY_PLAN.md`.

- O build de Go/Nuxt acontece no **GitHub Actions** (`.github/workflows/build-images.yml`),
  que publica `ghcr.io/mikewade2k16/omni-{api,web}:<sha>` no GHCR.
- A **VPS nunca compila**: o deploy so faz `docker compose pull` + `up -d --no-build`.
  O motivo e' a sobrecarga (o build do Nuxt pede 4GB de heap numa VPS de ~6GB).
- A VPS so precisa do `docker-compose.prod.yml` (enviado por `scp` a cada deploy) e do
  `.env.<ambiente>` (que ja vive na VPS; nunca e' sobrescrito pelos scripts).
- **Staging** roda na mesma VPS, projeto Compose separado (`omni-staging`), sob demanda.
- **Promocao** sobe pra producao a MESMA imagem (mesmo `IMAGE_TAG`/SHA) validada em staging;
  **rollback** = apontar `IMAGE_TAG` pro SHA anterior e refazer pull+up.

## Ambiente alvo atual

- host: `85.31.62.33`
- usuario SSH: `deploy`
- caminho remoto prod: `/home/deploy/lista-atendimento`
- caminho remoto staging: `/home/deploy/lista-atendimento-staging`
- dominio publico prod: `https://omni.crowvisuals.com.br`
- dominio publico staging: `https://preview.whenthelightsdie.com`
- proxy central: Caddy da stack `omnichannel-mvp`

## Regras do fluxo

- a VPS NAO builda imagem: deploy e' `pull` + `up -d --no-build` (build fica no CI/GHCR)
- o `IMAGE_TAG` deployado deve ser um SHA concreto que o CI ja publicou (nunca `latest` em prod)
- promover pra prod = usar a MESMA tag que passou por staging (`promote.ps1` le do `.env.staging`)
- os scripts nunca sobrescrevem o `.env.<ambiente>` remoto (so atualizam a linha `IMAGE_TAG`)
- antes de copiar compose ou trocar `IMAGE_TAG`, o deploy valida segredos obrigatorios no
  `.env.<ambiente>` sem imprimir valores; `OMNI_SECRETS_KEY` deve ser base64 de exatamente 32 bytes
- quando `R2_ENABLED=true`, o mesmo preflight remoto exige `R2_ACCOUNT_ID`, `R2_BUCKET`,
  `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY` e `R2_ANALYTICS_API_TOKEN` nao vazios; a validacao
  acontece antes de copiar o Compose, trocar `IMAGE_TAG`, fazer pull ou recriar containers
- `R2_ALLOW_NONEMPTY_BUCKET_INITIALIZATION` aceita somente `true` ou `false`; seu default seguro
  e `false` e `true` serve apenas para registrar uma instalacao nova em bucket dedicado ja populado
- antes de qualquer build/pull, `assert-compose-api-env.ps1` valida que o bloco `api.environment`
  do Compose repassa as variaveis criticas de boot, Evolution e R2; existir apenas no `.env` ou
  em outro servico nao basta
- os scripts nunca apagam volumes Docker de producao
- `staging-down.ps1` so remove volumes com `-RemoveVolumes` explicito
- smoke tests publicos continuam sendo a validacao padrao depois do deploy
- `deploy:fast:prod` reconcilia por padrao os profiles `automation` e `omnichannel`;
  workflows usam os JSONs versionados e a Evolution sobe com volumes preservados
- `assert-compose-service-parity.ps1` impede que um container novo exista somente no
  Compose local sem uma definicao prod ou excecao local-only documentada
- imagens privadas no GHCR: o deploy rapido reutiliza a credencial local apenas num
  `DOCKER_CONFIG` remoto temporario; nao persistir token na VPS

## Regras de implementacao

- preferir PowerShell como orquestrador local no Windows
- quando precisar empacotar o workspace, usar o wrapper de Git Bash ja existente em `../dev/git-bash.cmd`
- defaults de host, usuario, caminho remoto e URL publica podem ficar versionados porque ja foram auditados para este projeto
- qualquer novo parametro de deploy deve ser exposto como argumento simples do script principal
- mudancas que afetem banco, importacao de dados ou restore devem oferecer caminho claro para backup antes da execucao
- evitar adicionar comportamento automatico destrutivo sem opt-in explicito

## Scripts atuais

Fluxo RAPIDO (build local, sem git — recomendado pro dia a dia):

- `deploy-fast.ps1` — builda as imagens na maquina local (`docker build` de back/ e web/),
  faz `docker push` pro GHCR (so as camadas que mudaram sobem) e chama o `deploy-pull.ps1`.
  `-Environment staging|prod`, `-Tag` (default `local-<timestamp>`), `-Service both|api|web`,
  `-BackupDatabase`, `-WebBuildHeapMB`, `-GhcrUser`/`-GhcrToken`. Sem parametros explicitos,
  reutiliza com seguranca a credencial do Docker Desktop para o pull remoto temporario.
  No build web, pausa e restaura somente os servicos locais que ja estavam rodando para liberar
  memoria; `-KeepLocalStackRunning` e opt-out. Nao toca git, nao builda na VPS. npm:
  `deploy:fast` (staging), `deploy:fast:prod`. Em prod, o alias inclui Automation/workflows
  versionados e Omnichannel/Evolution por padrao.

Fluxo registry via CI (opcao completa/rastreavel):

- `assert-compose-api-env.ps1` — guarda fail-closed compartilhada por `deploy-fast.ps1`,
  `deploy-pull.ps1` e o workflow manual; impede recriar a API sem o wiring critico da Evolution.
- `assert-compose-service-parity.ps1` — compara todos os servicos/profiles local x prod e falha
  para container esquecido. Excecoes atuais: `crow-nuxt` (preview local) e
  `meta-ads-assistant` (ainda sem runtime OAuth prod-safe).
- `deploy-pull.ps1` — nucleo: `-Environment staging|prod`, `-Tag <sha>`, faz preflight remoto,
  envia/valida o compose, grava `IMAGE_TAG`, pull + `up --no-build`, smoke. Switches: `-BackupDatabase`,
  `-ForceRecreate`, `-SkipSmokeTests`, `-DeployAutomation`, `-DeployOmnichannel`,
  `-GhcrUser`/`-GhcrToken` (credencial opcional, efemera no pull remoto).
- `staging-up.ps1` — atalho: sobe staging sob demanda (wrapper de `deploy-pull.ps1 -Environment staging`).
- `staging-down.ps1` — derruba staging (preserva volumes; `-RemoveVolumes` zera o banco de staging).
- `promote.ps1` — le o `IMAGE_TAG` testado em staging e promove a MESMA imagem pra prod (backup por padrao).

Atalhos npm: `deploy:staging`, `deploy:staging:down`, `deploy:prod`, `deploy:promote`.

Legado (mantidos no repo como fallback de emergencia, SEM atalho npm — os shortcuts foram
removidos em 2026-06-23 na consolidacao dos docs). Buildam o Nuxt NA VPS (lento, risco de OOM):

- `deploy-vps-fast.ps1` — tar + `up -d --build` na VPS
- `deploy-vps-incremental.ps1` — so arquivos alterados por hash + `up -d --build` na VPS
- `deploy-ship.ps1` — build local + `docker save | ssh 'docker load'` (sem registry)

Rode via `.ps1` direto so se o GHCR estiver indisponivel. O caminho normal e' sempre GHCR.

## Validacao minima

Ao alterar scripts desta pasta:

1. validar a sintaxe do PowerShell quando houver mudanca em `.ps1`
   (ex.: `[System.Management.Automation.Language.Parser]::ParseFile(path, [ref]$null, [ref]$errors)`)
2. validar os workflows em `.github/workflows/build-images.yml` e `deploy-vps.yml` se a mudanca tocar o fluxo GitHub
3. garantir que os comandos documentados em `docs/DEPLOY_VPS.md` ainda correspondem ao comportamento real

## Referencias

- `../../docs/deploy/REGISTRY_STAGING_DEPLOY_PLAN.md` (plano canonico)
- `../../docs/deploy/STAGING_SETUP.md` (Caddy/DNS/up-down do staging)
- `../../docs/DEPLOY_VPS.md` (doc unico de deploy)
- `../../.github/workflows/build-images.yml`
- `../../.github/workflows/deploy-vps.yml`
- `../dev/AGENT.md`
- `../backup/AGENT.md` (backup agendado do Postgres — complementa o `-BackupDatabase` on-demand)
- `../../docs/BACKUP_RESTORE.md` (runbook de backup/restore)
