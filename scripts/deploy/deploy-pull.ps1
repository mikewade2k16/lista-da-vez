param(
  [ValidateSet("staging", "prod")]
  [string]$Environment = "staging",
  # Tag da imagem no GHCR a subir. Vazio => "sha-<git HEAD>" (a tag imutavel que o
  # build-images.yml publica com type=sha,format=long). Tem que JA existir no GHCR
  # (ou seja, o CI precisa ter buildado esse SHA).
  [string]$Tag = "",
  [string]$VpsHost = "85.31.62.33",
  [int]$Port = 22,
  [string]$User = $(if ($env:OMNI_DEPLOY_USER) { $env:OMNI_DEPLOY_USER } else { "deploy" }),
  [string]$KeyPath = $(if ($env:OMNI_DEPLOY_KEY_PATH) { $env:OMNI_DEPLOY_KEY_PATH } else { Join-Path $HOME ".ssh/gh_actions_omnichannel_vps" }),
  [string]$ImageArchive = "",
  [switch]$UseLoadedImages,
  [switch]$BackupDatabase,
  [switch]$ForceRecreate,
  [switch]$SkipSmokeTests,
  # Sobe/reconcilia tambem o profile automation (redis/waha/n8n/whisper) e reimporta
  # os workflows versionados em automation/export/workflow-*.json quando houver mudanca.
  # Nao envia credentials.decrypted.json; credenciais continuam geridas no volume do n8n.
  [switch]$DeployAutomation,
  # Reconcilia o profile omnichannel (Evolution + banco proprio, volumes preservados).
  [switch]$DeployOmnichannel,
  [switch]$ForceAutomationWorkflowImport,
  # Credencial opcional do GHCR para o pull de imagens privadas. Quando informada,
  # vive somente em um DOCKER_CONFIG temporario remoto e e removida ao final do pull.
  [string]$GhcrUser = "",
  [string]$GhcrToken = "",
  # Fases para o deploy rapido intercalar trabalho remoto com o build local:
  # check   = segredos + recursos da VPS (nada muda);
  # prepare = snapshot para rollback + backup em segundo plano;
  # publish = imagens, espera do backup, Compose, IMAGE_TAG, rollout e smoke.
  # all (padrao) = as tres em sequencia (comportamento anterior).
  [ValidateSet("all", "check", "prepare", "publish")]
  [string]$Phase = "all",
  # Transporte por tunel (image-tunnel.ps1): digests no registry local 127.0.0.1:<porta>.
  # Se o tunel falhar, cai para o pacote SFTP das imagens locais da mesma tag.
  [int]$TunnelPort = 0,
  [string]$TunnelApiDigest = "",
  [string]$TunnelWebDigest = "",
  [ValidateSet("linux/amd64", "linux/arm64")]
  [string]$TunnelPlatform = "linux/amd64"
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
. (Join-Path $scriptDir 'deploy-timing.ps1')
$repoDir = (Resolve-Path (Join-Path $scriptDir "..\..")).Path
$composeLocal = Join-Path $repoDir "docker-compose.prod.yml"
$workflowLocalDir = Join-Path $repoDir "automation\export"
$composeEnvGuard = Join-Path $scriptDir "assert-compose-api-env.ps1"

if (-not (Test-Path $KeyPath)) { throw "Chave SSH nao encontrada em $KeyPath" }
if (-not (Test-Path $composeLocal)) { throw "docker-compose.prod.yml nao encontrado em $composeLocal" }
if (-not (Test-Path -LiteralPath $composeEnvGuard -PathType Leaf)) {
  throw "Preflight de env do Compose nao encontrado: $composeEnvGuard"
}
& $composeEnvGuard -ComposePath $composeLocal

$automationWorkflowFiles = @()
if ($DeployAutomation) {
  if (-not (Test-Path $workflowLocalDir)) { throw "Diretorio de workflows nao encontrado em $workflowLocalDir" }
  $automationWorkflowFiles = @(Get-ChildItem -Path $workflowLocalDir -Filter "workflow-*.json" -File | Sort-Object Name)
  if ($automationWorkflowFiles.Count -eq 0) { throw "Nenhum workflow-*.json encontrado em $workflowLocalDir" }
}

# Config por ambiente.
switch ($Environment) {
  "prod" {
    $envFile = ".env.production"
    $remotePath = "/home/deploy/omni"
    $publicBaseUrl = "https://omni.crowvisuals.com.br"
  }
  "staging" {
    $envFile = ".env.staging"
    $remotePath = "/home/deploy/lista-atendimento-staging"
    $publicBaseUrl = "https://preview.whenthelightsdie.com"
  }
}

# Resolve a tag: default = sha-<HEAD>.
if ([string]::IsNullOrWhiteSpace($Tag)) {
  $headSha = (& git -C $repoDir rev-parse HEAD).Trim()
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($headSha)) {
    throw "Nao consegui resolver o SHA do HEAD; passe -Tag explicitamente."
  }
  $Tag = "sha-$headSha"
  Write-Host "Tag nao informada; usando o HEAD local: $Tag"
  Write-Host "AVISO: confirme que o build-images.yml ja publicou essa tag no GHCR."
}

