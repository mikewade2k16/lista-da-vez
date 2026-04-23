$ErrorActionPreference = "Stop"

$pgBin = if ($env:PG_BIN) { $env:PG_BIN } else { "C:\Program Files\PostgreSQL\16\bin" }
$pgRoot = Split-Path $pgBin -Parent
$serviceName = if ($env:PG_SERVICE) { $env:PG_SERVICE } else { "postgresql-x64-16" }
$baseDir = Join-Path $env:LOCALAPPDATA "lista-da-vez\postgres16"
$dataDir = if ($env:PGDATA) { $env:PGDATA } else { Join-Path $baseDir "data" }
$serviceDataDir = Join-Path $pgRoot "data"
$logFile = Join-Path $baseDir "postgres.log"
$port = if ($env:PGPORT) { $env:PGPORT } else { "5432" }
$pgIsReady = Join-Path $pgBin "pg_isready.exe"
$pgAdmin = Join-Path $pgRoot "pgAdmin 4\runtime\pgAdmin4.exe"
$service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue

Write-Host "PG_BIN=$pgBin"
Write-Host "PGPORT=$port"
if ($null -ne $service) {
  Write-Host "modo=service"
  Write-Host "service=$serviceName"
  Write-Host "serviceStatus=$($service.Status)"
  Write-Host "PGDATA=$serviceDataDir"
} else {
  Write-Host "modo=manual"
  Write-Host "PGDATA=$dataDir"
  Write-Host "PGLOG=$logFile"
  if (Test-Path (Join-Path $dataDir "PG_VERSION")) {
    Write-Host "cluster=initialized"
  } else {
    Write-Host "cluster=missing"
  }
}

if (Test-Path $pgAdmin) {
  Write-Host "pgAdmin=$pgAdmin"
}

Write-Host "---"

if (Test-Path $pgIsReady) {
  & $pgIsReady -h localhost -p $port
}
