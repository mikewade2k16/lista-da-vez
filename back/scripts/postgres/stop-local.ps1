$ErrorActionPreference = "Stop"

$pgBin = if ($env:PG_BIN) { $env:PG_BIN } else { "C:\Program Files\PostgreSQL\16\bin" }
$serviceName = if ($env:PG_SERVICE) { $env:PG_SERVICE } else { "postgresql-x64-16" }
$baseDir = Join-Path $env:LOCALAPPDATA "lista-da-vez\postgres16"
$dataDir = if ($env:PGDATA) { $env:PGDATA } else { Join-Path $baseDir "data" }
$pgCtl = Join-Path $pgBin "pg_ctl.exe"
$service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue

if ($null -ne $service) {
  if ($service.Status -eq "Running") {
    Stop-Service -Name $serviceName
  }
} else {
  if (!(Test-Path (Join-Path $dataDir "PG_VERSION"))) {
    throw "Cluster local ainda nao foi inicializado."
  }

  & $pgCtl -D $dataDir stop
}
