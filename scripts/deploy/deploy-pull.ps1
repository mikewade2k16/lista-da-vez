param(
  [ValidateSet("staging", "prod")]
  [string]$Environment = "staging",
  # Tag da imagem no GHCR a subir. Vazio => "sha-<git HEAD>" (a tag imutavel que o
  # build-images.yml publica com type=sha,format=long). Tem que JA existir no GHCR
  # (ou seja, o CI precisa ter buildado esse SHA).
  [string]$Tag = "",
  [string]$VpsHost = "85.31.62.33",
  [int]$Port = 22,
  [string]$User = "deploy",
  [string]$KeyPath = (Join-Path $HOME ".ssh\gh_actions_omnichannel_vps"),
  [switch]$BackupDatabase,
  [switch]$ForceRecreate,
  [switch]$SkipSmokeTests,
  # Sobe/reconcilia tambem o profile automation (redis/waha/n8n/whisper) e reimporta
  # os workflows versionados em automation/export/workflow-*.json quando houver mudanca.
  # Nao envia credentials.decrypted.json; credenciais continuam geridas no volume do n8n.
  [switch]$DeployAutomation,
  [switch]$ForceAutomationWorkflowImport,
  # Login opcional no GHCR antes do pull (imagens privadas). Se vazios, assume que
  # a VPS ja tem `docker login ghcr.io` valido (~/.docker/config.json).
  [string]$GhcrUser = "",
  [string]$GhcrToken = ""
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoDir = (Resolve-Path (Join-Path $scriptDir "..\..")).Path
$composeLocal = Join-Path $repoDir "docker-compose.prod.yml"
$workflowLocalDir = Join-Path $repoDir "automation\export"
$composeEnvGuard = Join-Path $scriptDir "assert-compose-api-env.ps1"

if (-not (Test-Path $KeyPath)) { throw "Chave SSH nao encontrada em $KeyPath" }
if (-not (Test-Path $composeLocal)) { throw "docker-compose.prod.yml nao encontrado em $composeLocal" }
if (-not (Test-Path -LiteralPath $composeEnvGuard -PathType Leaf)) {
  throw "Preflight de env do Compose nao encontrado: $composeEnvGuard"
}
& $composeEnvGuard -ComposePath $composeLocal

$automationWorkflowFiles = @()
if ($DeployAutomation) {
  if (-not (Test-Path $workflowLocalDir)) { throw "Diretorio de workflows nao encontrado em $workflowLocalDir" }
  $automationWorkflowFiles = @(Get-ChildItem -Path $workflowLocalDir -Filter "workflow-*.json" -File | Sort-Object Name)
  if ($automationWorkflowFiles.Count -eq 0) { throw "Nenhum workflow-*.json encontrado em $workflowLocalDir" }
}

# Config por ambiente.
switch ($Environment) {
  "prod" {
    $envFile = ".env.production"
    $remotePath = "/home/deploy/lista-atendimento"
    $publicBaseUrl = "https://omni.crowvisuals.com.br"
  }
  "staging" {
    $envFile = ".env.staging"
    $remotePath = "/home/deploy/lista-atendimento-staging"
    $publicBaseUrl = "https://preview.whenthelightsdie.com"
  }
}

# Resolve a tag: default = sha-<HEAD>.
if ([string]::IsNullOrWhiteSpace($Tag)) {
  $headSha = (& git -C $repoDir rev-parse HEAD).Trim()
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($headSha)) {
    throw "Nao consegui resolver o SHA do HEAD; passe -Tag explicitamente."
  }
  $Tag = "sha-$headSha"
  Write-Host "Tag nao informada; usando o HEAD local: $Tag"
  Write-Host "AVISO: confirme que o build-images.yml ja publicou essa tag no GHCR."
}

$SshExe = Join-Path $env:SystemRoot "System32\OpenSSH\ssh.exe"
$ScpExe = Join-Path $env:SystemRoot "System32\OpenSSH\scp.exe"
foreach ($exe in @($SshExe, $ScpExe)) {
  if (-not (Test-Path $exe)) { throw "Executavel nao encontrado: $exe" }
}

