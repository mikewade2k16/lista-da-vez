# Somente nomes de etapas e duracoes; nunca gravar comandos, env ou credenciais.
function Write-DeployTiming {
  param([string]$Name, [string]$Status, [double]$DurationMs, [string]$Id = '', [int]$Depth = 0)
  $event = [ordered]@{
    timestamp = [DateTime]::UtcNow.ToString('o')
    name = $Name
    status = $Status
    durationMs = [Math]::Round($DurationMs, 1)
    id = $Id
    depth = $Depth
  }
  if ($env:OMNI_DEPLOY_TIMING_FILE) {
    try {
      $line = ($event | ConvertTo-Json -Compress) + "`n"
      [IO.File]::AppendAllText($env:OMNI_DEPLOY_TIMING_FILE, $line, [Text.UTF8Encoding]::new($false))
    } catch { Write-Warning 'Nao foi possivel registrar a duracao da etapa.' }
  }
  $seconds = '{0:N1}s' -f ($DurationMs / 1000)
  Write-Host "[tempo] $Status | $Name | $seconds"
}

function Invoke-DeployStep {
  param([Parameter(Mandatory = $true)][string]$Name, [Parameter(Mandatory = $true)][scriptblock]$Action,
    [switch]$ObserveNativeExitCode)
  $depth = if ($null -eq $global:OmniDeployTimingDepth) { 0 } else { $global:OmniDeployTimingDepth }
  $id = [guid]::NewGuid().ToString('N')
  Write-DeployTiming -Name $Name -Status 'started' -DurationMs 0 -Id $id -Depth $depth
  $watch = [Diagnostics.Stopwatch]::StartNew()
  $status = 'failed'
  $global:OmniDeployTimingDepth = $depth + 1
  try {
    & $Action
    $status = if ($ObserveNativeExitCode -and $global:LASTEXITCODE -ne 0) { 'failed' } else { 'ok' }
  } finally {
    $watch.Stop()
    $global:OmniDeployTimingDepth = $depth
    Write-DeployTiming -Name $Name -Status $status -DurationMs $watch.Elapsed.TotalMilliseconds -Id $id -Depth $depth
  }
}

function Write-DeployRemoteTiming {
  param([string]$Line)
  if (-not $Line.StartsWith('OMNI_DEPLOY_STAGE ')) { Write-Host $Line; return }
  $event = $Line.Substring('OMNI_DEPLOY_STAGE '.Length) | ConvertFrom-Json
  if ($event.name -notmatch '^remote\.[a-z.]+$' -or $event.status -notin @('started', 'ok', 'failed') -or $event.durationMs -lt 0) {
    throw 'Evento de duracao remoto invalido.'
  }
  Write-DeployTiming -Name $event.name -Status $event.status -DurationMs $event.durationMs -Depth 2
}
