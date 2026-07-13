<#
.SYNOPSIS
  Importa os workflows de automation/export/ no n8n do dev, reativa e reinicia.

.DESCRIPTION
  POR QUE existe: o n8n guarda os workflows no PROPRIO banco (nao le do arquivo). Entao, toda vez
  que um workflow em automation/export/workflow-*.json muda, o n8n continua rodando a versao ANTIGA
  ate re-importar. Este script faz o ciclo completo num comando so, para ninguem precisar lembrar do
  passo manual:
    1. copia o(s) arquivo(s) para dentro do container;
    2. n8n import:workflow (atualiza pelo id que ja esta no JSON - sem duplicar);
    3. reativa (o import DESATIVA o workflow, pois o JSON exportado vem com "active": false);
    4. reinicia o n8n UMA vez (o webhook so re-registra depois do restart).

  Gotchas embutidos (ja custaram tempo):
    - Rodar o n8n CLI via: docker exec <c> sh -lc "... /tmp/...". Passar /tmp/x direto como
      argumento faz o Git Bash/MSYS converter para caminho Windows (ENOENT no container).
    - O SQLite do n8n usa WAL: docker cp database.sqlite NAO leva as escritas recentes; para
      CONFERIR o banco use n8n export:workflow (le com o WAL aplicado), nao o cp.
    - Este arquivo e SO ASCII de proposito (Windows PowerShell 5.1 le .ps1 sem BOM como ANSI e
      quebra o parser em acentos/travessao).

.EXAMPLE
  npm run n8n:import          # todos os workflow-*.json
  npm run n8n:import:chat     # so o calendar-chat
  pwsh scripts/dev/n8n-import.ps1 -Only calendar-omni
#>
param(
  [string]$Container = "omni-n8n-1",
  [string]$Only = "",
  [switch]$NoRestart
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$dir = Join-Path $root "automation/export"

$files = Get-ChildItem -Path $dir -Filter "workflow-*.json"
if ($Only) { $files = $files | Where-Object { $_.Name -like "*$Only*" } }
if (-not $files) { throw "Nenhum workflow encontrado em $dir (filtro '$Only')." }

$ids = @()
foreach ($f in $files) {
  $json = Get-Content $f.FullName -Raw | ConvertFrom-Json
  if ($json -is [Array]) { $wf = $json[0] } else { $wf = $json }
  $id = "$($wf.id)".Trim()
  if (-not $id) { Write-Warning "  $($f.Name): sem 'id' no JSON, pulando."; continue }
  $remote = "/tmp/n8nimport_$($f.BaseName).json"
  Write-Host "==> $($f.Name) (id=$id)"
  docker cp "$($f.FullName)" "${Container}:${remote}" | Out-Null
  docker exec $Container sh -lc "n8n import:workflow --input=$remote" 2>&1 |
    Select-String -Pattern "Successfully imported|error|ENOENT" |
    ForEach-Object { Write-Host "   $_" }
  $ids += $id
}

foreach ($id in $ids) {
  docker exec $Container sh -lc "n8n update:workflow --id=$id --active=true" 2>&1 | Out-Null
  Write-Host "   reativado: $id"
}

if ($NoRestart) {
  Write-Host "OK: importado(s) + reativado(s). Restart pulado (rode: docker compose restart n8n)."
  return
}

Write-Host "==> reiniciando n8n (re-registra os webhooks)..."
Push-Location $root
try { docker compose restart n8n | Out-Null } finally { Pop-Location }
Write-Host "OK: workflow(s) importado(s), reativado(s) e n8n reiniciado."
