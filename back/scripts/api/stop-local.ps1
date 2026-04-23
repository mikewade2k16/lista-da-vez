param(
  [int]$Port = 8080
)

$ErrorActionPreference = "Stop"

$listeners = netstat.exe -ano | Select-String ":$Port"
$stopped = @()

foreach ($line in $listeners) {
  $parts = ($line.ToString() -split "\s+") | Where-Object { $_ }
  if ($parts.Length -lt 5) {
    continue
  }

  if ($parts[3] -ne "LISTENING") {
    continue
  }

  $procId = [int]$parts[4]
  if ($stopped -contains $procId) {
    continue
  }

  try {
    Stop-Process -Id $procId -Force -ErrorAction Stop
    $stopped += $procId
  } catch {
  }
}

if ($stopped.Count -eq 0) {
  Write-Host "Nenhum processo escutando a porta $Port."
  exit 0
}

Write-Host "Processos encerrados na porta ${Port}: $($stopped -join ', ')"
