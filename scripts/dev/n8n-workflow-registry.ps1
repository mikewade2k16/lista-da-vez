<#
.SYNOPSIS
  Shared ownership registry and fail-closed helpers for n8n workflow scripts.

.DESCRIPTION
  This file is dot-sourced by n8n-import.ps1 and n8n-export.ps1. Keep it ASCII and
  PowerShell 5.1 compatible. The registry is the only place where workflow keys,
  owners, canonical IDs and export paths are declared for these scripts.
#>

function Get-N8nWorkflowRegistry {
  return @(
    [PSCustomObject]@{
      Key = "calendar-chat"
      Module = "calendar"
      WorkflowId = "calendarchat0001"
      ExportPath = "automation/export/workflow-calendar-chat.json"
      Writable = $true
    },
    [PSCustomObject]@{
      Key = "calendar-omni"
      Module = "calendar"
      WorkflowId = "calendaromni0001"
      ExportPath = "automation/export/workflow-calendar-omni.json"
      Writable = $true
    },
    [PSCustomObject]@{
      Key = "calendar-transcribe"
      Module = "calendar"
      WorkflowId = "calendartrans001"
      ExportPath = "automation/export/workflow-calendar-transcribe.json"
      Writable = $true
    },
    [PSCustomObject]@{
      Key = "instagram-first-contact"
      Module = "omnichannel"
      WorkflowId = "instafirst000001"
      ExportPath = "automation/export/workflow-instagram-first-contact.json"
      Writable = $true
    },
    [PSCustomObject]@{
      Key = "omnichannel-brain"
      Module = "omnichannel"
      WorkflowId = "omnibrain0000001"
      ExportPath = "automation/export/workflow-omnichannel-brain.json"
      Writable = $true
    },
    [PSCustomObject]@{
      Key = "omni-chat"
      Module = "operation"
      WorkflowId = "omnichatmvp00001"
      ExportPath = "automation/export/workflow-omni-chat.json"
      Writable = $true
    },
    [PSCustomObject]@{
      Key = "whatsapp"
      Module = "automation"
      WorkflowId = "lzhb5JjN5kdcVuRR"
      ExportPath = "automation/export/workflow-whatsapp.json"
      Writable = $true
    }
  )
}

