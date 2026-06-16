param(
  [string]$VpsHost = "85.31.62.33",
  [int]$Port = 22,
  [string]$User = "deploy",
  [string]$KeyPath = (Join-Path $HOME ".ssh\gh_actions_omnichannel_vps"),
  [string]$RemotePath = "/home/deploy/lista-atendimento",
  [string[]]$Services = @("api", "web"),
  [switch]$NoBackup,       # pula o backup remoto do PostgreSQL
  [switch]$DeleteRemoved,  # remove na VPS os arquivos que nao existem mais localmente
  [switch]$DryRun,         # so mostra o que mudaria, nao envia nada
  [switch]$ForceRecreate
)

# Deploy incremental para a VPS: sobe SO os arquivos novos/alterados (diff por
# tamanho + mtime contra um manifest remoto), em vez de reenviar o workspace
# inteiro como o deploy-vps-fast.ps1. Espelha o modelo do crow-php, adaptado
# para esta stack (Postgres + docker-compose.prod.yml, servicos api/web).
#
# IMPORTANTE: o codigo e' copiado para dentro da imagem no Dockerfile (Go bina-
# rio + build Nuxt), entao depois de sincronizar os arquivos alterados ainda e'
# necessario `up -d --build` para a imagem incorporar a mudanca. O build pesado
# do Nuxt continua acontecendo na VPS — este script encurta o ENVIO, nao o build.

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoDir = (Resolve-Path (Join-Path $scriptDir "..\..")).Path

if (-not (Test-Path $KeyPath)) {
  throw "Chave SSH nao encontrada em $KeyPath"
}

$TarExe = Join-Path $env:SystemRoot "System32\tar.exe"
$SshExe = Join-Path $env:SystemRoot "System32\OpenSSH\ssh.exe"
$ScpExe = Join-Path $env:SystemRoot "System32\OpenSSH\scp.exe"

if (-not (Test-Path $TarExe)) { throw "tar.exe nao encontrado em $TarExe" }
if (-not (Test-Path $SshExe)) { throw "ssh.exe nao encontrado em $SshExe" }
if (-not (Test-Path $ScpExe)) { throw "scp.exe nao encontrado em $ScpExe" }

$resolvedKeyPath = (Resolve-Path $KeyPath).Path
# Opcoes anti-hang: nunca pedir senha (BatchMode), timeout de conexao e keepalive
# para abortar conexoes mortas em vez de pendurar indefinidamente.
$sshHardening = @(
  "-o", "BatchMode=yes",
  "-o", "ConnectTimeout=15",
  "-o", "ServerAliveInterval=10",
  "-o", "ServerAliveCountMax=3"
)
$sshArgs = @("-i", $resolvedKeyPath, "-o", "StrictHostKeyChecking=accept-new", "-p", $Port.ToString()) + $sshHardening
$remoteTarget = "$User@$VpsHost"
$composeArgs = "--env-file .env.production -f docker-compose.prod.yml"
$serviceArgs = if ($Services.Count -gt 0) { " " + ($Services -join " ") } else { "" }
$forceRecreateFlag = if ($ForceRecreate) { " --force-recreate" } else { "" }

# --- Regras de exclusao (espelhadas entre local e remoto) ----------------------
# Mesmo conjunto do deploy-vps-fast.ps1 (payload identico e ja validado), apenas
# o transporte muda. NAO excluir *.sql: os arquivos sao migrations que precisam
# subir e serem embutidas no binario Go durante o build.

# Diretorios que nunca entram no deploy (casa por NOME em qualquer profundidade).
$ExcludedDirs = @(
  ".git", ".claude", ".playwright-mcp",
  "node_modules", ".nuxt", ".output", "dist", ".logs",
  ".venv", "artifacts",
  "erp-source-local", "Controlle10 - ftp", "tmp",
  "backups", "dumps"
)

# Arquivos que nunca entram (segredos e artefatos locais).
$ExcludedFiles = @(
  ".env", ".env.production", ".env.local", ".DS_Store",
  "token.txt", "token_gen.js", "verify.sh"
)

