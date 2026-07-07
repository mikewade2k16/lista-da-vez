# Plano — Módulo `theme` (global) + Tema Liquid Glass

> Status: **plano (pending)**. Fonte de verdade da fase `liquid-glass-ui` (código GLASS)
> do roadmap. Espelhado em `web/app/components/roadmap/roadmap-data.ts`.
> Decisões do dono (2026-07-02) fixadas nas seções 1 e 2. **Não implementar back/migration
> antes do OK deste plano** (regra doc-first / plan-before-implement).

---

## 1. Contexto e decisões

Nasceu de dois pedidos:

1. Deixar o painel com a estética **liquid glass** do `/calendario` (aurora ambiente +
   superfícies de vidro que a refratam + botões flutuantes) em todo o sistema.
2. O toggle de **Page Headers** (eyebrow/título/descrição) "volta e meia" reativa — não
   está persistindo.

**Decisões do dono (2026-07-02):**

- **D1 — Liquid Glass é um TEMA PRÓPRIO** (5º tema, ao lado de `light`/`dark`/`apple`/`custom`),
  selecionável e persistido. NÃO é um modificador sobre o tema base. Mantém Original/Padrão
  intactos; ativa/desativa pelo Theme Studio.
- **D2 — A persistência de tema/aparência vira um MÓDULO `theme` separado e GLOBAL da
  plataforma.** Não pode ficar acoplada ao módulo `queue` (Fila), como está hoje. É config de
  nível plataforma (igual ao `menu_layout`).

---

## 2. Diagnóstico do bug de persistência (por que "Page Headers volta e meia")

Hoje o **appearance** (tema ativo + overrides por tema, **incluindo** os toggles
`admin-page-header-eyebrow/title/description-display`) É gravado no banco — MAS **dentro do
módulo `queue/settings`**, tenant-scoped:

- Back: `back/internal/modules/queue/settings` → `AppearanceSectionRecord{TenantID, Appearance}`,
  `PATCH /v1/settings/appearance` (RequireAuth). Enum `normalizeAppearanceTheme` só aceita
  `light|dark|apple|custom`.
- Front: `web/layers/core/composables/useOmniTheme.ts` só grava no banco quando
  `isRemoteManaged` = autenticado **com `activeTenantId` não-vazio**
  (`persistRemoteAppearance` faz `if (!tenantId) return`). Senão, cai em `localStorage`.

Causa-raiz:

- **`platform_admin` tem `TenantID` vazio** (memória `operacao_scope_account`) → o tema dele só
  vai pra `localStorage`, nunca ao banco.
- Contas **sem o módulo `queue`** não carregam `/v1/settings` (403 `module_disabled`) → idem.
- O `localStorage` é **apagado** (`clearLocalThemeStorage`) exatamente quando o admin entra em
  "ver como" um cliente que TEM tenant, ou ao trocar de navegador/device/limpar cache.

Resultado: para quem opera como platform_admin, o toggle de Page Headers e o tema só vivem no
localStorage e somem. **Mover a persistência para um armazenamento global, sem depender de
tenant nem do módulo Fila, resolve o bug** — e a própria UI descreve o controle como "controle
GLOBAL do cabeçalho das páginas admin", ou seja, é para ser global mesmo.

---

## 3. Arquitetura proposta (back)

Reaproveitar a infra global já provada: **`core.platform_settings`** (key-value singleton,
`key text pk`, `config jsonb`, `updated_at`, `updated_by`), a mesma que guarda `menu_layout`.

- **Sem nova tabela / sem nova migration de schema.** O appearance vira uma nova **chave**
  `appearance` em `core.platform_settings`. (Alternativa, se o dono preferir schema físico
  próprio: `theme.settings` com migration idempotente — mais isolado, mas exige migration. A
  recomendação é a chave em `platform_settings`, zero migration, mesma semântica global.)
- **Onde mora o "módulo theme" (decisão de arquitetura):** um módulo do **Module Registry** (como
  `calendar`) é um FEATURE-MODULE contratável por conta (entra no catálogo, liga/desliga por
  account). Tema NÃO é isso — é config **global de plataforma**, exatamente como `menu_layout`,
  que vive no módulo `core` (sempre-ligado, `IsCore`). Portanto "módulo theme separado" (D2) se
  traduz como **concern próprio e desacoplado da Fila, DENTRO do `core`**: novos arquivos
  `platform_appearance_{model,service,http}.go` espelhando `platform_settings_*` (menu-layout),
  reusando o mesmo `PostgresPlatformSettingsRepository`. NÃO criar um pacote/registry-module
  `theme` — isso o exporia como módulo toggleável por conta, o que é semanticamente errado.

