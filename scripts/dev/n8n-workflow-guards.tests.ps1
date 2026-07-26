<#
.SYNOPSIS
  Isolated tests for the n8n workflow ownership registry and guards.

.DESCRIPTION
  Does not call Docker, n8n or a real runtime. It uses a temporary inventory to
  prove exact selection, owner denial, ID integrity and protected hashes.
#>

$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$root = Split-Path -Parent (Split-Path -Parent $scriptDir)
$registryPath = Join-Path $scriptDir "n8n-workflow-registry.ps1"
. $registryPath

$passed = 0

function Assert-True {
  param(
    [Parameter(Mandatory = $true)]
    [bool]$Condition,
    [Parameter(Mandatory = $true)]
    [string]$Message
  )
  if (-not $Condition) { throw "ASSERT FAILED: $Message" }
  $script:passed++
}

function Assert-ThrowsLike {
  param(
    [Parameter(Mandatory = $true)]
    [scriptblock]$Action,
    [Parameter(Mandatory = $true)]
    [string]$Pattern,
    [Parameter(Mandatory = $true)]
    [string]$Message
  )
  $thrown = $false
  try { & $Action }
  catch {
    $thrown = $true
    if (-not "$($_.Exception.Message)".Contains($Pattern)) {
      throw "ASSERT FAILED: $Message (erro inesperado: $($_.Exception.Message))"
    }
  }
  if (-not $thrown) { throw "ASSERT FAILED: $Message (nenhum erro foi lancado)" }
  $script:passed++
}

function Invoke-TestPowerShellScript {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Path,
    [string[]]$Arguments = @()
  )

  $previousPreference = $ErrorActionPreference
  $ErrorActionPreference = "Continue"
  try {
    $output = & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $Path @Arguments 2>&1
    $exitCode = $LASTEXITCODE
  }
  finally {
    $ErrorActionPreference = $previousPreference
  }
  return [PSCustomObject]@{
    ExitCode = $exitCode
    Output = (@($output) -join "`n")
  }
}

$registry = @(Get-N8nWorkflowRegistry)
Assert-N8nWorkflowRegistry -Registry $registry
Assert-N8nWorkflowLocalInventory -Root $root -Registry $registry
Assert-True -Condition ($registry.Count -eq 8) -Message "registro canonico deve conter oito workflows"

$brain = @(Resolve-N8nWorkflowSelection -Registry $registry -Only "omnichannel-brain" -Owner "omnichannel" -RequireWritable)
Assert-True -Condition ($brain.Count -eq 1 -and $brain[0].WorkflowId -eq "omnibrain0000001") -Message "brain deve resolver por key exata"

$instagram = @(Resolve-N8nWorkflowSelection -Registry $registry -Only "instafirst000001" -Owner "omnichannel" -RequireWritable)
Assert-True -Condition ($instagram.Count -eq 1 -and $instagram[0].Key -eq "instagram-first-contact") -Message "Instagram deve resolver por ID exato"

Assert-ThrowsLike -Action {
  Resolve-N8nWorkflowSelection -Registry $registry -Only "whatsapp" -Owner "omnichannel" -RequireWritable
} -Pattern "Ownership negado" -Message "owner omnichannel nao pode selecionar WhatsApp de automation"

Assert-ThrowsLike -Action {
  Resolve-N8nWorkflowSelection -Registry $registry -Only "omni" -Owner "omnichannel" -RequireWritable
} -Pattern "desconhecido ou ambiguo" -Message "selecao parcial deve falhar"

Assert-ThrowsLike -Action {
  Resolve-N8nWorkflowSelection -Registry $registry -Owner "omnichannel" -RequireWritable
} -Pattern "exige -Only exato" -Message "owner sem alvo exato deve falhar"

Assert-ThrowsLike -Action {
  Resolve-N8nWorkflowSelection -Registry $registry -Only "omnichannel-brain" -RequireWritable
} -Pattern "-Owner explicito" -Message "alvo sem owner deve falhar"

