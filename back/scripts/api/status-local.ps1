param(
  [int]$Port = 8080
)

$ErrorActionPreference = "Stop"

$listeners = netstat.exe -ano | Select-String ":$Port"

if (-not $listeners) {
  Write-Host "Nenhum listener encontrado na porta $Port."
} else {
  Write-Host "Listeners na porta ${Port}:"
  $listeners
}

try {
  $listenerLine = $listeners | Where-Object { $_.ToString() -match "LISTENING" } | Select-Object -First 1
  if ($listenerLine) {
    $parts = ($listenerLine.ToString() -split "\s+") | Where-Object { $_ }
    if ($parts.Length -ge 5) {
      $procId = [int]$parts[4]
      $processInfo = Get-Process -Id $procId -ErrorAction SilentlyContinue | Select-Object Id, ProcessName, StartTime, Path
      Write-Host ""
      Write-Host "Processo:"
      if ($processInfo) {
        $processInfo | ConvertTo-Json -Depth 4
      } else {
        Write-Host "Processo nao encontrado."
      }
    }
  }
} catch {
}

try {
  $health = Invoke-RestMethod -Method Get -Uri "http://localhost:$Port/healthz"
  Write-Host ""
  Write-Host "/healthz:"
  $health | ConvertTo-Json -Depth 6
} catch {
  Write-Host ""
  Write-Host "Nao foi possivel consultar http://localhost:$Port/healthz"
}
