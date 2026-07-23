<#
.SYNOPSIS
  Importa os workflows de automation/export/ no n8n do dev, reativa e reinicia.

.DESCRIPTION
  POR QUE existe: o n8n guarda os workflows no PROPRIO banco (nao le do arquivo). Entao, toda vez
  que um workflow em automation/export/workflow-*.json muda, o n8n continua rodando a versao ANTIGA
  ate re-importar. Este script faz o ciclo completo num comando so, para ninguem precisar lembrar do
  passo manual:
    1. copia o(s) arquivo(s) para dentro do container;
    2. n8n import:workflow (atualiza pelo id que ja esta no JSON - sem duplicar);
    3. reativa (o import DESATIVA o workflow, pois o JSON exportado vem com "active": false);
    4. reinicia o n8n UMA vez (o webhook so re-registra depois do restart).

  Gotchas embutidos (ja custaram tempo):
    - Rodar o n8n CLI via: docker exec <c> sh -lc "... /tmp/...". Passar /tmp/x direto como
      argumento faz o Git Bash/MSYS converter para caminho Windows (ENOENT no container).
    - O SQLite do n8n usa WAL: docker cp database.sqlite NAO leva as escritas recentes; para
      CONFERIR o banco use n8n export:workflow (le com o WAL aplicado), nao o cp.
    - Este arquivo e SO ASCII de proposito (Windows PowerShell 5.1 le .ps1 sem BOM como ANSI e
      quebra o parser em acentos/travessao).

.EXAMPLE
  npm run n8n:import:chat     # calendar-chat, owner+key explicitos no alias
  npm run n8n:import:omnichannel-brain
  pwsh scripts/dev/n8n-import.ps1 -Owner omnichannel -Only omnichannel-brain