if ($Tag -notmatch '^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$' -or $Tag -eq 'latest') { throw 'Tag de release invalida.' }

$SshExe = (Get-Command ssh -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
$ScpExe = (Get-Command scp -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
foreach ($exe in @($SshExe, $ScpExe)) {
  if (-not (Test-Path $exe)) { throw "Executavel nao encontrado: $exe" }
}

$resolvedKeyPath = (Resolve-Path $KeyPath).Path
$sshHardening = @(
  "-o", "BatchMode=yes",
  "-o", "ConnectTimeout=15",
  "-o", "ServerAliveInterval=10",
  "-o", "ServerAliveCountMax=3"
)
# Reaproveita uma unica conexao SSH entre os ~15 comandos remotos (economiza o
# handshake de cada um). OpenSSH do Windows nao suporta ControlMaster.
if ($env:OS -ne 'Windows_NT') {
  $sshHardening += @("-o", "ControlMaster=auto", "-o", "ControlPath=$HOME/.ssh/omni-deploy-%C", "-o", "ControlPersist=180")
}
$sshArgs = @("-i", $resolvedKeyPath, "-o", "StrictHostKeyChecking=accept-new", "-p", $Port.ToString()) + $sshHardening
$scpBaseArgs = @("-i", $resolvedKeyPath, "-o", "StrictHostKeyChecking=accept-new", "-P", $Port.ToString()) + $sshHardening
$remoteTarget = "$User@$VpsHost"
$forceRecreateFlag = if ($ForceRecreate) { " --force-recreate" } else { "" }

function Convert-ToBashSingleQuoted {
  param([Parameter(Mandatory = $true)][string]$Value)
  $replacement = "'" + '"' + "'" + '"' + "'"
  return "'" + $Value.Replace("'", $replacement) + "'"
}

function Invoke-RemoteCommand {
  param(
    [Parameter(Mandatory = $true)][string]$Description,
    [Parameter(Mandatory = $true)][string]$Command,
    [switch]$CaptureOutput,
    # Vem ANTES de $sshArgs: no OpenSSH vale a primeira ocorrencia de cada opcao.
    [string[]]$ExtraSshArgs = @()
  )
  Write-Host "==> $Description"
  Invoke-DeployStep $Description {
    # Entrega o script pela STDIN do bash remoto, NAO como argumento do ssh.exe: passar
    # como argumento faz o PowerShell 5.1 comer as aspas duplas embutidas (o
    # `sed -i "s|...|...|"` e o `pg_dump -U "$POSTGRES_USER"` viram lixo no shell remoto).
    # Detalhe: ao fazer pipe pra stdin de um nativo, o PS 5.1 reconverte \n em \r\n, e o
    # \r quebra o bash (`cd <dir>\r` => "No such file or directory"). Por isso o remoto
    # roda `tr -d '\r' | bash -s` pra limpar o CR antes de executar.
    $timingFunctions = Get-Content -LiteralPath (Join-Path $scriptDir 'remote-timing.sh') -Raw
    $normalizedCommand = ($timingFunctions + "`n" + $Command) -replace "`r`n", "`n" -replace "`r", "`n"
    if ($CaptureOutput) {
      $output = $normalizedCommand | & $SshExe @ExtraSshArgs @sshArgs $remoteTarget "tr -d '\r' | bash -s"
      if ($LASTEXITCODE -ne 0) { throw "Falha ao executar: $Description" }
      return (($output | ForEach-Object { $_.ToString() }) -join "`n").Trim()
    }
    $normalizedCommand | & $SshExe @ExtraSshArgs @sshArgs $remoteTarget "tr -d '\r' | bash -s" | ForEach-Object {
      Write-DeployRemoteTiming -Line $_.ToString()
    }
    if ($LASTEXITCODE -ne 0) { throw "Falha ao executar: $Description" }
  }
}

$remotePathQ = Convert-ToBashSingleQuoted $remotePath
$envFileQ = Convert-ToBashSingleQuoted $envFile
$composeArgs = "--env-file $envFileQ -f docker-compose.prod.yml"
$composeAutomationArgs = "$composeArgs --profile automation"
$composeOmnichannelArgs = "$composeArgs --profile omnichannel"

Write-Host "Ambiente:     $Environment"
Write-Host "Host remoto:  ${remoteTarget}:$remotePath"
Write-Host "Imagem tag:   $Tag"
if ($DeployAutomation) {
  Write-Host "Automation:   redis/waha/n8n/whisper + $($automationWorkflowFiles.Count) workflow(s)"
}
if ($DeployOmnichannel) {
  Write-Host "Omnichannel:  evolution-db/evolution"
}

# 1. Falha ANTES de copiar compose/trocar tag se o segredo obrigatorio do boot
#    estiver ausente ou malformado. Nunca imprime o valor da chave.
$deployAutomationValue = if ($DeployAutomation) { "1" } else { "0" }
$deployOmnichannelValue = if ($DeployOmnichannel) { "1" } else { "0" }
$preflightCmd = @"
set -euo pipefail
mkdir -p $remotePathQ
cd $remotePathQ
if [ ! -f $envFileQ ]; then
  echo "ERRO: $envFile nao existe em $remotePath. Crie-o a partir do .example antes do deploy." >&2
  exit 1
fi
omni_key=`$(sed -n 's/^OMNI_SECRETS_KEY=//p' $envFileQ | tail -n 1)
if [ -z "`$omni_key" ]; then
  echo "ERRO: OMNI_SECRETS_KEY ausente/vazia em $envFile. Gere uma vez com: openssl rand -base64 32" >&2
  exit 1
fi
if ! printf '%s' "`$omni_key" | grep -Eq '^[A-Za-z0-9+/]{43}=$'; then
  echo "ERRO: OMNI_SECRETS_KEY nao e base64 canonico de 32 bytes em $envFile." >&2
  exit 1
fi
if ! decoded_bytes=`$(printf '%s' "`$omni_key" | openssl base64 -d -A 2>/dev/null | wc -c | tr -d ' '); then
  echo "ERRO: OMNI_SECRETS_KEY nao e base64 valido em $envFile." >&2
  exit 1
fi
unset omni_key
if [ "`$decoded_bytes" != "32" ]; then
  echo "ERRO: OMNI_SECRETS_KEY deve decodificar exatamente 32 bytes; encontrado(s): `$decoded_bytes." >&2
  exit 1
fi
r2_enabled=`$(sed -n 's/^R2_ENABLED=//p' $envFileQ | tail -n 1 | tr '[:upper:]' '[:lower:]')
case "`$r2_enabled" in
  true)
    for required_key in R2_ACCOUNT_ID R2_BUCKET R2_ACCESS_KEY_ID R2_SECRET_ACCESS_KEY R2_ANALYTICS_API_TOKEN; do
      if ! grep -q "^`$required_key=." $envFileQ; then
        echo "ERRO: `$required_key ausente/vazia em $envFile enquanto R2_ENABLED=true." >&2
        exit 1
      fi
    done
    ;;
  false|"")
    ;;
  *)
    echo "ERRO: R2_ENABLED deve ser true ou false em $envFile." >&2
    exit 1
    ;;
