param(
  [ValidateSet("staging", "prod")]
  [string]$Environment = "staging",
  # Tag da imagem no GHCR a subir. Vazio => "sha-<git HEAD>" (a tag imutavel que o
  # build-images.yml publica com type=sha,format=long). Tem que JA existir no GHCR
  # (ou seja, o CI precisa ter buildado esse SHA).
  [string]$Tag = "",
  [string]$VpsHost = "85.31.62.33",
  [int]$Port = 22,
  [string]$User = "deploy",
  [string]$KeyPath = (Join-Path $HOME ".ssh\gh_actions_omnichannel_vps"),
  [switch]$BackupDatabase,
  [switch]$ForceRecreate,
  [switch]$SkipSmokeTests,
  # Login opcional no GHCR antes do pull (imagens privadas). Se vazios, assume que
  # a VPS ja tem `docker login ghcr.io` valido (~/.docker/config.json).
  [string]$GhcrUser = "",
  [string]$GhcrToken = ""
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoDir = (Resolve-Path (Join-Path $scriptDir "..\..")).Path
$composeLocal = Join-Path $repoDir "docker-compose.prod.yml"

if (-not (Test-Path $KeyPath)) { throw "Chave SSH nao encontrada em $KeyPath" }
if (-not (Test-Path $composeLocal)) { throw "docker-compose.prod.yml nao encontrado em $composeLocal" }

# Config por ambiente.
switch ($Environment) {
  "prod" {
    $envFile = ".env.production"
    $remotePath = "/home/deploy/lista-atendimento"
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

$SshExe = Join-Path $env:SystemRoot "System32\OpenSSH\ssh.exe"
$ScpExe = Join-Path $env:SystemRoot "System32\OpenSSH\scp.exe"
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
$sshArgs = @("-i", $resolvedKeyPath, "-o", "StrictHostKeyChecking=accept-new", "-p", $Port.ToString()) + $sshHardening
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
    [switch]$CaptureOutput
  )
  Write-Host "==> $Description"
  # Entrega o script pela STDIN do bash remoto, NAO como argumento do ssh.exe: passar
  # como argumento faz o PowerShell 5.1 comer as aspas duplas embutidas (o
  # `sed -i "s|...|...|"` e o `pg_dump -U "$POSTGRES_USER"` viram lixo no shell remoto).
  # Detalhe: ao fazer pipe pra stdin de um nativo, o PS 5.1 reconverte \n em \r\n, e o
  # \r quebra o bash (`cd <dir>\r` => "No such file or directory"). Por isso o remoto
  # roda `tr -d '\r' | bash -s` pra limpar o CR antes de executar.
  $normalizedCommand = $Command -replace "`r`n", "`n" -replace "`r", "`n"
  if ($CaptureOutput) {
    $output = $normalizedCommand | & $SshExe @sshArgs $remoteTarget "tr -d '\r' | bash -s"
    if ($LASTEXITCODE -ne 0) { throw "Falha ao executar: $Description" }
    return (($output | ForEach-Object { $_.ToString() }) -join "`n").Trim()
  }
  $normalizedCommand | & $SshExe @sshArgs $remoteTarget "tr -d '\r' | bash -s"
  if ($LASTEXITCODE -ne 0) { throw "Falha ao executar: $Description" }
}

$remotePathQ = Convert-ToBashSingleQuoted $remotePath
$envFileQ = Convert-ToBashSingleQuoted $envFile
$composeArgs = "--env-file $envFileQ -f docker-compose.prod.yml"

Write-Host "Ambiente:     $Environment"
Write-Host "Host remoto:  ${remoteTarget}:$remotePath"
Write-Host "Imagem tag:   $Tag"

# 1. Garante o diretorio remoto e envia o compose (a VPS so precisa do compose +
#    .env; o codigo vive nas imagens). NAO sobrescreve o .env remoto.
Invoke-RemoteCommand -Description "Garantindo diretorio remoto" -Command "mkdir -p $remotePathQ"

Write-Host "==> Enviando docker-compose.prod.yml"
$scpArgs = @("-i", $resolvedKeyPath, "-o", "StrictHostKeyChecking=accept-new", "-P", $Port.ToString()) + $sshHardening + @($composeLocal, "${remoteTarget}:${remotePath}/docker-compose.prod.yml")
& $ScpExe @scpArgs
if ($LASTEXITCODE -ne 0) { throw "Falha ao enviar o docker-compose.prod.yml (scp)." }

# 2. Confirma que o .env do ambiente existe e grava o IMAGE_TAG escolhido.
$tagQ = Convert-ToBashSingleQuoted $Tag
$setTagCmd = @"
set -euo pipefail
cd $remotePathQ
if [ ! -f $envFileQ ]; then
  echo "ERRO: $envFile nao existe em $remotePath. Crie-o a partir do .example antes do deploy." >&2
  exit 1
fi
if grep -q '^IMAGE_TAG=' $envFileQ; then
  sed -i "s|^IMAGE_TAG=.*|IMAGE_TAG=$Tag|" $envFileQ
else
  printf 'IMAGE_TAG=%s\n' $tagQ >> $envFileQ
fi
grep '^IMAGE_TAG=' $envFileQ
"@
Invoke-RemoteCommand -Description "Gravando IMAGE_TAG=$Tag em $envFile" -Command $setTagCmd

# 3. Login opcional no GHCR (imagens privadas).
if (-not [string]::IsNullOrWhiteSpace($GhcrToken)) {
  if ([string]::IsNullOrWhiteSpace($GhcrUser)) { throw "-GhcrToken informado sem -GhcrUser." }
  $loginCmd = "printf '%s' " + (Convert-ToBashSingleQuoted $GhcrToken) + " | docker login ghcr.io -u " + (Convert-ToBashSingleQuoted $GhcrUser) + " --password-stdin"
  Invoke-RemoteCommand -Description "docker login ghcr.io" -Command $loginCmd
}

# 4. Backup opcional do Postgres (antes de mexer nos containers).
if ($BackupDatabase) {
  $backupCmd = @"
set -euo pipefail
cd $remotePathQ
mkdir -p backups
docker compose $composeArgs exec -T postgres sh -lc 'pg_dump -U "`$POSTGRES_USER" -d "`$POSTGRES_DB"' | gzip > "backups/backup_`$(date +%Y%m%d_%H%M%S).sql.gz"
latest=`$(ls -t backups | head -n 1)
printf '%s\n' "$remotePath/backups/`$latest"
"@
  $backupFile = Invoke-RemoteCommand -Description "Backup remoto do Postgres ($Environment)" -Command $backupCmd -CaptureOutput
  if ($backupFile) { Write-Host "Backup remoto: $backupFile" }
}

# 5. Pull + up SEM build (a VPS nunca compila).
$deployCmd = @"
set -euo pipefail
cd $remotePathQ
docker compose $composeArgs pull api web
docker compose $composeArgs up -d --no-build$forceRecreateFlag api web
docker compose $composeArgs ps api web
"@
Invoke-RemoteCommand -Description "Pull + up --no-build (api web) no $Environment" -Command $deployCmd

# 6. Smoke tests publicos.
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
Write-Host "Deploy ($Environment) finalizado: imagem $Tag no ar."
