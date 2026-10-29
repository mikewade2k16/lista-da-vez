param(
  [ValidateSet("staging", "prod")]
  [string]$Environment = "staging",
  # Tag das imagens. Vazio => local-<timestamp> (rastreavel/rollback). Pode passar
  # uma tag fixa rolante (ex.: "dev") se preferir sobrescrever sempre a mesma.
  [string]$Tag = "",
  # Quais imagens buildar+push. "both" (padrao) garante que a tag tenha api E web.
  # "api"/"web" so funcionam com uma -Tag que ja exista no GHCR para o outro servico.
  [ValidateSet("both", "api", "web")]
  [string]$Service = "both",
  # A VPS e amd64; nao publicar imagens ARM por acidente ao usar Apple Silicon.
  [ValidateSet("linux/amd64", "linux/arm64")]
  [string]$TargetPlatform = "linux/amd64",
  [ValidateSet("auto", "registry", "ssh")]
  [string]$Transport = "auto",
  [string]$ApiImage = "ghcr.io/mikewade2k16/omni-api",
  [string]$WebImage = "ghcr.io/mikewade2k16/omni-web",
  [string]$VpsHost = "85.31.62.33",
  [int]$Port = 22,
  [string]$User = $(if ($env:OMNI_DEPLOY_USER) { $env:OMNI_DEPLOY_USER } else { "deploy" }),
  [string]$KeyPath = $(if ($env:OMNI_DEPLOY_KEY_PATH) { $env:OMNI_DEPLOY_KEY_PATH } else { Join-Path $HOME ".ssh/gh_actions_omnichannel_vps" }),
  [switch]$BackupDatabase,
  [switch]$ForceRecreate,
  [switch]$SkipSmokeTests,
  # Tambem reconcilia o profile automation na VPS (redis/waha/n8n/whisper) e
  # reimporta automation/export/workflow-*.json quando os arquivos mudarem.
  [switch]$DeployAutomation,
  # Reconcilia o provider do atendimento na VPS (evolution-db/evolution).
  [switch]$DeployOmnichannel,
  [switch]$ForceAutomationWorkflowImport,
  # Escopo obrigatorio do unico workflow que pode ser consultado/sincronizado no n8n dev.
  # O deploy-pull continua sendo uma operacao de plataforma sobre todos os workflows.
  [string]$WorkflowOwner = "",
  [string]$WorkflowOnly = "",
  # Pula o auto-export dos workflows n8n (gatilho OBS-08). Use quando quiser deployar uma
  # versao versionada especifica em vez do que esta rodando no n8n dev agora.
  [switch]$SkipWorkflowExport,
  # O build Nitro supera 4 GB. O deploy pausa e restaura somente os servicos
  # locais que ja estavam rodando para liberar a memoria do Docker Desktop.
  [ValidateRange(4096, 8192)]
  [int]$WebBuildHeapMB = 5120,
  [switch]$KeepLocalStackRunning,
  # docker login LOCAL no GHCR antes do push (one-time; depois fica no config.json).
  [string]$GhcrUser = "",
  [string]$GhcrToken = ""
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoDir = (Resolve-Path (Join-Path $scriptDir "..\..")).Path
$deployPull = Join-Path $scriptDir "deploy-pull.ps1"
$composeLocal = Join-Path $repoDir "docker-compose.prod.yml"
$composeDev = Join-Path $repoDir "docker-compose.yml"
$composeEnvGuard = Join-Path $scriptDir "assert-compose-api-env.ps1"
$composeServiceParityGuard = Join-Path $scriptDir "assert-compose-service-parity.ps1"

# Ownership preflight must finish before any docker login/ps/build. DeployAutomation is
# explicit platform scope downstream, but its local runtime sync is always one exact owner/key.
$workflowSelection = @()
$n8nExportScript = Join-Path $scriptDir "..\dev\n8n-export.ps1"
$n8nRegistryScript = Join-Path $scriptDir "..\dev\n8n-workflow-registry.ps1"
if ($DeployAutomation) {
  if (-not (Test-Path -LiteralPath $n8nRegistryScript -PathType Leaf)) {
    throw "Registro de ownership n8n nao encontrado: $n8nRegistryScript"
  }
  . $n8nRegistryScript
  $workflowRegistry = @(Get-N8nWorkflowRegistry)
  Assert-N8nWorkflowLocalInventory -Root $repoDir -Registry $workflowRegistry

  if (-not $SkipWorkflowExport) {
    if ([string]::IsNullOrWhiteSpace($WorkflowOwner) -or [string]::IsNullOrWhiteSpace($WorkflowOnly)) {
      throw "-DeployAutomation exige -WorkflowOwner e -WorkflowOnly para sync do n8n. Use -SkipWorkflowExport para deployar os JSONs versionados."
    }
    if (-not (Test-Path -LiteralPath $n8nExportScript -PathType Leaf)) {
      throw "Script de export owner-scoped nao encontrado: $n8nExportScript"
    }
    $workflowSelection = @(Resolve-N8nWorkflowSelection -Registry $workflowRegistry -Owner $WorkflowOwner -Only $WorkflowOnly -RequireWritable)
  }
  elseif (-not [string]::IsNullOrWhiteSpace($WorkflowOwner) -or -not [string]::IsNullOrWhiteSpace($WorkflowOnly)) {
    throw "WorkflowOwner/WorkflowOnly nao devem ser usados com -SkipWorkflowExport."
  }
  Write-Warning "DeployAutomation encaminha para deploy-pull, que ainda opera todos os workflows como acao explicita de plataforma. Nao use este caminho em tarefa omnichannel isolada."
}
elseif ($ForceAutomationWorkflowImport -or $SkipWorkflowExport -or
        -not [string]::IsNullOrWhiteSpace($WorkflowOwner) -or
        -not [string]::IsNullOrWhiteSpace($WorkflowOnly)) {
  throw "WorkflowOwner/WorkflowOnly/SkipWorkflowExport/ForceAutomationWorkflowImport exigem -DeployAutomation explicito."
}

if (-not (Test-Path -LiteralPath $KeyPath -PathType Leaf)) {
  throw "Chave SSH nao encontrada: $KeyPath. Informe -KeyPath antes de iniciar o build."
}
$sshProbe = (Get-Command ssh -CommandType Application -ErrorAction Stop).Source
& $sshProbe -i $KeyPath -p $Port -o BatchMode=yes -o ConnectTimeout=15 -o StrictHostKeyChecking=accept-new "$User@$VpsHost" true
if ($LASTEXITCODE -ne 0) { throw 'Preflight SSH falhou; nenhum build foi iniciado.' }
Write-Host 'Preflight SSH: OK'

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
  throw "docker nao encontrado no PATH. Instale ou abra o Docker Desktop antes do deploy."
}
& docker info --format '{{.ServerVersion}}' *> $null
if ($LASTEXITCODE -ne 0) {
  throw "Docker engine local indisponivel. Inicie o Docker Desktop e aguarde o engine Linux ficar pronto antes de executar npm run deploy."
}

