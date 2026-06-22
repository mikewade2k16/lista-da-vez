# Runbook — Mostarda em produção (omni.crowvisuals.com.br)

> Levar os dados/imagens do Mostarda Bar Bistrô pra VPS e ligar o front TAVOLA
> (já no ar em https://mostarda.crowvisuals.com.br/). Criado 2026-06-19, atualizado
> depois do recon ao vivo na VPS.
> Regra: **o dono roda as escritas em prod** — o agente só prepara e verifica
> (leituras em prod são liberadas; escritas, não).

## Estado descoberto na VPS (recon 2026-06-19)
- Conta **Crow Visuals em PROD**: `0d88ff7b-b274-4a0f-83c0-972983e9a081` (≠ do id local `80caf5d5…`).
- Módulo `cardapio` **já habilitado** para a Crow (não precisa do passo de seed/enable).
- `cardapio.restaurants` estava **vazio** (schema criado pelas migrations; sem duplicata).
- Existe uma conta "Mostarda" separada (`eb205e8b…`) **sem** o módulo cardápio — o restaurante
  vai sob a **Crow** (igual ao local). Mover depois pelo "Cliente" inline, se quiser.
- Host: `omni.crowvisuals.com.br` e `lista.whenthelightsdie.com` apontam pro **mesmo** api
  (Caddy). Ambos servem `/v1/public/*` e `/uploads/*`. Front TAVOLA buildado p/ `omni.…`.

## ⚠️ Furo do compose (corrigido) — leia antes
As imagens do público são absolutizadas por `PUBLIC_API_BASE_URL` (`/uploads/…` → base+path).
**O `docker-compose.prod.yml` não passava essa env pro container** (bloco `environment:`
explícito, sem `env_file`). Sem ela o público devolve caminho **relativo** e o front em
outro domínio (TAVOLA) não acha a imagem.
- **Correção (já feita local):** adicionada a linha `PUBLIC_API_BASE_URL: ${PUBLIC_API_BASE_URL:-}`
  no `environment:` do `api` em `docker-compose.prod.yml`.
- O script abaixo seta `PUBLIC_API_BASE_URL=https://omni.crowvisuals.com.br` no `.env.production`,
  re-envia o compose e **recria** o `api` (sem rebuild de imagem — a env é lida em runtime).

## Abordagem: dump do banco LOCAL → restore no prod
O banco local é a fonte única (1 restaurante `mk`, 203 produtos todos com imagem, 17 categorias,
17 zonas; 0 variação/adicional/review/order). Em vez de replicar seeds, fazemos
`pg_dump --data-only` das 4 tabelas com conteúdo (`restaurants, categories, products,
delivery_zones`), trocamos o id da conta por `sed` (isso conserta `account_id` **e** os
caminhos em `image_url` de uma vez), e damos restore atômico. As imagens vão por `tar` por SSH
pro volume `api_uploads`.

## Como rodar — um comando (DONO, no Git Bash)
```bash
bash scripts/deploy/upload-mostarda-prod.sh
```
O script (`scripts/deploy/upload-mostarda-prod.sh`, idempotente/re-rodável) faz:
1. Gera o dump re-targetado do **banco local** (`80caf5d5…` → `0d88ff7b…`) e valida que o id
   antigo sumiu.
2. **Carrega no prod** (limpa o restaurante + COPY, tudo em `--single-transaction`,
   `ON_ERROR_STOP=1`) e imprime as contagens.
3. Copia as **203 imagens** (`C:/tmp/mostarda_jpg/*.jpg`) pro volume em
   `/app/data/uploads/cardapio/0d88ff7b…/mostarda/`.
4. Garante `PUBLIC_API_BASE_URL=https://omni.crowvisuals.com.br` no `.env.production`,
   re-envia o `docker-compose.prod.yml` e recria o `api` (`up -d --no-build api`).
5. Registra o domínio `mostarda.crowvisuals.com.br` (primário) — upsert idempotente.
6. Smoke: espera o `/healthz`, busca `/v1/public/restaurants/mk` e confere que a 1ª imagem
   volta **absoluta** e responde 200.

Pré-requisitos: Docker Desktop com o stack local up, chave `~/.ssh/gh_actions_omnichannel_vps`,
e `C:/tmp/mostarda_jpg/` com os 203 jpg.

## Testar
- Público: `https://omni.crowvisuals.com.br/v1/public/restaurants/mk` → cardápio + imagens
  absolutas em `https://omni.crowvisuals.com.br/uploads/…`.
- Front por host: `https://mostarda.crowvisuals.com.br/`.
- Front sem depender do domínio: `https://mostarda.crowvisuals.com.br/?slug=mk`.

## Notas de Deploy (afetam prod)
- **`docker-compose.prod.yml`**: nova env `PUBLIC_API_BASE_URL` no serviço `api`. Recriar o
  `api` depois de re-enviar o compose (`up -d --no-build api`). Não exige rebuild de imagem.
- **`.env.production` (VPS)**: adicionar `PUBLIC_API_BASE_URL=https://omni.crowvisuals.com.br`.
  É **global** (vale p/ cardápio e bio): faz o público devolver URLs absolutas — comportamento
  correto/esperado; fronts que usam a URL como veio (TAVOLA/bio) seguem funcionando.
- Migrations do cardápio (já aplicadas no deploy do código): nada a rodar à parte.
- CORS de `/v1/public/*` é `*` (no código) — front em outro domínio chama sem problema; `<img>`
  cross-origin não precisa de CORS.
