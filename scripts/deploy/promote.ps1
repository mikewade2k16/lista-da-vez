param(
  [string]$VpsHost = "85.31.62.33",
  [int]$Port = 22,
  [string]$User = "deploy",
  [string]$KeyPath = (Join-Path $HOME ".ssh\gh_actions_omnichannel_vps"),
  # Por seguranca promote SEMPRE faz backup do banco de prod antes (migrations da
  # imagem nova rodam no boot). Use -NoBackup para pular (NAO recomendado).
  [switch]$NoBackup,
  [switch]$ForceRecreate,
  [string]$GhcrUser = "",
  [string]$GhcrToken = ""
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$deployPull = Join-Path $scriptDir "deploy-pull.ps1"
if (-not (Test-Path $deployPull)) { throw "deploy-pull.ps1 nao encontrado em $scriptDir" }
if (-not (Test-Path $KeyPath)) { throw "Chave SSH nao encontrada em $KeyPath" }

$SshExe = Join-Path $env:SystemRoot "System32\OpenSSH\ssh.exe"
if (-not (Test-Path $SshExe)) { throw "ssh.exe nao encontrado em $SshExe" }

$resolvedKeyPath = (Resolve-Path $KeyPath).Path
$sshArgs = @(
  "-i", $resolvedKeyPath, "-o", "StrictHostKeyChecking=accept-new", "-p", $Port.ToString(),
  "-o", "BatchMode=yes", "-o", "ConnectTimeout=15"
)
$remoteTarget = "$User@$VpsHost"
$stagingPath = "/home/deploy/lista-atendimento-staging"

# 1. Le o IMAGE_TAG que esta rodando em STAGING (a fonte da verdade da promocao).
$readCmd = "set -euo pipefail; cd '$stagingPath' && grep '^IMAGE_TAG=' .env.staging | head -1 | cut -d= -f2-"
Write-Host "==> Lendo IMAGE_TAG do staging (${remoteTarget}:$stagingPath)"
$stagingTag = (& $SshExe @sshArgs $remoteTarget $readCmd)
if ($LASTEXITCODE -ne 0) { throw "Falha ao ler o IMAGE_TAG do staging." }
$stagingTag = ($stagingTag | ForEach-Object { $_.ToString() }) -join "" | ForEach-Object { $_.Trim() }

# 2. Valida: precisa ser uma tag concreta testada (nao vazia, nao 'latest').
if ([string]::IsNullOrWhiteSpace($stagingTag)) {
  throw "IMAGE_TAG do staging esta vazio. Suba e teste algo em staging antes de promover."
}
if ($stagingTag -ieq "latest") {
  throw "IMAGE_TAG do staging = 'latest'. Promova um SHA concreto (use staging-up.ps1 -Tag sha-...)."
}

Write-Host "Tag testada em staging: $stagingTag"
Write-Host "==> Promovendo a MESMA imagem para PRODUCAO"

# 3. Promove: a MESMA tag vai pra prod (mesmo artefato testado).
$forward = @{ Environment = "prod"; Tag = $stagingTag }
if (-not $NoBackup) { $forward.BackupDatabase = $true }
if ($ForceRecreate) { $forward.ForceRecreate = $true }
if (-not [string]::IsNullOrWhiteSpace($GhcrUser)) { $forward.GhcrUser = $GhcrUser }
if (-not [string]::IsNullOrWhiteSpace($GhcrToken)) { $forward.GhcrToken = $GhcrToken }

& $deployPull @forward

Write-Host ""
Write-Host "Promocao concluida: producao rodando a imagem $stagingTag (a mesma validada em staging)."
Write-Host "Rollback, se preciso: scripts/deploy/deploy-pull.ps1 -Environment prod -Tag <sha-anterior>"