- **Enum de tema → CATÁLOGO data-driven (decisão de qualidade, dono 2026-07-02):** enum fechado
  de temas é smell (cada tema novo mexeria em enum no Go + `OmniThemeName` + 3 mapas no front).
  O back **não** guarda um set fechado: valida `activeTheme` só como **slug** (`^[a-z][a-z0-9-]*$`,
  com teto de tamanho) e persiste; a fonte de verdade de QUAIS temas existem é o **catálogo do
  front**. Adicionar um tema passa a ser: uma entrada no catálogo do front (+ CSS próprio do
  tema). Zero mudança no Go.

**Shape persistido (`appearance` jsonb)** — espelha o que o front já monta em
`buildAppearanceSnapshot()`:

```jsonc
{
  "version": 1,
  "activeTheme": "liquidglass",          // light | dark | apple | custom | liquidglass
  "customThemeName": "Custom",
  "overrides": {                          // por tema → { varKey: value }
    "liquidglass": { "primary": "...", "admin-page-header-eyebrow-display": "none" }
  }
}
```

**Endpoints (espelham `/v1/platform/menu-layout`):**

- `GET /v1/platform/appearance` — `RequireAuth` (todos os autenticados leem; é o que pinta o
  painel de qualquer usuário).
- `PUT /v1/platform/appearance` — `RequireAuth` + `platform_admin` (só admin escreve o tema
  global). Reusa `requirePlatformAdmin`.
- Registrar em `app.go` ao lado de `RegisterPlatformSettingsRoutes`.

**Temas válidos:** o back não mantém enum fechado (ver bullet acima) — valida `activeTheme` como
slug. `liquidglass` (e futuros temas) passam sem tocar Go. O `normalizeAppearanceTheme` do
queue-settings vira legado (seção 6).

**Regras obrigatórias (skill de engenharia):** IDs como string; validação de tema/placement no
service (400 em valor inválido); leitura autenticada / escrita só platform_admin; sem
`account_id` (é platform-global, exceção consciente igual ao platform_settings); < 450
linhas/arquivo.

---

## 4. Frontend

`web/layers/core/composables/useOmniTheme.ts`:

- Trocar a I/O de `/v1/settings/appearance` (queue) por `GET/PUT /v1/platform/appearance`
  (módulo theme). **Remove a dependência de `activeTenantId` não-vazio** → platform_admin passa
  a persistir. `isRemoteManaged` deixa de depender de tenant; passa a "autenticado".
- Carregar o appearance global no boot (plugin `omni-theme.client.ts` /
  `initializeFromStorage`) a partir do endpoint, não do runtime da fila.
- Adicionar `liquidglass` em `OmniThemeName`, defaults (paleta própria — seção 5),
  `selectorByTheme('liquidglass') = '.theme-liquidglass'`, e label em `OMNI_THEME_LABELS`.
- Theme Studio (`ThemeStudioHeaderControls.vue` / `themes.vue`): incluir Liquid Glass na lista
  de temas selecionáveis.

**Page Headers:** os toggles continuam sendo overrides de tema
(`admin-page-header-*-display`), mas agora salvos no appearance GLOBAL → persistem de verdade
para o platform_admin. `useAdminPageHeaderVisibility` não muda (só lê `getThemeValue`).

**Preferência pessoal light/dark (nuance a confirmar):** hoje `omni-ui-user-theme` no
localStorage deixa cada usuário escolher light/dark no seu device. Proposta: manter isso como
conveniência **apenas para alternar entre os temas-base**; quando o tema global ativo for
`liquidglass` (ou `apple`/`custom`), ele vence. Confirmar com o dono (seção 9).

---

## 5. Tema Liquid Glass (design)

Extrair para tokens do design system o que o `/calendario` já faz à mão, e definir a paleta do
tema `liquidglass` em `useOmniTheme` (`OMNI_THEME_DEFAULTS.liquidglass`) + regras em
`omni-tokens.css` sob `.theme-liquidglass`:

- **Aurora ambiente:** hoje está SEMPRE ligada como preview no shell
  (`.module-workspace-full::before` + `.workspace::before` em `layouts/dashboard.vue`). Passa a
  ser **gateada por `.theme-liquidglass`** (só aparece quando o tema Liquid Glass está ativo);
  nos demais temas, sem aurora.
