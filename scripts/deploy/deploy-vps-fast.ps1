param(
  [string]$VpsHost = "85.31.62.33",
  [int]$Port = 22,
  [string]$User = "deploy",
  [string]$KeyPath = (Join-Path $HOME ".ssh\gh_actions_omnichannel_vps"),
  [string]$RemotePath = "/home/deploy/lista-atendimento",
  [string]$PublicBaseUrl = "https://lista.whenthelightsdie.com",
  [string[]]$Services = @("api", "web"),
  [switch]$BackupDatabase,
  [switch]$SkipComposeConfig,
  [switch]$SkipSmokeTests,
  [switch]$ForceRecreate
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoDir = (Resolve-Path (Join-Path $scriptDir "..\..")).Path
$gitBashCmd = (Resolve-Path (Join-Path $repoDir "scripts\dev\git-bash.cmd")).Path

if (-not (Test-Path $gitBashCmd)) {
  throw "Git Bash wrapper nao encontrado em $gitBashCmd"
}

if (-not (Test-Path $KeyPath)) {
  throw "Chave SSH nao encontrada em $KeyPath"
}

$resolvedKeyPath = (Resolve-Path $KeyPath).Path
$sshArgs = @("-i", $resolvedKeyPath, "-o", "StrictHostKeyChecking=accept-new", "-p", $Port.ToString())
$remoteTarget = "$User@$VpsHost"
$serviceArgs = if ($Services.Count -gt 0) { " " + ($Services -join " ") } else { "" }
$forceRecreateFlag = if ($ForceRecreate) { " --force-recreate" } else { "" }

function Convert-ToGitBashPath {
  param(
    [Parameter(Mandatory = $true)]
    [string]$PathValue
  )

  $normalized = $PathValue.Replace("\", "/")
  if ($normalized.Length -ge 2 -and $normalized[1] -eq ':') {
    $drive = $normalized.Substring(0, 1).ToLowerInvariant()
    $tail = $normalized.Substring(2).TrimStart('/')
    return "/$drive/$tail"
  }

  return $normalized
}

function Convert-ToBashSingleQuoted {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Value
  )

  $replacement = "'" + '"' + "'" + '"' + "'"
  return "'" + $Value.Replace("'", $replacement) + "'"
}

function Invoke-RemoteCommand {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Description,
    [Parameter(Mandatory = $true)]
    [string]$Command,
    [switch]$CaptureOutput
  )

  Write-Host "==> $Description"

  $normalizedCommand = $Command -replace "`r`n", "`n" -replace "`r", "`n"

  if ($CaptureOutput) {
    $output = & ssh @sshArgs $remoteTarget $normalizedCommand
    if ($LASTEXITCODE -ne 0) {
      throw "Falha ao executar: $Description"
    }

    return (($output | ForEach-Object { $_.ToString() }) -join "`n").Trim()
  }

  & ssh @sshArgs $remoteTarget $normalizedCommand
  if ($LASTEXITCODE -ne 0) {
    throw "Falha ao executar: $Description"
  }
}

function Invoke-GitBashScript {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Description,
    [Parameter(Mandatory = $true)]
    [string]$ScriptBody
  )

  Write-Host "==> $Description"

  $tempScript = Join-Path $env:TEMP ("deploy-vps-" + [Guid]::NewGuid().ToString("N") + ".sh")
  try {
    Set-Content -Path $tempScript -Value $ScriptBody -Encoding Ascii
    $tempScriptBash = Convert-ToGitBashPath $tempScript

    & $gitBashCmd $tempScriptBash
    if ($LASTEXITCODE -ne 0) {
      throw "Falha ao executar em Git Bash: $Description"
    }
  } finally {
    if (Test-Path $tempScript) {
      Remove-Item $tempScript -Force
    }
  }
}

