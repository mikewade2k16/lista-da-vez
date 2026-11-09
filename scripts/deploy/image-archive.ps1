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

function Convert-ToSftpQuoted {
  param([string]$Value)
  if ($Value -match '[\r\n]') { throw 'Caminho SFTP nao pode conter quebra de linha.' }
  return '"' + $Value.Replace('\', '\\').Replace('"', '\"') + '"'
}

function Send-DeployImageArchive {
  param([string]$Archive, [string]$Tag, [string]$RemoteTarget, [string]$RemotePath,
    [string[]]$SftpArguments,
    [string]$SftpExe = (Get-Command sftp -CommandType Application -ErrorAction Stop).Source)
  if (-not (Test-Path -LiteralPath $Archive -PathType Leaf)) { throw "Arquivo de imagens ausente: $Archive" }
  $checksum = (Get-FileHash -LiteralPath $Archive -Algorithm SHA256).Hash.ToLowerInvariant()
  # O hash isola parciais de arquivos diferentes, mesmo se a tag for reutilizada.
  $remoteFile = "$RemotePath/.deploy/images/$Tag-$checksum.tar.gz"
  $partial = "$remoteFile.part"
  $directoryQ = Convert-ToBashSingleQuoted "$RemotePath/.deploy/images"
  $remoteFileQ = Convert-ToBashSingleQuoted $remoteFile
  $partialQ = Convert-ToBashSingleQuoted $partial
  Invoke-RemoteCommand -Description 'Preparando diretorio das imagens na VPS' -Command "set -euo pipefail`nmkdir -p -- $directoryQ`ntest -w $directoryQ"
  $batch = [IO.Path]::GetTempFileName()
  try {
    $localPath = (Get-Item -LiteralPath $Archive).FullName.Replace('\', '/')
    $put = 'reput ' + (Convert-ToSftpQuoted $localPath) + ' ' + (Convert-ToSftpQuoted $partial)
    [IO.File]::WriteAllText($batch, "$put`n", [Text.UTF8Encoding]::new($false))
    for ($attempt = 1; $attempt -le 3; $attempt++) {
      Write-Host "==> Enviando imagens por SFTP com retomada ($attempt/3)"
      & $SftpExe @SftpArguments -b $batch $RemoteTarget
      if ($LASTEXITCODE -eq 0) { break }
      if ($attempt -eq 3) {
        throw "Transferencia interrompida apos 3 tentativas. Arquivo local preservado: $Archive. Retome sem rebuild: npm run deploy:prod -- -Tag $Tag -ImageArchive <arquivo-local> (use deploy:staging para staging e repita as opcoes originais de acesso, backup e profiles)."
      }
    }
  } finally {
    Remove-Item -LiteralPath $batch -Force
  }
  $command = "set -euo pipefail`nprintf '%s  %s\n' '$checksum' $partialQ | sha256sum -c -`nmv -- $partialQ $remoteFileQ`ndocker load -i $remoteFileQ"
  Invoke-RemoteCommand -Description 'Verificando SHA256 e carregando imagens (sem build na VPS)' -Command $command
}
