---
name: revisao-dia
description: Balanço diário de manutenção, segurança e performance do Omni. Use quando o usuário pedir "analisa o que foi feito hoje", "revisão do dia", "balanço do dia", "o que faltou pra finalizar", "o que falta subir pro git / pode ir pro deploy", ou perguntar como está a performance/segurança/design do que foi feito. É uma revisão ADVISORY (só lê, nunca altera código nem roda git/deploy): faz o balanço do dia, varre o diff em busca de gargalos de banco, over-fetch, queries excessivas, vazamento multi-tenant e drift de design, cruza com o plano canônico pra dizer o que faltou, e entrega um plano de ação priorizado. Complementa (não substitui) /code-review, /security-review e o qa-bot/perf_audit.py.
---

# Revisão do dia — manutenção, segurança e performance

Skill de **triagem e síntese**: faz o balanço do que foi feito hoje, varre o diff
em busca de risco de escala (banco, over-fetch, multi-tenant, design) e entrega um
**plano de ação priorizado**. O objetivo de fundo é poder **vender acesso com
segurança de que o servidor aguenta**: sem gargalo de banco, sem query a mais, sem
campo que não se usa, sem drift que trave o crescimento do painel.

Não é um substituto de revisão profunda — é a camada que decide **onde** vale a pena
chamar as ferramentas pesadas (`/code-review`, `/security-review`, `qa-bot/perf_audit.py`).

## Regras de ouro desta skill (inegociáveis)

1. **Advisory. Nunca altera código.** Esta skill só LÊ e relata. Achou algo pra
   reverter/otimizar? Vira item no plano de ação, não um edit. O usuário aprova.
2. **Nunca roda git de escrita nem deploy.** `commit`, `push`, `deploy:*` são
   **devolvidos como comando pro usuário rodar**, nunca executados. Só comandos de
   leitura (`git log`, `git status`, `git diff`) podem rodar.
3. **Foco no diff de hoje por padrão.** Rápido e relevante. Varredura ampla do repo
   só quando o usuário pedir "varredura completa".
4. **Responder em português.** Sempre.
5. **Leia `principios-engenharia` ANTES** de classificar qualquer achado — os tickets
   `SEC-*`/`OPT-*` e as regras de fonte-única/isolamento vêm de lá.

## Passo 0 — carregar contexto

- Invocar/ler a skill **principios-engenharia** (regras canônicas e os checklists de
  `references/seguranca-multitenant.md`, `references/database.md`, `references/frontend.md`).
- Fontes de "o que faltou": [docs/MULTITENANT_COMPLETION_PLAN.md](../../../docs/MULTITENANT_COMPLETION_PLAN.md)
  (C1–C13) e `web/app/components/roadmap/roadmap-data.ts`.
- Backlog de risco já catalogado: [docs/SECURITY_OPTIMIZATION_BACKLOG.md](../../../docs/SECURITY_OPTIMIZATION_BACKLOG.md)
  (tickets SEC-1..5 / OPT-1..6 — referenciar pelo id ao classificar um achado).
- Fluxo de deploy (pra montar os comandos, NÃO rodar): [docs/DEPLOY_VPS.md](../../../docs/DEPLOY_VPS.md).

## Passo 1 — o que foi feito hoje

Rodar (somente leitura) e agrupar por módulo (`back/internal/modules/*`, `web/layers/*`, `web/app/*`, `docs/*`):

```bash
git log --since=midnight --oneline --stat          # commits de hoje
git status --short                                  # working tree (não commitado)
git diff --stat                                     # tamanho das mudanças não commitadas
git diff --stat --staged                            # idem staged
```

Resumir: "hoje mexeu em X módulos — back: …, front: …, docs: …". Apontar arquivos que
**passaram de ~450 linhas** (limite do projeto) entre os tocados.

## Passo 2 — git & deploy (status, com comandos — sem executar)

```bash
git diff main...HEAD --stat                          # o que esta branch tem a mais que a main
git log @{push}..HEAD --oneline 2>/dev/null || git log @{u}..HEAD --oneline   # commitado e ainda NÃO pushado
```