$backupFile = $null
if ($BackupDatabase) {
  $backupCommand = @'
mkdir -p "__REMOTE_PATH__/backups" &&
cd "__REMOTE_PATH__" &&
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  sh -lc 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB"' | gzip > \
  backups/backup_$(date +%Y%m%d_%H%M%S).sql.gz &&
latest=$(ls -t backups | head -n 1) &&
printf "%s\n" "__REMOTE_PATH__/backups/$latest"
'@
  $backupCommand = $backupCommand.Replace("__REMOTE_PATH__", $RemotePath)
  $backupFile = Invoke-RemoteCommand -Description "Gerando backup remoto do PostgreSQL" -Command $backupCommand -CaptureOutput
  if ($backupFile) {
    Write-Host "Backup remoto: $backupFile"
  }
}

$repoDirBash = Convert-ToGitBashPath $repoDir
$keyPathBash = Convert-ToGitBashPath $resolvedKeyPath
$remoteSyncCommand = @'
mkdir -p "__REMOTE_PATH__" &&
find "__REMOTE_PATH__" -mindepth 1 -maxdepth 1 ! -name '.env.production' ! -name 'backups' -exec rm -rf {} + &&
tar -xzf - -C "__REMOTE_PATH__"
'@
$remoteSyncCommand = $remoteSyncCommand.Replace("__REMOTE_PATH__", $RemotePath)

$syncScript = @"
#!/usr/bin/env bash
set -euo pipefail
cd $(Convert-ToBashSingleQuoted $repoDirBash)
tar -czf - \
    --exclude='.git' \
    --exclude='.claude' \
    --exclude='.playwright-mcp' \
    --exclude='.env' \
    --exclude='.env.production' \
    --exclude='token.txt' \
    --exclude='token_gen.js' \
    --exclude='verify.sh' \
    --exclude='node_modules' \
    --exclude='web/node_modules' \
    --exclude='web/.nuxt' \
    --exclude='web/.output' \
    --exclude='web/dist' \
    --exclude='web/.codex-devserver.*.log' \
    --exclude='back/.logs' \
    --exclude='qa-bot/.venv' \
    --exclude='qa-bot/artifacts' \
    --exclude='tmp' \
    . | ssh -i $(Convert-ToBashSingleQuoted $keyPathBash) \
    -o StrictHostKeyChecking=accept-new \
    -p $Port \
    $(Convert-ToBashSingleQuoted $remoteTarget) \
    $(Convert-ToBashSingleQuoted $remoteSyncCommand)
"@

Invoke-GitBashScript -Description "Sincronizando workspace para a VPS" -ScriptBody $syncScript

if (-not $SkipComposeConfig) {
  $configCommand = "cd '$RemotePath' && docker compose --env-file .env.production -f docker-compose.prod.yml config"
  Invoke-RemoteCommand -Description "Validando docker compose remoto" -Command $configCommand
}

$deployCommand = @'
cd "__REMOTE_PATH__" &&
docker compose --env-file .env.production -f docker-compose.prod.yml up -d --build__FORCE____SERVICES__ &&
docker compose --env-file .env.production -f docker-compose.prod.yml ps__SERVICES__
'@
$deployCommand = $deployCommand.Replace("__REMOTE_PATH__", $RemotePath)
$deployCommand = $deployCommand.Replace("__FORCE__", $forceRecreateFlag)
$deployCommand = $deployCommand.Replace("__SERVICES__", $serviceArgs)
Invoke-RemoteCommand -Description "Subindo servicos na VPS" -Command $deployCommand

if (-not $SkipSmokeTests) {
  $smokeCommand = @'
root_status=$(curl -sS -o /dev/null -w "%{http_code}" "__PUBLIC_BASE_URL__") &&
health_status=$(curl -sS -o /dev/null -w "%{http_code}" "__PUBLIC_BASE_URL__/healthz") &&
printf "GET __PUBLIC_BASE_URL__ => %s\n" "$root_status" &&
printf "GET __PUBLIC_BASE_URL__/healthz => %s\n" "$health_status" &&
[[ "$root_status" == "200" ]] &&
[[ "$health_status" == "200" ]]
'@
  $smokeCommand = $smokeCommand.Replace("__PUBLIC_BASE_URL__", $PublicBaseUrl)
  Invoke-RemoteCommand -Description "Executando smoke tests publicos" -Command $smokeCommand
}

Write-Host "Deploy finalizado com sucesso."
if ($backupFile) {
  Write-Host "Backup preservado em: $backupFile"
}
