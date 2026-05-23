# Briefing — Codex CLI (Trilha B: frontend administrativo)

> Você é o Codex CLI. Sua zona é o frontend administrativo: workspaces de Users, Feedback, Settings, Erp + componente de Theme. Ver [PARALELIZACAO.md](../PARALELIZACAO.md) para visão geral. Ver [README.md](README.md) para regras comuns.

## Sua zona — pode editar

- `web/app/components/users/**`
- `web/app/components/feedback/**`
- `web/app/components/settings/**`
- `web/app/components/erp/**`
- `web/layers/core/components/theme/**`
- `web/app/composables/**` — apenas para **criar** composables novos extraídos dos workspaces acima
- `web/app/domain/utils/**` — apenas para **criar** utils novos extraídos dos workspaces acima
- AGENT.md das pastas acima (criar se não existirem)

## NÃO toca

- `back/**`
- `web/layers/tasks/**`
- `web/app/components/operation/**` (zona do Copilot)
- `web/app/components/{alerts,banco,campaigns,consultant,crm,dashboard,data,demo,intelligence,layout,multistore,omni,ranking,reports,roadmap,tenants,ui}/**`
- `web/app/pages/**`
- `web/app/stores/**`
- `docs/**`
- `package.json` (qualquer)
- `web/nuxt.config.ts`, `web/tsconfig.json`, `web/eslint.config.mjs`, `web/.prettierrc.json`

---

## Antes de começar

Você NÃO roda comandos git. Mike já está na `refactor/multi-tenant-core` e cuida de pull/commit/rebase/push em lote. Você só:

```bash
npm --prefix web install    # se Mike avisar que vieram deps novas
npm --prefix web run build  # baseline verde antes de mexer
npm --prefix web run lint   # baseline: 0 errors, ~200 warnings
```

Se ao começar você notar que o working tree tem mudanças do Mike que conflitam com a sua zona, **pare e avise**. Não tente rebase/stash.

## Padrão de fatiamento Vue (válido para todos os componentes desta zona)

Diagnóstico comum nos arquivos da sua zona: `<template>` grande + `<script>` ainda maior com 20+ helpers, drafts, normalizers e actions inline.

Padrão alvo de divisão para um arquivo `XyzWorkspace.vue` com 1.300 linhas:

```
XyzWorkspace.vue                    (200-400 linhas — só orquestração)
├── components/xyz/
│   ├── XyzWorkspace.vue            ← antes era tudo, agora só template + state principal
│   ├── XyzList.vue                 ← extraído (linhas de tabela/lista)
│   ├── XyzDetailDrawer.vue         ← extraído (modal de detalhes)
│   ├── XyzCreateModal.vue          ← extraído (form de criação)
│   └── XyzFilters.vue              ← extraído (barra de filtros)
└── composables/
    └── useXyzDrafts.ts             ← extraído (criação de drafts, normalizers)
domain/utils/
└── xyz-access.ts                   ← extraído (helpers puros: normalizeText, getLabel)
```

Regras:

1. **Cada `<script setup>` filho fica abaixo de 250 linhas.**
2. **Cada `<template>` fica abaixo de 200 linhas.**
3. Composables extraídos só recebem **estado e ações relacionadas**, nunca state global.
4. Utils extraídos são **funções puras** (sem `ref`/`reactive`).
5. Props/emits dos sub-componentes precisam ser **explícitos e tipados** (`defineProps<{ user: User }>()`, `defineEmits<{ save: [draft: UserDraft] }>()`).
6. Preserve os data-test/CSS scoped existentes — não quebre estilos visuais.

---

## ONDA 1 — Tarefas

> **Ordem sugerida**: do menor para o maior, pra você ganhar confiança no padrão antes de atacar os monstros.

### Tarefa B1 — `ThemeColorInput.vue` (1.007 linhas) — o mais simples

Arquivo: [web/layers/core/components/theme/ThemeColorInput.vue](../../web/layers/core/components/theme/ThemeColorInput.vue)

Problema: input de cor + picker custom inline.

- [x] Extrair o color picker para `web/layers/core/components/theme/ThemeColorPicker.vue` (componente filho).
- [x] `ThemeColorInput.vue` fica como wrapper fino: input + abre/fecha picker via `<ThemeColorPicker v-model:open="open" :value="value" @apply="onApply" />`.
- [x] Helpers de conversao (hex/rgb/hsl) que estavam inline -> movidos para [web/app/domain/utils/color.ts](../../web/app/domain/utils/) (criar).
- [x] Validar: `npm --prefix web run build`, abrir `/themes` em dev e testar mudar cor. Build passou; `/themes` segue redirecionando para `/operacao` neste perfil/ambiente, entao a validacao visual direta ficou limitada a rota.
- [x] Critério: `ThemeColorInput.vue` ≤ 350 linhas, `ThemeColorPicker.vue` ≤ 500 linhas.

