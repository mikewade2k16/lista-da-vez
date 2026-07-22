<#
.SYNOPSIS
  Checks local n8n workflow ownership or exports one exact owner-scoped workflow.

.DESCRIPTION
  -Check without Owner/Only is LOCAL ONLY: validates registry, canonical IDs and
  SHA-256 hashes. It never calls Docker, reads runtime or creates a temp file.

  Every runtime check, export or sync requires both -Owner and -Only. The exact
  registry entry controls module, canonical ID and export path. Non-selected
  canonical workflows are protected by before/after hashes.

  Exit codes: 0=ok, 10=target drift, 20=registry/integrity/policy error,
  30=runtime/container operational error.

  Keep this file ASCII and PowerShell 5.1 compatible.

.EXAMPLE
  npm run n8n:export:check
  npm run n8n:export:chat
  npm run n8n:export:omnichannel-brain:check
  npm run n8n:export:omnichannel-brain:sync
#>
param(
  [string]$Container = "omni-n8n-1",
  [string]$Only = "",
  [string]$Owner = "",
  [switch]$Check,
  [switch]$Sync
)

$ErrorActionPreference = "Stop"
try {
if ($Check -and $Sync) { throw "-Check e -Sync sao mutuamente exclusivos." }

$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$registryPath = Join-Path $PSScriptRoot "n8n-workflow-registry.ps1"
$normalizerPath = Join-Path $PSScriptRoot "n8n-workflow-normalize.js"
if (-not (Test-Path -LiteralPath $registryPath -PathType Leaf)) {
  throw "Registro compartilhado n8n ausente: $registryPath"
}
. $registryPath

$registry = @(Get-N8nWorkflowRegistry)

# The only unscoped mode is a deterministic local inventory check. Return before
# resolving a target, checking Docker or allocating a temporary directory.
if ($Check -and [string]::IsNullOrWhiteSpace($Owner) -and [string]::IsNullOrWhiteSpace($Only)) {
  $inventory = @(Get-N8nWorkflowInventoryHashes -Root $root -Registry $registry)
  foreach ($row in $inventory) {
    Write-Host "LOCAL key=$($row.Key) owner=$($row.Module) id=$($row.WorkflowId) sha256=$($row.Sha256)"
  }
  Write-Host "n8n-export: inventario local valido ($($inventory.Count) workflows canonicos); runtime nao consultado."
  exit 0
}

if ([string]::IsNullOrWhiteSpace($Owner) -or [string]::IsNullOrWhiteSpace($Only)) {
  throw "Operacao de runtime/export exige -Owner e -Only explicitos. Use -Check sozinho apenas para inventario local."
}
if (-not (Test-Path -LiteralPath $normalizerPath -PathType Leaf)) {
  throw "Normalizador n8n ausente: $normalizerPath"
}

$selection = @(Resolve-N8nWorkflowSelection -Registry $registry -Only $Only -Owner $Owner -RequireWritable)
Assert-N8nWorkflowLocalInventory -Root $root -Registry $registry
$protectedHashes = Get-N8nProtectedWorkflowHashes -Root $root -Registry $registry -Selected $selection
Write-Host "n8n-export: owner-scoped owner='$Owner', alvo='$($selection[0].Key)'."

# Ownership, exact selection and local IDs are validated before the first Docker call.
$psId = docker ps -q -f "name=^${Container}$" 2>$null
if (-not $psId) {
  throw "Container n8n '$Container' nao esta rodando. Suba com: docker compose --profile automation up -d n8n"
}

$tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("n8n-export-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null
$mode = if ($Check) { "check" } else { "write" }
$drift = $false
$exported = 0
$remoteCleanup = @()

try {
  foreach ($entry in $selection) {
    $id = "$($entry.WorkflowId)"
    $target = Join-Path $root "$($entry.ExportPath)"
    $fileName = [System.IO.Path]::GetFileName($target)
    $remoteExport = "/tmp/exp_$id.json"
    $localRaw = Join-Path $tmpDir "$id.json"
    $remoteCleanup += $remoteExport

    $runtimeRc = Invoke-N8nCommandSilently { & docker exec $Container sh -lc "n8n export:workflow --id=$id --output=$remoteExport >/dev/null 2>&1" }
    if ($runtimeRc -ne 0) {
      throw "Workflow runtime '$($entry.Key)' com ID '$id' ausente ou inacessivel (docker exit $runtimeRc)."
    }

    $copyRc = Invoke-N8nCommandSilently { & docker cp "${Container}:${remoteExport}" "$localRaw" }
    if ($copyRc -ne 0 -or -not (Test-Path -LiteralPath $localRaw -PathType Leaf)) {
      throw "Falha ao ler runtime de '$($entry.Key)' (docker cp exit $copyRc)."
    }
    Assert-N8nWorkflowDocumentId -Path $localRaw -ExpectedId $id -Label "workflow runtime '$($entry.Key)'"

    if ($Sync) {
      $status = & node $normalizerPath $localRaw $target "check" $id "$($entry.Module)"
      $checkRc = $LASTEXITCODE
      if ($checkRc -ne 0) { throw "Normalizacao segura de '$fileName' falhou (exit $checkRc)." }
      if ("$status".Contains("drift")) {
        $writeStatus = & node $normalizerPath $localRaw $target "write" $id "$($entry.Module)"
        $writeRc = $LASTEXITCODE
        if ($writeRc -ne 0) { throw "Escrita normalizada de '$fileName' falhou (exit $writeRc)." }
        if ("$writeStatus".Contains("written")) {
          $exported++
          Write-Host "   exportado: $fileName"
        }
      }
    }
    else {
      $status = & node $normalizerPath $localRaw $target $mode $id "$($entry.Module)"
      $normalizerRc = $LASTEXITCODE
      if ($normalizerRc -ne 0) { throw "Normalizacao segura de '$fileName' falhou (exit $normalizerRc)." }
      if ($Check) {
        if ("$status".Contains("drift")) {
          $drift = $true
          Write-Host "   DIVERGE: $fileName (owner=$($entry.Module))"
        }
      }
      elseif ("$status".Contains("written")) {
        Write-Host "   exportado: $fileName"
      }
      else {
        Write-Host "   ja atual: $fileName"
      }
    }
  }
}
finally {
  $cleanupFailed = $false
  foreach ($remotePath in @($remoteCleanup | Select-Object -Unique)) {
    $cleanupRc = Invoke-N8nCommandSilently { & docker exec $Container sh -lc "rm -f -- '$remotePath'" }
    if ($cleanupRc -ne 0) { $cleanupFailed = $true }
  }
  Remove-Item -LiteralPath $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
  Assert-N8nProtectedWorkflowHashes -Before $protectedHashes
  if ($cleanupFailed) { throw "Falha ao remover artefato temporario sensivel do container n8n." }
}

if ($Check) {
  if ($drift) {
    Write-Host "n8n-export: alvo diverge do runtime. Execute o alias :sync owner-scoped correspondente."
    exit 10
  }
  Write-Host "n8n-export: alvo alinhado com o runtime."
  exit 0
}

if ($Sync) {
  if ($exported -gt 0) {
    Write-Host "n8n-export: alvo owner-scoped sincronizado; arquivo modificado deve ser revisado e commitado pelo dono."
  }
  else {
    Write-Host "n8n-export: alvo owner-scoped ja estava alinhado."
  }
  exit 0
}

Write-Host "OK: export owner-scoped concluido."
exit 0
}
catch {
  $safeMessage = "$($_.Exception.Message)"
  Write-Host "n8n-export ERRO: $safeMessage"
  if ($safeMessage -match '(?i)ID divergente|invalido|shape|registro|ownership|normaliza|proibido|exige -Owner|exige -Only|mutuamente') { exit 20 }
  if ($safeMessage -match '(?i)runtime|container n8n|docker') { exit 30 }
  exit 20
}
