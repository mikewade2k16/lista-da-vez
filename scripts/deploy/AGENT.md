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
- caminho remoto prod: `/home/deploy/omni`
- caminho remoto staging: `/home/deploy/lista-atendimento-staging`
- dominio publico prod: `https://omni.crowvisuals.com.br`
- dominio publico staging: `https://preview.whenthelightsdie.com`
- proxy central: Caddy da stack `omnichannel-mvp`

## Regras do fluxo

- preservar chaves de API, tokens, senhas, cofres e referências ativas em deploys, migrations e restores; remover, substituir ou rotacionar somente mediante solicitação explícita do usuário para essa alteração, sem expor valores

- a VPS NAO builda imagem: recebe por pull GHCR ou docker load verificado e usa up --no-build
- o `IMAGE_TAG` deve ser concreto: SHA do CI ou local-<timestamp> do build local (nunca latest em prod)
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
- o deploy rapido valida primeiro se o Docker CLI existe e se o engine Linux responde;
  Docker Desktop parado falha imediatamente com mensagem acionavel, antes de avaliar os Composes
- os scripts nunca apagam volumes Docker de producao
- `staging-down.ps1` so remove volumes com `-RemoveVolumes` explicito
- smoke tests publicos continuam sendo a validacao padrao depois do deploy
- `npm run deploy` e o comando canonico do deploy rapido de producao; o alias
  `deploy:fast:prod` permanece por compatibilidade e ambos reconciliam por padrao
  o profile `omnichannel` (Automation exige opt-in explicito);
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

## Deploy verificado no Mac (2026-10-29)

validate-release.ps1 aplica lint/testes focados antes do build.
Antes do ESLint, executa `nuxt prepare` no web local e exige sucesso para evitar
consumir uma configuracao `.nuxt/eslint.config.mjs` incompleta ou desatualizada.
Pausa/restauracao usa Compose efetivo, incluindo local-access do Mac. Transporte auto seleciona GHCR com
credencial ou image-archive.ps1 (save/gzip/SFTP retomavel/SHA256/load). rollout-app.sh espera
healthchecks e HTTP; em erro restaura snapshot privado de env/Compose e imagens
anteriores. Backup pg_dump validado antes de trocar Compose/tag. Sem build na VPS.

## Scripts atuais

Mac/Linux (2026-10-29): atalhos npm passam por `run-deploy.mjs` (pwsh no Unix,
powershell.exe no Windows), sem shell intermediario. Ajuda e tratada pelo launcher
sem executar PowerShell/preflight/build. SSH/SCP resolvidos pelo PATH; helper Docker
recebe .exe so no Windows. `deploy-fast.ps1` usa linux/amd64 por padrao e verifica a
existencia da chave antes de buildar. Testes focados: `node --test scripts/deploy/run-deploy.test.mjs`
e parser PowerShell nos scripts alterados. Setup em `docs/SETUP_MAC.md`.

Acesso por maquina: launcher le `~/.config/omni/deploy.json` (sshUser/sshKeyPath)
e injeta OMNI_DEPLOY_USER/OMNI_DEPLOY_KEY_PATH so no processo filho; scripts usam
essas variaveis nos defaults. Flags explicitas vencem; sem configuracao mantem os
defaults anteriores. Arquivo e chave ficam fora do repo. Chave valida nao e prova de
permissao na VPS: validar autenticacao com comando remoto somente leitura, sem build.

Fluxo RAPIDO (build local, sem git — recomendado pro dia a dia):

- `deploy-fast.ps1` — builda as imagens na maquina local (`docker build` de back/ e web/),
  faz `docker push` pro GHCR (so as camadas que mudaram sobem) e chama o `deploy-pull.ps1`.
  `-Environment staging|prod`, `-Tag` (default `local-<timestamp>`), `-Service both|api|web`,
  `-BackupDatabase`, `-WebBuildHeapMB`, `-GhcrUser`/`-GhcrToken`. Sem parametros explicitos,
  reutiliza com seguranca a credencial do Docker Desktop para o pull remoto temporario.
  No build web, pausa e restaura somente os servicos locais que ja estavam rodando para liberar
  memoria; `-KeepLocalStackRunning` e opt-out. Nao toca git, nao builda na VPS. npm:
  `deploy:fast` (staging), `deploy` (producao canonica) e `deploy:fast:prod`
  (compatibilidade). Em prod, o alias inclui Omnichannel/Evolution; Automation e opt-in.

Fluxo registry via CI (opcao completa/rastreavel):

- `assert-compose-api-env.ps1` — guarda fail-closed compartilhada por `deploy-fast.ps1`,
  `deploy-pull.ps1` e o workflow manual; impede recriar a API sem o wiring critico da Evolution.