$readOnlyRegistry = @(
  [PSCustomObject]@{
    Key = "read-only"
    Module = "test"
    WorkflowId = "readonly0000001"
    ExportPath = "automation/export/workflow-read-only.json"
    Writable = $false
  }
)
Assert-ThrowsLike -Action {
  Resolve-N8nWorkflowSelection -Registry $readOnlyRegistry -Only "read-only" -Owner "test" -RequireWritable
} -Pattern "writable=false" -Message "entrada read-only deve falhar fechada"

$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("n8n-guard-tests-" + [System.Guid]::NewGuid().ToString("N"))
$tempExport = Join-Path $tempRoot "automation/export"
New-Item -ItemType Directory -Path $tempExport -Force | Out-Null
$testRegistry = @(
  [PSCustomObject]@{
    Key = "one"
    Module = "alpha"
    WorkflowId = "workflowone0001"
    ExportPath = "automation/export/workflow-one.json"
    Writable = $true
  },
  [PSCustomObject]@{
    Key = "two"
    Module = "beta"
    WorkflowId = "workflowtwo0002"
    ExportPath = "automation/export/workflow-two.json"
    Writable = $true
  }
)
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$onePath = Join-Path $tempExport "workflow-one.json"
$twoPath = Join-Path $tempExport "workflow-two.json"

try {
  [System.IO.File]::WriteAllText($onePath, '[{"id":"workflowone0001","nodes":[]}]', $utf8NoBom)
  [System.IO.File]::WriteAllText($twoPath, '[{"id":"workflowtwo0002","nodes":[]}]', $utf8NoBom)
  Assert-N8nWorkflowLocalInventory -Root $tempRoot -Registry $testRegistry
  $script:passed++

  [System.IO.File]::WriteAllText($twoPath, '[{"id":"wrong-id","nodes":[]}]', $utf8NoBom)
  Assert-ThrowsLike -Action {
    Assert-N8nWorkflowLocalInventory -Root $tempRoot -Registry $testRegistry
  } -Pattern "ID divergente" -Message "ID local divergente deve falhar"

  [System.IO.File]::WriteAllText($twoPath, '[{"id":"workflowtwo0002","nodes":[]}]', $utf8NoBom)
  Remove-Item -LiteralPath $twoPath -Force
  Assert-ThrowsLike -Action {
    Assert-N8nWorkflowLocalInventory -Root $tempRoot -Registry $testRegistry
  } -Pattern "ausente" -Message "arquivo local ausente deve falhar"

  $parseSecret = "PARSE_SECRET_MUST_NOT_LEAK"
  [System.IO.File]::WriteAllText($twoPath, "{invalid-$parseSecret", $utf8NoBom)
  $parseMessage = ""
  try { Read-N8nWorkflowDocument -Path $twoPath -Label "workflow de teste" | Out-Null }
  catch { $parseMessage = "$($_.Exception.Message)" }
  Assert-True -Condition (-not $parseMessage.Contains($parseSecret)) -Message "erro de parse nao pode ecoar conteudo do JSON"

  [System.IO.File]::WriteAllText($twoPath, '[{"id":"workflowtwo0002","nodes":[]}]', $utf8NoBom)
  $selectedOne = @(Resolve-N8nWorkflowSelection -Registry $testRegistry -Only "one" -Owner "alpha" -RequireWritable)
  $hashes = Get-N8nProtectedWorkflowHashes -Root $tempRoot -Registry $testRegistry -Selected $selectedOne
  Assert-N8nProtectedWorkflowHashes -Before $hashes
  $script:passed++

  [System.IO.File]::WriteAllText($twoPath, '[{"id":"workflowtwo0002","nodes":[],"changed":true}]', $utf8NoBom)
  Assert-ThrowsLike -Action {
    Assert-N8nProtectedWorkflowHashes -Before $hashes
  } -Pattern "nao selecionado foi alterado" -Message "hash de workflow nao selecionado deve ser protegido"
}
finally {
  Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}

