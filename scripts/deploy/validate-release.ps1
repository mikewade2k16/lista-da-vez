$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
Push-Location $root
try {
  & node --test scripts/deploy/run-deploy.test.mjs
  if ($LASTEXITCODE -ne 0) { throw 'Testes do launcher falharam.' }
  $files = @(& git diff --name-only --diff-filter=ACMRT HEAD) + @(& git ls-files --others --exclude-standard)
  $sources = @($files | Where-Object { $_ -match '^web/.*\.(vue|ts|js|mjs)$' -and $_ -notmatch '\.test\.' -and $_ -notmatch '/roadmap/data/' } | Sort-Object -Unique | ForEach-Object { $_.Substring(4) })
  if ($sources.Count) {
    $webContainer = (& docker compose ps -q web).Trim()
    if (-not $webContainer) { throw 'Inicie o container web local para validar o release.' }
    foreach ($source in $sources) {
      & docker cp (Join-Path $root "web/$source") "${webContainer}:/app/$source"
      if ($LASTEXITCODE -ne 0) { throw "Falha ao sincronizar $source para validacao." }
    }
    & docker compose exec -T web npx eslint --no-warn-ignored @sources
    if ($LASTEXITCODE -ne 0) { throw 'Lint dos arquivos alterados falhou.' }
  }
  if ($files | Where-Object { $_ -match '^web/.*cardapio' }) {
    & docker compose exec -T web npx vitest run app/domain/cardapio/product-filters.test.ts app/composables/useCardapioImportCompletion.test.ts app/domain/cardapio/product-transfer.test.ts app/domain/cardapio/money-input.test.ts app/stores/cardapio.test.ts
    if ($LASTEXITCODE -ne 0) { throw 'Testes do cardapio falharam.' }
  }
  $packages = @($files | Where-Object { $_ -match '^back/.*\.go$' } | ForEach-Object { './' + (($_ -replace '^back/', '') -replace '/[^/]+$', '') } | Sort-Object -Unique)
  if ($packages.Count) {
    $platform = (& docker info --format '{{.Architecture}}').Trim()
    $architecture = if ($platform -in @('aarch64', 'arm64')) { 'arm64' } else { 'amd64' }
    & docker build --target build --platform "linux/$architecture" -t omni-deploy-go-check back
    if ($LASTEXITCODE -ne 0) { throw 'Falha ao preparar verificacoes Go.' }
    & docker run --rm --network none omni-deploy-go-check go test @packages
    if ($LASTEXITCODE -ne 0) { throw 'Testes dos pacotes Go alterados falharam.' }
    & docker run --rm --network none omni-deploy-go-check go vet @packages
    if ($LASTEXITCODE -ne 0) { throw 'go vet dos pacotes alterados falhou.' }
  }
} finally { Pop-Location }
