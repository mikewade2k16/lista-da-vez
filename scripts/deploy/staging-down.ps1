param(
  [string]$VpsHost = "85.31.62.33",
  [int]$Port = 22,
  [string]$User = "deploy",
  [string]$KeyPath = (Join-Path $HOME ".ssh\gh_actions_omnichannel_vps"),
  # Por padrao preserva os volumes (so para os containers). -RemoveVolumes apaga
  # tambem os volumes omni-staging_* (zera o banco de staging) — use com cuidado.
  [switch]$RemoveVolumes
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $KeyPath)) { throw "Chave SSH nao encontrada em $KeyPath" }

$SshExe = Join-Path $env:SystemRoot "System32\OpenSSH\ssh.exe"
if (-not (Test-Path $SshExe)) { throw "ssh.exe nao encontrado em $SshExe" }

$resolvedKeyPath = (Resolve-Path $KeyPath).Path
$sshArgs = @(
  "-i", $resolvedKeyPath, "-o", "StrictHostKeyChecking=accept-new", "-p", $Port.ToString(),
  "-o", "BatchMode=yes", "-o", "ConnectTimeout=15"
)
$remoteTarget = "$User@$VpsHost"
$remotePath = "/home/deploy/lista-atendimento-staging"

$volumesFlag = if ($RemoveVolumes) { " --volumes" } else { "" }
if ($RemoveVolumes) {
  Write-Host "ATENCAO: -RemoveVolumes vai APAGAR os volumes omni-staging_* (zera o banco de staging)."
}

$downCmd = "set -euo pipefail; cd '$remotePath' && docker compose --env-file .env.staging -f docker-compose.prod.yml down$volumesFlag && echo 'staging derrubado.'"

Write-Host "==> Derrubando staging (${remoteTarget}:$remotePath)"
& $SshExe @sshArgs $remoteTarget $downCmd
if ($LASTEXITCODE -ne 0) { throw "Falha ao derrubar a stack de staging." }

$volumesMsg = if ($RemoveVolumes) { "REMOVIDOS." } else { "preservados." }
Write-Host "Staging derrubado. Volumes $volumesMsg"