esac
unset r2_enabled
r2_allow_nonempty=`$(sed -n 's/^R2_ALLOW_NONEMPTY_BUCKET_INITIALIZATION=//p' $envFileQ | tail -n 1 | tr '[:upper:]' '[:lower:]')
case "`$r2_allow_nonempty" in
  true|false|"")
    ;;
  *)
    echo "ERRO: R2_ALLOW_NONEMPTY_BUCKET_INITIALIZATION deve ser true ou false em $envFile." >&2
    exit 1
    ;;
esac
unset r2_allow_nonempty
if [ "$deployAutomationValue" = "1" ]; then
  for required_key in AUTOMATION_REDIS_PASSWORD AUTOMATION_N8N_ENCRYPTION_KEY AUTOMATION_RUNTIME_TOKEN; do
    if ! grep -q "^`$required_key=." $envFileQ; then
      echo "ERRO: `$required_key ausente/vazia em $envFile para o profile automation." >&2
      exit 1
    fi
  done
fi
if [ "$deployOmnichannelValue" = "1" ]; then
  for required_key in EVOLUTION_API_KEY EVOLUTION_DB_PASSWORD; do
    if ! grep -q "^`$required_key=." $envFileQ; then
      echo "ERRO: `$required_key ausente/vazia em $envFile para o profile omnichannel." >&2
      exit 1
    fi
  done