- `assert-compose-service-parity.ps1` — compara todos os servicos/profiles local x prod e falha
  para container esquecido. Excecoes atuais: `crow-nuxt` (preview local) e
  `meta-ads-assistant` (runner legado/read-only de compatibilidade interna,
  deliberadamente fora do produto e da produção).
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

`validate-release.ps1` constrói os pacotes Go usando o contexto `back/`. Os testes
do Calendar leem `automation/export/workflow-calendar-chat.json`; por isso, a etapa
de teste monta a pasta `automation/` do repositório como `/automation` somente para
`go test` e `go vet`. A pasta fica somente leitura e não entra na imagem de produção.

## Medicao de duracoes — 2026-11-12

O launcher grava somente eventos de tempo em `.tmp/deploy/<execucao>/timings.jsonl`
e o resultado em `summary.json` (ambos locais e ignorados pelo Git). Preservar o exit
code do filho, registrar falhas/interrupcoes e nunca gravar argumentos, env, tokens
ou comandos no relatorio. `deploy-timing.ps1` preserva stdout capturado (backup e
checksum) e os exit codes usados pelo retry SFTP. `remote-timing.sh` emite eventos
por etapa; `Invoke-RemoteCommand` encaminha esses metadados para o relatorio local.
Tempos de pais e filhos se sobrepoem e nao devem ser somados.

O preflight do engine usa `check-docker.mjs`: consulta `docker version` com limite
de 5 s, mata somente o CLI da verificacao em timeout e exige servidor Linux.
Nunca usar `docker info *> $null` sem limite: em 2026-11-12 ficou bloqueado por
mais de seis minutos depois do SSH OK. Engine, env e paridade Compose devem ter
etapas visiveis separadas. Esse limite nao reinicia Docker Desktop nem containers. A verificacao deve ser
a primeira acao do deploy rapido, antes de chave SSH, conexao remota e builds.

Nesta etapa somente a instrumentacao foi adicionada: sem otimizacoes de build,
cache, transporte, sincronizacao ou selecao de servicos. Primeiro medir o deploy
manual do usuario, depois otimizar com base no relatorio. Nao executar deploy por
conta propria quando o usuario decidir faze-lo manualmente.

Validar com `node --test scripts/deploy/run-deploy.test.mjs`,
`pwsh -NoProfile -File scripts/deploy/deploy-timing.test.ps1` e
`pwsh -NoProfile -File scripts/deploy/image-archive.test.ps1`. Os testes simulam
falhas, interrupcao, transporte e rollback sem acessar Docker nem VPS.

## Referencias

- `../../docs/deploy/REGISTRY_STAGING_DEPLOY_PLAN.md` (plano canonico)
- `../../docs/deploy/STAGING_SETUP.md` (Caddy/DNS/up-down do staging)
- `../../docs/DEPLOY_VPS.md` (doc unico de deploy)
- `../../.github/workflows/build-images.yml`
- `../../.github/workflows/deploy-vps.yml`
- `../dev/AGENT.md`
- `../backup/AGENT.md` (backup agendado do Postgres — complementa o `-BackupDatabase` on-demand)
- `../../docs/BACKUP_RESTORE.md` (runbook de backup/restore)

## Sincronização de arquivos novos — 2026-11-04

validate-release.ps1 cria o diretório pai no web local antes de cada docker cp.
Isso permite validar componentes em pastas novas ainda ausentes na imagem, inclusive
com espaços no caminho. Falha no mkdir interrompe a validação antes da cópia/lint.
A sincronização continua limitada aos fontes selecionados pelo script; não executa
deploy nem precisa reiniciar o container para criar esses diretórios.


## Transporte resiliente de imagens — 2026-11-04

`image-archive.ps1` cria/verifica o diretório remoto antes do envio. SFTP `reput`
retoma um `.part` identificado pela tag e pelo SHA256, com até três tentativas.
Somente após SHA256 válido renomeia o arquivo e executa `docker load`; falhas
preservam o arquivo local para retomar com `deploy-pull.ps1 -ImageArchive`, mesma
tag/ambiente/opções. O fluxo rápido exige cliente SFTP antes do build. Não repetir
rollout automaticamente nem afirmar que testes simulados validam a rede da VPS.
Teste sem rede: `pwsh -NoProfile -File scripts/deploy/image-archive.test.ps1`.

## Validacao com web local

`validate-release.ps1` exige um unico container web ativo antes de sincronizar
fontes ou executar testes do cardapio. Ausencia e falha de consulta do Docker
geram mensagens acionaveis; nenhuma etapa de validacao e pulada. Os comandos
de validacao usam o ID do container localizado. No Mac, `npm run dev:mac:up`
inicia a stack e o watch; aguardar o web antes de tentar novamente.