Classificar e relatar:
- **Não commitado** (working tree sujo) — listar; sugerir mensagem de commit, **não commitar**.
- **Commitado, não pushado** — listar; lembrar que o `deploy:prod` (CI) exige push antes.
- **Pode ir pro deploy?** Só dizer "sim/depende" + o comando exato, **devolvido pro usuário**:
  - `npm run deploy:fast:prod` — builda local e manda (não precisa git). É o do dia-a-dia.
  - `npm run deploy:prod` — só após `git push` + CI verde (`gh run list --workflow=build-images.yml --limit 1`).
- **Migration/env nova no diff?** Avisar: migration roda no startup da imagem (precisa estar
  NA imagem — `deploy:fast` pega do local, `deploy:prod` exige commit/push); **env var nova NÃO
  viaja na imagem** — tem que entrar no `.env.production` na VPS, manual.

> Esta skill imprime os comandos. Quem roda é o usuário (regra: trabalhar só local).

## Passo 3 — o que faltou pra finalizar

Cruzar o que foi tocado hoje com o plano canônico e o roadmap:
- Da fase/item que o trabalho de hoje encosta (no `MULTITENANT_COMPLETION_PLAN.md` e em
  `roadmap-data.ts`), listar o que ainda está `pending`/`done:false`/sem `verifiable` cumprido.
- Apontar **inconsistência de doc**: se o código andou mas o AGENT.md do módulo, o doc canônico
  ou o `roadmap-data.ts` não refletem (regra dos 3 docs em sync).
- Se algo ficou **legado/mock/só-local** no diff de hoje: marcar como "NÃO está pronto" e cobrar
  registro em [docs/LEGADO.md](../../../docs/LEGADO.md).

## Passo 4 — performance & banco (o coração, pensando em escala)

Varrer os arquivos do diff (e, em modo completo, os hot paths) por estes cheiros. Cada achado
recebe severidade (P0/P1/P2) e, quando houver, o ticket `OPT-*` correspondente.

Backend (Go, `back/internal/modules/*/store_*.go`, `service*.go`):
- **N+1** — query dentro de `for`/`range` em vez de `WHERE id = ANY($1)`. (OPT-2) **P0/P1.**
- **`SELECT *` / projeção gorda** — listagem trazendo o objeto inteiro quando a tela usa poucos
  campos; devolver só o que a UI consome (modelo `AccountSummary`). (OPT-4) **P1.**
- **Lista sem paginação/cursor** que cresce com o tenant (users, leads, operações, pedidos).
  Offset+COUNT fica caro; cursor `?after=`. (OPT-5) **P1.**
- **Índice faltando** — coluna nova usada em `WHERE`/`JOIN`/`ORDER BY` em hot path sem índice
  na migration. **P1.**
- **Fetch não gateado por permissão** disparando no boot e tomando 403 (round-trip morto). (OPT-1) **P2.**
- **Detalhe carregado junto da lista** (memberships, stores) em vez de no clique. (OPT-6) **P2.**

Frontend (`web/`):
- Fetch **bloqueante no `setup`/top-level `await`** que segura a troca de rota (sem skeleton) —
  vide o caso `/usuarios` no PERFORMANCE_AUDIT_PLAN §17. **P1.**
- Lista grande renderizada inteira sem windowing/lazy-mount (caso `/tasks`, 247 cards). **P2.**
- Sem `AbortController` ao trocar de rota; sem skeleton < 100ms no clique. **P2.**

Regra ao reportar número de performance: **só vale contra build de produção.** O dev compila a
rota sob demanda (Vite) e mente o tempo. Se o achado exige medição, recomendar rodar o harness —
não chutar número:

```bash
# medir de verdade (não no dev): build prod local + perf_audit
docker build -t omni-web-prod ./web
docker run --rm -d --name omni-web-prod -p 3055:3003 \
  -e NUXT_PUBLIC_API_BASE=http://localhost:9091 \
  -e NUXT_API_INTERNAL_BASE=http://host.docker.internal:9091 omni-web-prod
OMNI_QA_EMAIL=.. OMNI_QA_PASSWORD=.. python qa-bot/perf_audit.py --base-url http://localhost:3055
```