fi
echo "Preflight de segredos obrigatorios: OK"
"@
$backupValue = if ($BackupDatabase) { "1" } else { "0" }
$resourcesCmd = @"
set -euo pipefail
cd $remotePathQ
api=`$(timeout 15 docker version --format '{{.Server.APIVersion}}') || { echo 'ERRO: Docker da VPS nao respondeu em 15s.' >&2; exit 1; }
api_major=`${api%%.*}
api_minor=`${api#*.}
if [ "`$api_major" -lt 1 ] || { [ "`$api_major" -eq 1 ] && [ "`$api_minor" -lt 44 ]; }; then
  echo "ERRO: Docker da VPS (API `$api) precisa ser 25+ (API 1.44) para os healthchecks com start_interval." >&2
  exit 1
fi
timeout 15 docker compose version >/dev/null || { echo 'ERRO: docker compose indisponivel na VPS.' >&2; exit 1; }
disk_kb=`$(df -Pk . | awk 'NR==2 {print `$4}')
mem_kb=`$(awk '/^MemAvailable:/ {print `$2}' /proc/meminfo)
load=`$(cut -d' ' -f1 /proc/loadavg)
cpus=`$(nproc)
printf 'VPS: Docker API %s | disco livre %s GB | memoria disponivel %s MB | load %s (%s CPUs)\n' "`$api" "`$((disk_kb / 1048576))" "`$((mem_kb / 1024))" "`$load" "`$cpus"
[ "`$disk_kb" -ge 3145728 ] || { echo 'ERRO: menos de 3 GB livres na VPS. Limpe imagens antigas (docker image prune) ou backups antes do deploy.' >&2; exit 1; }
[ "`$mem_kb" -ge 307200 ] || { echo 'ERRO: menos de 300 MB de memoria disponivel na VPS; o rollout pode derrubar servicos.' >&2; exit 1; }
[ "`$mem_kb" -ge 716800 ] || echo 'AVISO: memoria da VPS apertada (< 700 MB disponiveis).' >&2
awk -v l="`$load" -v c="`$cpus" 'BEGIN { exit !(l > c * 2) }' && echo 'AVISO: VPS com carga alta; o deploy pode demorar mais.' >&2
mkdir -p backups .deploy
test -w backups && test -w .deploy
if [ "$backupValue" = "1" ]; then
  if [ -z "`$(docker compose --env-file $envFileQ -f docker-compose.prod.yml ps -q --status running postgres)" ]; then
    echo 'ERRO: Postgres de producao nao esta rodando; o backup pre-deploy falharia.' >&2
    exit 1
  fi
fi
docker compose --env-file $envFileQ -f docker-compose.prod.yml ps --format '{{.Service}}: {{.State}} {{.Health}}' api web postgres || true
echo 'Preflight de recursos da VPS: OK'
"@

if ($Phase -in @('all', 'check')) {
  Invoke-RemoteCommand -Description "Validando ambiente remoto antes do deploy" -Command $preflightCmd
  Invoke-RemoteCommand -Description "Recursos e Docker da VPS" -Command $resourcesCmd
}
if ($Phase -eq 'check') { return }

$releaseDirQ = Convert-ToBashSingleQuoted ".deploy/$Tag"
$snapshotCmd = @"
set -euo pipefail
cd $remotePathQ
mkdir -p $releaseDirQ
chmod 700 $releaseDirQ
cp -p $envFileQ $releaseDirQ/env.before
cp -p docker-compose.prod.yml $releaseDirQ/compose.before.yml
chmod 600 $releaseDirQ/env.before
docker ps --format '{{.Names}} {{.Image}}' > $releaseDirQ/images.before.txt
available=`$(df -Pk . | awk 'NR==2 {print `$4}')
[ "`$available" -ge 3145728 ] || { echo 'Menos de 3 GB livres na VPS'; exit 1; }
echo 'Snapshot de configuracao e espaco remoto: OK'
"@
# Backup em segundo plano: comeca na fase prepare (antes do build local) e a fase
# publish espera o resultado antes de qualquer alteracao na stack.
$backupScript = (Get-Content -LiteralPath (Join-Path $scriptDir 'remote-backup.sh') -Raw).
  Replace('@REMOTE_PATH_RAW@', $remotePath).Replace('@REMOTE_PATH@', $remotePathQ).Replace('@RELEASE_DIR@', $releaseDirQ).
  Replace('@ENV_FILE@', $envFileQ).Replace('@TAG@', $Tag).Replace('@WITH_EVOLUTION@', $deployOmnichannelValue)