function Test-DeployExcluded {
  param([Parameter(Mandatory = $true)][string]$RelativePath)

  $p = $RelativePath.Replace("\", "/").TrimStart("./")
  if ($p -eq "") { return $true }

  $segments = $p.Split("/")
  if ($segments.Count -gt 1) {
    $dirSegments = $segments[0..($segments.Count - 2)]
    foreach ($seg in $dirSegments) {
      if ($ExcludedDirs -contains $seg) { return $true }
    }
  }

  $leaf = $segments[-1]
  if ($ExcludedFiles -contains $leaf) { return $true }
  # web/.codex-devserver.<pid>.log
  if ($leaf -like ".codex-devserver.*.log") { return $true }

  return $false
}

function Convert-ToBashSingleQuoted {
  param([Parameter(Mandatory = $true)][string]$Value)
  $replacement = "'" + '"' + "'" + '"' + "'"
  return "'" + $Value.Replace("'", $replacement) + "'"
}

function Invoke-RemoteCommand {
  param(
    [Parameter(Mandatory = $true)][string]$Description,
    [Parameter(Mandatory = $true)][string]$Command,
    [switch]$CaptureOutput
  )

  Write-Host "==> $Description"
  $normalizedCommand = $Command -replace "`r`n", "`n" -replace "`r", "`n"

  if ($CaptureOutput) {
    $output = & $SshExe @sshArgs $remoteTarget $normalizedCommand
    if ($LASTEXITCODE -ne 0) { throw "Falha ao executar: $Description" }
    return (($output | ForEach-Object { $_.ToString() }) -join "`n").Trim()
  }

  & $SshExe @sshArgs $remoteTarget $normalizedCommand
  if ($LASTEXITCODE -ne 0) { throw "Falha ao executar: $Description" }
}

# --- Enumeracao local com poda de diretorios pesados ---------------------------

function Get-LocalDeployFiles {
  $results = New-Object System.Collections.Generic.List[object]
  $stack = New-Object System.Collections.Generic.Stack[string]
  $stack.Push($repoDir)
  $repoPrefixLen = $repoDir.TrimEnd("\").Length + 1

  while ($stack.Count -gt 0) {
    $dir = $stack.Pop()
    foreach ($entry in [IO.Directory]::EnumerateFileSystemEntries($dir)) {
      $name = [IO.Path]::GetFileName($entry)
      if ([IO.Directory]::Exists($entry)) {
        if ($ExcludedDirs -contains $name) { continue }
        $stack.Push($entry)
      } else {
        $rel = $entry.Substring($repoPrefixLen).Replace("\", "/")
        if (Test-DeployExcluded $rel) { continue }
        $info = [IO.FileInfo]::new($entry)
        $results.Add([pscustomobject]@{
          Path     = $rel
          FullName = $entry
          Length   = $info.Length
          Mtime    = [int64]([DateTimeOffset]$info.LastWriteTimeUtc).ToUnixTimeSeconds()
        })
      }
    }
  }

  return $results
}

# --- Manifest remoto (find com prune por nome de diretorio) --------------------

function Get-RemoteManifest {
  $remotePathQ = Convert-ToBashSingleQuoted $RemotePath

  $pruneExpr = ($ExcludedDirs | ForEach-Object { "-name '$_'" }) -join " -o "
  $fileExcludes = ($ExcludedFiles | ForEach-Object { "! -name '$_'" }) -join " "

  $cmd = @"
if [ -d $remotePathQ ]; then
  cd $remotePathQ &&
  find . \( -type d \( $pruneExpr \) -prune \) -o \( -type f $fileExcludes ! -name '.codex-devserver.*.log' -printf '%P\t%s\t%T@\n' \)
fi
"@

  $manifest = @{}
  $raw = Invoke-RemoteCommand -Description "Lendo manifest remoto" -Command $cmd -CaptureOutput
  if ([string]::IsNullOrWhiteSpace($raw)) { return $manifest }

  foreach ($line in ($raw -split "`n")) {
    $parts = $line -split "`t"
    if ($parts.Count -lt 3) { continue }
    $manifest[$parts[0]] = [pscustomobject]@{
      Length = [int64]$parts[1]
      Mtime  = [int64][math]::Floor([double]::Parse($parts[2], [Globalization.CultureInfo]::InvariantCulture))
    }
  }

  return $manifest
}

# --- Backup remoto do PostgreSQL -----------------------------------------------

function Invoke-RemoteBackup {
  $backupCommand = @'
mkdir -p "__REMOTE_PATH__/backups" &&
cd "__REMOTE_PATH__" &&
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  sh -lc 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB"' | gzip > \
  "backups/backup_$(date +%Y%m%d_%H%M%S).sql.gz" &&
latest=$(ls -t backups | head -n 1) &&
printf "%s\n" "__REMOTE_PATH__/backups/$latest"
'@
  $backupCommand = $backupCommand.Replace("__REMOTE_PATH__", $RemotePath)
  return Invoke-RemoteCommand -Description "Gerando backup remoto do PostgreSQL" -Command $backupCommand -CaptureOutput
}

# --- Empacotamento e envio dos arquivos alterados ------------------------------

function New-ChangedArchive {
  param([Parameter(Mandatory = $true)][array]$Files)

  # Staging por hardlink: o bsdtar do Windows nao casa nomes acentuados/decompostos via
  # lista -T. Montamos uma arvore espelho com hardlinks (instantaneos, sem copiar bytes)
  # e deixamos o tar enumerar essa arvore, o que preserva os nomes Unicode exatos.
  # Hardlink exige mesma particao do repo.
  $repoRoot = [IO.Path]::GetPathRoot($repoDir)
  $tempBase = if ([IO.Path]::GetPathRoot($env:TEMP) -ieq $repoRoot) { $env:TEMP } else { $repoRoot }
  $tempRoot = Join-Path $tempBase ("omni-vps-incremental-" + [Guid]::NewGuid().ToString("N"))
  New-Item -ItemType Directory -Path $tempRoot | Out-Null
  $stageDir = Join-Path $tempRoot "stage"
  $archive = Join-Path $tempRoot "changed.tar.gz"

  try {
    New-Item -ItemType Directory -Path $stageDir | Out-Null
    $sepChar = [IO.Path]::DirectorySeparatorChar
    $linked = 0
    foreach ($file in $Files) {
      $linked++
      $target = Join-Path $stageDir ($file.Path -replace '/', $sepChar)
      $targetDir = Split-Path -Parent $target
      if (-not (Test-Path -LiteralPath $targetDir)) { New-Item -ItemType Directory -Path $targetDir -Force | Out-Null }
      try {
        [void](New-Item -ItemType HardLink -Path $target -Target $file.FullName -ErrorAction Stop)
      } catch {
        # fallback (ex.: particao diferente): copia o arquivo
        Copy-Item -LiteralPath $file.FullName -Destination $target -Force
      }
      if (($linked % 50) -eq 0 -or $linked -eq $Files.Count) {
        $percent = [math]::Round(($linked / $Files.Count) * 100, 1)
        Write-Progress -Activity "Montando pacote incremental" -Status "$percent% preparado ($linked/$($Files.Count))" -PercentComplete $percent
      }
    }
    Write-Progress -Activity "Montando pacote incremental" -Completed

    Write-Host "==> Compactando pacote incremental ($($Files.Count) arquivos)"
    & $TarExe -czf $archive -C $stageDir .
    if ($LASTEXITCODE -ne 0) { throw "Falha ao criar pacote incremental." }

    return [pscustomobject]@{ TempRoot = $tempRoot; Archive = $archive }
  } catch {
    if (Test-Path -LiteralPath $tempRoot) { Remove-Item -LiteralPath $tempRoot -Recurse -Force }
    throw
  }
}

function Send-ArchiveWithProgress {
  param([Parameter(Mandatory = $true)][string]$ArchivePath)

  $archiveSize = (Get-Item -LiteralPath $ArchivePath).Length
  Write-Host "==> Enviando pacote incremental para a VPS"
  Write-Host ("Pacote: {0:N2} MB" -f ($archiveSize / 1MB))

  # scp lida com binario de forma confiavel (o streaming via stdin do ssh corrompe o gzip
  # no Windows). Copiamos para /tmp da VPS, extraimos e removemos o pacote.
  $remoteTmp = "/tmp/omni-deploy-" + [Guid]::NewGuid().ToString("N") + ".tar.gz"
  $scpArgs = @("-i", $resolvedKeyPath, "-o", "StrictHostKeyChecking=accept-new", "-P", $Port.ToString()) + $sshHardening + @($ArchivePath, "${remoteTarget}:${remoteTmp}")

  & $ScpExe @scpArgs
  if ($LASTEXITCODE -ne 0) { throw "Falha ao enviar o pacote incremental (scp)." }

  $remoteTmpQ = Convert-ToBashSingleQuoted $remoteTmp
  $remotePathQ = Convert-ToBashSingleQuoted $RemotePath
  $extractCmd = "set -euo pipefail; mkdir -p $remotePathQ && tar -xzf $remoteTmpQ -C $remotePathQ && rm -f $remoteTmpQ"
  Invoke-RemoteCommand -Description "Extraindo pacote incremental na VPS" -Command $extractCmd
}

function Invoke-DeleteRemoved {
  param(
    [Parameter(Mandatory = $true)][hashtable]$RemoteManifest,
    [Parameter(Mandatory = $true)][hashtable]$LocalManifest
  )

  $removed = @($RemoteManifest.Keys | Where-Object { -not $LocalManifest.ContainsKey($_) })
  if ($removed.Count -eq 0) { Write-Host "Nenhum arquivo remoto para remover."; return }

  Write-Host "==> Removendo arquivos ausentes localmente: $($removed.Count)"
  $deleteLines = $removed | ForEach-Object { "rm -f -- " + (Convert-ToBashSingleQuoted ("$RemotePath/" + $_)) }
  Invoke-RemoteCommand -Description "Aplicando limpeza incremental remota" -Command (($deleteLines -join "`n") + "`n")
}

# --- Fluxo principal -----------------------------------------------------------

# 1. Manifest remoto primeiro (nao precisa de backup pra so comparar).
$remoteManifest = Get-RemoteManifest

# 2. Enumeracao local + diff por tamanho/mtime.
$localFiles = @(Get-LocalDeployFiles)
$localManifest = @{}
foreach ($file in $localFiles) { $localManifest[$file.Path] = $file }

$changed = New-Object System.Collections.Generic.List[object]
foreach ($file in $localFiles) {
  $remote = $remoteManifest[$file.Path]
  if (-not $remote -or $remote.Length -ne $file.Length -or ($file.Mtime - $remote.Mtime) -gt 2) {
    $changed.Add($file)
  }
}
$changed = $changed.ToArray()

$changedMB = if ($changed.Count -gt 0) { ($changed | Measure-Object Length -Sum).Sum / 1MB } else { 0 }
Write-Host "Arquivos locais considerados: $($localFiles.Count)"
Write-Host "Arquivos novos/alterados:     $($changed.Count)"
Write-Host ("Tamanho a enviar:             {0:N2} MB" -f $changedMB)

if ($DryRun) {
  Write-Host ""
  Write-Host "[DryRun] Nada foi enviado. Arquivos que subiriam:"
  $changed | Sort-Object Path | ForEach-Object { Write-Host ("  {0}  ({1:N0} KB)" -f $_.Path, ($_.Length / 1KB)) }
  if ($DeleteRemoved) {
    $removed = @($remoteManifest.Keys | Where-Object { -not $localManifest.ContainsKey($_) })
    Write-Host "[DryRun] Arquivos que seriam removidos da VPS: $($removed.Count)"
    $removed | Sort-Object | ForEach-Object { Write-Host "  - $_" }
  }
  return
}

# 3. Backup (opcional) so depois de saber que ha o que fazer.
if ($NoBackup) {
  Write-Host "==> Backup remoto pulado (-NoBackup)."
} elseif ($changed.Count -gt 0 -or $DeleteRemoved) {
  $backupFile = Invoke-RemoteBackup
  Write-Host "Backup remoto: $backupFile"
} else {
  Write-Host "==> Sem mudancas; backup nao necessario."
}

# 4. Envio incremental.
if ($changed.Count -gt 0) {
  $archiveInfo = New-ChangedArchive -Files $changed
  try {
    Send-ArchiveWithProgress -ArchivePath $archiveInfo.Archive
  } finally {
    if (Test-Path $archiveInfo.TempRoot) { Remove-Item -LiteralPath $archiveInfo.TempRoot -Recurse -Force }
  }
} else {
  Write-Host "Nada novo para sincronizar."
}

# 5. Limpeza opcional de arquivos removidos.
if ($DeleteRemoved) {
  Invoke-DeleteRemoved -RemoteManifest $remoteManifest -LocalManifest $localManifest
}

# 6. Rebuild/up (necessario: o codigo e copiado pra dentro da imagem no Dockerfile).
if ($changed.Count -gt 0 -or $DeleteRemoved -or $ForceRecreate) {
  $remotePathQ = Convert-ToBashSingleQuoted $RemotePath
  $deployCommand = "cd $remotePathQ && docker compose $composeArgs up -d --build$forceRecreateFlag$serviceArgs && docker compose $composeArgs ps$serviceArgs"
  Invoke-RemoteCommand -Description "Subindo servicos na VPS" -Command $deployCommand
} else {
  Write-Host "==> Sem mudancas; pulando rebuild dos containers."
}

Write-Host ""
Write-Host "Deploy incremental finalizado com sucesso."
if ($backupFile) {
  Write-Host "Backup preservado em: $backupFile"
}
