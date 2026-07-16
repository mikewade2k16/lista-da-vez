<#
.SYNOPSIS
  Exporta os workflows do n8n do dev para automation/export/workflow-*.json (par do n8n-import.ps1).

.DESCRIPTION
  POR QUE existe: o n8n guarda os workflows no PROPRIO banco (SQLite, nao le do arquivo). O repo so
  reflete o n8n se alguem EXPORTAR. Ninguem lembra, entao o deploy leva versao velha ou trava ate
  exportar na mao. Este script e o INVERSO do n8n-import.ps1: traz o n8n rodando -> arquivo versionado,
  pelo mapa FIXO id->arquivo (nomes estaveis; NAO derivar do nome do workflow para nao poluir o diff).

  Materializa cada workflow no SHAPE versionado (array com 1 objeto, "active": false), fazendo
  pretty-print de 2 espacos com \n final (LF), byte a byte igual aos arquivos atuais, para minimizar
  ruido de diff. Rodar 2x sem mudar nada no n8n produz ZERO diff na 2a (idempotente).

  Modos de gatilho:
    (sem flag) exporta sempre           -> uso manual: npm run n8n:export
    -Check     nao escreve; sai !=0 se algum arquivo divergir  -> guard do git/pre-commit (AVISA)
    -Sync      verifica e, se divergir, ESCREVE e segue exit 0  -> gatilho do deploy (AUTO-EXPORTA)
  -Only <slug> filtra pelo nome do arquivo (ex.: -Only calendar-chat).

  Anti-vazamento de credencial: o export do n8n traz nos com credentials so como {id,name}. O script
  ABORTA aquele arquivo (exit 3 do checador) se algum node.credentials[*] tiver chave fora de id/name,
  para nunca gravar segredo no repo.

  Gotchas embutidos (herdados do n8n-import.ps1, ja custaram tempo):
    - Rodar o n8n CLI via: docker exec <c> sh -lc "... /tmp/...". Passar /tmp/x direto como argumento
      faz o Git Bash/MSYS converter para caminho Windows (ENOENT no container).
    - O SQLite do n8n usa WAL: NUNCA docker cp database.sqlite para ler estado (nao leva as escritas
      recentes). Sempre n8n export:workflow (le com o WAL aplicado). Aqui o docker cp e do /tmp/exp.json
      JA exportado pelo CLI (byte-exato), nao do banco.
    - Este arquivo e SO ASCII de proposito (Windows PowerShell 5.1 le .ps1 sem BOM como ANSI e quebra o
      parser em acentos/travessao). Sem &&, ternario ou ?? (nao existem no 5.1).

.EXAMPLE
  npm run n8n:export          # todos os workflow-*.json (uso manual)
  npm run n8n:export:chat     # so o calendar-chat
  npm run n8n:export:check    # sai 1 se o repo esta atras do n8n (guard)
  npm run n8n:export:sync     # auto-exporta se divergir e segue (deploy)
#>
param(
  [string]$Container = "omni-n8n-1",
  [string]$Only = "",
  [switch]$Check,     # nao escreve; sai !=0 se algum arquivo divergir (guard do pre-commit: AVISA)
  [switch]$Sync       # verifica e, se divergir, ESCREVE e segue exit 0 (gatilho do deploy: AUTO-EXPORTA)
)

$ErrorActionPreference = "Stop"

if ($Check -and $Sync) { throw "-Check e -Sync sao mutuamente exclusivos." }

$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$dir = Join-Path $root "automation/export"
if (-not (Test-Path $dir)) { throw "Diretorio nao encontrado: $dir" }

# MAPA FIXO id -> arquivo. MESMO conjunto de n8n-import.ps1 (os 5 workflow-*.json) e de
# deploy-pull.ps1 (linha ~246, lista de ids reativados). Contrato: os tres devem casar.
# NAO derivar o nome do arquivo do nome do workflow (nome muda sem renomear arquivo).
$map = @(
  @{ id = "calendarchat0001"; file = "workflow-calendar-chat.json" },
  @{ id = "calendaromni0001"; file = "workflow-calendar-omni.json" },
  @{ id = "calendartrans001"; file = "workflow-calendar-transcribe.json" },
  @{ id = "omnichatmvp00001"; file = "workflow-omni-chat.json" },
  @{ id = "lzhb5JjN5kdcVuRR"; file = "workflow-whatsapp.json" }
)
if ($Only) { $map = @($map | Where-Object { $_.file -like "*$Only*" }) }
if (-not $map -or $map.Count -eq 0) { throw "Nenhum workflow no mapa para o filtro '$Only'." }