O transporte cria o arquivo parcial com `touch` antes de `reput`: OpenSSH exige
que o destino exista mesmo no primeiro envio. `touch` preserva dados parciais
existentes para retomada; SHA256 continua obrigatorio antes do docker load.

A validacao do cardapio sincroniza tambem os cinco arquivos de teste executados
com os fontes atuais, evitando rodar testes antigos da imagem contra codigo novo.
Os testes permanecem fora da lista de lint focado.

## Build rapido — 2026-11-12

Medido no deploy de 12/11: web 6min28s, API 1min23s, go test+vet 29s.

- Web (`-WebBuildMode native`, padrao): `build-web-native.mjs` compila o Nuxt no
  host em `.tmp/web-build` (isolado do `.nuxt`/`.output` do dev) e
  `web/Dockerfile.prebuilt` empacota so o `.output` (~4 s). Medido: ~1,5 min
  (ate ~2,5 min com o host carregado). Nao pausa a stack local.
  Garantias: Node do host igual ao `FROM` do Dockerfile.prebuilt (falha se
  diferente); `npm ci` pelo `package-lock` quando lock/Node/plataforma mudam;
  exclui `.env*` como o `.dockerignore`; remove `NUXT_/NITRO_/VITE_/NODE_` do
  ambiente; o smoke da imagem continua recusando modulos `.node` e exigindo
  HTTP 200 em linux/amd64. Manter a base do Dockerfile.prebuilt igual ao stage
  final de `web/Dockerfile`.
- Fallback: `-WebBuildMode docker` usa o build completo antigo (com pausa da
  stack). O GitHub Actions continua usando `web/Dockerfile`.
- API: `back/Dockerfile` usa cache mount do BuildKit (`omni-go-build`) para o
  `go build`; medido 24 s frio e ~3 s incremental. `.gomodcache` saiu do contexto.
- Validacao Go: `go test -count=1` e `go vet` usam o volume `omni_deploy_gobuild`
  (so cache de compilacao; pode ser removido sem perda — o proximo run recompila).
  `-count=1` mantem os testes sempre executados. Medido ~2 s cada com cache quente.

## Preflights, tunel e backup assincrono — 2026-11-12

Ordem do `deploy-fast.ps1`: preflight Docker -> `preflight-resources.mjs` (memoria
real do host via vm_stat/MemAvailable, disco do host e da VM Docker, Node da imagem,
porta do tunel) -> SSH -> teste `ssh -R` -> `deploy-pull -Phase check` (segredos +
Docker API >= 1.44, Compose, disco, memoria, carga, Postgres rodando) -> validacao
-> `deploy-pull -Phase prepare` (snapshot + backup em segundo plano) -> builds ->
transporte -> `deploy-pull -Phase publish`. `-Phase all` (padrao) mantem o fluxo
unico para `deploy:prod`/staging.

- Memoria do host (decisao do usuario em 2026-11-25): o build web nativo so e bloqueado
  abaixo de 2,5 GB livres. Entre 2,5 GB e o pico (~4,5 GB) o preflight apenas avisa que
  havera swap. `OMNI_WEB_BUILD_MIN_FREE_MEM_GB` continua sobrescrevendo o minimo.

- Trava: `run-deploy.mjs` cria `.tmp/deploy/deploy.lock` com o PID; PID morto = trava
  abandonada substituida. Nao existe trava remota entre maquinas diferentes.
- Tunel (`image-tunnel.ps1`): container `omni-deploy-registry` (registry:2.8.3, so em
  127.0.0.1:5055, volume `omni_deploy_registry`, GC + stop ao final). A VPS faz pull
  por digest via `ssh -R` (loopback dos dois lados) e retaga para `ghcr.io/...:<tag>`.
  O primeiro deploy pelo tunel ainda baixa todas as camadas (a VPS nao tem o mapeamento
  digest->camada de imagens vindas de `docker load`); os seguintes so as novas.
  Qualquer falha do tunel cai para o pacote SFTP no mesmo deploy.
- Backup (`remote-backup.sh`): iniciado com `nohup setsid` na fase prepare; dumps em
  paralelo com `pigz -1`/`gzip -1`, `.part` + `gzip -t` + rename + `.sha256`.
  A fase publish espera `backup.status` = 0 (limite 30 min) antes de enviar Compose ou
  trocar IMAGE_TAG; falha aborta sem tocar na stack. O ponto de restauracao fica ~2 min
  antes do rollout (antes era ~1 min).
- SSH multiplexado (`ControlMaster`/`ControlPersist=180`, exceto Windows). Opcoes extras
  do `Invoke-RemoteCommand` vem antes das padrao (OpenSSH usa a primeira ocorrencia).
- Healthchecks api/web com `start_interval: 2s` (Docker 25+): o `--wait` libera assim
  que o servico responde.
