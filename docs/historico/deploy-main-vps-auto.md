# Referencia curta

Este arquivo continua nao sendo a fonte de verdade do deploy.
O documento oficial segue sendo:

- `docs/DEPLOY_VPS.md`

O que existe hoje de fluxo reutilizavel neste repositorio:

- script local: `scripts/deploy/deploy-vps-fast.ps1`
- comando npm: `npm run prod:deploy:vps`
- workflow manual: `.github/workflows/deploy-vps.yml`

Quando usar cada um:

1. usar `npm run prod:deploy:vps` para o deploy rapido manual a partir da sua maquina
2. usar o workflow `Deploy VPS` quando quiser disparar por GitHub com `git_ref` e inputs controlados

O workflow atual e manual por `workflow_dispatch` e espera que a VPS ja esteja preparada com:

- diretorio `/home/deploy/lista-atendimento`
- `.env.production` remoto
- DNS de `lista.whenthelightsdie.com`
- bloco do Caddy ja aplicado

Inputs do workflow:

1. `git_ref`
2. `services`
3. `backup_database`
4. `force_recreate`
5. `skip_smoke_tests`

Secret necessario no GitHub:

1. `DEPLOY_VPS_SSH_KEY`

Os dois caminhos usam o mesmo fluxo tecnico:

1. sincronizacao por `tar` + SSH
2. limpeza remota preservando `.env.production` e `backups`
3. `docker compose config`
4. `docker compose up -d --build`
5. smoke tests publicos