param(
  [int]$Port = 8080,
  [string]$DatabaseUrl = "postgres://lista_da_vez:lista_da_vez_dev@localhost:5432/lista_da_vez?sslmode=disable",
  [string]$CorsAllowedOrigins = "http://localhost:*,http://127.0.0.1:*,http://[::1]:*"
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$backDir = Resolve-Path (Join-Path $scriptDir "..\\..")
$logsDir = Join-Path $backDir ".logs"
$outLog = Join-Path $logsDir "api-local.out.log"
$errLog = Join-Path $logsDir "api-local.err.log"

New-Item -ItemType Directory -Force -Path $logsDir | Out-Null

$stopScript = Join-Path $scriptDir "stop-local.ps1"
powershell -ExecutionPolicy Bypass -File $stopScript -Port $Port | Out-Null

$env:DATABASE_URL = $DatabaseUrl
$env:DATABASE_MIN_CONNS = "0"
$env:DATABASE_MAX_CONNS = "10"
$env:CORS_ALLOWED_ORIGINS = $CorsAllowedOrigins
$env:APP_ADDR = ":$Port"

$process = Start-Process -FilePath "go" `
  -ArgumentList "run", "./cmd/api" `
  -WorkingDirectory $backDir `
  -RedirectStandardOutput $outLog `
  -RedirectStandardError $errLog `
  -PassThru

$ready = $false
for ($index = 0; $index -lt 40; $index++) {
  Start-Sleep -Milliseconds 500

  try {
    $health = Invoke-RestMethod -Method Get -Uri "http://localhost:$Port/healthz"
    if ($health.status -eq "ok") {
      $ready = $true
      break
    }
  } catch {
  }
}

if (-not $ready) {
  throw "A API nao ficou pronta em http://localhost:$Port/healthz"
}

Write-Host "API local iniciada."
Write-Host "Porta: $Port"
Write-Host "PID: $($process.Id)"
Write-Host "Health: http://localhost:$Port/healthz"
Write-Host "Logs: $outLog"
