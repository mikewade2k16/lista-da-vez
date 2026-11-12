$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'release-web-container.ps1')
$root = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
Push-Location $root
try {
  & node --test scripts/deploy/run-deploy.test.mjs
  if ($LASTEXITCODE -ne 0) { throw 'Testes do launcher falharam.' }
  $files = @(& git diff --name-only --diff-filter=ACMRT HEAD) + @(& git ls-files --others --exclude-standard)
  $sources = @($files | Where-Object { $_ -match '^web/.*\.(vue|ts|js|mjs)$' -and $_ -notmatch '\.test\.' -and $_ -notmatch '/roadmap/data/' } | Sort-Object -Unique | ForEach-Object { $_.Substring(4) })
  $cardapioChanged = [bool]($files | Where-Object { $_ -match '^web/.*cardapio' })
  $cardapioTests = @('app/domain/cardapio/product-filters.test.ts', 'app/composables/useCardapioImportCompletion.test.ts', 'app/domain/cardapio/product-transfer.test.ts', 'app/domain/cardapio/money-input.test.ts', 'app/stores/cardapio.test.ts')
  $syncFiles = @($sources) + @(if ($cardapioChanged) { $cardapioTests })
  if ($syncFiles.Count) {
    $webContainer = Get-ReleaseWebContainer
  }
  if ($syncFiles.Count) {
    foreach ($source in ($syncFiles | Sort-Object -Unique)) {
      # docker cp nao cria diretorios pais para arquivos novos no container.
      $parent = [System.IO.Path]::GetDirectoryName($source).Replace('\', '/')
      & docker exec $webContainer mkdir -p -- "/app/$parent"
      if ($LASTEXITCODE -ne 0) { throw "Falha ao criar diretorio para $source na validacao." }
      & docker cp (Join-Path $root "web/$source") "${webContainer}:/app/$source"
      if ($LASTEXITCODE -ne 0) { throw "Falha ao sincronizar $source para validacao." }
    }
  }
  if ($sources.Count) {
    # A configuracao do ESLint depende de .nuxt; aguarde a geracao completa.
    & docker exec $webContainer npx nuxt prepare
    if ($LASTEXITCODE -ne 0) { throw 'Falha ao preparar a configuracao Nuxt para o lint.' }
    & docker exec $webContainer npx eslint --no-warn-ignored @sources
    if ($LASTEXITCODE -ne 0) { throw 'Lint dos arquivos alterados falhou.' }
  }
  if ($cardapioChanged) {
    & docker exec $webContainer npx vitest run @cardapioTests
    if ($LASTEXITCODE -ne 0) { throw 'Testes do cardapio falharam.' }
  }
  $packages = @($files | Where-Object { $_ -match '^back/.*\.go$' } | ForEach-Object { './' + (($_ -replace '^back/', '') -replace '/[^/]+$', '') } | Sort-Object -Unique)
  if ($packages.Count) {
    $platform = (& docker info --format '{{.Architecture}}').Trim()
    $architecture = if ($platform -in @('aarch64', 'arm64')) { 'arm64' } else { 'amd64' }
    & docker build --target build --platform "linux/$architecture" -t omni-deploy-go-check back
    if ($LASTEXITCODE -ne 0) { throw 'Falha ao preparar verificacoes Go.' }
    $automationPath = (Resolve-Path (Join-Path $root 'automation')).Path
    # Os testes do Calendar leem exports versionados no repositorio irmão de back/.
    # A imagem Go é construída com contexto back/, então monte automation no caminho
    # esperado pelo teste sem empacotar os workflows no binário ou na imagem de produção.
    $automationMount = "type=bind,source=$automationPath,target=/automation,readonly"
    & docker run --rm --network none --mount $automationMount omni-deploy-go-check go test @packages
    if ($LASTEXITCODE -ne 0) { throw 'Testes dos pacotes Go alterados falharam.' }
    & docker run --rm --network none --mount $automationMount omni-deploy-go-check go vet @packages
    if ($LASTEXITCODE -ne 0) { throw 'go vet dos pacotes alterados falhou.' }
  }
} finally { Pop-Location }
