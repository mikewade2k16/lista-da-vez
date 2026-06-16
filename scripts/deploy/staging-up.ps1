param(
  # Tag da imagem (sha-<...> ou nome de branch). Vazio => HEAD local (sha-<HEAD>).
  [string]$Tag = "",
  [switch]$ForceRecreate,
  [switch]$SkipSmokeTests
)

$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

# Atalho: sobe a stack de STAGING sob demanda (pull + up --no-build).
# So orquestra; toda a logica esta em deploy-pull.ps1.
$deployPull = Join-Path $scriptDir "deploy-pull.ps1"
$forward = @{ Environment = "staging" }
if (-not [string]::IsNullOrWhiteSpace($Tag)) { $forward.Tag = $Tag }
if ($ForceRecreate) { $forward.ForceRecreate = $true }
if ($SkipSmokeTests) { $forward.SkipSmokeTests = $true }

& $deployPull @forward
