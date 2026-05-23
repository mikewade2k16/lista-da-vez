# Briefing — GitHub Copilot Codex (Trilha C: frontend operação + docs inline)

> Você é o GitHub Copilot (Codex). Sua zona é o módulo de operação (em especial o monstro `OperationFinishModal.vue`) + documentação inline. Ver [PARALELIZACAO.md](../PARALELIZACAO.md) para visão geral. Ver [README.md](README.md) para regras comuns.

## Sua zona — pode editar

- `web/app/components/operation/**` (todos os componentes da pasta `operation/`)
- `docs/adr/**` (criar a pasta)
- JSDoc/TSDoc em arquivos da Onda 1 da Tarefa C2 (lista abaixo) — apenas comentários, não alterar lógica
- `web/app/components/operation/AGENT.md` (atualizar com nova estrutura pós-fatiamento)

## NÃO toca

- `back/**`
- `web/layers/**`
- Outras pastas de `web/app/components/` (zona do Codex CLI ou outras)
- `web/app/composables/**`, `web/app/stores/**`, `web/app/pages/**`
- `docs/**` exceto `docs/adr/**` que você vai criar
- `package.json` (qualquer)
- Configs (eslint, prettier, tsconfig, nuxt.config)

---

## Antes de começar

Você NÃO roda comandos git. Mike já está na `refactor/multi-tenant-core` e cuida de pull/commit/rebase/push em lote. Você só:

```bash
npm --prefix web install     # se Mike avisar que vieram deps novas
npm --prefix web run build   # baseline verde
npm --prefix web run lint    # baseline: 0 errors, ~200 warnings
```

Se ao começar você notar que o working tree tem mudanças do Mike que conflitam com a sua zona, **pare e avise**. Não tente rebase/stash.

## Contexto rápido da pasta `operation/`

15 componentes em [web/app/components/operation/](../../web/app/components/operation/):

```
operation/
├── AGENT.md
├── AlertDisplayCenterModal.vue
├── AlertDisplayCornerPopup.vue
├── AlertDisplayFullscreen.vue
├── AlertDisplayHost.vue
├── OperationActiveServiceCard.vue
├── OperationAlertBanner.vue
├── OperationCampaignBrief.vue          ← marcado como morto, será removido pelo Claude
├── OperationConsultantStrip.vue
├── OperationFinishModal.vue            ← **2.143 linhas — ALVO PRINCIPAL**
├── OperationOverviewBoard.vue
├── OperationPauseReasonDialog.vue
├── OperationProductPicker.vue
├── OperationQueueColumns.vue
├── OperationScopeBar.vue
└── OperationWorkspace.vue
```

Já foi movido de `features/operation/components/` para `components/operation/` na Fase 6.7. Use isso como dado.

---

## ONDA 1 — Tarefas

### Tarefa C1 — Fatiar `OperationFinishModal.vue` (2.143 linhas)

Arquivo: [web/app/components/operation/OperationFinishModal.vue](../../web/app/components/operation/OperationFinishModal.vue)

Problema: modal multi-passo (wizard) de encerramento de atendimento. Toda a lógica do fluxo de finalização (cliente, produto visto, produto fechado, motivos, desfecho, observações) está inline.

> **Cuidado crítico**: este modal é o coração do fluxo operacional. Qualquer regressão visual/comportamental impacta o uso real. Trabalhe em commits pequenos para rollback granular. Leia [docs/operacao/operations.md](../operacao/operations.md) para entender o fluxo antes de mexer.

**Padrão alvo:**

```
OperationFinishModal.vue                       (≤ 400 linhas — orquestração do wizard)
├── components/operation/finish/
│   ├── FinishStepClient.vue                   ← passo 1: dados do cliente + motivo da visita
│   ├── FinishStepProduct.vue                  ← passo 2: produto visto / fechado / picker
│   ├── FinishStepOutcome.vue                  ← passo 3: desfecho (compra/reserva/nao-compra)
│   └── FinishStepNotes.vue                    ← passo 4: observações + finalização
└── domain/utils/
    └── finish-modal.ts                        ← validações puras (já pode existir como helper)
```

Subtarefas:

- [x] **Leia o arquivo todo primeiro** (não pule essa parte — 2.143 linhas).
- [x] Identifique os passos do wizard no `<template>` (provavelmente 3-4 passos delimitados por estados tipo `currentStep === 1`).
- [x] Identifique helpers de validação dentro do `<script>` que podem virar funções puras.
- [x] Crie `web/app/components/operation/finish/` (subpasta nova dentro de `operation/`).
- [x] Para cada passo, crie um sub-componente:
  - Props: tudo que o passo lê (`draft`, `options`, `disabled`, etc).
  - Emits: tudo que o passo emite de volta para o modal pai (`update:draft`, `next`, `previous`).
  - O sub-componente NÃO toca store diretamente — recebe via prop, emite via emit.
- [ ] Crie [web/app/domain/utils/finish-modal.ts](../../web/app/domain/utils/) com funções puras de validação e normalização (`validateClientStep`, `normalizeReasonInput`, etc).
- [x] `OperationFinishModal.vue` fica como orquestrador:
  - Mantém estado do draft (multi-step state)
  - Mantém comunicação com `operationsStore`
  - Renderiza `<FinishStepClient v-if="currentStep === 1" :draft="draft" @update:draft="..." @next="..." />`
- [x] Preserve TODOS os data-test, classes CSS, IDs visíveis. Validação visual é crítica.
- [ ] Smoke obrigatório: abrir `/operacao`, simular atendimento completo até o `finish`. Validar cada passo.

Status atual de C1:

- `OperationFinishModal.vue` foi reduzido para orquestrador fino.
- A logica foi movida para `components/operation/finish/useFinishModalController.js` para respeitar a zona `operation/**` sem invadir `web/app/domain/utils/**`.
- O smoke autenticado da `/operacao` depende de sessao local do Mike no navegador.

Sugestões de commits separados (avise Mike a cada passo extraído pra ele commitar granular):
1. `refactor(operation): criar pasta finish/ e extrair FinishStepClient`
2. `refactor(operation): extrair FinishStepProduct para finish/`
3. `refactor(operation): extrair FinishStepOutcome para finish/`
4. `refactor(operation): extrair FinishStepNotes para finish/`
5. `refactor(operation): mover validações para domain/utils/finish-modal`

Critério: `OperationFinishModal.vue` ≤ 400 linhas, cada `FinishStep*.vue` ≤ 500 linhas.

> **Lembre-se da memória do projeto**: "Modal e board card espelhados" — qualquer mudança comportamental no modal deve ser refletida no card do board do `/operacao`. NÃO mexa nos cards (estão em `OperationActiveServiceCard.vue` e `OperationQueueColumns.vue`) sem coordenar.

### Tarefa C2 — Fase 8.7: JSDoc/TSDoc em pontos críticos do front

Adicione comentário TSDoc no topo de cada **composable** e **store** da lista abaixo. Não altere lógica, só comentários.

Formato esperado:

```ts
/**
 * Gerencia o ciclo de vida da operação por loja: snapshot inicial, mutações via store,
 * sincronização com WebSocket.
 *
 * Fluxo típico: `hydrate()` → store assina canal realtime → mutações chegam por
 * `applyEvent()`. Em modo multi-loja, usa `useOperationsRealtimeMulti()` em paralelo.
 *
 * @example
 * ```ts
 * const { state, hydrate, applyEvent } = useOperationsRealtime()
 * await hydrate(storeId)
 * ```
 *
 * @see web/app/stores/operations.ts
 * @see docs/operacao/operations.md
 */
export function useOperationsRealtime() { ... }
```

Lista de arquivos para documentar:

- [x] [web/app/composables/useOperationsRealtime.ts](../../web/app/composables/useOperationsRealtime.ts) (1 comentário no `useOperationsRealtime`)
- [x] [web/app/composables/useContextRealtime.ts](../../web/app/composables/useContextRealtime.ts) (1 comentário no `useContextRealtime`)
- [x] [web/app/composables/useDashboardShell.ts](../../web/app/composables/useDashboardShell.ts) (1 comentário no `useDashboardShell`)
- [x] [web/app/composables/useDashboardNav.ts](../../web/app/composables/useDashboardNav.ts) (1 comentário)
- [x] [web/layers/core/composables/usePermission.ts](../../web/layers/core/composables/usePermission.ts) (1 comentário — chave para segurança)
- [x] [web/layers/core/composables/useNav.ts](../../web/layers/core/composables/useNav.ts) (1 comentário)
- [x] [web/layers/core/composables/useOmniTheme.ts](../../web/layers/core/composables/useOmniTheme.ts) (1 comentário)

**IMPORTANTE**: não toca o conteúdo das funções. Só adiciona o bloco `/** ... */` antes do `export function`.