$resolvedKeyPath = (Resolve-Path $KeyPath).Path
$sshHardening = @(
  "-o", "BatchMode=yes",
  "-o", "ConnectTimeout=15",
  "-o", "ServerAliveInterval=10",
  "-o", "ServerAliveCountMax=3"
)
$sshArgs = @("-i", $resolvedKeyPath, "-o", "StrictHostKeyChecking=accept-new", "-p", $Port.ToString()) + $sshHardening
$scpBaseArgs = @("-i", $resolvedKeyPath, "-o", "StrictHostKeyChecking=accept-new", "-P", $Port.ToString()) + $sshHardening
$remoteTarget = "$User@$VpsHost"
$forceRecreateFlag = if ($ForceRecreate) { " --force-recreate" } else { "" }

function Convert-ToBashSingleQuoted {
  param([Parameter(Mandatory = $true)][string]$Value)
  $replacement = "'" + '"' + "'" + '"' + "'"
  return "'" + $Value.Replace("'", $replacement) + "'"
}

function Invoke-RemoteCommand {
  param(
    [Parameter(Mandatory = $true)][string]$Description,
    [Parameter(Mandatory = $true)][string]$Command,
    [switch]$CaptureOutput
  )
  Write-Host "==> $Description"
  # Entrega o script pela STDIN do bash remoto, NAO como argumento do ssh.exe: passar
  # como argumento faz o PowerShell 5.1 comer as aspas duplas embutidas (o
  # `sed -i "s|...|...|"` e o `pg_dump -U "$POSTGRES_USER"` viram lixo no shell remoto).
  # Detalhe: ao fazer pipe pra stdin de um nativo, o PS 5.1 reconverte \n em \r\n, e o
  # \r quebra o bash (`cd <dir>\r` => "No such file or directory"). Por isso o remoto
  # roda `tr -d '\r' | bash -s` pra limpar o CR antes de executar.
  $normalizedCommand = $Command -replace "`r`n", "`n" -replace "`r", "`n"
  if ($CaptureOutput) {
    $output = $normalizedCommand | & $SshExe @sshArgs $remoteTarget "tr -d '\r' | bash -s"
    if ($LASTEXITCODE -ne 0) { throw "Falha ao executar: $Description" }
    return (($output | ForEach-Object { $_.ToString() }) -join "`n").Trim()
  }
  $normalizedCommand | & $SshExe @sshArgs $remoteTarget "tr -d '\r' | bash -s"
  if ($LASTEXITCODE -ne 0) { throw "Falha ao executar: $Description" }
}

$remotePathQ = Convert-ToBashSingleQuoted $remotePath
$envFileQ = Convert-ToBashSingleQuoted $envFile
$composeArgs = "--env-file $envFileQ -f docker-compose.prod.yml"
$composeAutomationArgs = "$composeArgs --profile automation"

Write-Host "Ambiente:     $Environment"
Write-Host "Host remoto:  ${remoteTarget}:$remotePath"
Write-Host "Imagem tag:   $Tag"
if ($DeployAutomation) {
  Write-Host "Automation:   redis/waha/n8n/whisper + $($automationWorkflowFiles.Count) workflow(s)"
}

# 1. Falha ANTES de copiar compose/trocar tag se o segredo obrigatorio do boot
#    estiver ausente ou malformado. Nunca imprime o valor da chave.
$preflightCmd = @"
set -euo pipefail
mkdir -p $remotePathQ
cd $remotePathQ
if [ ! -f $envFileQ ]; then
  echo "ERRO: $envFile nao existe em $remotePath. Crie-o a partir do .example antes do deploy." >&2
  exit 1
fi
omni_key=`$(sed -n 's/^OMNI_SECRETS_KEY=//p' $envFileQ | tail -n 1)
if [ -z "`$omni_key" ]; then
  echo "ERRO: OMNI_SECRETS_KEY ausente/vazia em $envFile. Gere uma vez com: openssl rand -base64 32" >&2
  exit 1
fi
if ! printf '%s' "`$omni_key" | grep -Eq '^[A-Za-z0-9+/]{43}=$'; then
  echo "ERRO: OMNI_SECRETS_KEY nao e base64 canonico de 32 bytes em $envFile." >&2
  exit 1
