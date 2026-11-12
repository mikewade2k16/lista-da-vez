$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'release-web-container.ps1')
$script:output = @()
$script:exitCode = 0
function docker {
  $global:LASTEXITCODE = $script:exitCode
  return $script:output
}
function Assert-Error([string]$expected) {
  try { Get-ReleaseWebContainer; throw 'A consulta deveria falhar.' }
  catch { if ($_.Exception.Message -notlike "*$expected*") { throw } }
}
Assert-Error 'nao esta em execucao'
$script:exitCode = 1
Assert-Error 'Nao foi possivel consultar'
$script:exitCode = 0
$script:output = @('  abc123  ')
if ((Get-ReleaseWebContainer) -ne 'abc123') { throw 'ID do container incorreto.' }
$script:output = @('abc', 'def')
Assert-Error 'nao esta em execucao'
Write-Host 'Validacao do container web: testes passaram.'
