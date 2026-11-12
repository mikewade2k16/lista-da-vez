$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/image-archive.ps1"
function Convert-ToBashSingleQuoted {
  param([string]$Value)
  return "'" + $Value.Replace("'", "'" + '"' + "'" + '"' + "'") + "'"
}
function Assert($Condition, $Message) { if (-not $Condition) { throw $Message } }
$directory = Join-Path ([IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString())
New-Item -ItemType Directory $directory | Out-Null
$archive = Join-Path $directory 'image with spaces.tar.gz'
[IO.File]::WriteAllText($archive, 'test archive')
function Invoke-RemoteCommand {
  param($Description, $Command)
  $script:commands += $Command
  if ($script:rejectChecksum -and $Command.Contains('sha256sum')) { throw 'checksum mismatch' }
}
function Test-Sftp {
  $script:attempts++
  $batchIndex = [array]::IndexOf($args, '-b')
  $script:batchPath = $args[$batchIndex + 1]
  $script:batchContent = [IO.File]::ReadAllText($script:batchPath)
  $global:LASTEXITCODE = if ($script:attempts -le $script:failures) { 255 } else { 0 }
}
try {
  foreach ($failures in @(0, 1, 3)) {
    $script:commands = @(); $script:attempts = 0; $script:failures = $failures
    $script:rejectChecksum = $false
    $failed = $false
    try {
      Send-DeployImageArchive -Archive $archive -Tag test -RemoteTarget test -RemotePath "/tmp/deploy's dir" -SftpExe Test-Sftp
    } catch { $failed = $true }
    Assert ($failed -eq ($failures -eq 3)) 'Resultado incorreto na falha de transporte'
    Assert ($script:attempts -eq [Math]::Min(3, $failures + 1)) 'Limite de tentativas incorreto'
    Assert ($script:commands[0].Contains('mkdir -p --') -and $script:commands[0].Contains('test -w')) 'Diretorio deve ser preparado e validado antes do envio'
    Assert ($script:commands[0].Contains('touch --') -and $script:commands[0].Contains('.tar.gz.part')) 'Parcial deve existir sem truncar antes do reput'
    Assert ($script:batchContent.StartsWith('reput "')) 'Envio precisa retomar o parcial'
    Assert ($script:batchContent.Contains('.tar.gz.part"')) 'Envio precisa usar arquivo parcial'
    Assert (-not (Test-Path $script:batchPath)) 'Batch temporario nao foi removido'
    Assert (Test-Path $archive) 'Arquivo local precisa sobreviver para retomada'
    Assert ($script:commands.Count -eq $(if ($failed) { 1 } else { 2 })) 'Falha no transporte nao pode carregar imagem'
  }
  $script:attempts = 0; $script:failures = 0; $script:commands = @(); $script:rejectChecksum = $true
  $failed = $false
  try { Send-DeployImageArchive -Archive $archive -Tag test -RemoteTarget test -RemotePath /tmp/test -SftpExe Test-Sftp } catch { $failed = $true }
  Assert $failed 'Falha de integridade deve propagar'
  Assert ($script:commands[1] -match '(?s)set -euo pipefail.*sha256sum -c -.*mv --.*docker load') 'Verificar integridade antes de promover/carregar'
  Write-Host 'Testes de transporte de imagens: OK (sem acesso remoto)'
} finally { Remove-Item -LiteralPath $directory -Recurse -Force }
