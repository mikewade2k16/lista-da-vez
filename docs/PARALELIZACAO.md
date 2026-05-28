# Plano de Paralelização — Levar Omni para Produção

> Documento mestre. Coordena 3 agentes (Claude Code, Codex CLI, Codex no GitHub Copilot) trabalhando em paralelo na branch `refactor/multi-tenant-core` para fechar as Fases 3-9 do [PLANO_REFATORACAO.md](PLANO_REFATORACAO.md) e fazer deploy em produção.
>
> Data de criação: 2026-05-21
> Branch alvo: `refactor/multi-tenant-core` (todos os agentes commitam aqui)
> Estado anterior: 4 de 10 fases concluídas (0, 1, 2, 6) — ver [plano-refatoracao.html](plano-refatoracao.html)

---

## Estratégia em uma frase

3 agentes trabalham simultaneamente em **zonas de arquivo disjuntas** (sem overlap), na mesma branch local, **sem rodar comandos git** (só o usuário commita) e usam este documento como tabuleiro de status. Briefings por agente em [agents/](agents/).

---

## Ondas

### 🌊 Onda 1 — Paralela (3 trilhas simultâneas)

Cada trilha tem seu briefing dedicado. Nenhuma trilha toca arquivos fora da sua zona.

| Trilha | Agente        | Fases                                                      | Briefing                               |
| ------ | ------------- | ---------------------------------------------------------- | -------------------------------------- |
| **A**  | Claude Code   | 3 (back) + 5 + 7-BACK + 8.6 + 9 setup                      | [agents/CLAUDE.md](agents/CLAUDE.md)   |
| **B**  | Codex CLI     | 7-FRONT-admin (Users/Feedback/Settings/Erp/Theme)          | [agents/CODEX.md](agents/CODEX.md)     |
| **C**  | Copilot (IDE) | 7-FRONT-operation (OperationFinishModal) + 8.7 docs inline | [agents/COPILOT.md](agents/COPILOT.md) |

**Critério de saída da Onda 1**: cada trilha reporta ✅ no seu briefing + build/lint verde + smoke manual local. (Nenhum agente roda git — o usuário commita em lote.)

### 🌊 Onda 2 — Sequencial (qualquer agente, um após o outro)

- **Fase 8.1** Lazy load — depende da Onda 1 (`OperationFinishModal` e `UsersAccessManager` já fatiados)
- **Fase 9** Testes do front — 1 teste por store crítica + composables de realtime
- **Fase 8.4** HTTP security headers — alteração no Caddy/docs
- **Fase 8.5** CI audits — `npm audit` + `govulncheck` no workflow
- **Fase 8.8** Component inventory script — `scripts/dev/gen-component-inventory.mjs`

### 🌊 Onda 3 — Big bang sozinho (todos pausam)

- **Fase 4** Renomeação para Omni
  - Touch massivo: `package.json`, compose, `.env`, `config.go`, `password_reset_delivery.go`, `nuxt.config.ts`, README, AGENT.md, **DB prod via `ALTER DATABASE`**.
  - **Apenas 1 agente** roda. Os outros 2 pausam até terminar.
  - Janela de manutenção para o DB prod (procedimento detalhado na [Fase 4.6 do PLANO_REFATORACAO.md](PLANO_REFATORACAO.md)).

### 🌊 Onda 4 — Validação e deploy

- Build final do front e back
- `golangci-lint run` + `npm run lint` + `vue-tsc` + `go test`
- Smoke local completo (login, operação, tasks, erp, multistore)
- Deploy VPS via [DEPLOY_CHECKLIST.md](DEPLOY_CHECKLIST.md)
- `git tag` da release

### Fora deste ciclo

- **Fase 10** (CSS centralizado em SASS) — adiada para depois do deploy em prod. Coerente com a decisão de 2026-05-18 (refator mecânico em ~60 componentes faz mais sentido como sprint dedicado pós-estabilização).

---

## Zonas disjuntas (regra de ouro inegociável)

Cada agente edita APENAS os caminhos da sua zona. Tocou fora, parou e avisou no chat com o usuário.

### Zona Claude — backend, infra, docs, scripts

**Pode editar:**

- `back/**` (todo o backend Go)
- `.github/**` (workflows CI/CD)
- `docs/**` exceto `docs/agents/CODEX.md` e `docs/agents/COPILOT.md`
- `scripts/**`
- `.husky/**`
- `.gitignore`, `.golangci.yml`
- `package.json` raiz (scripts de orquestração)
- README.md, AGENT.md raiz

**NÃO toca:**

- `web/**`
- `web/package.json` (deps do front)

