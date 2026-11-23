$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'deploy-timing.ps1')
$directory = Join-Path ([IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString('N'))
[IO.Directory]::CreateDirectory($directory) | Out-Null
$previousFile = $env:OMNI_DEPLOY_TIMING_FILE
$env:OMNI_DEPLOY_TIMING_FILE = Join-Path $directory 'events.jsonl'
try {
  $value = Invoke-DeployStep 'Retorno capturado' { 'resultado' }
  if ($value -ne 'resultado') { throw 'Cronometro alterou a saida capturada.' }
  Invoke-DeployStep 'Etapa pai' { Invoke-DeployStep 'Etapa filha' { } }
  $failed = $false
  try { Invoke-DeployStep 'Etapa com erro' { throw 'erro original' } }
  catch { $failed = $_.Exception.Message -eq 'erro original' }
  if (-not $failed) { throw 'Cronometro escondeu ou trocou o erro original.' }
  Invoke-DeployStep -Name 'Nativo com retry' -ObserveNativeExitCode -Action { $global:LASTEXITCODE = 255 }
  if ($LASTEXITCODE -ne 255) { throw 'Cronometro alterou o exit code nativo.' }
  Write-DeployRemoteTiming 'OMNI_DEPLOY_STAGE {"name":"remote.images.pull","status":"ok","durationMs":1200}'
  $events = @(Get-Content $env:OMNI_DEPLOY_TIMING_FILE | ForEach-Object { $_ | ConvertFrom-Json })
  if (@($events | Where-Object { $_.name -eq 'Retorno capturado' -and $_.status -eq 'ok' }).Count -ne 1) { throw 'Etapa de sucesso ausente.' }
  if (@($events | Where-Object { $_.name -eq 'Etapa com erro' -and $_.status -eq 'failed' }).Count -ne 1) { throw 'Etapa falha ausente.' }
  if (@($events | Where-Object { $_.name -eq 'Nativo com retry' -and $_.status -eq 'failed' }).Count -ne 1) { throw 'Falha nativa deveria ser registrada sem interromper o retry.' }
  if (@($events | Where-Object { $_.name -eq 'Etapa filha' -and $_.depth -eq 1 }).Count -ne 2) { throw 'Hierarquia das etapas incorreta.' }
  if (@($events | Where-Object { $_.name -eq 'remote.images.pull' -and $_.durationMs -eq 1200 }).Count -ne 1) { throw 'Duracao remota ausente.' }
  if ($global:OmniDeployTimingDepth -ne 0) { throw 'Erro deixou hierarquia de cronometros inconsistente.' }

  # Exercita a funcao real de SSH com transporte simulado: nenhuma conexao externa.
  $scriptDir = $PSScriptRoot
  $SshExe = 'Test-RemoteSsh'
  $sshArgs = @()
  $remoteTarget = 'fixture'
  $script:remoteFailure = $false
  function Test-RemoteSsh {
    process { $script:receivedCommand = $_ }
    end {
      $global:LASTEXITCODE = if ($script:remoteFailure) { 17 } else { 0 }
      '/tmp/backup-test.sql.gz'
    }
  }
  $ast = [Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'deploy-pull.ps1'), [ref]$null, [ref]$null)
  $definition = $ast.Find({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Invoke-RemoteCommand' }, $true)
  Invoke-Expression $definition.Extent.Text
  $output = Invoke-RemoteCommand -Description 'Backup simulado' -Command 'true' -CaptureOutput
  if ($output -ne '/tmp/backup-test.sql.gz') { throw 'Medicao contaminou a saida capturada do SSH.' }
  if ($script:receivedCommand -notmatch 'omni_timed_step') { throw 'Funcoes de medicao remotas nao foram enviadas.' }
  $script:remoteFailure = $true
  $failed = $false
  try { Invoke-RemoteCommand -Description 'Falha SSH simulada' -Command 'false' }
  catch { $failed = $_.Exception.Message -like '*Falha ao executar: Falha SSH simulada*' }
  if (-not $failed) { throw 'Medicao escondeu falha de SSH.' }

  # Engine indisponivel deve abortar antes ate de consultar a chave SSH.
  $global:OmniDockerCheckTestReached = $false
  function node { $global:OmniDockerCheckTestReached = $true; $global:LASTEXITCODE = 1 }
  function docker { throw 'Nao deveria executar Docker real neste teste.' }
  $failed = $false
  try { & (Join-Path $PSScriptRoot 'deploy-fast.ps1') -KeyPath (Join-Path $directory 'missing-key') }
  catch { $failed = $_.Exception.Message -like '*Preflight Docker falhou*' }
  if (-not $failed -or -not $global:OmniDockerCheckTestReached) { throw 'Deploy deveria checar Docker antes de chave/SSH/build.' }
  Remove-Variable -Name OmniDockerCheckTestReached -Scope Global
  Write-Host 'Testes dos cronometros: OK (sem Docker/SSH)'
} finally {
  $env:OMNI_DEPLOY_TIMING_FILE = $previousFile
  Remove-Item -LiteralPath $directory -Recurse -Force
}
