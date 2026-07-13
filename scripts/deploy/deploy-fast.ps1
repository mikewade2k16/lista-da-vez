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
  [string]$ApiImage = "ghcr.io/mikewade2k16/omni-api",
  [string]$WebImage = "ghcr.io/mikewade2k16/omni-web",
  [string]$VpsHost = "85.31.62.33",
  [int]$Port = 22,
  [string]$User = "deploy",
  [string]$KeyPath = (Join-Path $HOME ".ssh\gh_actions_omnichannel_vps"),
  [switch]$BackupDatabase,
  [switch]$ForceRecreate,
  [switch]$SkipSmokeTests,
  # Tambem reconcilia o profile automation na VPS (redis/waha/n8n/whisper) e
  # reimporta automation/export/workflow-*.json quando os arquivos mudarem.
  [switch]$DeployAutomation,
  [switch]$ForceAutomationWorkflowImport,
  # docker login LOCAL no GHCR antes do push (one-time; depois fica no config.json).
  [string]$GhcrUser = "",
  [string]$GhcrToken = ""
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoDir = (Resolve-Path (Join-Path $scriptDir "..\..")).Path
$deployPull = Join-Path $scriptDir "deploy-pull.ps1"
if (-not (Test-Path $deployPull)) { throw "deploy-pull.ps1 nao encontrado em $scriptDir" }
if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { throw "docker nao encontrado no PATH (Docker Desktop precisa estar rodando)." }

if ([string]::IsNullOrWhiteSpace($Tag)) {
  $Tag = "local-" + (Get-Date -Format "yyyyMMdd-HHmmss")
}

$apiRef = "${ApiImage}:${Tag}"
$webRef = "${WebImage}:${Tag}"

Write-Host "Build LOCAL + push GHCR + pull na VPS (sem git, sem build na VPS)."
Write-Host "Ambiente: $Environment | Tag: $Tag | Servico: $Service"

# Login local opcional no GHCR (one-time).
if (-not [string]::IsNullOrWhiteSpace($GhcrToken)) {
  if ([string]::IsNullOrWhiteSpace($GhcrUser)) { throw "-GhcrToken informado sem -GhcrUser." }
  Write-Host "==> docker login ghcr.io (local)"
  $GhcrToken | docker login ghcr.io -u $GhcrUser --password-stdin
  if ($LASTEXITCODE -ne 0) { throw "Falha no docker login local." }
}

function Build-And-Push {
  param([string]$Context, [string]$Ref, [string]$Label)
  Write-Host "==> docker build $Label -> $Ref"
  & docker build -t $Ref $Context
  if ($LASTEXITCODE -ne 0) { throw "Falha no build da imagem $Label." }
  Write-Host "==> docker push $Ref (so as camadas que mudaram sobem)"
  & docker push $Ref
  if ($LASTEXITCODE -ne 0) { throw "Falha no push de $Ref (esta logado no ghcr.io? use -GhcrUser/-GhcrToken)." }
}

# back/Dockerfile e web/Dockerfile: o stage final do web JA e o de producao.
if ($Service -eq "both" -or $Service -eq "api") {
  Build-And-Push -Context (Join-Path $repoDir "back") -Ref $apiRef -Label "api"
}
if ($Service -eq "both" -or $Service -eq "web") {
  Build-And-Push -Context (Join-Path $repoDir "web") -Ref $webRef -Label "web"
}

if ($Service -ne "both") {
  Write-Host "AVISO: -Service $Service buildou so um servico. A tag '$Tag' precisa ja ter o outro no GHCR (rode -Service both pelo menos uma vez por tag)."
}

# Deploy: a VPS so faz pull + up --no-build (reusa o script existente).
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
if ($DeployAutomation) { $forward.DeployAutomation = $true }
if ($ForceAutomationWorkflowImport) { $forward.ForceAutomationWorkflowImport = $true }

& $deployPull @forward

Write-Host ""
Write-Host "Deploy rapido ($Environment) concluido: imagem $Tag no ar."
Write-Host "Rollback: scripts/deploy/deploy-pull.ps1 -Environment $Environment -Tag <tag-anterior>"
