$ErrorActionPreference = "Stop"

$pgBin = if ($env:PG_BIN) { $env:PG_BIN } else { "C:\Program Files\PostgreSQL\16\bin" }
$pgRoot = Split-Path $pgBin -Parent
$serviceName = if ($env:PG_SERVICE) { $env:PG_SERVICE } else { "postgresql-x64-16" }
$baseDir = Join-Path $env:LOCALAPPDATA "lista-da-vez\postgres16"
$dataDir = if ($env:PGDATA) { $env:PGDATA } else { Join-Path $baseDir "data" }
$serviceDataDir = Join-Path $pgRoot "data"
$logFile = Join-Path $baseDir "postgres.log"
$port = if ($env:PGPORT) { $env:PGPORT } else { "5432" }
$superUser = if ($env:PG_SUPERUSER) { $env:PG_SUPERUSER } else { "postgres" }
$superPassword = if ($env:PG_SUPERPASSWORD) { $env:PG_SUPERPASSWORD } else { "postgres" }
$appUser = if ($env:APP_DB_USER) { $env:APP_DB_USER } else { "lista_da_vez" }
$appPassword = if ($env:APP_DB_PASSWORD) { $env:APP_DB_PASSWORD } else { "lista_da_vez_dev" }
$appDatabase = if ($env:APP_DB_NAME) { $env:APP_DB_NAME } else { "lista_da_vez" }

if (!(Test-Path $pgBin)) {
  throw "Nao encontrei o diretorio de binarios do PostgreSQL em '$pgBin'."
}

New-Item -ItemType Directory -Force -Path $baseDir | Out-Null

$initdb = Join-Path $pgBin "initdb.exe"
$pgCtl = Join-Path $pgBin "pg_ctl.exe"
$psql = Join-Path $pgBin "psql.exe"
$pgIsReady = Join-Path $pgBin "pg_isready.exe"
$pgAdmin = Join-Path $pgRoot "pgAdmin 4\runtime\pgAdmin4.exe"
$pwFile = Join-Path $baseDir "pwfile.txt"
$service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
$mode = if ($null -ne $service) { "service" } else { "manual" }

function Invoke-Native {
  param(
    [Parameter(Mandatory = $true)]
    [string]$FilePath,
    [string[]]$Arguments,
    [Parameter(Mandatory = $true)]
    [string]$ErrorMessage
  )

  & $FilePath @Arguments
  if ($LASTEXITCODE -ne 0) {
    throw $ErrorMessage
  }
}

if ($mode -eq "service") {
  if ($service.Status -ne "Running") {
    Start-Service -Name $serviceName
  }
} else {
  if (!(Test-Path (Join-Path $dataDir "PG_VERSION"))) {
    $superPassword | Out-File -Encoding ascii $pwFile
    Invoke-Native -FilePath $initdb -Arguments @("-D", $dataDir, "-U", $superUser, "-A", "scram-sha-256", "--pwfile", $pwFile, "--encoding", "UTF8", "--locale", "C") -ErrorMessage "Falha ao inicializar o cluster local do PostgreSQL."
    Remove-Item $pwFile -Force -ErrorAction SilentlyContinue
  }

  Invoke-Native -FilePath $pgCtl -Arguments @("-D", $dataDir, "-l", $logFile, "-o", "-p $port", "start") -ErrorMessage "Falha ao iniciar o PostgreSQL local no modo manual."
}

for ($i = 0; $i -lt 30; $i++) {
  & $pgIsReady -h localhost -p $port | Out-Null
  if ($LASTEXITCODE -eq 0) {
    break
  }

  Start-Sleep -Seconds 1
}

if ($LASTEXITCODE -ne 0) {
  throw "PostgreSQL nao respondeu em localhost:$port."
}

$env:PGPASSWORD = $superPassword

$roleExists = & $psql -h localhost -p $port -U $superUser -d postgres -tA -c "select 1 from pg_roles where rolname = '$appUser';"
if ($LASTEXITCODE -ne 0) {
  throw "Falha ao consultar as roles existentes no PostgreSQL."
}
if (($roleExists | Out-String).Trim() -eq "1") {
  Invoke-Native -FilePath $psql -Arguments @("-h", "localhost", "-p", $port, "-U", $superUser, "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-c", "alter role $appUser with login password '$appPassword';") -ErrorMessage "Falha ao atualizar a role da aplicacao."
} else {
  Invoke-Native -FilePath $psql -Arguments @("-h", "localhost", "-p", $port, "-U", $superUser, "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-c", "create role $appUser login password '$appPassword';") -ErrorMessage "Falha ao criar a role da aplicacao."
}

$dbExists = & $psql -h localhost -p $port -U $superUser -d postgres -tA -c "select 1 from pg_database where datname = '$appDatabase';"
if ($LASTEXITCODE -ne 0) {
  throw "Falha ao consultar os bancos existentes no PostgreSQL."
}
if (($dbExists | Out-String).Trim() -eq "1") {
  Invoke-Native -FilePath $psql -Arguments @("-h", "localhost", "-p", $port, "-U", $superUser, "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-c", "alter database $appDatabase owner to $appUser;") -ErrorMessage "Falha ao atualizar o owner do banco da aplicacao."
} else {
  Invoke-Native -FilePath $psql -Arguments @("-h", "localhost", "-p", $port, "-U", $superUser, "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-c", "create database $appDatabase owner $appUser;") -ErrorMessage "Falha ao criar o banco da aplicacao."
}

$env:DATABASE_URL = "postgres://$($appUser):$($appPassword)@localhost:$($port)/$($appDatabase)?sslmode=disable"

Push-Location (Join-Path $PSScriptRoot "..\..")
try {
  Invoke-Native -FilePath "go" -Arguments @("run", "./cmd/migrate", "up") -ErrorMessage "Falha ao aplicar as migrations do backend."
} finally {
  Pop-Location
}

Write-Host ""
Write-Host "PostgreSQL local pronto."
Write-Host "modo=$mode"
if ($mode -eq "service") {
  Write-Host "service=$serviceName"
  Write-Host "PGDATA=$serviceDataDir"
} else {
  Write-Host "PGDATA=$dataDir"
  Write-Host "PGLOG=$logFile"
}
Write-Host "DATABASE_URL=$($env:DATABASE_URL)"
if (Test-Path $pgAdmin) {
  Write-Host "pgAdmin=$pgAdmin"
}
