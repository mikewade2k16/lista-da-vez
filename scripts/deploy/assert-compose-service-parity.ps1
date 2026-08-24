param(
  [string]$DevComposePath,
  [string]$ProdComposePath
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoDir = (Resolve-Path (Join-Path $scriptDir "..\..")).Path

if ([string]::IsNullOrWhiteSpace($DevComposePath)) {
  $DevComposePath = Join-Path $repoDir "docker-compose.yml"
}
if ([string]::IsNullOrWhiteSpace($ProdComposePath)) {
  $ProdComposePath = Join-Path $repoDir "docker-compose.prod.yml"
}

foreach ($composePath in @($DevComposePath, $ProdComposePath)) {
  if (-not (Test-Path -LiteralPath $composePath -PathType Leaf)) {
    throw "Compose nao encontrado: $composePath"
  }
}
if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
  throw "docker nao encontrado no PATH."
}

function Get-ComposeServices {
  param([Parameter(Mandatory = $true)][string]$ComposePath)

  $services = @(& docker compose -f $ComposePath --profile "*" config --services)
  if ($LASTEXITCODE -ne 0) {
    throw "Falha ao listar servicos de $ComposePath"
  }
  return @($services | ForEach-Object { $_.Trim() } | Where-Object { $_ } | Sort-Object -Unique)
}

$devServices = @(Get-ComposeServices -ComposePath $DevComposePath)
$prodServices = @(Get-ComposeServices -ComposePath $ProdComposePath)

# Excecoes deliberadas precisam de justificativa versionada. Qualquer novo servico
# fora desta lista bloqueia o deploy ate ganhar definicao prod ou uma decisao explicita.
$allowedDevOnly = [ordered]@{
  "crow-nuxt" = "preview local do editor de bio; o front publico tera deploy proprio"
  "meta-ads-assistant" = "runner legado/read-only de compatibilidade interna; fora do produto e da producao por decisao arquitetural"
}

$unexpectedDevOnly = @($devServices | Where-Object {
    $_ -notin $prodServices -and -not $allowedDevOnly.Contains($_)
  })
$unexpectedProdOnly = @($prodServices | Where-Object { $_ -notin $devServices })

if ($unexpectedDevOnly.Count -gt 0) {
  throw "Servico(s) do Compose local ausente(s) em producao e sem excecao documentada: $($unexpectedDevOnly -join ', ')"
}
if ($unexpectedProdOnly.Count -gt 0) {
  throw "Servico(s) do Compose de producao sem equivalente local: $($unexpectedProdOnly -join ', ')"
}

Write-Host "Preflight de paridade dos servicos Compose: OK"
foreach ($service in $allowedDevOnly.Keys) {
  if ($service -in $devServices -and $service -notin $prodServices) {
    Write-Host "  local-only permitido: $service - $($allowedDevOnly[$service])"
  }
}