Sugestão de commit (para Mike): `docs(composables): adicionar TSDoc nos composables críticos`

### Tarefa C3 — Fase 8.7: Criar primeiro ADR (`docs/adr/0001-rename-omni.md`)

ADR = Architecture Decision Record. Documenta uma decisão arquitetural importante com contexto, alternativas consideradas e consequências.

- [x] Criar pasta `docs/adr/`.
- [x] Criar `docs/adr/README.md` explicando o que é a pasta e o template ADR.
- [x] Criar `docs/adr/0001-rename-omni.md` com o conteúdo abaixo (preencha contexto puxando de [../PARALELIZACAO.md](../PARALELIZACAO.md) e [../PLANO_REFATORACAO.md](../PLANO_REFATORACAO.md) Fase 4):

```markdown
# ADR-0001: Renomear o produto para Omni

- **Status**: Aceito
- **Data**: 2026-05-16
- **Decisores**: Mike (product owner)

## Contexto

Convivem 4 nomes no projeto: `fila-atendimento` (pasta), `lista-da-vez` (compose/package), `Fila de Atendimento` (UI/README), `listaatendimento` (prod). Inconsistência presente em 11 pontos do código + 1 banco prod.

O nome `omni` já está difuso pela base (design system `omni-tokens.css`, componentes `OmniEditor`, composable `useOmniTheme`, domínio técnico `acesso.omni.local`, `docs/BACKLOG.md` chama produto de "Omni").

## Decisão

Padronizar para **Omni** (display) / **omni** (slug). Renomear em todos os 11 pontos + ALTER DATABASE em prod com janela.

## Alternativas consideradas

1. **Manter `lista-da-vez`** — coerente com slug histórico, mas dissonante com UI/README e direção do produto.
2. **Manter `Fila de Atendimento`** — bate com README mas ignora o que já existe codificado como `omni`.
3. **Nome novo (outro)** — gera mais retrabalho e ignora afinidade existente.

Vencedor: opção atual (Omni) por minimizar retrabalho — o nome já estava sendo usado tacitamente.

## Consequências

**Positivas:**
- 1 nome em todo o stack.
- Documentação coerente.
- Branding alinhado entre UI, código e infra.

**Negativas / riscos:**
- ALTER DATABASE em prod exige janela curta de manutenção.
- Risco de quebrar integrações externas que referenciam nome antigo (se houver — auditar).
- Bookmark/atalhos do usuário precisam ser atualizados.

## NÃO confundir com

- `omnichannel` — módulo de chat (continua existindo)
- `omnichannel-mvp_default` — rede Docker de outro projeto na VPS

## Referências

- [PLANO_REFATORACAO.md](../PLANO_REFATORACAO.md) Fase 4
- [PARALELIZACAO.md](../PARALELIZACAO.md) Onda 3
```

- [x] Criar template em `docs/adr/template.md` para próximos ADRs (estrutura igual ao 0001, com placeholders `{{...}}`).

Sugestão de commit (para Mike): `docs(adr): criar pasta docs/adr e ADR-0001 sobre rename Omni`

---

## Validação por tarefa

Antes de marcar uma tarefa como `[x]`:

1. `npm --prefix web run build` ✅
2. `npm --prefix web run lint` → não introduziu novo error
3. **Smoke manual obrigatório** da `/operacao` (especialmente após cada extração de `FinishStep*`)
4. `web/app/components/operation/AGENT.md` atualizado se mudou estrutura
5. Marcou 🟢 na sua linha em [PARALELIZACAO.md](../PARALELIZACAO.md)
6. Avisou no chat: "Tarefa CX pronta, sugestão de commit: ..." — **Mike commita**, você não.

---

## ONDA 2 — Tarefas (depois da Onda 1)

### Tarefa C4 — Fase 8.1: Lazy load de `OperationFinishModal`

Pré-condição: Tarefa C1 concluída (modal já fatiado em wizard de passos).

- [x] Em [web/app/components/operation/OperationWorkspace.vue](../../web/app/components/operation/OperationWorkspace.vue) (e quem mais importar o modal), substituir:
  ```ts
  import OperationFinishModal from '~/components/operation/OperationFinishModal.vue'
  ```
  por:
  ```ts
  const OperationFinishModal = defineAsyncComponent(
    () => import('~/components/operation/OperationFinishModal.vue')
  )
  ```
- [x] Envolver o uso do modal em `<Suspense>`:
  ```html
  <Suspense>
    <OperationFinishModal v-if="finishOpen" ... />
    <template #fallback>
      <CoreLoadingOverlay />
    </template>
  </Suspense>
  ```
