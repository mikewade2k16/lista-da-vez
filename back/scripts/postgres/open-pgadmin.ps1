$ErrorActionPreference = "Stop"

$pgBin = if ($env:PG_BIN) { $env:PG_BIN } else { "C:\Program Files\PostgreSQL\16\bin" }
$pgRoot = Split-Path $pgBin -Parent
$pgAdmin = Join-Path $pgRoot "pgAdmin 4\runtime\pgAdmin4.exe"

if (!(Test-Path $pgAdmin)) {
  throw "Nao encontrei o pgAdmin 4 em '$pgAdmin'."
}

Start-Process -FilePath $pgAdmin
Write-Host "Abrindo pgAdmin 4..."
