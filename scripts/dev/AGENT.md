# AGENT

## Escopo

Estas instrucoes valem para `scripts/dev`.

## Objetivo

Os scripts desta pasta sao a camada de entrada para desenvolvimento local usando o terminal integrado do VS Code com Git Bash.

## Regras

- assumir Git Bash como shell padrao do workspace
- chamar scripts PowerShell existentes do backend apenas como infraestrutura interna
- expor comandos simples para:
  - banco local
  - API local
  - status/parada da API
  - frontend Nuxt
  - stack completa

## Fluxo padrao

No terminal Git Bash do VS Code, a entrada principal deve ser:

```bash
npm run dev        # docker compose up --build --watch (stack + sync do web)
```

Esse comando deve:

1. garantir o PostgreSQL local
2. subir a API Go em `:8080`
3. subir o Nuxt em `:3003` com `compose watch` sincronizando ./web -> container

## Dev do web no Docker (watch-web.ps1)

`watch-web.ps1` (= `npm run dev:watch`) e o caminho quando a stack ja esta de
pe: roda `docker compose up -d --build web` (reconcilia edicoes feitas com o
watch desligado — o watch NAO faz sync inicial) e liga `docker compose watch
--no-up web`. A janela precisa ficar aberta; fechar o watch congela o codigo
do container no ultimo build/sync. O bind mount ./web:/app nao existe mais
(ponte 9P do WSL2 inviabilizava o dev); `start-web-native.ps1` vira fallback.

## Observacao

Se algum script precisar chamar PowerShell, usar `powershell.exe -ExecutionPolicy Bypass -File ...` com caminho convertido para Windows via `cygpath -w`.

O gerador `gen-component-inventory.mjs` tambem vive aqui e deve produzir `docs/COMPONENT_INVENTORY_AUTO.md` a partir do estado atual dos componentes Vue do projeto, sem sobrescrever o inventario humano em `docs/COMPONENT_INVENTORY.md`.
