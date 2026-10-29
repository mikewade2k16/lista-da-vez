function Export-DeployImageArchive {
  param([string]$Tag, [string[]]$Images)
  $directory = Join-Path ([IO.Path]::GetTempPath()) "omni-deploy-$Tag"
  New-Item -ItemType Directory -Path $directory -Force | Out-Null
  $raw = Join-Path $directory 'images.tar'
  $archive = "$raw.gz"
  Write-Host '==> Empacotando imagens locais para SSH'
  & docker image save -o $raw @Images
  if ($LASTEXITCODE -ne 0) { throw 'Falha no docker image save.' }
  $inputStream = [IO.File]::OpenRead($raw)
  $outputStream = [IO.File]::Create($archive)
  $gzip = [IO.Compression.GZipStream]::new($outputStream, [IO.Compression.CompressionLevel]::Fastest)
  try { $inputStream.CopyTo($gzip) }
  finally { $gzip.Dispose(); $outputStream.Dispose(); $inputStream.Dispose() }
  Remove-Item -LiteralPath $raw
  return $archive
}

function Send-DeployImageArchive {
  param([string]$Archive, [string]$Tag, [string]$RemoteTarget, [string]$RemotePath,
    [string]$ScpExe, [string[]]$ScpArguments)
  if (-not (Test-Path -LiteralPath $Archive -PathType Leaf)) { throw "Arquivo de imagens ausente: $Archive" }
  $checksum = (Get-FileHash -LiteralPath $Archive -Algorithm SHA256).Hash.ToLowerInvariant()
  $remoteFile = "$RemotePath/.deploy/images/$Tag.tar.gz"
  Invoke-RemoteCommand -Description 'Preparando diretorio das imagens na VPS' -Command "mkdir -p '$RemotePath/.deploy/images'"
  & $ScpExe @ScpArguments $Archive "${RemoteTarget}:$remoteFile"
  if ($LASTEXITCODE -ne 0) { throw 'Falha ao enviar imagens por SSH.' }
  $command = "set -euo pipefail`nprintf '%s  %s\n' '$checksum' '$remoteFile' | sha256sum -c -`ndocker load -i '$remoteFile'"
  Invoke-RemoteCommand -Description 'Verificando SHA256 e carregando imagens (sem build na VPS)' -Command $command
}
