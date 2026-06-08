# Codex — P0 para deploy (config + frontend)

> Você é o PROGRAMADOR. O engenheiro (Claude) cuida do backend (P0·2 sessões + P0·5 módulos no `app.go`/auth). **NÃO toque em `back/`** nem nos docs compartilhados (`roadmap-data.ts`, `*.html`, `SECURITY_OPTIMIZATION_BACKLOG.md`, `ARQUITETURA_*`). Suas 3 tarefas são isoladas e não conflitam com a parte do Claude.
> Contexto/critérios: `docs/ARQUITETURA_PANORAMA_2026-06-07.html` (aba Plano, itens P0·3/P0·4/P0·6).
> Regras do projeto: zero legado/mock; máx 450 linhas/arquivo; sem emoji em código; imports `~/`; atualizar AGENT.md do que tocar. Trabalho local — NÃO commitar/push.

---

## Tarefa A — P0·3: Alinhar defaults de produção (DESBLOQUEIA O DEPLOY)

**Problema:** prod sobe com `CORE_V2_ENABLED=false` e `AUTH_ROLES_SOURCE=core_with_fallback`, mas a migration `0135` removeu as tabelas de role legadas. Com fallback legado e Core V2 desligado, o guard de módulo fica inativo e o caminho de roles quebra.

**Arquivos e mudanças:**
1. `docker-compose.prod.yml`:
   - `CORE_V2_ENABLED: ${CORE_V2_ENABLED:-false}` → `${CORE_V2_ENABLED:-true}`
   - `AUTH_ROLES_SOURCE: ${AUTH_ROLES_SOURCE:-core_with_fallback}` → `${AUTH_ROLES_SOURCE:-core}`
2. `.env.production.example`: garantir `CORE_V2_ENABLED=true` e `AUTH_ROLES_SOURCE=core` (adicionar se faltar, com comentário curto explicando que `core` é o único caminho válido pós-0135).
3. Varrer `docker-compose.prod.yml` por nomeações/aliases antigos (ex.: nomes de serviço/imagem/volume divergentes do `docker-compose.yml`). **Não altere** — apenas liste no seu resumo final o que encontrou para o usuário decidir.

**Aceite:**
- `grep -E "CORE_V2_ENABLED|AUTH_ROLES_SOURCE" docker-compose.prod.yml` mostra `:-true` e `:-core`.
- `.env.production.example` reflete o mesmo.
- Resumo lista divergências de naming encontradas (sem alterá-las).

---

## Tarefa B — P0·4: Alinhar `X-Account-Id` para `activeAccountId`

**Problema:** o provider global injeta `X-Account-Id` a partir de `useAuthStore().activeTenantId` (legado), enquanto o Core V2 valida por account e tem `useCoreAccountStore().activeAccountId`. Hoje funciona por coincidência (`account.id == tenant.id`). Para o switcher de account funcionar de verdade, a fonte tem que ser `activeAccountId`.

**Arquivos e mudanças:**
1. `web/app/plugins/account-id-bridge.client.ts`: trocar o provider para ler `useCoreAccountStore().activeAccountId`, **com fallback** para `useAuthStore().activeTenantId` quando o account ainda não hidratou (evita header vazio no boot). Comentar o fallback como temporário (`// TODO remover quando activeAccountId for sempre populado no boot`).
   - Cuidado de ordem: o plugin é `.client.ts` pós-pinia; garanta que os dois stores existem no momento da leitura (o provider é uma função chamada por request, então leitura tardia — ok).
2. Confirmar em `web/layers/core/stores/account.ts` que `activeAccountId` existe e é populado por `/v2/me/accounts` (cookie `ldv_active_account_id`). Se o switcher (`CoreAccountSwitcher`) seta `activeAccountId`, nada mais a fazer lá.
3. Atualizar o `AGENT.md` relevante (core layer / plugins) registrando que `X-Account-Id` agora vem de `activeAccountId` com fallback temporário.

**NÃO faça agora:** renomear "tenant"→"account" em toda a UI (isso é P2·27, fora de escopo). Só a troca da FONTE do header.

**Aceite:**
- Trocar de account no `CoreAccountSwitcher` muda o `X-Account-Id` dos próximos requests imediatamente.
- Com `activeAccountId` vazio (boot), o header cai no `activeTenantId` (comportamento atual preservado — nada quebra).

---

## Tarefa C — P0·6: Corrigir dedupe de GET no `api-client.ts`

**Problema:** `web/app/utils/api-client.ts` deduplica GETs em voo por uma chave que **não inclui a query** (`baseURL + path + headers`). Dois GET no mesmo path com `options.query`/`params` diferentes podem compartilhar a mesma promise e receber a resposta errada.

**Arquivos e mudanças:**
1. `web/app/utils/api-client.ts`: localizar onde a chave de dedupe é montada. Incluir na chave: método, path **normalizado com a query serializada** (e `params`/`query` de `options` se forem usados), além do que já tem. Ordene as chaves da query para a serialização ser estável (`a=1&b=2` == `b=2&a=1`).
2. Adicionar teste unitário (no padrão de teste do front já existente; se não houver setup de teste pra esse util, criar um `.test.ts` mínimo ao lado) cobrindo: dois GET no mesmo path com query diferente → NÃO compartilham promise/resposta; mesmo path+query igual em voo → compartilham (dedupe ainda funciona).

**Aceite:**
- `/v1/tasks?board=a` e `/v1/tasks?board=b` nunca recebem a mesma resposta.
- Dedupe continua funcionando para requests idênticos simultâneos.
- Teste passa.

---

## Resumo que você deve devolver
- O que mudou em cada arquivo (Tarefa A/B/C).
- Divergências de naming achadas no `docker-compose.prod.yml` (Tarefa A, item 3) — só listar.
- Confirmação dos 3 critérios de aceite.
- NÃO rodar build/deploy; só deixar pronto pro Claude revisar e o usuário rodar.
