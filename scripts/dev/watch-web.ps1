$ErrorActionPreference = 'Stop'

$repo = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $repo

# Idempotente: se ja existe um watch do web ativo (janela manual ou task do
# VS Code), nao sobe um segundo sync duplicado.
$watchAtivo = Get-CimInstance Win32_Process |
  Where-Object { $_.Name -like 'docker*' -and $_.CommandLine -match 'compose\s+watch' -and $_.CommandLine -match '\bweb\b' }
if ($watchAtivo) {
  Write-Host "Watch do web ja esta ativo (PID $(@($watchAtivo)[0].ProcessId)) - nada a fazer." -ForegroundColor Yellow
  exit 0
}

Write-Host 'Reconciliando codigo do web (docker compose up -d --build --force-recreate --no-deps web)...' -ForegroundColor Cyan
docker compose up -d --build --force-recreate --no-deps web

Write-Host 'Painel: http://localhost:3003 | deixe este processo aberto para manter o sync ativo.' -ForegroundColor Green
docker compose watch --no-up web