### Zona Codex CLI — front administrativo

**Pode editar:**

- `web/app/components/users/**`
- `web/app/components/feedback/**`
- `web/app/components/settings/**`
- `web/app/components/erp/**`
- `web/layers/core/components/theme/**`
- `web/app/composables/**` (criar composables novos extraídos dos workspaces acima)
- `web/app/domain/utils/**` (criar utils novos extraídos dos workspaces acima)
- Os 3 AGENT.md correspondentes (`web/app/components/{users,feedback,settings,erp}/AGENT.md` — criar se não existirem)

**NÃO toca:**

- `back/**`
- `web/layers/tasks/**`
- `web/app/components/operation/**` (zona do Copilot)
- `web/app/pages/**`
- `web/app/stores/**` (zona da Fase 9 — depois)
- `docs/**`
- `package.json` (qualquer)
- `web/nuxt.config.ts`, `web/tsconfig.json`, `web/eslint.config.mjs`

### Zona Copilot — front operação + docs inline

**Pode editar:**

- `web/app/components/operation/**` (especialmente `OperationFinishModal.vue` que tem 2.143 linhas)
- `docs/adr/**` (criar a pasta com ADR-0001 sobre rename Omni)
- JSDoc/TSDoc em arquivos específicos da Fase 8.7 (lista no briefing)
- `web/app/components/operation/AGENT.md` (atualizar com nova estrutura pós-fatiamento)

**NÃO toca:**

- `back/**`
- `web/layers/**`
- Outras pastas de `web/app/components/`
- `web/app/composables/**`, `web/app/stores/**`, `web/app/pages/**`
- `docs/**` exceto `docs/adr/**` que vai criar
- `package.json` (qualquer)
- Configs

---

## Arquivos compartilhados (ninguém edita sem coordenar)

| Arquivo                     | Regra                                                                               |
| --------------------------- | ----------------------------------------------------------------------------------- |
| `package.json` raiz         | Só Claude (na Onda 1). Se outro agente precisar, abre issue no chat com o usuário.  |
| `web/package.json`          | Ninguém edita até Onda 3 (rename Omni).                                             |
| `docker-compose.yml`        | Só Claude (Fase 5 — rename ftp folder).                                             |
| `docs/PARALELIZACAO.md`     | Todos podem atualizar **apenas a seção "Status"** abaixo. Outras seções: só Claude. |
| `docs/PLANO_REFATORACAO.md` | Só Claude.                                                                          |
| `docs/agents/*.md`          | Cada agente edita só o próprio briefing para registrar avanço.                      |

---

## Regras comuns (vale para os 3 agentes)

1. **Nenhum comando git por agente**. Sem `add`, `commit`, `pull`, `rebase`, `push`, `merge`, `stash`, `checkout` em arquivo. O usuário (Mike) commita em lote quando achar pertinente. `git status` pra ler estado é tolerável; nada que escreva.
2. **Zona disjunta** (ver acima): tocou fora, parou e avisou no chat. Não há `git` pra desfazer, então o cuidado é antes — não no rollback.
3. **AGENT.md por módulo**: ao terminar trabalho num módulo, atualize o `AGENT.md` correspondente com o que mudou.
4. **Documentar antes de implementar**: para tarefas grandes (>1h estimadas), descreva no próprio briefing o que vai fazer ANTES de codar.
5. **Validar antes de marcar tarefa como done**: rodar `npm --prefix web run build` (front) ou `go test ./...` (back) localmente + smoke manual quando UI for tocada.
6. **Pre-commit hook**: como você não commita, ele não dispara via seus comandos. Mas o usuário vai disparar ao commitar; se o hook reclamar de algo, ele te volta o erro e você corrige.
7. **Status no PARALELIZACAO.md**: marca a SUA linha (`⚪ → 🟡 → 🟢`) e avisa no chat. Mike commita.
8. **Sem deploy/push pra prod por agente**: deploy é responsabilidade do usuário.

---

## Status (atualizar conforme avança)

> **Cada agente edita só sua linha.** Formato: `🟢 done` / `🟡 in progress` / `⚪ pending` / `🔴 blocked`.

### Onda 1