- Lookups de ssh/scp/sftp usam `Select-Object -First 1` (varios executaveis no PATH).

Validacao sem VPS: testes do launcher (trava), `bash -n` dos scripts remotos gerados por
`ssh` falso, backup com `docker` simulado (sucesso/falhas), tunel e ControlMaster contra
um sshd descartavel em container. O primeiro deploy real ainda precisa ser observado.

## Servico obs-agent (observabilidade, fase 6)

- `obs-agent` existe nos tres Composes (dev, mac-local, prod) com a MESMA imagem da API
  (`command: ["obs-agent"]`; binario compilado no `back/Dockerfile` e `compose.mac-api.Dockerfile`).
- Monta `/var/run/docker.sock` e `/` do host, ambos `:ro` (so leitura de metricas; ver
  `back/internal/obsagent/AGENT.md` para o risco do socket). Sem portas publicas.
- `rollout-app.sh` (deploy rapido/pull) e `deploy-ship.ps1` sobem o `obs-agent` DEPOIS dos
  health checks de api/web, com `|| aviso`: falha do agente nunca bloqueia nem reverte o
  deploy. O rollback continua restaurando so `api web`.
- Grava com `DATABASE_APP_URL`; os grants da role `omni_app` no schema `obs` vem do
  `SyncAppRoleGrants` no boot da API (que roda a migration 0330 antes).

- `OBS_AGENT_TLS_HOSTS` (producao): lista de dominios cujo certificado o agente confere a
  cada 6 h. Padrao no `docker-compose.prod.yml`: omni, n8n, waha, mostarda, bari, cardapio
  e aperolajoias.thecrow. Dominio novo no Caddy: incluir aqui (ou sobrescrever a variavel
  no `.env.production`). Mudar a variavel exige recriar so o `obs-agent`.

## Historico de deploys (observabilidade, fase 6)

`deploy-record.mjs` converte o `summary.json` em uma linha de `obs.deploys` (migration
0330) e grava por upsert (id = nome da pasta em `.tmp/deploy`). Status: exit code 0 =
success, diferente = failed, sem `finishedAt` = interrupted. Ambiente vem do comando
(`fast-prod`/`prod`/`promote` = prod; `fast-staging`/`staging` = staging). Git sha/branch
sao lidos com `git rev-parse` no momento da gravacao; a tag vem da etapa
"Gravando IMAGE_TAG=...".

- Automatico: `run-deploy.mjs` grava depois de `report.finish`, no banco do ambiente alvo,
  com timeout de 20 s (`OBS_DEPLOY_RECORD_TIMEOUT_MS`). Qualquer erro vira so
  `[obs] histórico de deploy não gravado: <motivo>`; o exit code do deploy nunca muda.
  `OBS_DEPLOY_RECORD=0` desliga.
- Transporte: o SQL (upsert com `jsonb_populate_recordset`, JSON como literal
  dollar-quoted, sem concatenar valores) vai pela stdin. Local: `docker exec -i
  omni-postgres-1 psql`. Prod/staging: mesmo SSH do deploy (usuario/chave de
  `OMNI_DEPLOY_USER`/`OMNI_DEPLOY_KEY_PATH`, sem criar ControlMaster), `docker compose
  exec -T postgres psql` no diretorio remoto; `POSTGRES_USER/DB` vem do proprio container,
  nenhum segredo passa pela maquina local nem e impresso.
- Importacao dos deploys antigos: `npm run obs:deploys` (lista) e
  `npm run obs:deploys -- --sync [--target=local|prod|staging]`.
- Teste: `node --test scripts/deploy/deploy-record.test.mjs`.
- `OBS_AGENT_TLS_HOSTS` (2026-11-24): n8n./waha. fora da lista padrão: `n8n.crowvisuals.com.br` e `waha.crowvisuals.com.br` respondem erro TLS (`tlsv1 alert internal error`), ou seja, o Caddy não serve certificado para eles. Reincluir quando estiverem publicados.

## Caddy central e projeto `omni` na VPS (2026-11-25)

- O Caddy e o servico `caddy` (profile `edge`) do `docker-compose.prod.yml`; configuracao em
  `deploy/caddy/Caddyfile`. O `deploy-pull.ps1` envia o Caddyfile com o compose e faz `caddy reload`
  quando ele muda (falha no reload so gera aviso).
- Projeto Docker de producao: `omni` (`COMPOSE_PROJECT_NAME`), pasta `/home/deploy/omni`
  (`lista-atendimento` virou link). Volumes de dados com prefixo `COMPOSE_VOLUME_PREFIX=listaatendimento`.
- Rede publica `edge`: omni (api/web/caddy), `crow-web` e `tavola-web`.
