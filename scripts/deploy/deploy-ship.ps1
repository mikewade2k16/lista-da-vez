param(
  [ValidateSet("staging", "prod")]
  [string]$Environment = "staging",
  # Tag local da imagem. Default: "preview" (staging) / "prod" (prod). Rolante (sobrescreve).
  [string]$Tag = "",
  [ValidateSet("both", "api", "web")]
  [string]$Service = "both",
  [string]$ApiImage = "ghcr.io/mikewade2k16/omni-api",
  [string]$WebImage = "ghcr.io/mikewade2k16/omni-web",
  [string]$VpsHost = "85.31.62.33",
  [int]$Port = 22,
  [string]$User = "deploy",
  [string]$KeyPath = (Join-Path $HOME ".ssh\gh_actions_omnichannel_vps"),
  [switch]$BackupDatabase,
  [switch]$SkipSmokeTests
)

# Deploy SEM registry (o caminho validado em 2026-06-16): builda as imagens na
# maquina LOCAL, manda prontas pra VPS por SSH (docker save -> load) e sobe com
# up --no-build. Zero docker login GHCR, zero build na VPS, sem apagar nada.
# Trade-off: manda a imagem inteira (sem dedup de camada). Para incremental por
# camada, use deploy-fast.ps1 (via GHCR, exige docker login no usuario deploy).

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoDir = (Resolve-Path (Join-Path $scriptDir "..\..")).Path
$composeLocal = Join-Path $repoDir "docker-compose.prod.yml"

if (-not (Test-Path $KeyPath)) { throw "Chave SSH nao encontrada em $KeyPath" }
if (-not (Test-Path $composeLocal)) { throw "docker-compose.prod.yml nao encontrado" }
if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { throw "docker nao encontrado (Docker Desktop precisa estar rodando)." }

switch ($Environment) {
  "prod" {
    $envFile = ".env.production"; $remotePath = "/home/deploy/lista-atendimento"
    $publicBaseUrl = "https://lista.whenthelightsdie.com"; $defaultTag = "prod"
  }
  "staging" {
    $envFile = ".env.staging"; $remotePath = "/home/deploy/lista-atendimento-staging"
    $publicBaseUrl = "https://preview.whenthelightsdie.com"; $defaultTag = "preview"
  }
}
if ([string]::IsNullOrWhiteSpace($Tag)) { $Tag = $defaultTag }

$apiRef = "${ApiImage}:${Tag}"
$webRef = "${WebImage}:${Tag}"

$SshExe = Join-Path $env:SystemRoot "System32\OpenSSH\ssh.exe"
$ScpExe = Join-Path $env:SystemRoot "System32\OpenSSH\scp.exe"
$resolvedKeyPath = (Resolve-Path $KeyPath).Path
$sshHardening = @("-o", "BatchMode=yes", "-o", "ConnectTimeout=20", "-o", "ServerAliveInterval=10")
$sshArgs = @("-i", $resolvedKeyPath, "-o", "StrictHostKeyChecking=accept-new", "-p", $Port.ToString()) + $sshHardening
$remoteTarget = "$User@$VpsHost"

Write-Host "Deploy SHIP ($Environment) | tag $Tag | servico $Service | $remoteTarget`:$remotePath"

function Build-Image {
  param([string]$Context, [string]$Ref, [string]$Label)
  Write-Host "==> docker build $Label -> $Ref"
  & docker build -t $Ref $Context
  if ($LASTEXITCODE -ne 0) { throw "Falha no build de $Label." }
}

# 1. build local
$refsToShip = @()
if ($Service -eq "both" -or $Service -eq "api") { Build-Image (Join-Path $repoDir "back") $apiRef "api"; $refsToShip += $apiRef }
if ($Service -eq "both" -or $Service -eq "web") { Build-Image (Join-Path $repoDir "web") $webRef "web"; $refsToShip += $webRef }