if (-not (Test-Path -LiteralPath $composeEnvGuard -PathType Leaf)) {
  throw "Preflight de env do Compose nao encontrado: $composeEnvGuard"
}
& $composeEnvGuard -ComposePath $composeLocal
if (-not (Test-Path -LiteralPath $composeServiceParityGuard -PathType Leaf)) {
  throw "Preflight de paridade dos servicos Compose nao encontrado: $composeServiceParityGuard"
}
& $composeServiceParityGuard -DevComposePath $composeDev -ProdComposePath $composeLocal

if (-not (Test-Path $deployPull)) { throw "deploy-pull.ps1 nao encontrado em $scriptDir" }

& (Join-Path $scriptDir 'validate-release.ps1')

function Get-LocalDockerRegistryCredential {
  param([Parameter(Mandatory = $true)][string]$Registry)

  $configPath = Join-Path $HOME ".docker\config.json"
  if (-not (Test-Path -LiteralPath $configPath -PathType Leaf)) {
    throw "Config Docker local nao encontrado em $configPath"
  }
  $config = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
  $helper = $null
  if ($config.credHelpers -and $config.credHelpers.$Registry) {
    $helper = $config.credHelpers.$Registry
  }
  elseif ($config.credsStore) {
    $helper = $config.credsStore
  }

  if ($helper) {
    $helperExe = "docker-credential-$helper"
    if ($env:OS -eq 'Windows_NT') { $helperExe += '.exe' }
    if (-not (Get-Command $helperExe -ErrorAction SilentlyContinue)) {
      throw "Helper de credencial Docker nao encontrado: $helperExe"
    }
    $rawCredential = $Registry | & $helperExe get
    if ($LASTEXITCODE -ne 0) { throw "Falha ao consultar $Registry no helper $helperExe" }
    $credential = $rawCredential | ConvertFrom-Json
    if ([string]::IsNullOrWhiteSpace($credential.Username) -or [string]::IsNullOrWhiteSpace($credential.Secret)) {
      throw "Credencial de $Registry vazia no helper $helperExe"
    }
    return [pscustomobject]@{ Username = $credential.Username; Secret = $credential.Secret }
  }

  $authEntry = $config.auths.$Registry.auth
  if ([string]::IsNullOrWhiteSpace($authEntry)) {
    throw "Credencial de $Registry nao encontrada no Docker local."
  }
  $decoded = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($authEntry))
  $parts = $decoded.Split(':', 2)
  if ($parts.Count -ne 2 -or [string]::IsNullOrWhiteSpace($parts[1])) {
    throw "Credencial de $Registry invalida no config Docker local."
  }
  return [pscustomobject]@{ Username = $parts[0]; Secret = $parts[1] }
}