# Confirma que o container esta up (fonte da verdade local). Se estiver fora, nao ha o que exportar.
$psId = docker ps -q -f "name=^${Container}$" 2>$null
if (-not $psId) { throw "Container n8n '$Container' nao esta rodando. Suba com: docker compose --profile automation up -d n8n" }

# Normalizador + checador de credencial em JS (o container e o host tem Node). Vai para um arquivo
# .js TEMPORARIO (nao versionado): a spec dos gotchas do n8n manda usar arquivo .js em vez de node -e
# inline (o shell/PowerShell comeria o $). Aqui e here-string SINGLE-QUOTE (literal), $ fica intacto.
$normJs = @'
"use strict";
// args: <rawPath> <targetPath> <mode>   mode = write | check
// stdout: uma linha STATUS:written | STATUS:unchanged | STATUS:drift
// exit:   0 ok  |  3 vazamento de credencial  |  4 erro de parse/IO  |  5 uso incorreto
const fs = require("fs");
const ALLOWED = ["id", "name"];
function fail(code, msg) { process.stderr.write(msg + "\n"); process.exit(code); }
const raw = process.argv[2], target = process.argv[3], mode = process.argv[4];
if (!raw || !target || (mode !== "write" && mode !== "check")) fail(5, "uso: node norm.js <raw> <target> <write|check>");
let parsed;
try { parsed = JSON.parse(fs.readFileSync(raw, "utf8")); }
catch (e) { fail(4, "erro ao ler/parsear export do n8n: " + e.message); }
const wf = Array.isArray(parsed) ? parsed[0] : parsed;
if (!wf || typeof wf !== "object") fail(4, "export do n8n vazio ou invalido");
// Anti-vazamento: cada node.credentials[k] so pode ter id/name.
for (const node of (wf.nodes || [])) {
  const creds = node && node.credentials;
  if (!creds || typeof creds !== "object") continue;
  for (const k of Object.keys(creds)) {
    const c = creds[k];
    if (c && typeof c === "object") {
      for (const field of Object.keys(c)) {
        if (!ALLOWED.includes(field)) {
          fail(3, "VAZAMENTO de credencial em '" + (node.name || node.id || "?") + "' credential '" + k + "': campo '" + field + "' fora de id/name. Export ABORTADO (nada gravado).");
        }
      }
    }
  }
}
// Projeta APENAS as 10 chaves do shape versionado, na ordem canonica do baseline
// (o CLI do n8n exporta ~21 chaves: updatedAt/createdAt/isArchived/shared/... que
// NAO entram no repo, senao cada export vira ruido gigante de diff). active sempre
// false (o import desativa e o deploy reativa; manter false evita flip-flop no diff).
const KEYS = ["id", "name", "nodes", "connections", "active", "settings", "staticData", "meta", "pinData", "versionId"];
const projected = {};
for (const key of KEYS) {
  if (key === "active") { projected.active = false; }
  else if (key in wf) { projected[key] = wf[key]; }
  else if (key === "pinData") { projected.pinData = {}; }
  else if (key === "staticData") { projected.staticData = null; }
  // demais chaves ausentes sao omitidas (nao deveriam faltar num export valido)
}
const out = JSON.stringify([projected], null, 2) + "\n"; // 2 espacos + \n final (LF), igual aos arquivos atuais
let current = null;
if (fs.existsSync(target)) { try { current = fs.readFileSync(target, "utf8"); } catch (e) { current = null; } }
const same = current !== null && current === out;
if (mode === "check") { process.stdout.write(same ? "STATUS:unchanged\n" : "STATUS:drift\n"); process.exit(0); }
// mode write: so escreve se mudou (evita mexer no mtime a toa); fs.writeFileSync grava UTF-8 sem BOM e LF.
if (same) { process.stdout.write("STATUS:unchanged\n"); process.exit(0); }
fs.writeFileSync(target, out); process.stdout.write("STATUS:written\n"); process.exit(0);
'@

$tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("n8n-export-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null
$normPath = Join-Path $tmpDir "norm.js"
# WriteAllText: UTF-8 sem BOM, sem traducao CRLF (o JS e ASCII, mas garante bytes limpos).
[System.IO.File]::WriteAllText($normPath, $normJs, (New-Object System.Text.UTF8Encoding($false)))

$mode = if ($Check) { "check" } else { "write" }  # -Sync faz check primeiro, depois write so no que divergir
$drift = $false
$exported = 0
$missing = @()

try {
  foreach ($m in $map) {
    $id = $m.id
    $target = Join-Path $dir $m.file
    $remoteExp = "/tmp/exp_$id.json"
    $localRaw = Join-Path $tmpDir "$id.json"

    # 1) Exporta o workflow do container para /tmp (via sh -lc: evita conversao de path do MSYS).
    docker exec $Container sh -lc "n8n export:workflow --id=$id --output=$remoteExp >/dev/null 2>&1" | Out-Null
    if ($LASTEXITCODE -ne 0) {
      Write-Warning "  $($m.file): id '$id' nao existe no n8n (export falhou). Pulando."
      $missing += $m.file
      continue
    }
    # 2) Traz o /tmp/exp.json JA exportado para o host, byte-exato (docker cp, nao cat via pipe do PS).
    docker cp "${Container}:${remoteExp}" "$localRaw" | Out-Null
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path $localRaw)) {
      Write-Warning "  $($m.file): falha ao trazer o export do container (docker cp). Pulando."
      $missing += $m.file
      continue
    }

    if ($Sync) {
      # -Sync: primeiro checa; so escreve se divergir (auto-export).
      $status = & node $normPath $localRaw $target "check"
      $rc = $LASTEXITCODE
      if ($rc -eq 3) { throw "  $($m.file): $status" }  # vazamento de credencial: aborta tudo
      if ($rc -ne 0) { throw "  $($m.file): erro do checador (exit $rc)." }
      if ("$status".Contains("drift")) {
        $wstatus = & node $normPath $localRaw $target "write"
        $wrc = $LASTEXITCODE
        if ($wrc -eq 3) { throw "  $($m.file): $wstatus" }
        if ($wrc -ne 0) { throw "  $($m.file): erro ao escrever (exit $wrc)." }
        if ("$wstatus".Contains("written")) { $exported++; Write-Host "   exportado (estava atras): $($m.file)" }
      }
    }
    else {
      # normal (write sempre-se-mudou) ou -Check (nunca escreve).
      $status = & node $normPath $localRaw $target $mode
      $rc = $LASTEXITCODE
      if ($rc -eq 3) { throw "  $($m.file): $status" }  # vazamento: aborta tudo (nada gravado)
      if ($rc -ne 0) { throw "  $($m.file): erro do normalizador (exit $rc)." }
      if ($Check) {
        if ("$status".Contains("drift")) { $drift = $true; Write-Host "   DIVERGE: $($m.file)" }
      }
      else {
        if ("$status".Contains("written")) { Write-Host "   exportado: $($m.file)" }
        else { Write-Host "   ja atual: $($m.file)" }
      }
    }
  }
}
finally {
  Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue
}

if ($missing.Count -gt 0) {
  Write-Warning "n8n-export: $($missing.Count) workflow(s) do mapa nao encontrados no n8n: $($missing -join ', ')"
}

if ($Check) {
  if ($drift) {
    Write-Host "n8n-export: os arquivos versionados estao ATRAS do n8n rodando. Rode: npm run n8n:export"
    exit 1   # guard do git: AVISA (pre-commit decide se bloqueia ou so alerta; ver AGENT.md / spec 4.3-A).
  }
  Write-Host "n8n-export: repo alinhado com o n8n (nenhuma divergencia)."
  exit 0
}

if ($Sync) {
  if ($exported -gt 0) {
    Write-Host "n8n-export: n8n estava a frente; $exported arquivo(s) exportado(s) automaticamente. Deploy seguira com a versao atual (NAO commitado; o dono commita depois)."
  }
  else {
    Write-Host "n8n-export: repo ja estava alinhado com o n8n (nada a exportar)."
  }
  # exit 0 de proposito: no deploy queremos SEGUIR com o export fresco, nunca travar.
  exit 0
}

Write-Host "OK: export concluido."
exit 0