fi
if ! decoded_bytes=`$(printf '%s' "`$omni_key" | openssl base64 -d -A 2>/dev/null | wc -c | tr -d ' '); then
  echo "ERRO: OMNI_SECRETS_KEY nao e base64 valido em $envFile." >&2
  exit 1
fi
unset omni_key
if [ "`$decoded_bytes" != "32" ]; then
  echo "ERRO: OMNI_SECRETS_KEY deve decodificar exatamente 32 bytes; encontrado(s): `$decoded_bytes." >&2
  exit 1
fi
echo "Preflight de segredos obrigatorios: OK"
"@
Invoke-RemoteCommand -Description "Validando ambiente remoto antes do deploy" -Command $preflightCmd

# 2. Envia o compose (a VPS so precisa do compose + .env; o codigo vive nas
#    imagens). O preflight acima garante que um erro de env nao altere nem o compose remoto.

Write-Host "==> Enviando docker-compose.prod.yml"
$scpArgs = $scpBaseArgs + @($composeLocal, "${remoteTarget}:${remotePath}/docker-compose.prod.yml")
& $ScpExe @scpArgs
if ($LASTEXITCODE -ne 0) { throw "Falha ao enviar o docker-compose.prod.yml (scp)." }

# 3. Valida o compose novo contra o env real e so entao grava o IMAGE_TAG escolhido.
$tagQ = Convert-ToBashSingleQuoted $Tag
$setTagCmd = @"
set -euo pipefail
cd $remotePathQ
docker compose $composeArgs config --quiet
if grep -q '^IMAGE_TAG=' $envFileQ; then
  sed -i "s|^IMAGE_TAG=.*|IMAGE_TAG=$Tag|" $envFileQ
else
  printf 'IMAGE_TAG=%s\n' $tagQ >> $envFileQ
fi
grep '^IMAGE_TAG=' $envFileQ
"@
Invoke-RemoteCommand -Description "Gravando IMAGE_TAG=$Tag em $envFile" -Command $setTagCmd

# 4. Login opcional no GHCR (imagens privadas).
if (-not [string]::IsNullOrWhiteSpace($GhcrToken)) {
  if ([string]::IsNullOrWhiteSpace($GhcrUser)) { throw "-GhcrToken informado sem -GhcrUser." }
  $loginCmd = "printf '%s' " + (Convert-ToBashSingleQuoted $GhcrToken) + " | docker login ghcr.io -u " + (Convert-ToBashSingleQuoted $GhcrUser) + " --password-stdin"
  Invoke-RemoteCommand -Description "docker login ghcr.io" -Command $loginCmd
}

# 5. Backup opcional do Postgres (antes de mexer nos containers).
if ($BackupDatabase) {
  $backupCmd = @"
set -euo pipefail
cd $remotePathQ
mkdir -p backups
docker compose $composeArgs exec -T postgres sh -lc 'pg_dump -U "`$POSTGRES_USER" -d "`$POSTGRES_DB"' | gzip > "backups/backup_`$(date +%Y%m%d_%H%M%S).sql.gz"
latest=`$(ls -t backups | head -n 1)
printf '%s\n' "$remotePath/backups/`$latest"
"@
  $backupFile = Invoke-RemoteCommand -Description "Backup remoto do Postgres ($Environment)" -Command $backupCmd -CaptureOutput
  if ($backupFile) { Write-Host "Backup remoto: $backupFile" }
}

# 6. Pull + up SEM build (a VPS nunca compila).
# AC-04b (self-healing): a partir da imagem que contem o ac-04b, o `migrate up` auto-provisiona
# a role least-privilege `omni_app` no boot (cria + converge senha + grant connect a partir de
# DATABASE_APP_URL) e em production falha alto e cedo se faltar senha — criar a role deixou de
# ser pre-requisito manual. IMAGENS ANTIGAS (pre-ac-04b): a api conecta como `omni_app` mas NAO
# a cria; se a role nao existir, entra em crash-loop (28P01), a web nao sobe e o painel da 502 —
# este script NAO cria a role. Runbook (fallback) em docs/DEPLOY_VPS.md (secao "Role de runtime
# omni_app").
$deployCmd = @"
set -euo pipefail
cd $remotePathQ
docker compose $composeArgs pull api web
docker compose $composeArgs up -d --no-build$forceRecreateFlag api web
docker compose $composeArgs ps api web
"@
Invoke-RemoteCommand -Description "Pull + up --no-build (api web) no $Environment" -Command $deployCmd