Sugestão de commit (para Mike): `refactor(theme): extrair picker de ThemeColorInput em ThemeColorPicker`

### Tarefa B2 — `ErpWorkspace.vue` (1.230 linhas)

Arquivo: [web/app/components/erp/ErpWorkspace.vue](../../web/app/components/erp/ErpWorkspace.vue)

Problema: workspace com várias abas (Status, Products, Runs, Sync) inline.

- [x] Inspecionar o `<template>` e identificar as 3-4 abas.
- [x] Cada aba → componente próprio em `web/app/components/erp/`:
  - `ErpStatusTab.vue` (já existe `ErpSyncStatus.vue` — verificar se reaproveita)
  - `ErpProductsTab.vue` (já existe `ErpProductsTable.vue` — verificar)
  - `ErpRunsTab.vue` (já existe `ErpSyncRunsTable.vue`)
  - `ErpSyncOverviewTab.vue` (já existe `ErpSyncOverview.vue`)
- [x] `ErpWorkspace.vue` fica como router/host: `<TabsHeader />` + `<component :is="currentTab" />`.
- [x] Helpers de formatação ERP → `web/app/domain/utils/erp-display.ts` (criar).
- [x] Validar: `/erp` carrega, troca de aba funciona, dados aparecem.
- [x] Critério: `ErpWorkspace.vue` ≤ 350 linhas, cada Tab ≤ 500.

Sugestão de commit (para Mike): `refactor(erp): dividir ErpWorkspace em sub-componentes por aba`

### Tarefa B3 — `SettingsWorkspace.vue` (1.282 linhas)

Arquivo: [web/app/components/settings/SettingsWorkspace.vue](../../web/app/components/settings/SettingsWorkspace.vue)

Problema: várias seções de configuração inline (operação, modal, alertas, options, catálogo).

- [x] Criar pasta `web/app/components/settings/sections/`.
- [x] Para cada seção visível no `<template>`, extrair:
  - `SettingsOperationSection.vue`
  - `SettingsModalSection.vue`
  - `SettingsAlertsSection.vue`
  - `SettingsOptionsSection.vue` (já existe `SettingsOptionManager.vue` — verificar)
  - `SettingsProductsSection.vue` (já existe `SettingsProductManager.vue`)
- [x] `SettingsWorkspace.vue` fica como host das tabs/seções.
- [x] Validar: `/configuracoes` carrega, salvar uma config funciona, refresh persiste.

> **Cuidado especial**: o módulo Settings é frágil historicamente. Não quebre o contrato com `settingsStore.actions`. Trabalhe em pequenos lotes (uma seção por vez) e avise o Mike ao terminar cada seção — assim ele pode commitar separado para rollback granular.

Sugestão de commits (para Mike, um por seção): `refactor(settings): extrair <X>Section de SettingsWorkspace`

### Tarefa B4 — `FeedbackWorkspace.vue` (1.297 linhas)

Arquivo: [web/app/components/feedback/FeedbackWorkspace.vue](../../web/app/components/feedback/FeedbackWorkspace.vue)

- [x] Extrair listagem em `FeedbackList.vue`.
- [x] Extrair filtros em `FeedbackFilters.vue`.
- [x] Extrair painel de detalhes em `FeedbackDetailPanel.vue`.
- [x] (Já existe `FeedbackFormModal.vue` — preservar.)
- [x] `FeedbackWorkspace.vue` orquestra o layout (3 colunas: list + detail + filtros) e os estados de seleção/loading.
- [x] Validar: `/feedback` carrega, abrir um item mostra detalhes, filtros funcionam.

Sugestão de commit (para Mike): `refactor(feedback): dividir FeedbackWorkspace em List/Filters/Detail`

### Tarefa B5 — `UsersAccessManager.vue` (2.187 linhas) — o monstro

Arquivo: [web/app/components/users/UsersAccessManager.vue](../../web/app/components/users/UsersAccessManager.vue)

Já tem mapeamento detalhado na Tarefa 7.1 do [PLANO_REFATORACAO.md](../PLANO_REFATORACAO.md):