- [x] Validar: `/operacao` carrega sem o modal pesado; ao clicar "Finalizar atendimento", o chunk lazy é baixado e o modal aparece com transição suave.
- [x] Medir ganho: validar que o chunk inicial da operação delega o modal para assets dinâmicos separados no build.

Status atual de C4:

- `OperationWorkspace.vue` agora faz `defineAsyncComponent()` do `OperationFinishModal.vue`.
- O modal so entra em `Suspense` quando `finishModalServiceId` estiver ativo, evitando carregar o chunk no boot da operacao.
- Diagnostics locais em `OperationWorkspace.vue` e `OperationFinishModal.vue` estao limpos.
- `npm --prefix web run build` passou e gerou o modal como entrada dinamica separada (`EJ_R_Lsc.js` + `OperationFinishModal.D0cjr9Bu.css`).
- Smoke autenticado concluido em `/operacao`: clique em `Encerrar atendimento`, abertura do modal, avancar para o passo 2 e fechar sem submissao.
- Medicao estrutural registrada: o chunk pai da operacao ficou em `CzBeAze-.js` (75.880 bytes), enquanto o modal lazy ficou em `EJ_R_Lsc.js` (76.579 bytes) + `OperationFinishModal.D0cjr9Bu.css` (144 bytes).

Sugestão de commit (para Mike): `perf(operation): lazy load de OperationFinishModal com Suspense`

### Tarefa C5 — Fase 8.8 transferida: gerador de `COMPONENT_INVENTORY`

Escopo transferido do briefing do Claude em 2026-05-21, por decisao do Mike.

- [x] Criar [scripts/dev/gen-component-inventory.mjs](../../scripts/dev/gen-component-inventory.mjs) para varrer `web/app/components/`, `web/app/features/` e `web/layers/*/components/`.
- [x] Detectar por componente: total de linhas, `<style scoped>`, uso de TipTap, uso de Pinia e imports de composables externos.
- [x] Gerar [docs/COMPONENT_INVENTORY_AUTO.md](../COMPONENT_INVENTORY_AUTO.md) sem sobrescrever o inventario humano em [docs/COMPONENT_INVENTORY.md](../COMPONENT_INVENTORY.md).
- [x] Adicionar comando `npm run inventory` no [package.json](../../package.json).
- [x] Rodar o gerador uma vez para materializar o output inicial.

Status atual de C5:

- `npm run inventory` passou e gerou `docs/COMPONENT_INVENTORY_AUTO.md` com 134 componentes Vue.
- O resumo atual saiu dividido por `web/app/components`, `web/app/features` e `web/layers/*/components`.
- As heuristicas bateram com amostras reais: `OmniEditor.vue` foi marcado com TipTap; `CrmWorkspace.vue` e `CoreAccountSwitcher.vue` foram marcados com Pinia; `DashboardSidebarNav.vue` listou `useDashboardNav` como composable externo.

Sugestão de commit (para Mike): `chore(scripts): adicionar gerador de inventario de componentes`

### Tarefa C6 — Fase 8.4 transferida: HTTP security headers (Caddy/docs)

Escopo transferido do briefing do Claude em 2026-05-21, por decisao do Mike.

- [x] Atualizar [docs/DEPLOY_VPS.md](../DEPLOY_VPS.md) com a secao `Headers de seguranca` dentro da integracao do Caddy da VPS.
- [x] Documentar a matriz base de headers (`HSTS`, `nosniff`, `X-Frame-Options`, `Referrer-Policy`, `Permissions-Policy`, `CSP`) com snippet pronto para `lista.whenthelightsdie.com`.
- [x] Documentar validacao pos-deploy por `curl -I` e `securityheaders.com`.
- [x] Atualizar o guia curto [docs/DEPLOY_CHECKLIST.md](../DEPLOY_CHECKLIST.md) com um passo explicito para conferir os headers no host publico.

Status atual de C6:

- [docs/DEPLOY_VPS.md](../DEPLOY_VPS.md) agora traz o bloco `header { ... }` para o `Caddyfile` do host e um exemplo completo do vhost com as rotas da app.
- A validacao pos-deploy ficou documentada por `curl -I` com filtro dos headers esperados e por checagem externa em `securityheaders.com`.
- [docs/DEPLOY_CHECKLIST.md](../DEPLOY_CHECKLIST.md) ganhou uma etapa curta de verificacao de headers para o fluxo operacional do dia a dia.

