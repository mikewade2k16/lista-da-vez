$ErrorActionPreference = "Stop"

$pgBin = if ($env:PG_BIN) { $env:PG_BIN } else { "C:\Program Files\PostgreSQL\16\bin" }
$serviceName = if ($env:PG_SERVICE) { $env:PG_SERVICE } else { "postgresql-x64-16" }
$baseDir = Join-Path $env:LOCALAPPDATA "lista-da-vez\postgres16"
$dataDir = if ($env:PGDATA) { $env:PGDATA } else { Join-Path $baseDir "data" }
$logFile = Join-Path $baseDir "postgres.log"
$port = if ($env:PGPORT) { $env:PGPORT } else { "5432" }
$pgCtl = Join-Path $pgBin "pg_ctl.exe"
$pgIsReady = Join-Path $pgBin "pg_isready.exe"
$service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue

if ($null -ne $service) {
  if ($service.Status -ne "Running") {
    Start-Service -Name $serviceName
  }
} else {
  if (!(Test-Path (Join-Path $dataDir "PG_VERSION"))) {
    throw "Cluster local ainda nao foi inicializado. Rode init-local.ps1 primeiro."
  }

  & $pgCtl -D $dataDir -l $logFile -o "-p $port" start
}

& $pgIsReady -h localhost -p $port