| Trilha | Agente  | Tarefa                                              | Status     | Última atualização |
| ------ | ------- | --------------------------------------------------- | ---------- | ------------------ |
| A      | Claude  | Fase 3 — remover código morto                       | 🟢 done    | 2026-05-21         |
| A      | Claude  | Fase 5 — renomear `Controlle10 - ftp/`              | 🟢 done    | 2026-05-21         |
| A      | Claude  | Fase 7-BACK — fatiar 8 arquivos Go >1000 linhas     | 🟢 done    | 2026-05-21         |
| A      | Claude  | Fase 8.6 — guard contra secret default em prod      | 🟢 done    | 2026-05-21         |
| A      | Claude  | Fase 9 setup — Vitest + 1 teste por store           | 🟢 done    | 2026-05-21         |
| B      | Codex   | Fase 7-FRONT-admin — `UsersAccessManager.vue`       | 🟢 done    | 2026-05-21         |
| B      | Codex   | Fase 7-FRONT-admin — `FeedbackWorkspace.vue`        | 🟢 done    | 2026-05-21         |
| B      | Codex   | Fase 7-FRONT-admin — `SettingsWorkspace.vue`        | 🟢 done    | 2026-05-21         |
| B      | Codex   | Fase 7-FRONT-admin — `ErpWorkspace.vue`             | 🟢 done    | 2026-05-21         |
| B      | Codex   | Fase 7-FRONT-admin — `ThemeColorInput.vue`          | 🟢 done    | 2026-05-21         |
| C      | Copilot | Fase 7-FRONT-operation — `OperationFinishModal.vue` | 🟢 done    | 2026-05-21         |
| C      | Copilot | Fase 8.7 — godoc/TSDoc nos pontos críticos          | 🟢 done    | 2026-05-21         |
| C      | Copilot | Fase 8.7 — criar `docs/adr/0001-rename-omni.md`     | 🟢 done    | 2026-05-21         |

### Onda 2

| Tarefa                                                    | Agente           | Status     | Última atualização |
| --------------------------------------------------------- | ---------------- | ---------- | ------------------ |
| Fase 8.1 — lazy load de modais pesados                    | Codex ou Copilot | 🟢 done    | 2026-05-21         |
| Fase 8.4 — HTTP security headers (Caddy/docs)             | Copilot          | 🟢 done    | 2026-05-21         |
| Fase 8.5 — CI audits (`npm audit` + `govulncheck`)        | Copilot          | 🟢 done    | 2026-05-21         |
| Fase 8.8 — script gerador de `COMPONENT_INVENTORY`        | Copilot          | 🟢 done    | 2026-05-21         |
| Fase 9 — completar testes (stores + composables realtime) | Copilot          | 🟢 done    | 2026-05-21         |

### Onda 3 — só 1 agente roda

| Tarefa                                    | Quem   | Status     | Última atualização |
| ----------------------------------------- | ------ | ---------- | ------------------ |
| Fase 4 — renomear para Omni (repo pronto; DB prod pendente) | Copilot | 🟡 in progress | 2026-05-21         |

### Onda 4 — validação e deploy

| Tarefa                           | Quem             | Status     |
| -------------------------------- | ---------------- | ---------- |
| Build final front + back         | Claude           | 🟢 done (2026-05-21) |
| Lint + typecheck full            | Claude           | 🟡 done com ressalvas (2026-05-21) |
| Smoke local completo             | Usuário + Claude | ⚪ pending |
| Deploy VPS via `prod:deploy:vps` | Usuário          | ⚪ pending |
| `git tag v0.2.0-omni`            | Usuário          | ⚪ pending |

---

## Validação final antes de fechar este ciclo

- [x] `npm --prefix web run build` ✅
- [ ] `npm --prefix web run lint` → 0 errors
- [ ] `npm --prefix web run typecheck` → ≤ 387 erros (baseline atual; meta: reduzir)
- [x] `go test ./...` em `back/` ✅
- [ ] `golangci-lint run ./...` em `back/` → ≤ 94 issues (baseline; meta: reduzir)
- [x] `docker compose config` + `docker compose -f docker-compose.prod.yml config` ✅
- [ ] `npm run dev` sobe 3 portas (`5432`, `8080`, `3003`)
- [ ] `http://localhost:3003` mostra `<title>Omni</title>` ✅
- [ ] DB prod renomeado para `omni` ✅
- [ ] Smoke completo: login, criar tenant, abrir operação, criar tarefa, abrir erp, abrir relatórios
- [ ] Deploy realizado e validado via `curl -I https://lista.whenthelightsdie.com`

---

## Referências

- Plano detalhado por fase → [PLANO_REFATORACAO.md](PLANO_REFATORACAO.md)
- Estado atual do projeto → [ESTADO_ATUAL.md](ESTADO_ATUAL.md)
- Visual com gráficos → [plano-refatoracao.html](plano-refatoracao.html)
- Deploy → [DEPLOY_VPS.md](DEPLOY_VPS.md) + [DEPLOY_CHECKLIST.md](DEPLOY_CHECKLIST.md)
- Briefings por agente → [agents/README.md](agents/README.md)