## Passo 5 — segurança multi-tenant

Varrer o diff por (cada um cita o ticket `SEC-*` quando aplicável):
- **Escopo vindo do body** — `account_id`/`tenant_id`/`store_id` lido de input em vez do
  Principal resolvido pelo middleware. **P0.**
- **Handler novo sem validação de escopo no service** e/ou **query sem filtro de tenant**
  (defesa em profundidade quebrada — sem RLS embaixo, vaza). (SEC-1) **P0.**
- **403 onde devia ser 404** pra recurso de outro tenant (enumeration). (SEC-2) **P1.**
- **SQL concatenado** (não parametrizado). **P0.**
- **Cache key / path de upload sem prefixo de tenant.** (SEC-5) **P1.**
- **CORS `*` / header de segurança faltando** em mudança de chain/proxy. (SEC-3/SEC-4) **P1.**

Achado de segurança real e não-trivial → recomendar `/security-review` no diff pra confirmar
a fundo (esta skill faz a triagem, não o veredito final).

## Passo 6 — design system & manutenibilidade

- **Cor/spacing hardcoded** em vez de token/classe semântica (drift que trava o crescimento). **P2.**
- **Componente/lógica duplicada** (apareceu 2×, não virou componente/helper). **P2.**
- **Modal e card do board fora de sync** (regra: mudou um, replica no outro). **P1.**
- **Arquivo > 450–500 linhas** sem divisão por responsabilidade. **P2.**
- **Página nova sem gating de menu** `isPlatformAdmin || has(...)` (some pro admin). **P1.**
- **`console.log`/código morto/`any`** no diff. **P2.**

## Passo 7 — candidatos a reverter/refatorar (advisory)

Listar (sem executar nada) o que vale **voltar atrás ou refatorar** pra não carregar dívida:
mudança de hoje que regrediu performance, que introduziu over-fetch, que duplicou algo já
existente, ou que desligou uma feature (proibido — features coexistem). Para cada um: o porquê
e o custo de deixar. **Só vira mudança depois que o usuário aprovar item a item.**

## Saída — formato do relatório

Imprimir em PT, sem emojis, nesta ordem. Ser direto: achado → arquivo:linha → severidade → ticket.

```
REVISÃO DO DIA — <data>  (branch <nome>)

1. O QUE FOI FEITO HOJE
   - back: <módulos/arquivos>   front: <…>   docs: <…>

2. GIT & DEPLOY
   - não commitado: <…>
   - commitado, não pushado: <…>
   - pode subir? <sim/depende> → comando: <devolvido, não executo>

3. O QUE FALTOU PRA FINALIZAR
   - <item do plano/roadmap ainda pendente>   - <doc fora de sync>

4. PERFORMANCE / BANCO     5. SEGURANÇA     6. DESIGN SYSTEM
   [P0] <achado> — <arquivo:linha> (OPT-2)
   [P1] ...

7. CANDIDATOS A REVERTER/REFATORAR (advisory)
   - <o quê> — <porquê> — <custo de deixar>

PLANO DE AÇÃO PRIORIZADO
   P0 (agora): ...
   P1 (esta semana): ...
   P2 (quando der): ...
   Comandos pra VOCÊ rodar (não executei): <git/deploy/perf>
```

Ao fim, **oferecer** (não fazer sozinho): salvar o relatório em
`docs/historico/revisao-<data>.md`, ou abrir `/code-review` / `/security-review` /
`perf_audit.py` nos pontos quentes. Espera o ok.

## Virar loop ou cron depois (hoje é manual)

Disparo atual = manual ("analisa o que foi feito hoje" ou `/revisao-dia`). Pra automatizar:
- **Loop por intervalo** (enquanto o Claude Code estiver aberto): `/loop 30m /revisao-dia`. Para com `/loop stop`.
- **Cron diário** (roda sozinho num horário, deixa o relatório): usar a skill `schedule` —
  ex.: rodar `/revisao-dia` todo dia 19h. (Continua advisory: só relata; nunca commita/deploya.)