#>
param(
  [string]$Container = "omni-n8n-1",
  [string]$Only = "",
  [string]$Owner = "",
  [switch]$NoRestart
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$registryPath = Join-Path $PSScriptRoot "n8n-workflow-registry.ps1"
$normalizerPath = Join-Path $PSScriptRoot "n8n-workflow-normalize.js"
if (-not (Test-Path -LiteralPath $registryPath -PathType Leaf)) {
  throw "Registro compartilhado n8n ausente: $registryPath"
}
if (-not (Test-Path -LiteralPath $normalizerPath -PathType Leaf)) {
  throw "Normalizador n8n ausente: $normalizerPath"
}
. $registryPath

$registry = @(Get-N8nWorkflowRegistry)
$selection = @(Resolve-N8nWorkflowSelection -Registry $registry -Only $Only -Owner $Owner -RequireWritable)
Assert-N8nWorkflowLocalInventory -Root $root -Registry $registry
$protectedHashes = Get-N8nProtectedWorkflowHashes -Root $root -Registry $registry -Selected $selection
Write-Host "n8n-import: owner-scoped owner='$Owner', alvo='$($selection[0].Key)'."

# Ownership, exact selection, local IDs and portable policy are validated before
# the first Docker call. Check mode compares the canonical projection with the same
# file, so anything that would be cleaned/rejected by export blocks import instead.
$selectedEntry = $selection[0]
$selectedLocalPath = Join-Path $root "$($selectedEntry.ExportPath)"
$policyStatus = & node $normalizerPath $selectedLocalPath $selectedLocalPath "check" "$($selectedEntry.WorkflowId)" "$($selectedEntry.Module)" 2>&1
$policyRc = $LASTEXITCODE
if ($policyRc -ne 0 -or -not "$policyStatus".Contains("STATUS:unchanged")) {
  throw "Workflow local '$($selectedEntry.Key)' viola a politica portavel/segura; normalize pelo export owner-scoped antes do import."
}

# Only after every local guard passes may the script inspect Docker/runtime.
$psId = docker ps -q -f "name=^${Container}$" 2>$null
if (-not $psId) {
  throw "Container n8n '$Container' nao esta rodando. Suba com: docker compose --profile automation up -d n8n"
}

$tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("n8n-import-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null
$ids = @()
$remoteCleanup = @()

try {
  foreach ($entry in $selection) {
    $id = "$($entry.WorkflowId)"
    $localPath = Join-Path $root "$($entry.ExportPath)"
    $fileName = [System.IO.Path]::GetFileName($localPath)
    $baseName = [System.IO.Path]::GetFileNameWithoutExtension($localPath)
    Assert-N8nWorkflowDocumentId -Path $localPath -ExpectedId $id -Label "workflow local '$($entry.Key)'"

    # Runtime must already expose the same canonical ID. Import never guesses or bootstraps
    # a missing workflow because that could create a duplicate under another owner.
    $runtimeRemote = "/tmp/n8npreflight_$id.json"
    $runtimeLocal = Join-Path $tmpDir ("runtime_" + $id + ".json")
    $remoteCleanup += $runtimeRemote
    $runtimeRc = Invoke-N8nCommandSilently { & docker exec $Container sh -lc "n8n export:workflow --id=$id --output=$runtimeRemote" }
    if ($runtimeRc -eq 0) {
      $copyRc = Invoke-N8nCommandSilently { & docker cp "${Container}:${runtimeRemote}" "$runtimeLocal" }
      if ($copyRc -ne 0 -or -not (Test-Path -LiteralPath $runtimeLocal -PathType Leaf)) {
        throw "Falha ao ler runtime de '$($entry.Key)' (docker cp exit $copyRc)."
      }
      Assert-N8nWorkflowDocumentId -Path $runtimeLocal -ExpectedId $id -Label "workflow runtime '$($entry.Key)'"
    }
    else {
      throw "Workflow runtime '$($entry.Key)' com ID '$id' ausente ou inacessivel (docker exit $runtimeRc). Operacao owner-scoped abortada."
    }

    $remoteImport = "/tmp/n8nimport_$baseName.json"
    $remoteCleanup += $remoteImport
    Write-Host "==> $fileName (owner=$($entry.Module), id=$id)"
    $copyImportRc = Invoke-N8nCommandSilently { & docker cp "$localPath" "${Container}:${remoteImport}" }
    if ($copyImportRc -ne 0) { throw "Falha no docker cp de '$fileName' (exit $copyImportRc)." }

    $importRc = Invoke-N8nCommandSilently { & docker exec $Container sh -lc "n8n import:workflow --input=$remoteImport" }
    if ($importRc -ne 0) { throw "Import de '$fileName' falhou (docker exit $importRc)." }
    Write-Host "   importado: $fileName"
    $ids += $id
  }

  foreach ($id in $ids) {
    $activateRc = Invoke-N8nCommandSilently { & docker exec $Container sh -lc "n8n update:workflow --id=$id --active=true" }
    if ($activateRc -ne 0) { throw "Falha ao reativar workflow '$id' (docker exit $activateRc)." }
    Write-Host "   reativado: $id"
  }

  if ($NoRestart) {
    Write-Host "OK: importado(s) + reativado(s). Restart pulado (rode: docker compose restart n8n)."
    return
  }

  Write-Host "==> reiniciando n8n (re-registra os webhooks)..."
  Push-Location $root
  try {
    $restartRc = Invoke-N8nCommandSilently { & docker compose restart n8n }
    if ($restartRc -ne 0) { throw "Falha ao reiniciar n8n (docker exit $restartRc)." }
  }
  finally { Pop-Location }
  Write-Host "OK: workflow(s) importado(s), reativado(s) e n8n reiniciado."
}
finally {
  $cleanupFailed = $false
  foreach ($remotePath in @($remoteCleanup | Select-Object -Unique)) {
    # docker cp materializa o arquivo como root; o container roda n8n como node.
    # Limpar com o usuario default falha com EPERM depois do import bem-sucedido.
    $cleanupRc = Invoke-N8nCommandSilently { & docker exec -u root $Container sh -lc "rm -f -- '$remotePath'" }
    if ($cleanupRc -ne 0) { $cleanupFailed = $true }
  }
  Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue
  Assert-N8nProtectedWorkflowHashes -Before $protectedHashes
  if ($cleanupFailed) { throw "Falha ao remover artefato temporario sensivel do container n8n." }
}