- **Superfície de vidro** (tokens novos, a partir do calendário —
  `backdrop-filter: blur(24px) saturate(1.7)`, `linear-gradient` de highlight,
  `inset 0 1px 0 rgba(...)`, `--surface / 0.46`): ex. `--glass-blur`, `--glass-saturate`,
  `--glass-surface-alpha`, `--glass-highlight`, `--glass-border`. Uma classe/utilitário
  reutilizável (`.omni-glass` ou `<OmniGlassSurface>`) para cards/painéis, com **fallback**
  (surface sólida translúcida) quando `backdrop-filter` não é suportado, e texto sempre em
  `--text-main` (contraste AA).
- **Botões flutuantes** translúcidos (padronizar o `.dashboard-feedback-btn`, hoje azul sólido
  com hex, e barras de ação flutuantes) reusando os tokens.
- **Perf/a11y:** `backdrop-filter` é caro — evitar muitas camadas de blur sobrepostas na mesma
  tela; `prefers-reduced-motion` para a aurora; medir conforme `ENGINEERING_PRINCIPLES.md`.

---

## 6. Legado / compatibilidade

- O appearance antigo (queue-settings, per-tenant) fica **legado**. Na 1ª carga, se não houver
  appearance global gravado ainda, ler o antigo como **fallback** (migração suave) e gravar no
  global no 1º save. Documentar em `docs/LEGADO.md` e marcar o caminho antigo
  (`/v1/settings/appearance`) para remoção.
- Não apagar dados de tema existentes sem backup/decisão (regra "nunca sobrescrever dado do
  usuário"). Como o novo é global e o antigo per-tenant, a transição é aditiva.
- `useOmniTheme` deixa de escrever no endpoint da fila; o endpoint antigo pode ficar inerte até
  a remoção.

---

## 7. Migration / Notas de Deploy

- **Recomendação (chave em `core.platform_settings`): sem migration de schema** — a tabela já
  existe (0160). Só código Go + front.
- Se o dono escolher schema físico próprio (`theme.settings`): migration idempotente, SQL plano,
  **sem** marcadores `-- +goose` e **sem** DROP (regra do migrator que roda o arquivo inteiro),
  schema qualificado, sem `account_id` (platform-global).
- **Toda alteração em `back/` exige rebuild:** `docker compose up -d --build api` (não basta
  restart). Registrar aqui as env/rotas novas se houver.

---

## 8. Fases de implementação (proposta)

1. **Back — módulo `theme`**: model/store(reuse)/service/http + `GET/PUT /v1/platform/appearance`
   + enum aceita `liquidglass` + registro em app.go. Rebuild api.
2. **Front — rewire**: `useOmniTheme` lê/grava no global; boot carrega do endpoint; Page Headers
   passam a persistir. (Corrige o bug.)
3. **Tema Liquid Glass**: `liquidglass` no enum/defaults/selector + label no Theme Studio +
   tokens de vidro + gate da aurora no `.theme-liquidglass`.
4. **Superfície de vidro reutilizável** (`.omni-glass`/componente) + botões flutuantes.
5. **Rollout página a página** (CRM, Tasks, ERP, Cardápio, Manage, Meta Ads...), cada uma
   validada no claro/escuro/mobile. Não remover funcionalidade para "encaixar" visual.
6. **Docs**: DESIGN_SYSTEM.md (tokens de vidro) + AGENT.md dos módulos tocados + roadmap.

## 9. Verificação

- platform_admin ativa Liquid Glass no Theme Studio → recarrega/troca de device/entra em "ver
  como" cliente → o tema e o toggle de Page Headers **permanecem** (vêm de
  `/v1/platform/appearance`, não do localStorage).
- Com Liquid Glass ativo: aurora + superfícies de vidro no painel todo. Com light/dark/apple:
  sem aurora, visual original intacto.
- Network mostra `/v1/platform/appearance` (não mais `/v1/settings/appearance`).
- `golangci-lint`, `vue-tsc`, `eslint` limpos; cada arquivo < 450 linhas.

## 10. Decisões ainda abertas (confirmar antes/na implementação)

- **Storage:** chave `appearance` em `core.platform_settings` (recomendado, zero migration) ×
  schema próprio `theme.settings` (migration).
- **Light/dark por-usuário:** manter a preferência local só para os temas-base, ou remover de
  vez e deixar 100% global?
- **Custom theme:** o tema `custom` hoje é per-tenant/local; ao virar global, o "custom" passa a
  ser único da plataforma — confirmar que é o desejado.
