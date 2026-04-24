# Start Local

Guia rapido para subir banco e API Go no Windows.

Este fluxo agora e fallback. O fluxo oficial do projeto esta no Docker Compose pela raiz com `npm run dev`.

Fluxo preferido do time:

- abrir o projeto no VS Code
- usar o terminal integrado com `Git Bash`
- pela raiz do repositorio rodar `npm run dev`

## 1. Subir o PostgreSQL local

No diretorio `back/`:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\postgres\init-local.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\postgres\status-local.ps1
```

Isso garante:

- PostgreSQL rodando
- banco `lista_da_vez`
- usuario `lista_da_vez`
- migrations aplicadas

## 2. Subir a API Go

No diretorio `back/`:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\api\start-local.ps1
```

API esperada:

- `http://localhost:8080/healthz`
- `http://localhost:8080/v1/operations/snapshot?storeId=...`
- `http://localhost:8080/v1/auth/invitations/{token}`

Para inspecionar ou parar:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\api\status-local.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\api\stop-local.ps1
```

## 3. Subir o frontend

No diretorio `web/`:

```powershell
$env:NUXT_PUBLIC_API_BASE='http://localhost:8080'
npm run dev -- --port 3003
```

Fluxo equivalente preferido em Git Bash pela raiz:

```bash
npm run dev
```

Se `3003` estiver ocupada:

```powershell
$env:NUXT_PUBLIC_API_BASE='http://localhost:8080'
npm run dev -- --port 3001
```

## 4. Login demo

- `proprietario@demo.local`
- `consultor@demo.local`
- senha: `dev123456`

## 5. Convites locais

Para o onboarding por convite funcionar certo fora do Docker, garanta:

- `WEB_APP_URL=http://localhost:3003`
- `AUTH_INVITE_TTL=168h`

## 6. Validacao rapida

- frontend: `http://localhost:3003/auth/login` ou `http://localhost:3001/auth/login`
- backend: `http://localhost:8080/healthz`

## Observacao importante

O backend aceita `localhost`, `127.0.0.1` e `[::1]` em qualquer porta local para evitar erro de CORS quando o Nuxt subir em `3003` ou `3001`.

## Troubleshooting rapido

Se `http://localhost:8080/healthz` responder `200`, mas uma rota nova como `GET /v1/operations/snapshot` voltar `404`, voce quase certamente esta com uma build antiga da API rodando em segundo plano.

Resolucao:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\api\stop-local.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\api\start-local.ps1
```