if ($backupScript -match 'OMNI_BACKUP_EOF') { throw 'Script de backup invalido.' }
$backupStartCmd = @"
set -euo pipefail
cd $remotePathQ
rm -f $releaseDirQ/backup.status $releaseDirQ/backup.log
cat > $releaseDirQ/backup.sh <<'OMNI_BACKUP_EOF'
$backupScript
OMNI_BACKUP_EOF
chmod 700 $releaseDirQ/backup.sh
nohup setsid bash $releaseDirQ/backup.sh > $releaseDirQ/backup.log 2>&1 < /dev/null &
echo "Backup iniciado em segundo plano (PID `$!)."
"@
$backupWaitCmd = @"
set -euo pipefail
cd $remotePathQ
status_file=$releaseDirQ/backup.status
[ -f $releaseDirQ/backup.sh ] || { echo 'ERRO: backup em segundo plano nao foi iniciado.' >&2; exit 1; }
for _ in `$(seq 1 1800); do
  [ -f "`$status_file" ] && break
  sleep 1
done
[ -f "`$status_file" ] || { echo 'ERRO: backup nao terminou em 30 min; deploy abortado sem alterar a stack.' >&2; exit 1; }
cat $releaseDirQ/backup.log
if [ "`$(cat "`$status_file")" != "0" ]; then
  echo 'ERRO: backup pre-deploy falhou; deploy abortado sem alterar a stack.' >&2
  exit 1
fi
"@

if ($Phase -in @('all', 'prepare')) {
  Invoke-RemoteCommand -Description 'Preservando configuracao anterior para rollback' -Command $snapshotCmd
  if ($BackupDatabase) {
    Invoke-RemoteCommand -Description "Iniciando backup do banco em segundo plano ($Environment)" -Command $backupStartCmd
  }
}
if ($Phase -eq 'prepare') { return }

$tunnelOk = $false
if ($TunnelPort -gt 0) {
  . (Join-Path $scriptDir 'image-tunnel.ps1')
  $tunnelCmd = "set -euo pipefail`nomni_timed_step remote.images.tunnel bash -c " + (Convert-ToBashSingleQuoted (Get-DeployTunnelPullScript -Port $TunnelPort -Tag $Tag -ApiDigest $TunnelApiDigest -WebDigest $TunnelWebDigest -Platform $TunnelPlatform))
  $tunnelArgs = @("-o", "ControlPath=none", "-o", "ExitOnForwardFailure=yes", "-R", "127.0.0.1:${TunnelPort}:127.0.0.1:${TunnelPort}")
  try {
    Invoke-RemoteCommand -Description 'Imagens pelo tunel SSH (so camadas novas)' -Command $tunnelCmd -ExtraSshArgs $tunnelArgs
    $tunnelOk = $true
  } catch {
    Write-Warning "Tunel de imagens falhou ($($_.Exception.Message)); usando o pacote SFTP."
    . (Join-Path $scriptDir 'image-archive.ps1')
    $ImageArchive = Export-DeployImageArchive -Tag $Tag -Images @("ghcr.io/mikewade2k16/omni-api:$Tag", "ghcr.io/mikewade2k16/omni-web:$Tag")
  }
}
if ($ImageArchive -and -not $tunnelOk) {
  . (Join-Path $scriptDir 'image-archive.ps1')
  Send-DeployImageArchive -Archive $ImageArchive -Tag $Tag -RemoteTarget $remoteTarget -RemotePath $remotePath -SftpArguments $scpBaseArgs
}

# 5. Backup (iniciado na fase prepare) precisa ter terminado com sucesso.
if ($BackupDatabase) {
  Invoke-RemoteCommand -Description "Aguardando backup do banco ($Environment)" -Command $backupWaitCmd
}

# 2. Envia o compose (a VPS so precisa do compose + .env; o codigo vive nas
#    imagens). O preflight acima garante que um erro de env nao altere nem o compose remoto.

