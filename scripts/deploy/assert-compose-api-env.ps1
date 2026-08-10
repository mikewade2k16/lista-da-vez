param(
  [Parameter(Mandatory = $true)]
  [string]$ComposePath,
  [string[]]$RequiredVariables = @(
    "OMNI_SECRETS_KEY",
    "EVOLUTION_BASE_URL",
    "EVOLUTION_API_KEY",
    "WEBHOOK_RECEIVER_BASE_URL",
    "R2_ENABLED",
    "R2_ACCOUNT_ID",
    "R2_BUCKET",
    "R2_ACCESS_KEY_ID",
    "R2_SECRET_ACCESS_KEY",
    "R2_ANALYTICS_API_TOKEN",
    "R2_ALLOW_NONEMPTY_BUCKET_INITIALIZATION"
)
)

$ErrorActionPreference = "Stop"

$resolvedComposePath = (Resolve-Path -LiteralPath $ComposePath).Path
$lines = @(Get-Content -LiteralPath $resolvedComposePath)

$apiStart = -1
for ($index = 0; $index -lt $lines.Count; $index++) {
  if ($lines[$index] -match '^  api:\s*(?:#.*)?$') {
    $apiStart = $index
    break
  }
}
if ($apiStart -lt 0) {
  throw "Preflight Compose: servico api nao encontrado em $resolvedComposePath."
}

$apiEnd = $lines.Count
for ($index = $apiStart + 1; $index -lt $lines.Count; $index++) {
  if ($lines[$index] -match '^  [A-Za-z0-9_-]+:\s*(?:#.*)?$') {
    $apiEnd = $index
    break
  }
}

$environmentStart = -1
for ($index = $apiStart + 1; $index -lt $apiEnd; $index++) {
  if ($lines[$index] -match '^    environment:\s*(?:#.*)?$') {
    $environmentStart = $index
    break
  }
}
if ($environmentStart -lt 0) {
  throw "Preflight Compose: bloco environment do servico api nao encontrado."
}

$environmentEnd = $apiEnd
for ($index = $environmentStart + 1; $index -lt $apiEnd; $index++) {
  if ($lines[$index] -match '^    \S') {
    $environmentEnd = $index
    break
  }
}

$environmentLines = @($lines[($environmentStart + 1)..($environmentEnd - 1)])
$missing = @()
foreach ($variable in $RequiredVariables) {
  $escapedVariable = [regex]::Escape($variable)
  if (-not ($environmentLines | Where-Object { $_ -match "^      ${escapedVariable}:" })) {
    $missing += $variable
  }
}

if ($missing.Count -gt 0) {
  throw "Preflight Compose: api nao recebe variavel(is) critica(s): $($missing -join ', '). Deploy abortado antes de recriar containers."
}

Write-Host "Preflight Compose da api: variaveis criticas declaradas."