function Write-TestWorkflow {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Path,
    [Parameter(Mandatory = $true)]
    [string]$Id,
    [Parameter(Mandatory = $true)]
    [object[]]$Nodes,
    [object]$PinData = $null,
    [object]$StaticData = $null
  )

  $workflow = [ordered]@{
    id = $Id
    name = "Test workflow"
    nodes = $Nodes
    connections = @{}
    active = $true
    settings = @{}
    staticData = $StaticData
    pinData = $PinData
    versionId = "test-version"
    createdAt = "must-not-be-exported"
  }
  $json = @($workflow) | ConvertTo-Json -Depth 30
  [System.IO.File]::WriteAllText($Path, $json, $utf8NoBom)
}

function Invoke-TestNormalizer {
  param(
    [Parameter(Mandatory = $true)]
    [string]$RawPath,
    [Parameter(Mandatory = $true)]
    [string]$TargetPath,
    [Parameter(Mandatory = $true)]
    [string]$ExpectedId,
    [Parameter(Mandatory = $true)]
    [string]$Module,
    [string]$Mode = "write"
  )

  $previousPreference = $ErrorActionPreference
  $ErrorActionPreference = "Continue"
  try {
    $output = & node (Join-Path $scriptDir "n8n-workflow-normalize.js") $RawPath $TargetPath $Mode $ExpectedId $Module 2>&1
    $exitCode = $LASTEXITCODE
  }
  finally {
    $ErrorActionPreference = $previousPreference
  }
  return [PSCustomObject]@{
    ExitCode = $exitCode
    Output = (@($output) -join "`n")
  }
}

$securityRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("n8n-security-tests-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $securityRoot -Force | Out-Null
try {
  $rawPath = Join-Path $securityRoot "raw.json"
  $targetPath = Join-Path $securityRoot "target.json"
  $safeNode = [ordered]@{
    id = "safe-node"
    name = "Safe code"
    type = "n8n-nodes-base.code"
    parameters = @{ jsCode = "return items;" }
    credentials = @{ openAiApi = @{ id = "credential-id"; name = "OpenAI" } }
  }
  Write-TestWorkflow -Path $rawPath -Id "safe00000000001" -Nodes @($safeNode) -PinData @{ sample = "pii" } -StaticData @{ memory = "pii" }
  $safeResult = Invoke-TestNormalizer -RawPath $rawPath -TargetPath $targetPath -ExpectedId "safe00000000001" -Module "omnichannel"
  Assert-True -Condition ($safeResult.ExitCode -eq 0) -Message "normalizador deve aceitar credential apenas com id/name"
  $normalized = Get-Content -LiteralPath $targetPath -Raw | ConvertFrom-Json
  $normalizedWorkflow = if ($normalized -is [System.Array]) { $normalized[0] } else { $normalized }
  Assert-True -Condition ($normalizedWorkflow.active -eq $false) -Message "normalizador deve forcar active=false"
  Assert-True -Condition ($null -eq $normalizedWorkflow.staticData) -Message "normalizador deve limpar staticData"
  Assert-True -Condition (@($normalizedWorkflow.pinData.PSObject.Properties).Count -eq 0) -Message "normalizador deve limpar pinData"
  Assert-True -Condition ($null -eq $normalizedWorkflow.createdAt) -Message "normalizador deve remover campos de runtime"

  $secretSentinel = "N8N_SECRET_VALUE_MUST_NOT_LEAK"
  $credentialLeakNode = [ordered]@{
    id = "credential-leak"
    name = "Credential leak"
    type = "n8n-nodes-base.code"
    parameters = @{}
    credentials = @{ openAiApi = @{ id = "credential-id"; name = "OpenAI"; token = $secretSentinel } }
  }
  Remove-Item -LiteralPath $targetPath -Force -ErrorAction SilentlyContinue
  Write-TestWorkflow -Path $rawPath -Id "safe00000000001" -Nodes @($credentialLeakNode)
  $credentialResult = Invoke-TestNormalizer -RawPath $rawPath -TargetPath $targetPath -ExpectedId "safe00000000001" -Module "omnichannel"
  Assert-True -Condition ($credentialResult.ExitCode -eq 3) -Message "credential materializada deve falhar com exit 3"
  Assert-True -Condition (-not $credentialResult.Output.Contains($secretSentinel)) -Message "erro de credential nao pode ecoar segredo"
  Assert-True -Condition (-not (Test-Path -LiteralPath $targetPath)) -Message "falha de credential nao pode gravar target"

  Write-TestWorkflow -Path $rawPath -Id "runtime-wrong-id" -Nodes @($safeNode)
  $idResult = Invoke-TestNormalizer -RawPath $rawPath -TargetPath $targetPath -ExpectedId "safe00000000001" -Module "omnichannel"
  Assert-True -Condition ($idResult.ExitCode -eq 4) -Message "ID runtime divergente deve falhar com exit 4"
  Assert-True -Condition (-not $idResult.Output.Contains("runtime-wrong-id")) -Message "erro de ID deve ser sanitizado"

  $blockedTypes = @(
    "n8n-nodes-waha.sendText",
    "n8n-nodes-base.whatsApp",
    "n8n-nodes-base.instagram",
    "n8n-nodes-base.facebookGraphApi",
    "n8n-nodes-evolution-api.sendMessage"
  )
  foreach ($blockedType in $blockedTypes) {
    $channelNode = [ordered]@{ id = "channel"; name = "Channel"; type = $blockedType; parameters = @{} }
    Write-TestWorkflow -Path $rawPath -Id "safe00000000001" -Nodes @($channelNode)
    $channelResult = Invoke-TestNormalizer -RawPath $rawPath -TargetPath $targetPath -ExpectedId "safe00000000001" -Module "omnichannel"
    Assert-True -Condition ($channelResult.ExitCode -eq 6) -Message "tipo de canal direto deve falhar: $blockedType"
  }

  $blockedUrls = @(
    "http://waha:3000/api/sendText",
    "http://evolution:8080/message/sendText/instance",
    "https://graph.facebook.com/v23.0/messages",
    "https://graph.instagram.com/v1/messages",
    "https://api.instagram.com/messages",
    "https://api.whatsapp.com/send",
    "https://wa.me/5511999999999"
  )
  foreach ($blockedUrl in $blockedUrls) {
    $httpNode = [ordered]@{ id = "http"; name = "HTTP"; type = "n8n-nodes-base.httpRequest"; parameters = @{ url = $blockedUrl } }
    Write-TestWorkflow -Path $rawPath -Id "safe00000000001" -Nodes @($httpNode)
    $urlResult = Invoke-TestNormalizer -RawPath $rawPath -TargetPath $targetPath -ExpectedId "safe00000000001" -Module "omnichannel"
    Assert-True -Condition ($urlResult.ExitCode -eq 6) -Message "URL de canal direto deve falhar: $blockedUrl"
  }

  $automationNode = [ordered]@{ id = "waha"; name = "WAHA legitimo"; type = "n8n-nodes-waha.sendText"; parameters = @{} }
  Write-TestWorkflow -Path $rawPath -Id "automation000001" -Nodes @($automationNode)
  $automationResult = Invoke-TestNormalizer -RawPath $rawPath -TargetPath $targetPath -ExpectedId "automation000001" -Module "automation"
  Assert-True -Condition ($automationResult.ExitCode -eq 0) -Message "guard de canal nao pode bloquear workflow do owner automation"
}
finally {
  Remove-Item -LiteralPath $securityRoot -Recurse -Force -ErrorAction SilentlyContinue
}

# Prove that global -Check is local-only by shadowing docker/node with marker commands.
$fakeBin = Join-Path ([System.IO.Path]::GetTempPath()) ("n8n-fake-bin-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $fakeBin -Force | Out-Null
$dockerMarker = Join-Path $fakeBin "docker-called.txt"
$nodeMarker = Join-Path $fakeBin "node-called.txt"
$dockerCmd = Join-Path $fakeBin "docker.cmd"
$nodeCmd = Join-Path $fakeBin "node.cmd"
$originalPath = $env:PATH
try {
  [System.IO.File]::WriteAllText($dockerCmd, "@echo off`r`necho called>`"$dockerMarker`"`r`nexit /b 97`r`n", $utf8NoBom)
  [System.IO.File]::WriteAllText($nodeCmd, "@echo off`r`necho called>`"$nodeMarker`"`r`nexit /b 98`r`n", $utf8NoBom)
  $beforeTemp = @((Get-ChildItem -LiteralPath ([System.IO.Path]::GetTempPath()) -Directory -Filter "n8n-export-*" -ErrorAction SilentlyContinue).FullName | Sort-Object)
  $env:PATH = "$fakeBin;$originalPath"
  $localCheckResult = Invoke-TestPowerShellScript -Path (Join-Path $scriptDir "n8n-export.ps1") -Arguments @("-Check")
  $afterTemp = @((Get-ChildItem -LiteralPath ([System.IO.Path]::GetTempPath()) -Directory -Filter "n8n-export-*" -ErrorAction SilentlyContinue).FullName | Sort-Object)
  Assert-True -Condition ($localCheckResult.ExitCode -eq 0) -Message "check global local deve passar"
  Assert-True -Condition (-not (Test-Path -LiteralPath $dockerMarker)) -Message "check global local nao pode chamar Docker"
  Assert-True -Condition (-not (Test-Path -LiteralPath $nodeMarker)) -Message "check global local nao pode chamar Node"
  Assert-True -Condition (($beforeTemp -join "|") -eq ($afterTemp -join "|")) -Message "check global local nao pode criar temporario"

  $deployFastPath = Join-Path $root "scripts/deploy/deploy-fast.ps1"
  $invalidDeployArguments = @(
    @("-WorkflowOwner", "omnichannel"),
    @("-WorkflowOnly", "omnichannel-brain"),
    @("-SkipWorkflowExport"),
    @("-ForceAutomationWorkflowImport"),
    @("-DeployAutomation", "-WorkflowOnly", "omnichannel-brain"),
    @("-DeployAutomation", "-WorkflowOwner", "omnichannel", "-WorkflowOnly", "whatsapp")
  )
  foreach ($deployArguments in $invalidDeployArguments) {
    Remove-Item -LiteralPath $dockerMarker -Force -ErrorAction SilentlyContinue
    $deployResult = Invoke-TestPowerShellScript -Path $deployFastPath -Arguments $deployArguments
    Assert-True -Condition ($deployResult.ExitCode -ne 0) -Message "deploy-fast deve falhar cedo para combinacao invalida: $($deployArguments -join ' ')"
    Assert-True -Condition (-not (Test-Path -LiteralPath $dockerMarker)) -Message "deploy-fast invalido nao pode chamar Docker: $($deployArguments -join ' ')"
  }

  # Fake runtime failure emits a secret, but n8n-import must return only its sanitized error.
  $runtimeSecret = "RUNTIME_SECRET_MUST_NOT_LEAK"
  $fakeDocker = @'
@echo off
if /I "%1"=="ps" (
  echo fake-container
  exit /b 0
)
if /I "%1"=="exec" (
  echo __RUNTIME_SECRET__ 1>&2
  exit /b 9
)
exit /b 9
'@
  $fakeDocker = $fakeDocker.Replace("__RUNTIME_SECRET__", $runtimeSecret)
  [System.IO.File]::WriteAllText($dockerCmd, $fakeDocker, $utf8NoBom)
  $exportRuntimeResult = Invoke-TestPowerShellScript -Path (Join-Path $scriptDir "n8n-export.ps1") -Arguments @("-Check", "-Owner", "omnichannel", "-Only", "omnichannel-brain", "-Container", "fake-n8n")
  Assert-True -Condition ($exportRuntimeResult.ExitCode -eq 30) -Message "runtime indisponivel no export deve usar exit 30"
  Assert-True -Condition (-not $exportRuntimeResult.Output.Contains($runtimeSecret)) -Message "export nao pode ecoar erro bruto/segredo do CLI"
  $importResult = Invoke-TestPowerShellScript -Path (Join-Path $scriptDir "n8n-import.ps1") -Arguments @("-Owner", "omnichannel", "-Only", "omnichannel-brain", "-Container", "fake-n8n")
  Assert-True -Condition ($importResult.ExitCode -ne 0) -Message "falha de runtime no import deve falhar fechado"
  Assert-True -Condition (-not $importResult.Output.Contains($runtimeSecret)) -Message "import nao pode ecoar erro bruto/segredo do CLI"
}
finally {
  $env:PATH = $originalPath
  Remove-Item -LiteralPath $fakeBin -Recurse -Force -ErrorAction SilentlyContinue
}

# Versioned callers must be exact. The only ownerless export call is local -Check.
$package = Get-Content -LiteralPath (Join-Path $root "package.json") -Raw | ConvertFrom-Json
foreach ($property in $package.scripts.PSObject.Properties) {
  $command = "$($property.Value)"
  if ($command.Contains("n8n-import.ps1")) {
    Assert-True -Condition ($command.Contains("-Owner ") -and $command.Contains("-Only ")) -Message "alias de import deve conter owner+key: $($property.Name)"
  }
  if ($command.Contains("n8n-export.ps1")) {
    $isLocalCheck = $command.Trim().EndsWith("n8n-export.ps1 -Check", [System.StringComparison]::OrdinalIgnoreCase)
    Assert-True -Condition ($isLocalCheck -or ($command.Contains("-Owner ") -and $command.Contains("-Only "))) -Message "alias de export runtime deve conter owner+key: $($property.Name)"
  }
}
Assert-True -Condition (-not "$($package.scripts.'deploy:fast:prod')".Contains("-DeployAutomation")) -Message "deploy:fast:prod nao pode acionar deploy global de workflows implicitamente"

$preCommit = Get-Content -LiteralPath (Join-Path $root ".husky/pre-commit") -Raw
Assert-True -Condition (-not $preCommit.Contains("docker ps")) -Message "pre-commit nao pode consultar Docker para n8n"
Assert-True -Condition ($preCommit.Contains("n8n-export.ps1 -Check")) -Message "pre-commit deve executar check local"

$deployFast = Get-Content -LiteralPath (Join-Path $root "scripts/deploy/deploy-fast.ps1") -Raw
Assert-True -Condition ($deployFast.Contains("-DeployAutomation exige -WorkflowOwner e -WorkflowOnly")) -Message "deploy-fast deve falhar cedo sem owner/key"
Assert-True -Condition ($deployFast.Contains("-Sync -Owner `$WorkflowOwner -Only `$WorkflowOnly")) -Message "deploy-fast nao pode chamar Sync global"

$exportScript = Get-Content -LiteralPath (Join-Path $scriptDir "n8n-export.ps1") -Raw
Assert-True -Condition ($exportScript.Contains("exit 10") -and $exportScript.Contains("exit 20") -and $exportScript.Contains("exit 30")) -Message "export deve distinguir drift, integridade e runtime por exit code"

$importScript = Get-Content -LiteralPath (Join-Path $scriptDir "n8n-import.ps1") -Raw
$importPolicyIndex = $importScript.IndexOf('& node $normalizerPath $selectedLocalPath $selectedLocalPath "check"')
$importDockerIndex = $importScript.IndexOf('docker ps -q')
Assert-True -Condition ($importPolicyIndex -ge 0 -and $importDockerIndex -gt $importPolicyIndex) -Message "import deve aplicar normalizador/policy local antes de Docker"
Assert-True -Condition ($importScript.Contains('Contains("STATUS:unchanged")')) -Message "import deve bloquear JSON local que exigiria normalizacao"

foreach ($path in @(
  (Join-Path $scriptDir "n8n-workflow-registry.ps1"),
  (Join-Path $scriptDir "n8n-workflow-guards.tests.ps1"),
  (Join-Path $scriptDir "n8n-import.ps1"),
  (Join-Path $scriptDir "n8n-export.ps1"),
  (Join-Path $scriptDir "n8n-workflow-normalize.js")
)) {
  $bytes = [System.IO.File]::ReadAllBytes($path)
  $nonAscii = @($bytes | Where-Object { $_ -gt 127 })
  Assert-True -Condition ($nonAscii.Count -eq 0) -Message "script deve permanecer ASCII: $path"
}

Write-Host "OK: $passed testes de ownership n8n passaram sem Docker/runtime."