function Assert-N8nWorkflowRegistry {
  param(
    [Parameter(Mandatory = $true)]
    [object[]]$Registry
  )

  if (-not $Registry -or $Registry.Count -eq 0) {
    throw "Registro n8n vazio. Operacao abortada."
  }

  $keys = @{}
  $ids = @{}
  $paths = @{}
  foreach ($entry in $Registry) {
    $key = "$($entry.Key)".Trim()
    $module = "$($entry.Module)".Trim()
    $workflowId = "$($entry.WorkflowId)".Trim()
    $exportPath = "$($entry.ExportPath)".Trim().Replace("\", "/")

    if (-not $key -or -not $module -or -not $workflowId -or -not $exportPath) {
      throw "Entrada incompleta no registro n8n. key/module/workflowId/exportPath sao obrigatorios."
    }
    if ($key -notmatch '^[a-z0-9][a-z0-9-]*$' -or $module -notmatch '^[a-z0-9][a-z0-9-]*$' -or
        $workflowId -notmatch '^[A-Za-z0-9_-]+$') {
      throw "Key/module/workflowId inseguro no registro n8n para '$key'."
    }
    if ($null -eq $entry.Writable -or $entry.Writable -isnot [bool]) {
      throw "Entrada '$key' sem writable booleano explicito."
    }
    if ([System.IO.Path]::IsPathRooted($exportPath) -or $exportPath.Contains("..")) {
      throw "ExportPath inseguro no registro n8n para '$key': $exportPath"
    }
    if ($exportPath -notmatch '^automation/export/workflow-[a-z0-9-]+\.json$') {
      throw "ExportPath fora do inventario canonico para '$key': $exportPath"
    }

    $keyIndex = $key.ToLowerInvariant()
    $idIndex = $workflowId.ToLowerInvariant()
    $pathIndex = $exportPath.ToLowerInvariant()
    if ($keys.ContainsKey($keyIndex)) { throw "Key n8n duplicada no registro: $key" }
    if ($ids.ContainsKey($idIndex)) { throw "WorkflowId n8n duplicado no registro: $workflowId" }
    if ($paths.ContainsKey($pathIndex)) { throw "ExportPath n8n duplicado no registro: $exportPath" }
    $keys[$keyIndex] = $true
    $ids[$idIndex] = $true
    $paths[$pathIndex] = $true
  }
}

function Resolve-N8nWorkflowSelection {
  param(
    [Parameter(Mandatory = $true)]
    [object[]]$Registry,
    [string]$Only = "",
    [string]$Owner = "",
    [switch]$RequireWritable
  )

  Assert-N8nWorkflowRegistry -Registry $Registry
  $selector = "$Only".Trim()
  $ownerName = "$Owner".Trim()

  if (-not $ownerName) {
    throw "-Owner explicito e obrigatorio para operacoes n8n com alvo."
  }
  if (-not $selector) {
    throw "Owner '$ownerName' exige -Only exato. Operacao global owner-scoped e proibida."
  }

  $matches = @($Registry | Where-Object {
    $leaf = [System.IO.Path]::GetFileName("$($_.ExportPath)")
    $base = [System.IO.Path]::GetFileNameWithoutExtension($leaf)
    $selector.Equals("$($_.Key)", [System.StringComparison]::OrdinalIgnoreCase) -or
      $selector.Equals("$($_.WorkflowId)", [System.StringComparison]::OrdinalIgnoreCase) -or
      $selector.Equals("$($_.ExportPath)", [System.StringComparison]::OrdinalIgnoreCase) -or
      $selector.Equals($leaf, [System.StringComparison]::OrdinalIgnoreCase) -or
      $selector.Equals($base, [System.StringComparison]::OrdinalIgnoreCase)
  })

  if ($matches.Count -ne 1) {
    throw "Seletor n8n '$selector' desconhecido ou ambiguo. Use key, workflowId ou exportPath exato do registro."
  }

  $selected = $matches[0]
  if ($ownerName -and -not $ownerName.Equals("$($selected.Module)", [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Ownership negado: owner '$ownerName' nao pode operar '$($selected.Key)' (owner canonico '$($selected.Module)')."
  }
  if ($RequireWritable -and -not [bool]$selected.Writable) {
    throw "Workflow '$($selected.Key)' esta marcado como writable=false. Operacao abortada."
  }

  return @($selected)
}

function Read-N8nWorkflowDocument {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Path,
    [string]$Label = "workflow"
  )

  if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
    throw "$Label ausente: $Path"
  }
  try {
    $parsed = Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
  }
  catch {
    throw "$Label invalido em '$Path'."
  }

  if ($parsed -is [System.Array]) {
    if ($parsed.Count -ne 1) { throw "$Label deve conter exatamente um workflow: $Path" }
    $workflow = $parsed[0]
  }
  else {
    $workflow = $parsed
  }
  if ($null -eq $workflow -or $workflow -isnot [PSCustomObject]) {
    throw "$Label vazio ou com shape invalido: $Path"
  }
  return $workflow
}

function Assert-N8nWorkflowDocumentId {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Path,
    [Parameter(Mandatory = $true)]
    [string]$ExpectedId,
    [string]$Label = "workflow"
  )

  $workflow = Read-N8nWorkflowDocument -Path $Path -Label $Label
  $actualId = "$($workflow.id)".Trim()
  if (-not $actualId) { throw "$Label sem id em '$Path'." }
  if (-not $actualId.Equals($ExpectedId, [System.StringComparison]::Ordinal)) {
    throw "$Label com ID divergente em '$Path': esperado '$ExpectedId', recebido '$actualId'."
  }
}

