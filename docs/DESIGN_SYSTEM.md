# Design System — Omni (painel `web/`)

> Documento de referência da camada de UI do painel Omni. Descreve o que **existe hoje** no
> código (`web/`), não um ideal. Fonte de verdade dos tokens: os arquivos em
> [web/app/assets/styles/](../web/app/assets/styles/). Sempre que mexer em cor, espaçamento ou
> componente base, atualize este doc junto.

---

## 1. Stack e arquitetura visual

| Camada | Tecnologia | Onde |
| --- | --- | --- |
| Framework | Nuxt 4 (Vue 3, `<script setup>`) | [web/nuxt.config.ts](../web/nuxt.config.ts) |
| Biblioteca de componentes | **Nuxt UI v3** (`@nuxt/ui`) | módulo em `nuxt.config.ts` |
| CSS utilitário | **Tailwind CSS v4** (importado via `@import 'tailwindcss'`) | [omni-design-system.css](../web/app/assets/styles/omni-design-system.css) |
| Estado | Pinia (`@pinia/nuxt`) | — |
| Color mode | `@nuxtjs/color-mode` (embutido no Nuxt UI), **default `dark`** | `nuxt.config.ts` → `colorMode` |

O sistema é **híbrido**, e isso é intencional:

1. **Tokens CSS + classes semânticas próprias** (`.queue-card`, `.admin-panel`, `.metric-card`…) —
   o grosso do painel. Escritas à mão em [components.css](../web/app/assets/styles/components.css)
   (~4.5k linhas), [layout.css](../web/app/assets/styles/layout.css) e afins.
2. **Componentes Nuxt UI** (`UButton`, `UPopover`, `UIcon`, `UModal`…) — usados em telas mais
   novas (theme switcher, popovers, ícones).
3. **Componentes base próprios** prefixados `App*` (`AppPanelButton`, `AppSelectField`…) em
   [web/app/components/ui/](../web/app/components/ui/).

Ordem de carregamento do CSS (definida em `nuxt.config.ts` → `css[]`):

```
omni-design-system.css  → tailwind + @nuxt/ui + omni-tokens.css
tokens.css              → tokens semânticos derivados
base.css                → reset, body, scrollbar, focus
layout.css              → shells e grids de página
components.css          → todas as classes de componente
tasks-modal.css         → modal de tarefas
presentation.css        → modo apresentação / TV
```

---

## 2. Sistema de temas

Os temas são **classes aplicadas no `<html>`** que trocam o valor dos tokens primitivos. Definidos
em [omni-tokens.css](../web/app/assets/styles/omni-tokens.css).

| Tema | Seletor | Uso |
| --- | --- | --- |
| Claro (default) | `:root` | base light |
| Escuro | `.dark` | **preferência padrão do app** |
| Apple Blue | `.theme-apple-blue` | tema alternativo (azul iOS) |

- A troca de tema é feita pelo [DashboardThemeSwitcher.vue](../web/app/components/dashboard/DashboardThemeSwitcher.vue),
  que persiste a escolha em `localStorage` na chave `omni-ui-user-theme` e usa o composable
  `useOmniTheme()`.
- `platform_admin` (ou quem tem o workspace `themes`) vê o atalho para o **Theme Studio** (`/themes`).
- Cada tema também ajusta `color-scheme` (light/dark) e `--brand-logo-filter` (inverte o logo no
  tema claro).

---

## 3. Tokens de cor

### 3.1 Como as cores funcionam (importante)

Os tokens primitivos **não são cores prontas** — são *triplets RGB sem `rgb()`*:

```css
--primary: 99 102 241;
```

Isso permite controlar a opacidade no ponto de uso, padrão dominante no projeto:

```css
color: rgb(var(--primary));               /* cor cheia */
background: rgb(var(--primary) / 0.12);   /* mesma cor a 12% */
border-color: rgb(var(--ring) / 0.36);
```

> **Regra:** ao usar um token primitivo, **sempre** envolva em `rgb(var(--token) / alpha)`.
> Nunca escreva hex direto num componente novo — use o token.

### 3.2 Primitivos por tema

