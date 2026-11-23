# Transporte de imagens por registry local + tunel SSH reverso.
#
# O Mac (ou Windows) mantem um registry:2 so em 127.0.0.1:<porta>. O deploy faz push
# para ele (local, segundos) e a VPS faz `docker pull` por DIGEST atraves de
# `ssh -R 127.0.0.1:<porta>:127.0.0.1:<porta>`. Como o pull e por camada, so sobem
# as camadas que a VPS ainda nao tem (tipicamente o .output do web e os binarios da
# API, dezenas de MB), em vez do pacote de ~350 MB do docker save.
#
# Seguranca: o registry escuta apenas no loopback local; o tunel escuta apenas no
# loopback da VPS e existe so durante o pull. O pull por digest e verificado por
# conteudo (sha256) pelo proprio Docker. A VPS nao builda nada.
# Falha em qualquer etapa => o deploy cai para o pacote SFTP (image-archive.ps1).

# Constantes como funcoes: independem do escopo de quem faz dot-source.
function Get-DeployRegistryImage { 'registry:2.8.3' }
function Get-DeployRegistryContainer { 'omni-deploy-registry' }

function Start-DeployLocalRegistry {
  param([int]$Port)
  $container = Get-DeployRegistryContainer
  $running = "$(& docker ps -q --filter "name=^$container$")".Trim()
  if (-not $running) {
    $exists = "$(& docker ps -aq --filter "name=^$container$")".Trim()
    if ($exists) {
      $boundPort = "$(& docker inspect --format '{{(index (index .HostConfig.PortBindings "5000/tcp") 0).HostPort}}' $container)".Trim()
      if ($boundPort -ne "$Port") {
        & docker rm -f $(Get-DeployRegistryContainer) | Out-Null
        $exists = ''
      }
    }
    if ($exists) {
      & docker start $(Get-DeployRegistryContainer) | Out-Null
    } else {
      # Volume nomeado: camadas ja enviadas continuam no registry entre deploys.
      & docker run -d --name $(Get-DeployRegistryContainer) --restart no `
        -e REGISTRY_STORAGE_DELETE_ENABLED=true `
        -p "127.0.0.1:${Port}:5000" -v omni_deploy_registry:/var/lib/registry `
        $(Get-DeployRegistryImage) | Out-Null
    }
    if ($LASTEXITCODE -ne 0) { throw 'Nao consegui iniciar o registry local do deploy.' }
  }
  for ($attempt = 0; $attempt -lt 50; $attempt++) {
    try {
      $response = Invoke-WebRequest -Uri "http://127.0.0.1:$Port/v2/" -UseBasicParsing -TimeoutSec 2
      if ($response.StatusCode -eq 200) { return }
    } catch { Start-Sleep -Milliseconds 200 }
  }
  throw "Registry local nao respondeu em 127.0.0.1:$Port."
}

# Retorna @{ api = 'sha256:...'; web = 'sha256:...' } (digest do manifesto no registry local).
function Publish-DeployImagesToLocalRegistry {
  param([hashtable]$Images, [int]$Port)
  Start-DeployLocalRegistry -Port $Port
  $digests = @{}
  foreach ($name in $Images.Keys) {
    $localRef = "127.0.0.1:$Port/omni-${name}:deploy"
    & docker tag $Images[$name] $localRef
    if ($LASTEXITCODE -ne 0) { throw "Falha ao marcar $($Images[$name]) para o registry local." }
    $output = @(& docker push $localRef 2>&1 | ForEach-Object { $_.ToString() })
    if ($LASTEXITCODE -ne 0) { throw "Falha no push local de ${name}: $($output -join ' ')" }
    $digest = ($output | Select-String -Pattern 'digest: (sha256:[0-9a-f]{64})' | Select-Object -Last 1).Matches.Groups[1].Value
    if (-not $digest) { throw "Digest do push local de $name nao encontrado." }
    & docker rmi $localRef | Out-Null
    $digests[$name] = $digest
  }
  return $digests
}

# Remove manifestos antigos (a tag "deploy" e sobrescrita a cada release) para o
# volume nao crescer indefinidamente; camadas da release atual continuam la e o
# proximo push reaproveita. Falha aqui nunca afeta o deploy.
function Stop-DeployLocalRegistry {
  & docker exec $(Get-DeployRegistryContainer) registry garbage-collect --delete-untagged /etc/docker/registry/config.yml 2>$null | Out-Null
  & docker stop $(Get-DeployRegistryContainer) 2>$null | Out-Null
  $global:LASTEXITCODE = 0
}

# Script remoto: pull por digest pelo tunel e tag com o nome usado pelo Compose.
function Get-DeployTunnelPullScript {
  param([int]$Port, [string]$Tag, [string]$ApiDigest, [string]$WebDigest, [string]$Platform = 'linux/amd64')
  if ($Platform -notmatch '^linux/(amd64|arm64)$') { throw 'Plataforma invalida para o tunel.' }
  foreach ($digest in @($ApiDigest, $WebDigest)) {
    if ($digest -notmatch '^sha256:[0-9a-f]{64}$') { throw 'Digest de imagem invalido para o tunel.' }
  }
  if ($Tag -notmatch '^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$') { throw 'Tag invalida para o tunel.' }
  return @"
set -euo pipefail
registry=127.0.0.1:$Port
curl -fsS --max-time 5 "http://`$registry/v2/" >/dev/null
for item in omni-api@$ApiDigest omni-web@$WebDigest; do
  name=`${item%@*}
  source="`$registry/`$item"
  docker pull -q --platform $Platform "`$source" >/dev/null
  docker tag "`$source" "ghcr.io/mikewade2k16/`$name:$Tag"
  echo "`$name: `${item#*@}"
done
"@
}
