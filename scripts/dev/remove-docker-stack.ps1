[CmdletBinding()]
param(
  [string]$Confirmation = "",
  [switch]$RemoveVolumes
)

$ErrorActionPreference = "Stop"

$expectedConfirmation = if ($RemoveVolumes) {
  "DELETE_OMNI_LOCAL_DATA"
} else {
  "REMOVE_OMNI_CONTAINERS"
}

if ($Confirmation -cne $expectedConfirmation) {
  $scope = if ($RemoveVolumes) {
    "containers e volumes persistentes locais (PostgreSQL, n8n, WAHA, Evolution e midias)"
  } else {
    "containers locais; os volumes persistentes serao preservados"
  }

  throw @"
Operacao bloqueada: este comando removeria $scope.
Para confirmar deliberadamente, execute novamente com:
  -Confirmation $expectedConfirmation
Para apenas parar a stack e preservar tudo, use: npm run dev:down
"@
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoDir = (Resolve-Path (Join-Path $scriptDir "..\..")).Path
$composePath = Join-Path $repoDir "docker-compose.yml"

if (-not (Test-Path -LiteralPath $composePath -PathType Leaf)) {
  throw "Compose canonico nao encontrado: $composePath"
}
if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
  throw "docker nao encontrado no PATH."
}

Push-Location $repoDir
try {
  if ($RemoveVolumes) {
    Write-Warning "Removendo containers E volumes persistentes do projeto local Omni."
    & docker compose -f $composePath down --volumes
  } else {
    Write-Warning "Removendo containers do projeto local Omni; volumes serao preservados."
    & docker compose -f $composePath down
  }

  if ($LASTEXITCODE -ne 0) {
    throw "docker compose down falhou com codigo $LASTEXITCODE."
  }
} finally {
  Pop-Location
}
