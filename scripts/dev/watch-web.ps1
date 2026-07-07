# watch-web.ps1 — dev do web NO Docker via `docker compose watch` (sync).
# Por quê: o bind mount ./web:/app atravessava a ponte 9P do WSL2 (Windows ->
# container), ~100x mais lento por arquivo — boot do nuxt dev e troca de página
# levavam minutos. Agora o código vive DENTRO do container (copiado no build) e
# este watch sincroniza as edições do host pra dentro (inotify real, sem polling).
#
# IMPORTANTE: deixe esta janela ABERTA enquanto desenvolve. Fechar o watch não
# derruba o servidor, mas as edições PARAM de chegar no container (a página
# "congela" no código antigo). Para retomar, rode o script de novo.
#
# Uso (PowerShell do Windows):
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev/watch-web.ps1
#
# O `docker compose watch web` builda a imagem se preciso, sobe o web (e a api,
# por depends_on) e fica assistindo ./web. Ctrl+C para só o watch.

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $repo

# 1) Rebuild incremental + recreate: reconcilia edicoes feitas com o watch
#    DESLIGADO (o watch NAO faz sync inicial — verificado no compose v2.38;
#    ele so rebuilda a imagem no boot, sem recriar o container). O BuildKit so
#    reenvia arquivos alterados, entao isso custa segundos, nao minutos.
Write-Host "Reconciliando codigo (up -d --build web)..." -ForegroundColor Cyan
docker compose up -d --build web

# 2) Watch: sync incremental host -> container enquanto a janela estiver aberta.
Write-Host "Painel: http://localhost:3003 — deixe esta janela aberta (sync ativo)." -ForegroundColor Green
docker compose watch --no-up web
