# start-web-native.ps1 — roda o Nuxt dev NATIVO no Windows (fora do Docker).
# Por quê: o dev dentro do Docker lê os fontes pela ponte FS do Windows com
# polling — compilação on-demand de cada rota levava ~10s e o watcher queimava
# CPU. Nativo, o Vite usa FS events reais e compila em fração disso.
#
# Produção NÃO muda: continua build multi-stage no Docker/GHCR.
#
# Pré-requisitos (one-time):
#   1. Node 24.x no host (mesma major do container)
#   2. cd web && npm ci        <- SEMPRE npm ci, NUNCA npm install
#      (npm install reescreve o package-lock e quebra o npm ci do container
#       — já aconteceu; ver docs/LEGADO e memória do projeto)
#   3. API e postgres continuam no Docker: docker compose up -d api
#
# Uso (PowerShell 5.1 do Windows):
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev/start-web-native.ps1
#   (para voltar pro dev no Docker: Ctrl+C aqui e `docker compose up -d web`)

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$web = Join-Path $repo 'web'

if (-not (Test-Path (Join-Path $web 'node_modules\.bin\nuxt.cmd')) -and -not (Test-Path (Join-Path $web 'node_modules\.bin\nuxt'))) {
  Write-Host "node_modules do host ausente. Rode primeiro:  cd web; npm ci" -ForegroundColor Yellow
  exit 1
}

# Evitar conflito de porta com o web do Docker (regra do projeto: web = 3003)
$dockerWeb = docker ps --format '{{.Names}}' 2>$null | Select-String 'web'
if ($dockerWeb) {
  Write-Host "Parando o container web do Docker (porta 3003)..." -ForegroundColor Cyan
  docker compose stop web | Out-Null
}

# Espelha o env do serviço web do docker-compose, trocando os endereços
# internos do Docker pelos do host. Polling DESLIGADO — é o ponto do nativo.
$env:NODE_ENV = 'development'
$env:PORT = '3003'
$env:NUXT_PUBLIC_API_BASE = 'http://localhost:9091'
$env:NUXT_API_INTERNAL_BASE = 'http://localhost:9091'   # no Docker era http://api:8080
$env:NUXT_PUBLIC_BIO_FRONT_URL = 'http://localhost:3300'
$env:NUXT_DEVTOOLS = 'false'
$env:CHOKIDAR_USEPOLLING = 'false'
$env:WATCHPACK_POLLING = 'false'

Write-Host "Nuxt dev nativo em http://localhost:3003 (api Docker em :9091)" -ForegroundColor Green
Set-Location $web
# --host :: (dual-stack) em vez do npm run dev (--host 0.0.0.0): com wildcard
# IPv4 o socket auxiliar do nuxi abocanha o [::]:3003 e o browser (que tenta
# IPv6 primeiro no Windows) cai num 426 Upgrade Required em vez do painel.
& (Join-Path $web 'node_modules\.bin\nuxt.cmd') dev --host '::' --port 3003