- [x] Extrair 20+ helpers do `<script>` (`normalizeText`, `normalizeSearch`, `getRoleLabel`, `isStoreScopedRole`, `isConsultantManaged`, `getStoreName`, `getStoreLabel`, `getOnboardingLabel`, `getOnboardingTone`, `getAccessStateLabel`, `getAccessStateTone`) → `web/app/domain/utils/user-access.ts`.
- [x] Extrair criação de drafts (`createRowDraft`, `createDetailDraft`, `assignDetailDraft`, `getRowDraft`, `resetRowDraft`, `resetCreateDraft`) → composable `web/app/composables/useUserAccessDrafts.ts`.
- [x] Dividir o `<template>` (548 linhas) em sub-componentes:
  - `UsersAccessTable.vue` (lista principal)
  - `UsersAccessDetailDrawer.vue` (painel lateral de detalhes)
  - `UsersAccessCreateModal.vue` (modal de criação)
  - `UsersAccessRoleBadge.vue` (badge de papel, se reutilizável)
- [x] Já existe a função `switchToInviteMode()` no script (adicionada na Fase 6.1) — preservar.
- [x] `UsersAccessManager.vue` orquestra estado e composição.
- [x] Validar: `/usuarios` carrega, criar usuário, editar usuário, resetar senha, alternar acesso por workspace.
- [x] Critério: arquivo principal ≤ 400 linhas, cada sub ≤ 500.

> **Cuidado**: este componente também é usado dentro de `multistore` provavelmente. Verifique imports antes de remover qualquer export.

Sugestões de commits separados (para Mike, avise a cada lote pronto):

1. `refactor(users): extrair helpers para domain/utils/user-access`
2. `refactor(users): extrair useUserAccessDrafts composable`
3. `refactor(users): dividir UsersAccessTable em sub-componente`
4. `refactor(users): dividir UsersAccessDetailDrawer em sub-componente`
5. `refactor(users): dividir UsersAccessCreateModal em sub-componente`

---

## Validação por tarefa (não pule)

Antes de marcar uma tarefa como `[x]`:

1. `npm --prefix web run build` ✅
2. `npm --prefix web run lint` → não introduziu novo error (warnings podem variar)
3. `npm --prefix web run typecheck` → não piorou (baseline atual: 387 erros)
4. **Smoke manual** da página correspondente em `npm run dev` (necessário para componentes Vue: type check não pega quebra visual)
5. AGENT.md da pasta tocada atualizado
6. Marcou 🟢 na sua linha em [PARALELIZACAO.md](../PARALELIZACAO.md)
7. Avisou no chat: "Tarefa BX pronta, sugestão de commit: ..." — **Mike commita**, você não.

---

## ONDA 2 — Tarefas (depois que A e C terminarem a Onda 1)

### Tarefa B6 — Testes Vitest de stores admin

Exemplo mínimo de teste de store Pinia (template que você pode copiar):

```ts
// web/app/stores/__tests__/users.test.ts
import { describe, it, expect, beforeEach } from "vitest";
import { setActivePinia, createPinia } from "pinia";
import { useUsersStore } from "~/stores/users";

describe("useUsersStore", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it("inicia sem usuários", () => {
    const store = useUsersStore();
    expect(store.users).toEqual([]);
  });

  it("normaliza nick ao criar draft", () => {
    const store = useUsersStore();
    const draft = store.createDraft({ displayName: "João da Silva" });
    expect(draft.nick).toBe("joao");
  });
});
```

Adicionar 1 teste por store que VOCÊ tocou na Onda 1:

- [ ] `web/app/stores/__tests__/users.test.ts`
- [ ] `web/app/stores/__tests__/feedback.test.ts`
- [ ] `web/app/stores/__tests__/settings.test.ts`
- [ ] `web/app/stores/__tests__/erp.test.ts`

Validar: `npm --prefix web test`

Sugestão de commit (para Mike): `test(stores): adicionar testes mínimos para stores administrativos`

---

## Se algo bloquear

- Componente importa de fora da sua zona e você precisa mexer → **pare e avise no chat**.
- Suspeita de regressão visual → screenshot antes/depois e pergunta no chat.
- Conflito no working tree (Mike puxou algo enquanto você trabalhava) → **NÃO RESOLVA SOZINHO** com git, pare e avise. Mike resolve.
- Pediram pra você commitar/pull/push? → não roda. Avisa "Mike, terminei X, sugestão: <mensagem>".

## Referências

- [PARALELIZACAO.md](../PARALELIZACAO.md) — visão geral, ondas, status
- [README.md](README.md) — regras comuns
- [../PLANO_REFATORACAO.md](../PLANO_REFATORACAO.md) Fase 7 — detalhamento das tarefas
- [../ESTADO_ATUAL.md](../ESTADO_ATUAL.md) Seção 9 — pilares de qualidade
- [../../web/AGENT.md](../../web/AGENT.md) — regras de arquitetura do front