function Assert-N8nWorkflowLocalInventory {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Root,
    [Parameter(Mandatory = $true)]
    [object[]]$Registry
  )

  Assert-N8nWorkflowRegistry -Registry $Registry
  $registeredPaths = @{}
  foreach ($entry in $Registry) {
    $relative = "$($entry.ExportPath)".Replace("\", "/")
    $registeredPaths[$relative.ToLowerInvariant()] = $true
    $fullPath = Join-Path $Root $relative
    Assert-N8nWorkflowDocumentId -Path $fullPath -ExpectedId "$($entry.WorkflowId)" -Label "workflow local '$($entry.Key)'"
  }

  $exportDir = Join-Path $Root "automation/export"
  $rootFull = [System.IO.Path]::GetFullPath($Root).TrimEnd("\", "/")
  $actualFiles = @(Get-ChildItem -LiteralPath $exportDir -Filter "workflow-*.json" -File)
  foreach ($file in $actualFiles) {
    $relative = $file.FullName.Substring($rootFull.Length + 1).Replace("\", "/")
    if (-not $registeredPaths.ContainsKey($relative.ToLowerInvariant())) {
      throw "Workflow local sem owner no registro: $relative"
    }
  }
}

function Get-N8nProtectedWorkflowHashes {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Root,
    [Parameter(Mandatory = $true)]
    [object[]]$Registry,
    [Parameter(Mandatory = $true)]
    [object[]]$Selected
  )

  $selectedPaths = @{}
  foreach ($entry in $Selected) {
    $fullSelected = [System.IO.Path]::GetFullPath((Join-Path $Root "$($entry.ExportPath)"))
    $selectedPaths[$fullSelected] = $true
  }

  $snapshot = @{}
  foreach ($entry in $Registry) {
    $fullPath = [System.IO.Path]::GetFullPath((Join-Path $Root "$($entry.ExportPath)"))
    if (-not $selectedPaths.ContainsKey($fullPath)) {
      if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
        throw "Workflow canonico ausente ao calcular hash: $($entry.ExportPath)"
      }
      $snapshot[$fullPath] = (Get-FileHash -LiteralPath $fullPath -Algorithm SHA256).Hash
    }
  }
  return $snapshot
}

function Get-N8nWorkflowInventoryHashes {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Root,
    [Parameter(Mandatory = $true)]
    [object[]]$Registry
  )

  Assert-N8nWorkflowLocalInventory -Root $Root -Registry $Registry
  $rows = @()
  foreach ($entry in $Registry) {
    $fullPath = [System.IO.Path]::GetFullPath((Join-Path $Root "$($entry.ExportPath)"))
    $rows += [PSCustomObject]@{
      Key = "$($entry.Key)"
      Module = "$($entry.Module)"
      WorkflowId = "$($entry.WorkflowId)"
      ExportPath = "$($entry.ExportPath)"
      Sha256 = (Get-FileHash -LiteralPath $fullPath -Algorithm SHA256).Hash
    }
  }
  return $rows
}

function Assert-N8nProtectedWorkflowHashes {
  param(
    [Parameter(Mandatory = $true)]
    [hashtable]$Before
  )

  foreach ($path in $Before.Keys) {
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
      throw "Protecao de ownership: workflow nao selecionado foi removido: $path"
    }
    $afterHash = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash
    if (-not "$afterHash".Equals("$($Before[$path])", [System.StringComparison]::OrdinalIgnoreCase)) {
      throw "Protecao de ownership: workflow nao selecionado foi alterado: $path"
    }
  }
}

function Invoke-N8nCommandSilently {
  param(
    [Parameter(Mandatory = $true)]
    [scriptblock]$Command
  )

  $previousPreference = $ErrorActionPreference
  $ErrorActionPreference = "SilentlyContinue"
  try {
    & $Command *> $null
    return [int]$LASTEXITCODE
  }
  finally {
    $ErrorActionPreference = $previousPreference
  }
}