Write-Host "==> Enviando docker-compose.prod.yml"
$scpArgs = $scpBaseArgs + @($composeLocal, "${remoteTarget}:${remotePath}/.deploy/$Tag/docker-compose.next.yml")
Invoke-DeployStep 'Envio do Compose por SCP' {
  & $ScpExe @scpArgs
  if ($LASTEXITCODE -ne 0) { throw "Falha ao enviar o docker-compose.prod.yml (scp)." }
  # Caddy central (servico caddy): o Caddyfile versionado vai junto e e aplicado
  # com reload abaixo, sem derrubar conexoes.
  $caddyLocal = Join-Path $repoDir 'deploy/caddy/Caddyfile'
  if (Test-Path -LiteralPath $caddyLocal -PathType Leaf) {
    $caddyArgs = $scpBaseArgs + @($caddyLocal, "${remoteTarget}:${remotePath}/.deploy/$Tag/Caddyfile")
    & $ScpExe @caddyArgs
    if ($LASTEXITCODE -ne 0) { throw "Falha ao enviar o deploy/caddy/Caddyfile (scp)." }
  }
}

# 3. Valida o compose novo contra o env real e so entao grava o IMAGE_TAG escolhido.
$tagQ = Convert-ToBashSingleQuoted $Tag
$setTagCmd = @"
set -euo pipefail
cd $remotePathQ
docker compose --env-file $envFileQ -f $releaseDirQ/docker-compose.next.yml config --quiet
cp $releaseDirQ/docker-compose.next.yml docker-compose.prod.yml
if [ -f $releaseDirQ/Caddyfile ]; then
  mkdir -p deploy/caddy
  if ! cmp -s $releaseDirQ/Caddyfile deploy/caddy/Caddyfile; then
    # cat preserva o inode do bind mount de arquivo unico (cp/mv quebraria o mount).
    if [ -f deploy/caddy/Caddyfile ]; then cat $releaseDirQ/Caddyfile > deploy/caddy/Caddyfile; else cp $releaseDirQ/Caddyfile deploy/caddy/Caddyfile; fi
    docker compose --env-file $envFileQ -f docker-compose.prod.yml exec -T caddy caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile || echo 'Aviso: caddy reload falhou; Caddyfile anterior segue ativo.' >&2
  fi
fi
if grep -q '^IMAGE_TAG=' $envFileQ; then
  sed -i "s|^IMAGE_TAG=.*|IMAGE_TAG=$Tag|" $envFileQ
else
  printf 'IMAGE_TAG=%s\n' $tagQ >> $envFileQ
fi
grep '^IMAGE_TAG=' $envFileQ
"@
Invoke-RemoteCommand -Description "Gravando IMAGE_TAG=$Tag em $envFile" -Command $setTagCmd

# 4. Auth opcional do GHCR e efemera: o config vive somente no comando de pull
# e o trap o remove mesmo se o registry/deploy falhar. Nunca persiste token na VPS.
$registryAuthSetup = ""
if (-not [string]::IsNullOrWhiteSpace($GhcrToken)) {
  if ([string]::IsNullOrWhiteSpace($GhcrUser)) { throw "-GhcrToken informado sem -GhcrUser." }
  $ghcrUserQ = Convert-ToBashSingleQuoted $GhcrUser
  $ghcrTokenQ = Convert-ToBashSingleQuoted $GhcrToken
  $registryAuthSetup = @"
ghcr_auth_dir=`$(mktemp -d)
trap 'rm -rf "`$ghcr_auth_dir"' EXIT
printf '%s' $ghcrTokenQ | DOCKER_CONFIG="`$ghcr_auth_dir" docker login ghcr.io -u $ghcrUserQ --password-stdin >/dev/null
export DOCKER_CONFIG="`$ghcr_auth_dir"
"@
}

# 6. Pull + up SEM build (a VPS nunca compila).
$imagePull = if ($ImageArchive -or $UseLoadedImages -or $tunnelOk) { "docker image inspect ghcr.io/mikewade2k16/omni-api:$Tag ghcr.io/mikewade2k16/omni-web:$Tag >/dev/null" } else { "docker compose $composeArgs pull api web" }
$deployCmd = Get-Content -LiteralPath (Join-Path $scriptDir 'rollout-app.sh') -Raw
$deployCmd = $deployCmd.Replace('@REMOTE_PATH@', $remotePathQ).Replace('@ENV_FILE@', $envFileQ).Replace('@RELEASE_DIR@', $releaseDirQ).Replace('@REGISTRY_AUTH@', $registryAuthSetup).Replace('@IMAGE_PULL_QUOTED@', (Convert-ToBashSingleQuoted $imagePull)).Replace('@FORCE_RECREATE@', $forceRecreateFlag).Replace('@PUBLIC_URL@', (Convert-ToBashSingleQuoted $publicBaseUrl))
Invoke-RemoteCommand -Description "Pull + up --no-build (api web) no $Environment" -Command $deployCmd