# 2. save (local) -> load (VPS), via SSH, sem registry
Write-Host "==> docker save | ssh 'docker load' ($($refsToShip -join ', '))"
$loadOnVps = "docker load"
# pipeline SEM compressao: docker save | ssh ... 'docker load'. O gzip local nao
# e' garantido no cmd do Windows; o tar cru passa byte-clean pela pipe do cmd. (Um
# `gunzip` orfao aqui fazia TODO deploy falhar em "not in gzip format" — as imagens
# nunca chegavam na VPS e o prod seguia na imagem velha.)
& cmd /c "docker save $($refsToShip -join ' ') | `"$SshExe`" $($sshArgs -join ' ') $remoteTarget `"$loadOnVps`""
if ($LASTEXITCODE -ne 0) { throw "Falha ao transferir/carregar a imagem na VPS." }

# 3. envia o compose
Write-Host "==> scp docker-compose.prod.yml"
$scpArgs = @("-i", $resolvedKeyPath, "-o", "StrictHostKeyChecking=accept-new", "-P", $Port.ToString()) + $sshHardening + @($composeLocal, "${remoteTarget}:${remotePath}/docker-compose.prod.yml")
& $ScpExe @scpArgs
if ($LASTEXITCODE -ne 0) { throw "Falha no scp do compose." }

# 4. set IMAGE_TAG + up --no-build (sem pull: a imagem ja foi carregada)
$backup = if ($BackupDatabase) { "mkdir -p backups && docker compose --env-file $envFile -f docker-compose.prod.yml exec -T postgres sh -lc 'pg_dump -U `"`$POSTGRES_USER`" -d `"`$POSTGRES_DB`"' | gzip > backups/backup_`$(date +%Y%m%d_%H%M%S).sql.gz;" } else { "" }
$upCmd = @"
set -euo pipefail
cd '$remotePath'
[ -f '$envFile' ] || { echo "ERRO: $envFile nao existe em $remotePath" >&2; exit 1; }
grep -q '^IMAGE_TAG=' '$envFile' && sed -i 's|^IMAGE_TAG=.*|IMAGE_TAG=$Tag|' '$envFile' || printf 'IMAGE_TAG=%s\n' '$Tag' >> '$envFile'
$backup
docker compose --env-file '$envFile' -f docker-compose.prod.yml up -d --no-build api web
docker compose --env-file '$envFile' -f docker-compose.prod.yml ps --format '{{.Name}}  {{.State}}'
"@
Write-Host "==> up --no-build na VPS"
# Forca bash no remoto: o login shell do deploy e' sh/dash e rejeita
# 'set -o pipefail' ("set: pipefail: invalid option name") -> o up abortava e o
# prod seguia nos containers velhos. O upCmd vai pelo STDIN do bash -s; assim o
# pipefail vale de novo e PROTEGE o backup (pg_dump | gzip nao mascara dump vazio).
$upCmd | & $SshExe @sshArgs $remoteTarget "bash -s"
if ($LASTEXITCODE -ne 0) { throw "Falha no up na VPS." }

# 5. smoke
if (-not $SkipSmokeTests) {
  Write-Host "==> smoke $publicBaseUrl/healthz"
  $smoke = @"
for i in 1 2 3 4 5 6; do
  c=`$(curl -sS -o /dev/null -w '%{http_code}' --max-time 12 '$publicBaseUrl/healthz' 2>/dev/null)
  echo "healthz=`$c"
  [ "`$c" = '200' ] && exit 0
  sleep 6
done
exit 1
"@
  & $SshExe @sshArgs $remoteTarget $smoke
  if ($LASTEXITCODE -ne 0) { Write-Host "AVISO: smoke nao retornou 200 (DNS/cert/boot?). Confira os logs." }
}

Write-Host ""
Write-Host "Deploy SHIP ($Environment) concluido: $publicBaseUrl (imagem $Tag)."