| Token | Light (`:root`) | Dark (`.dark`) | Apple Blue | Significado |
| --- | --- | --- | --- | --- |
| `--bg` | `248 250 252` (#F8FAFC) | `6 10 18` | `236 246 255` | fundo da página |
| `--surface` | `255 255 255` | `13 18 29` | `247 252 255` | superfície de card/painel |
| `--surface-2` | `244 246 250` | `18 25 38` | `228 241 255` | superfície secundária / hover |
| `--border` | `226 232 240` (#E2E8F0) | `31 41 55` | `176 210 242` | linhas e bordas |
| `--text` | `15 23 42` (#0F172A) | `226 232 240` | `12 52 98` | texto principal |
| `--muted` | `100 116 139` (#64748B) | `148 163 184` | `67 105 147` | texto secundário |
| `--primary` | `99 102 241` (#6366F1) | igual | `10 132 255` (#0A84FF) | marca / ação primária |
| `--primary-600` | `79 70 229` (#4F46E5) | igual | `0 122 255` (#007AFF) | primária forte (gradiente) |
| `--success` | `34 197 94` (#22C55E) | igual | igual | sucesso / ativo |
| `--danger` | `239 68 68` (#EF4444) | `248 113 113` | `239 68 68` | erro / destrutivo |
| `--ring` | `99 102 241` | igual | `10 132 255` | foco / outline |

Cor de aviso (warning) **não é triplet** — vive em [tokens.css](../web/app/assets/styles/tokens.css)
como `--accent-warning: #fbbf24` (amber-400).

### 3.3 Tokens semânticos (camada de aliases)

Definidos em [tokens.css](../web/app/assets/styles/tokens.css) — apontam para os primitivos e dão
nomes de *intenção*. **Prefira estes em componentes de layout/texto:**

| Token | Resolve para | Uso |
| --- | --- | --- |
| `--bg-page` / `--bg-shell` | `rgb(var(--bg))` | fundo geral |
| `--bg-panel` | `rgb(var(--surface))` | fundo de card/painel |
| `--bg-muted` | `rgb(var(--surface-2))` | fundo suave |
| `--text-main` | `rgb(var(--text))` | texto principal |
| `--text-muted` | `rgb(var(--muted))` | texto secundário |
| `--text-inverse` | `rgb(var(--surface))` | texto sobre fundo de marca |
| `--line-soft` | `rgb(var(--border) / 0.72)` | borda padrão |
| `--line-strong` | `rgb(var(--border))` | borda forte |
| `--accent-info` | `rgb(var(--primary))` | destaque informativo |
| `--accent-success` | `rgb(var(--success))` | destaque positivo |
| `--accent-warning` | `#fbbf24` | atenção |
| `--accent-focus` | `rgb(var(--ring))` | foco / timer / valor em destaque |

### 3.4 Cores de status (convenção semântica)

Padrão recorrente: fundo da cor a ~12–18% + texto na cor cheia.

| Estado | Fundo | Texto | Exemplo de classe |
| --- | --- | --- | --- |
| Info / primária | `rgb(var(--primary) / 0.12–0.16)` | `rgb(var(--primary-600))` | `.summary-pill`, `.service-card__status-badge` |
| Sucesso / ativo | `rgb(var(--success) / 0.14)` | `rgb(var(--success))` | `.summary-pill--active`, `.campaign-status--ativa` |
| Erro / crítico | `rgb(var(--danger) / 0.12)` | `rgb(var(--danger))` | `.intel-badge--critical`, `.alert-list` |
| Neutro / encerrado | `rgb(var(--muted) / 0.18)` | `rgb(var(--muted))` | `.campaign-status--encerrada` |

> **Exceção conhecida:** a tela de **login/auth** ([admin-auth-* em components.css](../web/app/assets/styles/components.css))
> usa uma paleta própria **hardcoded** (roxo `#6c63ff → #7c74ff`, fundos `#0f1117`) fora dos tokens.
> É proposital (visual de marca da entrada), mas não replique esses hex em telas internas.

---

## 4. Tipografia

- **Família única** via `--font-sans` (system stack):
  `ui-sans-serif, system-ui, -apple-system, 'SF Pro Display', 'SF Pro Text', 'Inter', 'Segoe UI', Roboto, Arial…`
  Aplicada no `body` em [base.css](../web/app/assets/styles/base.css). Não há fonte web carregada
  (`ui.fonts: false`) — só a stack do sistema.
- **Idioma:** `<html lang="pt-BR">` e Nuxt UI com `locale: 'pt-BR'`
  ([app.config.ts](../web/app/app.config.ts)).

Escala observada (rem, sem tokens dedicados — valores convencionados):

| Papel | Tamanho | Peso |
| --- | --- | --- |
| Título de workspace | `1.45rem` | — |
| Título de painel | `1.05–1.2rem` | 700–800 |
| Corpo | `0.8–0.95rem` | 400–500 |
| Rótulo / meta | `0.72–0.78rem` | 700 (geralmente `uppercase`) |
| Micro / hint | `0.6–0.68rem` | 600–700 |
| Timer / valor-destaque | `1.2–1.5rem` | 800 |

Pesos de fonte são usados de forma expressiva: **700–800** para ênfase é comum no painel.

---

## 5. Raios, sombras e foco

Tokens em [omni-tokens.css](../web/app/assets/styles/omni-tokens.css):

**Border radius**

| Token | Valor | Uso típico |
| --- | --- | --- |
| `--radius-xs` | `10px` | botões pequenos, chips |
| `--radius-sm` | `12px` | inputs, itens de lista |
| `--radius-md` | `14px` | cards (`--radius-card`) |
| `--radius-lg` | `18px` | shells/painéis (`--radius-shell`) |

Aliases em `tokens.css`: `--radius-shell` (lg), `--radius-card` (md), `--radius-soft` (sm).
Elementos tipo pílula usam `border-radius: 999px`.

**Sombras** (suaves, baseadas em `color-mix`)

| Token | Uso |
| --- | --- |
| `--shadow-xs` | linha de 1px |
| `--shadow-sm` | card (`--shadow-card`) |
| `--shadow-md` | shell/painel/dropdown (`--shadow-shell`) |
| `--shadow-glow` | realce com anel na cor primária |

A cor da sombra muda por tema (`--shadow-color`) — mais opaca no dark.

**Foco** (acessibilidade) — definido globalmente em `base.css`:

```css
:focus-visible { outline: 2px solid rgb(var(--ring) / 0.48); outline-offset: 2px; }
```

**Scrollbar** — fina (4px), thumb em `rgb(var(--primary) / 0.45)`, global.

---

## 6. Ícones

O projeto usa **dois sistemas de ícones** em paralelo:

| Sistema | Como usar | Onde | Contexto |
| --- | --- | --- | --- |
| **Material Icons Round** | `<span class="material-icons-round">nome</span>` | folha carregada via `<link>` Google Fonts em `nuxt.config.ts` | telas com CSS próprio (operação, settings, roadmap…) |
| **Lucide** (via `@nuxt/icon`) | `<UIcon name="i-lucide-..." />` ou `:icon="'i-lucide-...'"` | `icon.collections: ['lucide']`, `provider: 'server'` | componentes Nuxt UI e telas novas |

> Para componentes **novos** prefira Lucide via `UIcon`/`UButton` (offline, tree-shake, sem request
> externo). Material Icons permanece pelo legado do CSS custom.

---

## 7. Componentes base próprios (`App*`)

Em [web/app/components/ui/](../web/app/components/ui/) — auto-importados pelo Nuxt.

| Componente | Props principais | Resumo |
| --- | --- | --- |
| **AppPanelButton** | `variant` (`primary`\|`secondary`\|`danger`\|`ghost`), `block`, `as`, `disabled` | Botão canônico. `primary` = gradiente `--primary → --primary-600`; `min-height 36px`; `radius 14px`. |
| **AppToggleSwitch** | `modelValue`, `label`, `compact`, `disabled` | Switch (track 2.7rem). Ligado = trilho `--success/0.34`. `role="switch"` + `aria-checked`. |
| **AppSelectField** | (select estilizado) | Dropdown próprio (`.app-select-field__trigger`), usado no header e em settings. |
| **AppPasswordInput** | — | Input de senha com toggle de visibilidade. |
| **AppDetailDialog** | — | Diálogo de detalhe/edição. |
| **AppDialogHost** / **AppToastStack** | — | Hosts globais de diálogo e de toasts (notificações empilhadas). |
| **AppEntityGrid** / **OmniEntityDrawer** | — | Grade e drawer genéricos de entidade (CRUD). |
| **AppInfoPopover** | — | Popover de ajuda/informação. |

Exemplo de uso do botão:

```vue
<AppPanelButton variant="primary" block @click="salvar">Salvar</AppPanelButton>
<AppPanelButton variant="ghost" :disabled="carregando">Cancelar</AppPanelButton>
```

---

## 8. Padrões de componente (classes semânticas)

As classes seguem um **BEM frouxo** (`bloco__elemento--modificador`) e consomem só tokens. As mais
reutilizadas em [components.css](../web/app/assets/styles/components.css):

**Contêineres**
- `.admin-panel` — painel padrão de seção (borda `--line-soft`, `radius 18px`, `--shadow-card`).
- `.workspace` / `.page-workspace` — wrapper de página (largura máx. 1900px, padding 16px). Ver
  [layout.css](../web/app/assets/styles/layout.css). Operação usa `.workspace-host`/`.queue-grid`.
- `.settings-card`, `.metric-card`, `.insight-card`, `.ranking-card` — cards de conteúdo.

**Operação (fila)**
- `.queue-column`, `.queue-card` (+`--next`), `.service-card` (+`--alert-active`), `.employee` strip.
- Timer em destaque: `.service-card__timer` (cor `--accent-focus`, 1.2rem, peso 800).

**Botões / ações**
- `.app-panel-button` (componente), `.column-action--primary/secondary`, `.summary-action`.
- Gradiente primário canônico: `linear-gradient(135deg, rgb(var(--primary)) 0%, rgb(var(--primary-600)) 100%)`.

**Badges / pills** (todos com `border-radius: 999px`)
- `.summary-pill`, `.service-card__status-badge`, `.intel-badge--{critical|attention|healthy}`,
  `.campaign-status--{ativa|aguardando|encerrada|inativa}`, `.report-quality-badge--*`.

**Navegação / abas**
- `.workspace-nav__button` (+`--active`), `.settings-tabs__btn` (`.is-active`),
  `.admin-selector__button` (+`--active`). Ativo = gradiente primário, texto branco.

**Feedback de progresso**
- `.progress-bar` + `.progress-bar__fill` (fill com gradiente primário, largura via `--progress`).
- `.dist-bar-row` (barras de distribuição em relatórios).

**Avatar**
- `.queue-card__avatar` / `.employee__avatar` — círculo `--avatar-accent`, texto branco, iniciais.

---

## 9. Componentes Nuxt UI (`U*`)

Disponíveis globalmente (`@nuxt/ui`). Usados sobretudo em telas novas. Exemplos no código:
`UButton`, `UIcon`, `UPopover`, `UModal`, `UInput`. Convenções vistas:

- `color="neutral"`, `variant="ghost"`, `size="sm"` no header.
- Ícones sempre Lucide (`i-lucide-*`).
- O tema do Nuxt UI **não** foi remapeado por `app.config.ts` (só define `locale`); o casamento
  visual com o resto do painel vem dos tokens CSS globais e de overrides pontuais.

> Ao escolher entre **Nuxt UI** e **classe própria**: para algo que já existe como classe semântica
> (cards de operação, badges de status), use a classe; para overlays/inputs/popovers genéricos,
> prefira o componente Nuxt UI.

---

## 10. Convenções (checklist ao codar UI)

1. **Cor sempre por token**: `rgb(var(--token) / alpha)`. Sem hex solto (salvo a tela de auth, que
   é exceção histórica).
2. **Texto**: `--text-main` / `--text-muted`. **Fundo**: `--bg-panel` / `--bg-muted`. **Borda**:
   `--line-soft`.
3. **Radius e sombra**: use os tokens (`--radius-card`, `--shadow-card`), não valores mágicos.
4. **Status** segue a convenção da §3.4 (fundo a ~12% + texto cheio).
5. **Pílulas** = `border-radius: 999px`. **Cards** = `--radius-md/lg`.
6. **Ícone novo** = Lucide via `UIcon`; legado = `.material-icons-round`.
7. **Botão** = `AppPanelButton` (ou `UButton`), nunca `<button>` cru estilizado do zero.
8. **Dark é o tema padrão** — teste qualquer tela nos dois (claro/escuro) antes de fechar.
9. **Espelhamento modal ⇄ card**: mudou um, replique no outro (regra de produto).
10. **Arquivo de CSS por componente** quando a folha global crescer demais; o limite de ~450 linhas
    por arquivo dos princípios de engenharia vale aqui também.

---

## 11. Arquivos-fonte (onde editar)

| Quero mudar… | Edite |
| --- | --- |
| Uma cor / raio / sombra base | [web/app/assets/styles/omni-tokens.css](../web/app/assets/styles/omni-tokens.css) |
| Um alias semântico (`--text-main`…) | [web/app/assets/styles/tokens.css](../web/app/assets/styles/tokens.css) |
| Reset / body / scrollbar / foco | [web/app/assets/styles/base.css](../web/app/assets/styles/base.css) |
| Grid/shell de página | [web/app/assets/styles/layout.css](../web/app/assets/styles/layout.css) |
| Uma classe de componente | [web/app/assets/styles/components.css](../web/app/assets/styles/components.css) |
| Modal de tarefas | [web/app/assets/styles/tasks-modal.css](../web/app/assets/styles/tasks-modal.css) |
| Modo apresentação/TV | [web/app/assets/styles/presentation.css](../web/app/assets/styles/presentation.css) |
| Botão / switch / select base | [web/app/components/ui/](../web/app/components/ui/) |
| Temas e troca de tema | [omni-tokens.css](../web/app/assets/styles/omni-tokens.css) + [DashboardThemeSwitcher.vue](../web/app/components/dashboard/DashboardThemeSwitcher.vue) |
| Ícones / fontes / CSS global | [web/nuxt.config.ts](../web/nuxt.config.ts) |

---

_Gerado a partir da leitura direta do código em `web/`. Mantenha sincronizado ao alterar tokens ou
componentes base._
