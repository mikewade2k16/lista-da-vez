# Perfis de Docker Compose para desenvolvimento

Este projeto deve subir por capacidade, não sempre com todos os serviços ligados.

## Perfis disponíveis

- Sem perfil: stack mínima para o shell e módulo `fila-atendimento`
  - `postgres`
  - `redis`
  - `plataforma-api`
  - `painel-web`
- `atendimento`: API operacional do `atendimento-online`
- `workers`: API operacional e workers do `atendimento-online`
- `channels`: Evolution API
- `ops`: ferramentas locais como Adminer, Redis Commander e Mailpit

## Comandos úteis

Subir apenas a fila hospedada no shell:

```powershell
docker compose -f docker-compose.dev.yml up -d postgres redis plataforma-api painel-web
```

Subir atendimento-online sem workers:

```powershell
docker compose -f docker-compose.dev.yml --profile atendimento up -d
```

Subir atendimento-online com workers:

```powershell
docker compose -f docker-compose.dev.yml --profile workers up -d
```

Subir Evolution API:

```powershell
docker compose -f docker-compose.dev.yml --profile channels up -d whatsapp-evolution-gateway
```

Subir ferramentas operacionais:

```powershell
docker compose -f docker-compose.dev.yml --profile ops up -d
```

## Watcher do painel

Por padrão, o `painel-web` usa polling no watcher do Nuxt dentro do container dev. Esse é o modo oficial para Docker Desktop no Windows, porque evita reinício manual do container a cada alteração.

Se você precisar reduzir uso de CPU numa máquina Linux ou WSL onde o watcher nativo já estiver estável, pode desligar o polling manualmente nessa sessão:

```powershell
$env:NUXT_WATCH_POLLING = "false"
docker compose -f docker-compose.dev.yml up -d painel-web
```