if ($Transport -ne 'ssh' -and [string]::IsNullOrWhiteSpace($GhcrToken)) {
  try {
    $credential = Get-LocalDockerRegistryCredential -Registry 'ghcr.io'
    $GhcrUser = $credential.Username
    $GhcrToken = $credential.Secret
    $Transport = 'registry'
  } catch {
    if ($Transport -eq 'registry') { throw }
    $Transport = 'ssh'
    Write-Host 'GHCR sem credencial local; usando transferencia de imagens por SSH.'
  }
} elseif ($Transport -eq 'auto') { $Transport = 'registry' }

if ([string]::IsNullOrWhiteSpace($Tag)) {
  $Tag = "local-" + (Get-Date -Format "yyyyMMdd-HHmmss")
}

if ($Tag -notmatch '^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$' -or $Tag -eq 'latest') { throw 'Tag de release invalida.' }

$apiRef = "${ApiImage}:${Tag}"
$webRef = "${WebImage}:${Tag}"

Write-Host "Build LOCAL + entrega $Transport na VPS (sem git, sem build na VPS)."
Write-Host "Ambiente: $Environment | Tag: $Tag | Servico: $Service"
if ($Service -eq "both" -or $Service -eq "web") {
  Write-Host "Heap do build web: $WebBuildHeapMB MB"
}

# Login local opcional no GHCR (one-time).
if (-not [string]::IsNullOrWhiteSpace($GhcrToken)) {
  if ([string]::IsNullOrWhiteSpace($GhcrUser)) { throw "-GhcrToken informado sem -GhcrUser." }
  Write-Host "==> docker login ghcr.io (local)"
  $GhcrToken | docker login ghcr.io -u $GhcrUser --password-stdin
  if ($LASTEXITCODE -ne 0) { throw "Falha no docker login local." }
}

# Sync local owner-scoped: antes do build, materializa somente WorkflowOwner/WorkflowOnly.
# Container ausente falha fechado; -SkipWorkflowExport e o opt-in para usar deliberadamente
# o JSON versionado sem consultar runtime. O deploy-pull downstream ainda e plataforma-global
# e permanece proibido para uma tarefa isolada de modulo.
if ($DeployAutomation -and -not $SkipWorkflowExport) {
  $n8nDevContainer = "omni-n8n-1"
  $n8nUp = docker ps -q -f "name=^${n8nDevContainer}$" 2>$null
  if (-not $n8nUp) {
    throw "n8n dev fora: sync owner-scoped de '$($workflowSelection[0].Key)' nao pode ser provado. Use -SkipWorkflowExport apenas para deployar deliberadamente o JSON versionado."
  }
  Write-Host "==> E0: sync owner-scoped owner='$WorkflowOwner' key='$WorkflowOnly' antes do deploy"
  $shellName = if ($PSVersionTable.PSVersion.Major -lt 6) { "powershell.exe" } elseif ($env:OS -eq "Windows_NT") { "pwsh.exe" } else { "pwsh" }
  & (Join-Path $PSHOME $shellName) -NoProfile -ExecutionPolicy Bypass -File $n8nExportScript -Sync -Owner $WorkflowOwner -Only $WorkflowOnly -Container $n8nDevContainer
  if ($LASTEXITCODE -ne 0) {
    throw "Falha no sync owner-scoped do workflow '$WorkflowOnly' (exit $LASTEXITCODE)."
  }
}

