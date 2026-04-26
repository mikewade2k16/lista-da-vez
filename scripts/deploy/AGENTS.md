# AGENTS

## Escopo

Estas instrucoes valem para `scripts/deploy`.

## Objetivo

Os scripts desta pasta sao a camada de entrada para deploy de producao deste repositorio.
Eles existem para reduzir erro manual e padronizar o fluxo real de deploy para a VPS auditada.

## Ambiente alvo atual

- host: `85.31.62.33`
- usuario SSH: `deploy`
- caminho remoto: `/home/deploy/lista-atendimento`
- dominio publico: `https://lista.whenthelightsdie.com`
- proxy central: Caddy da stack `omnichannel-mvp`

## Regras do fluxo

- o deploy rapido local deve continuar sem depender de `git clone` na VPS
- a sincronizacao de codigo deve continuar por `tar` + SSH
- a limpeza remota deve preservar `.env.production` e `backups`
- o script nao deve apagar volumes Docker de producao
- o script nao deve sobrescrever secrets remotos
- `docker compose config` deve continuar como validacao antes do `up -d --build`
- smoke tests publicos devem continuar sendo a validacao padrao depois do deploy

## Regras de implementacao

- preferir PowerShell como orquestrador local no Windows
- quando precisar empacotar o workspace, usar o wrapper de Git Bash ja existente em `../dev/git-bash.cmd`
- defaults de host, usuario, caminho remoto e URL publica podem ficar versionados porque ja foram auditados para este projeto
- qualquer novo parametro de deploy deve ser exposto como argumento simples do script principal
- mudancas que afetem banco, importacao de dados ou restore devem oferecer caminho claro para backup antes da execucao
- evitar adicionar comportamento automatico destrutivo sem opt-in explicito

## Script principal atual

- `deploy-vps-fast.ps1`

## Validacao minima

Ao alterar scripts desta pasta:

1. validar a sintaxe do PowerShell quando houver mudanca em `.ps1`
2. validar o workflow relacionado em `.github/workflows/deploy-vps.yml` se a mudanca tocar o fluxo GitHub
3. garantir que os comandos documentados em `docs_depoy/deploy-producao-checklist.md` ainda correspondem ao comportamento real

## Referencias

- `../../docs/DEPLOY_VPS.md`
- `../../docs_depoy/deploy-producao-checklist.md`
- `../../.github/workflows/deploy-vps.yml`
- `../dev/AGENT.md`