# 7. Omnichannel opcional: reconcilia Evolution e seu banco proprio. O up normal
# preserva os volumes; nunca usa down nem remove dados.
if ($DeployOmnichannel) {
  $omnichannelCmd = @"
set -euo pipefail
cd $remotePathQ
docker compose $composeOmnichannelArgs pull evolution-db evolution
docker compose $composeOmnichannelArgs up -d --no-build --wait --wait-timeout 180$forceRecreateFlag evolution-db evolution
docker compose $composeOmnichannelArgs ps evolution-db evolution
"@
  Invoke-RemoteCommand -Description "Profile omnichannel (Evolution) no $Environment" -Command $omnichannelCmd
}

# 8. Automation opcional: sobe profile automation e reimporta workflows versionados se mudaram.
if ($DeployAutomation) {
  $remoteWorkflowDir = "$remotePath/automation/export"
  $remoteWorkflowDirQ = Convert-ToBashSingleQuoted $remoteWorkflowDir
  Invoke-RemoteCommand -Description "Garantindo diretorio remoto dos workflows n8n" -Command "mkdir -p $remoteWorkflowDirQ"

  Write-Host "==> Enviando workflows n8n versionados (sem credenciais)"
  foreach ($workflowFile in $automationWorkflowFiles) {
    $remoteWorkflowPath = "${remoteTarget}:${remoteWorkflowDir}/$($workflowFile.Name)"
    $workflowScpArgs = $scpBaseArgs + @($workflowFile.FullName, $remoteWorkflowPath)
    & $ScpExe @workflowScpArgs
    if ($LASTEXITCODE -ne 0) { throw "Falha ao enviar workflow $($workflowFile.Name) (scp)." }
  }

  $forceAutomationImportValue = if ($ForceAutomationWorkflowImport) { "1" } else { "0" }
  $automationCmd = @"
set -euo pipefail
cd $remotePathQ
mkdir -p .deploy backups/n8n

docker compose $composeAutomationArgs pull redis waha n8n whisper
docker compose $composeAutomationArgs up -d --no-build$forceRecreateFlag redis waha n8n whisper
docker compose $composeAutomationArgs ps redis waha n8n whisper

manifest=`$(find automation/export -maxdepth 1 -type f -name 'workflow-*.json' -print0 | sort -z | xargs -0 sha256sum)
marker=.deploy/automation-workflows.sha256
changed=1
if [ "$forceAutomationImportValue" = "0" ] && [ -f "`$marker" ] && printf '%s\n' "`$manifest" | cmp -s "`$marker" -; then
  changed=0
fi

if [ "`$changed" = "0" ]; then
  echo "Workflows n8n sem mudanca; import pulado."
  exit 0
fi

backup_name="backups/n8n/workflows-before-automation-deploy_`$(date +%Y%m%d_%H%M%S).json"
active_ids_file=.deploy/n8n-active-workflows-before.txt
: > "`$active_ids_file"
# Este bash chega por stdin (`bash -s`). Todo `docker compose exec -T` precisa ler de
# /dev/null; caso contrario o processo dentro do container pode consumir silenciosamente
# o restante do script remoto e devolver exit 0 antes do import/verificacao.
if docker compose $composeAutomationArgs exec -T n8n n8n export:workflow --all --output=/tmp/workflows-before-automation-deploy.json </dev/null; then
  docker compose $composeAutomationArgs exec -T n8n node -e "const fs=require('fs'); const raw=JSON.parse(fs.readFileSync('/tmp/workflows-before-automation-deploy.json','utf8')); const list=Array.isArray(raw)?raw:(Array.isArray(raw.data)?raw.data:(Array.isArray(raw.workflows)?raw.workflows:[raw])); for (const w of list) { if (w && w.id && (w.active===true || w.active==='true' || w.isActive===true)) console.log(w.id); }" </dev/null > "`$active_ids_file" || true
  docker compose $composeAutomationArgs cp n8n:/tmp/workflows-before-automation-deploy.json "`$backup_name" || true
  echo "Backup workflows n8n: $remotePath/`$backup_name"
else
  echo "AVISO: nao consegui exportar backup dos workflows n8n; seguindo com import." >&2
fi

import_ok=1
for wf in automation/export/workflow-*.json; do
  base=`$(basename "`$wf")
  docker compose $composeAutomationArgs cp "`$wf" "n8n:/tmp/`$base"
  # O import tem de reportar "Successfully imported"; senao a versao nova NAO entrou
  # (import silencioso ja deixou a VPS defasada do arquivo por dias). Falhou -> nao
  # grava o marker (proximo deploy re-tenta em vez de "marcar como feito").
  import_out=`$(docker compose $composeAutomationArgs exec -T n8n n8n import:workflow --input="/tmp/`$base" </dev/null 2>&1)
  printf '%s\n' "`$import_out" | grep -qiE 'Successfully imported' || { echo "ERRO: import de `$base falhou:" >&2; printf '%s\n' "`$import_out" >&2; import_ok=0; }
done

# Mantem ativos os workflows internos que prod usa hoje e preserva qualquer workflow
# que ja estava ativo antes do import (ex.: WhatsApp, quando estiver em uso).
{
  printf '%s\n' calendaromni0001 calendarchat0001 calendartrans001 omnichatmvp00001
  cat "`$active_ids_file"
} | awk 'NF && !seen[`$0]++' | while IFS= read -r workflow_id; do
  docker compose $composeAutomationArgs exec -T n8n n8n update:workflow --id="`$workflow_id" --active=true </dev/null
done
docker compose $composeAutomationArgs restart n8n

# Verificacao pos-import: o n8n so re-le o workflow do banco depois do restart. Confere
# que cada workflow versionado REALMENTE ficou identico ao que esta no banco do n8n
# (via export:workflow, que aplica o WAL). Se algum divergir, o import nao pegou de fato
# (marker enganoso, WAL, id trocado) -> NAO grava o marker, para o proximo deploy re-tentar.
verify_ok=1
sleep 8
for wf in automation/export/workflow-*.json; do
  wid=`$(docker compose $composeAutomationArgs exec -T n8n node -e "console.log((JSON.parse(require('fs').readFileSync(process.argv[1],'utf8')).id)||'')" "/tmp/`$(basename "`$wf")" </dev/null 2>/dev/null | tr -d '\r')
  [ -z "`$wid" ] && continue
  if docker compose $composeAutomationArgs exec -T n8n n8n export:workflow --id="`$wid" --output="/tmp/verify_`$wid.json" </dev/null >/dev/null 2>&1; then
    # compara os 'nodes' normalizados (ordem estavel) entre arquivo e banco
    same=`$(docker compose $composeAutomationArgs exec -T n8n node -e "
      const fs=require('fs');
      const norm=o=>JSON.stringify(((Array.isArray(o)?o[0]:o).nodes||[]).map(n=>({name:n.name,type:n.type,parameters:n.parameters})).sort((a,b)=>a.name<b.name?-1:1));
      const a=JSON.parse(fs.readFileSync(process.argv[1],'utf8'));
      const b=JSON.parse(fs.readFileSync(process.argv[2],'utf8'));
      process.stdout.write(norm(a)===norm(b)?'SAME':'DIFF');
    " "/tmp/`$(basename "`$wf")" "/tmp/verify_`$wid.json" </dev/null 2>/dev/null)
    if [ "`$same" != "SAME" ]; then echo "ERRO: n8n `$wid diverge do arquivo apos import." >&2; verify_ok=0; fi
  else
    echo "AVISO: nao consegui verificar `$wid (export falhou)." >&2
  fi
done

if [ "`$import_ok" != "1" ] || [ "`$verify_ok" != "1" ]; then
  echo "ERRO: import/verificacao de workflows n8n NAO concluiu 100%. Marker nao gravado; rode o deploy de novo (ou com -ForceAutomationWorkflowImport)." >&2
  exit 1
fi

printf '%s\n' "`$manifest" > "`$marker"
echo "Workflows n8n importados, reiniciados e VERIFICADOS (banco == arquivo)."
"@
  Invoke-RemoteCommand -Description "Profile automation + import de workflows n8n no $Environment" -Command $automationCmd
}

# 9. Smoke tests publicos.
if (-not $SkipSmokeTests) {
  $smokeCmd = @"
set -euo pipefail
root=`$(curl -sS -o /dev/null -w "%{http_code}" "$publicBaseUrl")
health=`$(curl -sS -o /dev/null -w "%{http_code}" "$publicBaseUrl/healthz")
printf "GET %s => %s\n" "$publicBaseUrl" "`$root"
printf "GET %s/healthz => %s\n" "$publicBaseUrl" "`$health"
[ "`$root" = "200" ] && [ "`$health" = "200" ]
"@
  Invoke-RemoteCommand -Description "Smoke tests em $publicBaseUrl" -Command $smokeCmd
}

Write-Host ""
Invoke-RemoteCommand -Description 'Registrando deploy concluido' -Command "touch '$remotePath/.deploy/$Tag/deploy.success'"
Write-Host "Deploy ($Environment) finalizado: imagem $Tag no ar."