function Build-And-Push {
  param(
    [string]$Context,
    [string]$Ref,
    [string]$Label,
    [string[]]$BuildArgs = @()
  )
  Write-Host "==> docker build $Label -> $Ref"
  & docker build --platform $TargetPlatform @BuildArgs -t $Ref $Context
  if ($LASTEXITCODE -ne 0) { throw "Falha no build da imagem $Label." }
  if ($Label -eq 'web') {
    Get-Content -LiteralPath (Join-Path $scriptDir 'smoke-web-image.mjs') -Raw | & docker run --rm -i --platform $TargetPlatform --network none --entrypoint node $Ref --input-type=module
    if ($LASTEXITCODE -ne 0) { throw 'Smoke da imagem web falhou; publicacao abortada.' }
  }
  if ($Transport -eq "ssh") { return }
  Write-Host "==> docker push $Ref (so as camadas que mudaram sobem)"
  & docker push $Ref
  if ($LASTEXITCODE -ne 0) { throw "Falha no push de $Ref (esta logado no ghcr.io? use -GhcrUser/-GhcrToken)." }
}

# O Docker Desktop deste projeto tem ~6 GB. Pausa apenas os servicos deste Compose
# que ja estavam rodando e os restaura no finally, inclusive quando o build falha.
$localServicesToRestore = @()
$buildsWeb = $Service -eq "both" -or $Service -eq "web"
try {
  if ($buildsWeb -and -not $KeepLocalStackRunning) {
    $localServicesToRestore = @(& docker compose --profile "*" ps --status running --services)
    if ($LASTEXITCODE -ne 0) { throw "Falha ao inventariar a stack local antes do build web." }
    $localServicesToRestore = @($localServicesToRestore | ForEach-Object { $_.Trim() } | Where-Object { $_ } | Sort-Object -Unique)
    if ($localServicesToRestore.Count -gt 0) {
      Write-Host "==> Pausando stack local durante o build web: $($localServicesToRestore -join ', ')"
      & docker compose --profile "*" stop @localServicesToRestore
      if ($LASTEXITCODE -ne 0) { throw "Falha ao pausar a stack local antes do build web." }
    }
  }

  # back/Dockerfile e web/Dockerfile: o stage final do web JA e o de producao.
  if ($Service -eq "both" -or $Service -eq "api") {
    Build-And-Push -Context (Join-Path $repoDir "back") -Ref $apiRef -Label "api"
  }
  if ($buildsWeb) {
    $webBuildArgs = @("--build-arg", "WEB_BUILD_HEAP_MB=$WebBuildHeapMB")
    Build-And-Push -Context (Join-Path $repoDir "web") -Ref $webRef -Label "web" -BuildArgs $webBuildArgs
  }
}
finally {
  if ($localServicesToRestore.Count -gt 0) {
    Write-Host "==> Restaurando stack local: $($localServicesToRestore -join ', ')"
    & docker compose --profile "*" start @localServicesToRestore
    if ($LASTEXITCODE -ne 0) {
      Write-Warning "Nao consegui restaurar toda a stack local. Rode: docker compose --profile '*' start $($localServicesToRestore -join ' ')"
    }
  }
}

if ($Service -ne "both") {
  Write-Host "AVISO: -Service $Service buildou so um servico. A tag '$Tag' precisa ja ter o outro no GHCR (rode -Service both pelo menos uma vez por tag)."
}

. (Join-Path $scriptDir 'image-archive.ps1')
$imageArchive = ''
if ($Transport -eq 'ssh') {
  if ($Service -ne 'both') { throw 'Transporte SSH exige -Service both para garantir as duas imagens.' }
  $imageArchive = Export-DeployImageArchive -Tag $Tag -Images @($apiRef, $webRef)
}

$forward = @{
  Environment    = $Environment
  Tag            = $Tag
  VpsHost        = $VpsHost
  Port           = $Port
  User           = $User
  KeyPath        = $KeyPath
}
if ($BackupDatabase) { $forward.BackupDatabase = $true }
if ($ForceRecreate) { $forward.ForceRecreate = $true }
if ($SkipSmokeTests) { $forward.SkipSmokeTests = $true }
if ($DeployAutomation) {
  $forward.DeployAutomation = $true
}
if ($DeployOmnichannel) { $forward.DeployOmnichannel = $true }
if ($ForceAutomationWorkflowImport) { $forward.ForceAutomationWorkflowImport = $true }
if ($imageArchive) { $forward.ImageArchive = $imageArchive }
if ($Transport -eq "registry" -and -not [string]::IsNullOrWhiteSpace($GhcrToken)) {
  $forward.GhcrUser = $GhcrUser
  $forward.GhcrToken = $GhcrToken
}

& $deployPull @forward

Write-Host ""
Write-Host "Deploy rapido ($Environment) concluido: imagem $Tag no ar."
Write-Host "Rollback: npm run deploy:prod -- -Tag <tag-anterior> $(if ($Transport -eq 'ssh') { '-UseLoadedImages' })"