Sugestão de commit (para Mike): `docs(deploy): adicionar matriz de HTTP security headers para o Caddy`

### Tarefa C7 — Fase 8.5 transferida: CI audits (`npm audit` + `govulncheck`)

Escopo transferido do briefing do Claude em 2026-05-21, por decisao do Mike.

- [x] Atualizar [.github/workflows/deploy-vps.yml](../../.github/workflows/deploy-vps.yml) com auditoria de dependencias antes de qualquer passo de SSH/deploy.
- [x] Adicionar `npm audit --audit-level=high` para as dependencias da raiz.
- [x] Adicionar auditoria de dependencias do `web/` no mesmo workflow.
- [x] Adicionar `go list -m -u all` e `govulncheck ./...` no mesmo workflow.
- [x] Definir politica explicita de `block vs warn` para os achados atuais.

Status atual de C7:

- O workflow `deploy-vps.yml` agora roda `actions/setup-node` com Node `24.11.1` e audita a raiz com `npm audit --package-lock-only --audit-level=high` como gate bloqueante.
- O `web/` tambem passou a ser auditado com `npm audit --package-lock-only --audit-level=high`, mas por enquanto em modo warning-only porque o baseline atual ainda falha por advisories altas do Nuxt (`GHSA-g8wj-3cr3-6w7v` e `GHSA-77vg-94rm-hx3p`).
- O workflow agora roda `actions/setup-go` com Go `1.26.3`, alinhado ao [back/Dockerfile](../../back/Dockerfile), lista updates com `go list -m -u all` e executa `govulncheck` como gate bloqueante.
- A policy ficou assim: raiz npm = block; web npm = warn temporario; `go list -m -u all` = informativo; `govulncheck` = block.

Validacao usada em C7:

- `npm audit --audit-level=high` na raiz retornou `0 vulnerabilities`.
- `npm audit --package-lock-only --audit-level=high` em [web/package-lock.json](../../web/package-lock.json) confirmou o baseline atual com vulnerabilidade alta do Nuxt.
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` em container `golang:1.26.3-bookworm` retornou `No vulnerabilities found`, alinhando a auditoria ao toolchain real do build do backend.

Sugestão de commit (para Mike): `ci(deploy): adicionar audits de dependencias no workflow da VPS`

### Tarefa C8 — Fase 9 transferida: testes do frontend e gates de teste no CI

Escopo transferido do briefing do Claude em 2026-05-21, por decisao do Mike.

- [x] Confirmar `vitest` rodando em [web/](../../web/) com descoberta de testes alem de `layers/**`.
- [x] Adicionar 1 teste por store critica: `auth`, `operations`, `settings`, `tasks`.
- [x] Adicionar 1 teste por composable de realtime: `useOperationsRealtime`, `useContextRealtime`, `useTasksRealtime`.
- [x] Adicionar 1 teste por util de dominio: `permissions`, `campaigns`, `reports`.
- [x] Atualizar [.github/workflows/deploy-vps.yml](../../.github/workflows/deploy-vps.yml) para rodar `vitest` e `go test ./...` antes do deploy.

Status atual de C8:

- [web/vitest.config.ts](../../web/vitest.config.ts) agora inclui testes em `app/**`, define aliases de `~` e `@` para o diretório `app/`, e carrega [web/test/setup.ts](../../web/test/setup.ts) para o runtime minimo dos testes.
- A suite do frontend ganhou cobertura para stores criticas em [web/app/stores/auth.test.ts](../../web/app/stores/auth.test.ts), [web/app/stores/settings.test.ts](../../web/app/stores/settings.test.ts), [web/app/stores/operations.test.ts](../../web/app/stores/operations.test.ts) e [web/layers/tasks/stores/tasks.test.ts](../../web/layers/tasks/stores/tasks.test.ts).
- Os composables de realtime ficaram cobertos em [web/app/composables/useOperationsRealtime.test.ts](../../web/app/composables/useOperationsRealtime.test.ts), [web/app/composables/useContextRealtime.test.ts](../../web/app/composables/useContextRealtime.test.ts) e [web/layers/tasks/composables/useTasksRealtime.test.ts](../../web/layers/tasks/composables/useTasksRealtime.test.ts), com `MockWebSocket` em [web/test/helpers/mock-websocket.ts](../../web/test/helpers/mock-websocket.ts).
- Os utils pedidos pela fase ficaram cobertos em [web/app/domain/utils/permissions.test.ts](../../web/app/domain/utils/permissions.test.ts), [web/app/domain/utils/campaigns.test.ts](../../web/app/domain/utils/campaigns.test.ts) e [web/app/domain/utils/reports.test.ts](../../web/app/domain/utils/reports.test.ts).
- O workflow `deploy-vps.yml` agora instala deps do `web`, executa `npm run test` no frontend e `go test ./...` no backend como gates bloqueantes antes dos passos de SSH/deploy.

Validacao usada em C8:

- `npm --prefix web run test -- app/stores/auth.test.ts` passou como prova inicial da harness nova.
- `npm --prefix web run test` passou com `12` arquivos e `23` testes verdes.
- `go test ./...` em [back/](../../back/) passou antes de ligar o gate no workflow.
- `get_errors` em [.github/workflows/deploy-vps.yml](../../.github/workflows/deploy-vps.yml) retornou sem erros.

Sugestão de commit (para Mike): `test(front): cobrir stores e composables criticos com vitest`

### Tarefa C9 — Fase 4 transferida: renomear para Omni sem mexer direto em producao

Escopo transferido para o Copilot em 2026-05-21, com foco no repo, configs, docs e runbook seguro de cutover.

- [x] Padronizar nomes de package, compose e env para `Omni` / `omni` nos pontos vivos do repo.
- [x] Atualizar branding default em backend e frontend (`APP_NAME`, `SMTP_FROM_NAME`, titulo do app e templates de email).
- [x] Atualizar docs operacionais vivas ([README.md](../../README.md), [back/README.md](../../back/README.md), [back/START_LOCAL.md](../../back/START_LOCAL.md), [docs/DEPLOY_VPS.md](../DEPLOY_VPS.md)).
- [x] Criar runbook de rename do banco em [docs/deploy/db-rename.md](../deploy/db-rename.md) com caminho principal (`ALTER DATABASE`) e fallback (`dump/restore`).
- [x] Regenerar [package-lock.json](../../package-lock.json) e [web/package-lock.json](../../web/package-lock.json) apos o rename dos packages.
- [x] Validar com `docker compose config`, `docker compose --env-file .env.production.example -f docker-compose.prod.yml config`, `go test ./...`, `npm --prefix web run test` e `npm --prefix web run build`.

Status atual de C9:

- O repo agora usa `omni` como nome canonico de package, compose, banco local, usuario local e app backend.
- Os identificadores externos de maior risco foram preservados por design nesta janela: `omnichannel-mvp_default`, aliases `lista-api` / `lista-web`, `/opt/omnichannel/Caddyfile` e o path remoto `/home/deploy/lista-atendimento`.
- O frontend passou em build e testes com `omni-web`; o backend passou em `go test ./...`; os compose files dev/prod resolveram corretamente com volumes e redes `omni_*`.
- O build do Nuxt terminou sem erro, apenas com warnings conhecidos de sourcemap do Tailwind e deprecations de dependencias durante a etapa Nitro.
- O rename do banco em producao nao foi executado daqui; ficou documentado para janela controlada em [docs/deploy/db-rename.md](../deploy/db-rename.md).

Sugestão de commit (para Mike): `chore(omni): padronizar naming do repo e documentar rename seguro do banco`

---

## Se algo bloquear

- O `OperationFinishModal` está usando algo de fora da sua zona (composable de stores, etc) → você só LÊ, não modifica essas dependências. Se precisar mexer, **pare e avise no chat**.
- Suspeita de quebrar fluxo operacional → screenshot/gif antes/depois e pergunta.
- Conflito no working tree (Mike puxou algo enquanto você trabalhava) → **NÃO RESOLVA SOZINHO** com git, pare e avise. Mike resolve.
- Pediram pra você commitar/pull/push? → não roda. Avisa "Mike, terminei X, sugestão: <mensagem>".

## Referências

- [PARALELIZACAO.md](../PARALELIZACAO.md) — visão geral, ondas, status
- [README.md](README.md) — regras comuns
- [../PLANO_REFATORACAO.md](../PLANO_REFATORACAO.md) Fase 7.1 e 8.1
- [../operacao/operations.md](../operacao/operations.md) — fluxo operacional documentado (ler antes de fatiar o modal)
- [../../web/AGENT.md](../../web/AGENT.md) — regras de arquitetura do front
- Memória do projeto: "Modal e board card espelhados" — qualquer mudança em um deve ser replicada no outro