# 7. Automation opcional: sobe profile automation e reimporta workflows versionados se mudaram.
if ($DeployAutomation) {
  $remoteWorkflowDir = "$remotePath/automation/export"
  $remoteWorkflowDirQ = Convert-ToBashSingleQuoted $remoteWorkflowDir
  Invoke-RemoteCommand -Description "Garantindo diretorio remoto dos workflows n8n" -Command "mkdir -p $remoteWorkflowDirQ"

  Write-Host "==> Enviando workflows n8n versionados (sem credenciais)"
  foreach ($workflowFile in $automationWorkflowFiles) {
    $remoteWorkflowPath = "${remoteTarget}:${remoteWorkflowDir}/$($workflowFile.Name)"
    $workflowScpArgs = $scpBaseArgs + @($workflowFile.FullName, $remoteWorkflowPath)
    & $ScpExe @workflowScpArgs
    if ($LASTEXITCODE -ne 0) { throw "Falha ao enviar workflow $($workflowFile.Name) (scp)." }
  }

  $forceAutomationImportValue = if ($ForceAutomationWorkflowImport) { "1" } else { "0" }
  $automationCmd = @"
set -euo pipefail
cd $remotePathQ
mkdir -p .deploy backups/n8n

docker compose $composeAutomationArgs pull redis waha n8n whisper
docker compose $composeAutomationArgs up -d --no-build$forceRecreateFlag redis waha n8n whisper
docker compose $composeAutomationArgs ps redis waha n8n whisper

manifest=`$(find automation/export -maxdepth 1 -type f -name 'workflow-*.json' -print0 | sort -z | xargs -0 sha256sum)
marker=.deploy/automation-workflows.sha256
changed=1
if [ "$forceAutomationImportValue" = "0" ] && [ -f "`$marker" ] && printf '%s\n' "`$manifest" | cmp -s "`$marker" -; then
  changed=0
fi

if [ "`$changed" = "0" ]; then
  echo "Workflows n8n sem mudanca; import pulado."
  exit 0
fi

backup_name="backups/n8n/workflows-before-automation-deploy_`$(date +%Y%m%d_%H%M%S).json"
active_ids_file=.deploy/n8n-active-workflows-before.txt
: > "`$active_ids_file"
if docker compose $composeAutomationArgs exec -T n8n n8n export:workflow --all --output=/tmp/workflows-before-automation-deploy.json; then
  docker compose $composeAutomationArgs exec -T n8n node -e "const fs=require('fs'); const raw=JSON.parse(fs.readFileSync('/tmp/workflows-before-automation-deploy.json','utf8')); const list=Array.isArray(raw)?raw:(Array.isArray(raw.data)?raw.data:(Array.isArray(raw.workflows)?raw.workflows:[raw])); for (const w of list) { if (w && w.id && (w.active===true || w.active==='true' || w.isActive===true)) console.log(w.id); }" > "`$active_ids_file" || true
  docker compose $composeAutomationArgs cp n8n:/tmp/workflows-before-automation-deploy.json "`$backup_name" || true
  echo "Backup workflows n8n: $remotePath/`$backup_name"
else
  echo "AVISO: nao consegui exportar backup dos workflows n8n; seguindo com import." >&2
fi

import_ok=1
for wf in automation/export/workflow-*.json; do
  base=`$(basename "`$wf")
  docker compose $composeAutomationArgs cp "`$wf" "n8n:/tmp/`$base"
  # O import tem de reportar "Successfully imported"; senao a versao nova NAO entrou
  # (import silencioso ja deixou a VPS defasada do arquivo por dias). Falhou -> nao
  # grava o marker (proximo deploy re-tenta em vez de "marcar como feito").
  import_out=`$(docker compose $composeAutomationArgs exec -T n8n n8n import:workflow --input="/tmp/`$base" 2>&1)
  printf '%s\n' "`$import_out" | grep -qiE 'Successfully imported' || { echo "ERRO: import de `$base falhou:" >&2; printf '%s\n' "`$import_out" >&2; import_ok=0; }
done

# Mantem ativos os workflows internos que prod usa hoje e preserva qualquer workflow
# que ja estava ativo antes do import (ex.: WhatsApp, quando estiver em uso).
{
  printf '%s\n' calendaromni0001 calendarchat0001 calendartrans001 omnichatmvp00001
  cat "`$active_ids_file"
} | awk 'NF && !seen[`$0]++' | while IFS= read -r workflow_id; do
  docker compose $composeAutomationArgs exec -T n8n n8n update:workflow --id="`$workflow_id" --active=true
done
docker compose $composeAutomationArgs restart n8n

# Verificacao pos-import: o n8n so re-le o workflow do banco depois do restart. Confere
# que cada workflow versionado REALMENTE ficou identico ao que esta no banco do n8n
# (via export:workflow, que aplica o WAL). Se algum divergir, o import nao pegou de fato
# (marker enganoso, WAL, id trocado) -> NAO grava o marker, para o proximo deploy re-tentar.
verify_ok=1
sleep 8
for wf in automation/export/workflow-*.json; do
  wid=`$(docker compose $composeAutomationArgs exec -T n8n node -e "console.log((JSON.parse(require('fs').readFileSync(process.argv[1],'utf8')).id)||'')" "/tmp/`$(basename "`$wf")" 2>/dev/null | tr -d '\r')
  [ -z "`$wid" ] && continue
  if docker compose $composeAutomationArgs exec -T n8n n8n export:workflow --id="`$wid" --output="/tmp/verify_`$wid.json" >/dev/null 2>&1; then
    # compara os 'nodes' normalizados (ordem estavel) entre arquivo e banco
    same=`$(docker compose $composeAutomationArgs exec -T n8n node -e "
      const fs=require('fs');
      const norm=o=>JSON.stringify(((Array.isArray(o)?o[0]:o).nodes||[]).map(n=>({name:n.name,type:n.type,parameters:n.parameters})).sort((a,b)=>a.name<b.name?-1:1));
      const a=JSON.parse(fs.readFileSync(process.argv[1],'utf8'));
      const b=JSON.parse(fs.readFileSync(process.argv[2],'utf8'));
      process.stdout.write(norm(a)===norm(b)?'SAME':'DIFF');
    " "/tmp/`$(basename "`$wf")" "/tmp/verify_`$wid.json" 2>/dev/null)
    if [ "`$same" != "SAME" ]; then echo "ERRO: n8n `$wid diverge do arquivo apos import." >&2; verify_ok=0; fi
  else
    echo "AVISO: nao consegui verificar `$wid (export falhou)." >&2
  fi
done

if [ "`$import_ok" != "1" ] || [ "`$verify_ok" != "1" ]; then
  echo "ERRO: import/verificacao de workflows n8n NAO concluiu 100%. Marker nao gravado; rode o deploy de novo (ou com -ForceAutomationWorkflowImport)." >&2
  exit 1
fi

printf '%s\n' "`$manifest" > "`$marker"
echo "Workflows n8n importados, reiniciados e VERIFICADOS (banco == arquivo)."
"@
  Invoke-RemoteCommand -Description "Profile automation + import de workflows n8n no $Environment" -Command $automationCmd
}

# 8. Smoke tests publicos.
if (-not $SkipSmokeTests) {
  $smokeCmd = @"
set -euo pipefail
root=`$(curl -sS -o /dev/null -w "%{http_code}" "$publicBaseUrl")
health=`$(curl -sS -o /dev/null -w "%{http_code}" "$publicBaseUrl/healthz")
printf "GET %s => %s\n" "$publicBaseUrl" "`$root"
printf "GET %s/healthz => %s\n" "$publicBaseUrl" "`$health"
[ "`$root" = "200" ] && [ "`$health" = "200" ]
"@
  Invoke-RemoteCommand -Description "Smoke tests em $publicBaseUrl" -Command $smokeCmd
}

Write-Host ""
Write-Host "Deploy ($Environment) finalizado: imagem $Tag no ar."
