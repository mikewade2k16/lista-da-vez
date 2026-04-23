# Referencia arquivada

Este arquivo veio de outro projeto e nao se aplica a este repositorio.

Para o deploy real deste app, use somente:

- `docs/DEPLOY_VPS.md`

O que era especifico do outro projeto e foi descartado para este repo:

- `plataforma-api`
- `painel-web`
- `redis`
- `worker`
- `evolution`
- `caddy` dentro do compose do proprio projeto
- deploy automatico da `main`

Smoke adicional quando o deploy incluir o módulo `indicators`:

1. fazer login administrativo no painel
2. abrir `https://app.${DOMAIN}/admin/indicadores`
3. validar carregamento do perfil ativo e do dashboard sem erro de bootstrap
4. validar a rota de exportação administrativa do módulo
5. se o operador usar contexto root, validar a troca para o cliente alvo e leitura com `clientId` correspondente

## Comandos úteis

```bash
# status
docker ps --format 'table {{.Names}}\t{{.Status}}'

# logs do shell
docker compose -f docker-compose.yml -f docker-compose.prod.yml --profile channels --env-file .env.prod logs -f plataforma-api --tail=100

# logs do painel
docker compose -f docker-compose.yml -f docker-compose.prod.yml --profile channels --env-file .env.prod logs -f painel-web --tail=100

# logs da API operacional
docker compose -f docker-compose.yml -f docker-compose.prod.yml --profile channels --env-file .env.prod logs -f atendimento-online-api --tail=100
```

## O que não fazer

- não subir `fila-atendimento` com compose paralelo no servidor
- não abrir processo manual em `cmd`, `powershell` ou `screen` para compensar falha de container
- não criar segundo PostgreSQL só para o módulo sem necessidade operacional real
- não expor subdomínio separado do módulo se o host oficial está dentro do `painel-web`
- não deixar `Adminer` exposto permanentemente no domínio público
- não depender de `npm run dev`, `tsx watch` ou bootstrap automático de schema para manter produção viva
- não voltar a bind mount de `apps/painel-web` e `apps/atendimento-online-api` no runtime de produção; isso reintroduz build lento no startup
- não corrigir dependência faltante de produção com `npm install` dentro do container; em produção o caminho correto é rebuildar a imagem correspondente

## Observação de arquitetura

O `fila-atendimento` continua sendo um módulo isolado por contrato, mas não precisa de infraestrutura duplicada para isso. O isolamento principal dele agora está em:

- fronteira HTTP/BFF
- schema próprio no banco
- sessão própria após o shell bridge
- manifesto e `AGENTS.md` de módulo
