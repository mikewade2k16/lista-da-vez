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
npm run dev
```

Esse comando deve:

1. garantir o PostgreSQL local
2. subir a API Go em `:8080`
3. subir o Nuxt em `:3003`

## Observacao

Se algum script precisar chamar PowerShell, usar `powershell.exe -ExecutionPolicy Bypass -File ...` com caminho convertido para Windows via `cygpath -w`.
