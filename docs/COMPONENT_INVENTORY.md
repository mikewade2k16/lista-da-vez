# Inventario de Componentes e Design System

> Fase 10 do roadmap. Documento vivo: cada bloco executado deve atualizar este arquivo e a pagina `/roadmap`.

## Status

- Data de inicio: 2026-05-12
- Data de encerramento da Fase 10: 2026-05-12
- Fonte de referencia: `web-reference/`
- Estado atual: inventario funcional concluido e Fase 10 encerrada. Nuxt UI foi escolhido como base das paginas importadas; a ordem dos modulos agora e pendencia da Fase 6.
- Regra decidida: o front atual permanece como esta por enquanto. As paginas novas trazidas do `web-reference/` entram com o visual delas dentro do layer do modulo correspondente. A atualizacao visual das paginas atuais fica para uma revisao posterior.

## Decisoes da Fase 10

1. Nao substituir agora componentes ja usados no produto atual (`selects`, tabelas, modais, paginas de clientes/usuarios, etc.).
2. Usar o design system do `web-reference/` como referencia para os novos modulos.
3. Tratar a pagina de temas do `web-reference/` como parte central da analise, nao como detalhe cosmetico.
4. Portar componentes para `web/layers/core/components/` com prefixo `Core` somente quando forem realmente compartilhados pelo shell ou por multiplos modulos.
5. Componentes especificos continuam no layer do modulo (`Finance*`, `Tasks*`, `Omni*`, etc.).
6. Depois da migracao dos modulos, revisar pagina por pagina: permanece atual, sera removida/deprecada, ou recebera update para o design system novo.

## Estrutura Encontrada

- `web-reference/app/components/`: 63 componentes Vue.
- `web-reference/app/pages/`: 35 paginas Vue.
- `web-reference/app/composables/`: composables de dominio, tema, admin, BFF e omnichannel.
- `web-reference/app/types/`: tipos de dominio para clients, finances, tasks, omni, indicators, products, leads, users, qa, training etc.
- `web-reference/server/`: BFF/repositorios mock ou adaptadores para os dominios do front de referencia.

Diretorios principais de componentes:

| Diretorio | Papel |
|---|---|
| `admin` | Header, page header, auth shell, simulacao de sessao, finance cards. |
| `inputs` | Inputs/selects ricos baseados em Nuxt UI. |
| `omni` | Primitivos reutilizaveis: tabela, filtros, switch, money input, popover. |
| `omnichannel` | Modulo e subcomponentes de inbox/auditoria/operacao. |
| `indicators` | Dashboards, governance, configuracao e tabelas de avaliacao. |
| `manager/clients` | Popovers e toolbar da gestao de clientes. |
| `theme` | Theme Studio e input de cor. |
| `loading` | Loading shell proprio do front de referencia. |
| `ui` | Wrappers pequenos herdados (`AppSelectField`, `AppToastStack`, etc.). |

## Dependencias

O app atual (`web/`) ainda nao tem varias dependencias que o `web-reference/` usa. Migrar paginas do outro front exige decidir e adicionar essas dependencias de forma controlada.

| Dependencia | Uso no `web-reference` | Decisao |
|---|---|---|
| `@nuxt/ui` | Base de UI (`UButton`, `UInput`, `USelectMenu`, `UModal`, `UPopover`, `UCheckbox`, `USwitch`, etc.). | Necessaria para paginas novas. Entrar junto com primeiro modulo portado, sem reescrever paginas atuais. |
| `tailwindcss` 4 | Utility classes e tokens `rgb(var(--...))`. | Necessaria se preservarmos o visual das paginas novas. Nao converter CSS atual agora. |
| `@nuxt/icon` + `@iconify-json/lucide` | Icones `i-lucide-*` usados pelo Nuxt UI. | Necessaria para paginas novas. App atual usa `lucide-vue-next`; conviver inicialmente. |
| `@nuxt/fonts` | Fontes do Theme Studio (`Inter`, `Montserrat`, `Manrope`, `Poppins`, `Nunito Sans`). | Desejavel para Theme Studio; pode entrar quando tema for portado. |
| `@nuxt/image` | Imagens/otimizacao do front de referencia. | Avaliar por pagina; nao parece bloqueador imediato para finance/tasks. |
| `socket.io-client` | Realtime de tenant/omni. | Necessaria para omni e telas que dependem de realtime do outro front. |
| `xlsx`, `pdf-lib`, `qrcode`, `markdown-it`, `emoji-mart` | Ferramentas, docs, QR, exportacao e chat. | Entram apenas com modulos que usarem. |
| `ioredis` | Server/BFF do front de referencia. | Nao entra no browser; avaliar no backend/BFF se trouxermos server routes. |

## Design System e Temas

Arquivos centrais:

| Path | Classificacao | Observacao |
|---|---|---|
| `web-reference/app/assets/css/main.css` | `design-system` | Entrada global: importa Tailwind, Nuxt UI e `tokens.css`. |
| `web-reference/app/assets/css/tokens.css` | `design-system` | Define tokens light, dark, apple-blue e custom; inclui typography, radius, shadows, surfaces, accents, admin header e page header. |
| `web-reference/app/composables/useOmniTheme.ts` | `design-system` | Gerencia tema atual, labels, defaults, overrides, persistencia e injecao de CSS variables por seletor (`:root`, `.dark`, `.theme-apple-blue`, `.theme-custom`). |
| `web-reference/app/composables/useThemeStudio.ts` | `design-system` | Estado e regras da tela de edicao de temas: modo simples/detalhado, filtros, fontes, radius, cores, header e page header. |
| `web-reference/app/composables/useAdminPageHeaderVisibility.ts` | `design-system` | Le tokens de visibilidade do page header e decide se eyebrow/title/description aparecem. |
| `web-reference/app/pages/admin/themes.vue` | `design-system-page` | Pagina Theme Studio; conecta controles, painel simples e grid detalhado. |
| `web-reference/app/components/theme/ThemeColorInput.vue` | `design-system-component` | Editor compacto de cor/alpha/gradiente com suporte a EyeDropper. |
| `web-reference/app/components/theme/studio/ThemeStudioHeaderControls.vue` | `design-system-component` | Selecao/aplicacao/reset/criacao de tema custom e busca de tokens. |
| `web-reference/app/components/theme/studio/ThemeStudioSimplePanel.vue` | `design-system-component` | Editor simplificado de fontes, radius, shadows, acentos, header e page header. |
| `web-reference/app/components/theme/studio/ThemeStudioDetailedGrid.vue` | `design-system-component` | Editor detalhado por grupos de token. |

### Estrategia de Integracao dos Tokens

Decisao da Fase 10:

- Nao trocar agora os tokens globais do front atual (`--text-main`, `--text-muted`, `--panel-bg`, etc.).
- Nao importar `web-reference/app/assets/css/main.css` globalmente no app atual antes do primeiro modulo novo, porque isso traria Tailwind/Nuxt UI e poderia afetar o bundle/estilo do produto de fila.
- Quando o primeiro modulo novo for portado, criar uma entrada controlada para o design system do `web-reference`:
  - adicionar dependencias Nuxt UI/Tailwind/Icon no `web/package.json`;
  - importar os tokens novos de forma global, mas mantendo compatibilidade com os tokens atuais;
  - criar uma ponte CSS inicial, por exemplo:
    - `--text-main: rgb(var(--text))` somente se a tela estiver no escopo novo, ou
    - mapear componentes novos para `--text`, `--muted`, `--surface`, sem mexer nos componentes atuais.
- O Theme Studio virou pre-requisito da proxima leva: deve estabilizar tokens/tema antes de continuar a portar paginas Nuxt UI.

Atualizacao em 2026-05-12:

- Decisao confirmada: vamos usar Nuxt UI como base das paginas/componentes trazidos do front de referencia.
- A documentacao para LLMs ja esta local em `web-reference/Nuxt-ui-llms/llms.txt` e `web-reference/Nuxt-ui-llms/llms-full.txt`.
- `@nuxt/ui`, `@nuxt/icon`, `@iconify-json/lucide` e `tailwindcss` ja foram adicionados ao `web/package.json`, mas a ativacao global no `nuxt.config.ts` deve acompanhar o primeiro modulo escolhido.
- A entrada CSS preparada ficou em `web/app/assets/styles/omni-design-system.css`, com tokens em `web/app/assets/styles/omni-tokens.css`; ela so deve ser ligada quando a primeira pagina importada realmente entrar.

### Pontes de Token Provaveis

| Token atual | Token do `web-reference` | Decisao |
|---|---|---|
| `--text-main` | `rgb(var(--text))` | Criar ponte apenas se algum componente atual precisar conviver dentro do shell novo. |
| `--text-muted` | `rgb(var(--muted))` | Mesmo criterio acima. |
| fundos de card/painel atuais | `rgb(var(--surface))`, `rgb(var(--surface-2))` | Nao substituir agora; usar nas paginas novas. |
| bordas atuais | `rgb(var(--border))` | Usar nas paginas novas. |
| acento atual | `rgb(var(--primary))` | Manter separado ate a revisao visual ampla. |
| radius atuais | `--radius-xs/sm/md/lg` | Aceitar nas paginas novas; nao reprocessar cards atuais. |

## Componentes Base Candidatos

| Componente | Classificacao | API publica | Destino provavel | Decisao |
|---|---|---|---|---|
| `inputs/OmniSelectMenuInput.vue` | `candidate-core` | Props: `modelValue`, `items`, `multiple`, `creatable`, `searchable`, `loading`, `disabled`, `clear`, `size`, `color`, `variant`, `badgeMode`, `optionEditMode`, `showAvatar`; emits: `update:modelValue`, `create`, `update:open`. | `web/layers/core/components/CoreSelectMenuInput.vue` no futuro. | Forte candidato, mas depende de Nuxt UI. Nao substituir `AppSelectField` agora. |
| `inputs/OmniSelectInput.vue` | `candidate-core` | Props: `modelValue`, `items`, `multiple`, `creatable`, `searchInput`, `clear`, `manageOptions`, `overlayOnOpen`; emits: `update:modelValue`, `create`. | A decidir. | Mais especifico para tags/chips coloridos. Pode ficar como helper de tabela antes de virar Core. |
| `omni/table/OmniDataTable.vue` | `candidate-core` | Props: `rows`, `columns`, `viewerUserType`, `rowKey`, `loading`, `emptyText`, `selectable`, `modelValue`, `focusCell`; emits: `update:modelValue`, `update:cell`, `row-action`, `upload:image`; slot: `cell-<column.key>`. | `CoreDataTable` no futuro ou componente compartilhado `OmniDataTable` em layer comum. | Candidato importante para paginas novas. Nao substituir `AppEntityGrid` agora. |
| `omni/table/OmniTableColumnsConfig.vue` | `candidate-core` | Props: `columns`, `modelValue`, `excludeKeys`, `label`; emit: `update:modelValue`. | Junto da tabela. | Deve migrar junto com `OmniDataTable`. |
| `omni/filters/OmniCollectionFilters.vue` | `candidate-core` | Props: `modelValue`, `filters`, `viewerUserType`, `tableColumns`, `visibleColumns`, `columnExcludeKeys`, `showColumnFilter`, `loading`, `showReset`; emits: `update:modelValue`, `update:visibleColumns`, `reset`; slots: `actions`, `below`. | Junto da tabela. | Bom para telas de manager/listagem. |
| `omni/overlay/OmniMinimalPopover.vue` | `candidate-core` | Props de abertura/titulo/tamanho/foco/atalhos; emits de `update:open`, submit/cancel shortcut; slots: `trigger`, `header`, default, `footer`. | Core ou helper compartilhado. | Candidato, mas avaliar se Nuxt UI `UPopover/UModal` ja cobre no projeto atual. |
| `omni/inputs/OmniSwitchInput.vue` | `candidate-core` | Props: `modelValue`, `disabled`, `loading`, `ariaLabel`, `checkedIcon`, `uncheckedIcon`; emit: `update:modelValue`. | Core se Nuxt UI entrar. | Simples wrapper de `USwitch`; migrar quando necessario. |
| `omni/inputs/OmniMoneyInput.vue` | `finance-first` | Props: `modelValue`, `placeholder`, `disabled`; emit: `update:modelValue`. | Comeca em `finance`, pode virar Core depois. | Primeiro uso forte e no financeiro. |
| `admin/AdminPageHeader.vue` | `design-system-component` | Props: `eyebrow`, `title`, `description`; usa tokens de visibilidade. | Portado para `web/layers/core/components/admin/AdminPageHeader.vue`. | Base para paginas importadas. |
| `admin/AdminHeader.vue` | `shell-component` | Props: logo, menu, actions, profile, slideover, theme toggle; usa `useOmniTheme`. | Nao portar direto agora. | O shell atual ja tem sidebar/dashboard. Aproveitar ideias, nao substituir. |
| `loading/AppPageLoadingShell.vue` | `legacy-overlap` | Loading shell do outro front. | A decidir. | Sobrepoe Fase 9 (`CoreLoadingOverlay`, `CoreSkeleton`). Nao portar automaticamente. |

## Componentes Especificos por Modulo

### Finance

| Componente | API publica | Destino |
|---|---|---|
| `admin/finance/FinanceLineCard.vue` | Props de linha financeira, categoria, estados de popover, drafts e formatadores; emits para persistencia, efetivacao, data, ajuste, historico, remocao. | `web/layers/finance/components/FinanceLineCard.vue` |
| `admin/finance/FinanceRecurringGroupCard.vue` | Props de grupo recorrente, valores, lojas e formatadores; emits de efetivacao/data do grupo e filhos. | `web/layers/finance/components/FinanceRecurringGroupCard.vue` |
| `omni/inputs/OmniMoneyInput.vue` | Usado fortemente em financeiro. | Comecar em `finance`; promover depois se outro modulo precisar. |

Status em 2026-05-12:

- Finance nao sera o primeiro modulo importado.
- O placeholder atual de `/finance` permanece por enquanto.
- Os componentes Finance continuam inventariados para quando a ordem dos modulos chegar nele.

### Tasks

| Arquivo | Papel | Destino |
|---|---|---|
| `pages/admin/tasks.vue` | Workspace board/table com projetos, status configuraveis, filtros e editor de tarefa. | `web/layers/tasks/pages/index.vue` |
| `composables/useTasksWorkspace.ts` | Estado e operacoes do workspace. | `web/layers/tasks/composables/useTasksWorkspace.ts` |
| `types/tasks.ts` | Tipos de task/project/priority. | `web/layers/tasks/types.ts` ou `types/tasks.ts` do layer. |
| `OmniDataTable`, `OmniSelectMenuInput` | Base visual da tabela/configuracao. | Compartilhado ou duplicado temporariamente no layer ate Core existir. |

### Omni / Omnichannel

| Grupo | Papel | Destino |
|---|---|---|
| `pages/admin/omnichannel/*.vue` | Rotas de operacao, inbox, docs e auditoria. | `web/layers/omni/pages/` |
| `components/omnichannel/OmnichannelInboxModule.vue` | Modulo principal do inbox carregado async. | `web/layers/omni/components/` |
| `components/omnichannel/inbox/*.vue` | Chat, composer, sidebar, anexos, audio, reactions, contato, modal de sessao. | `web/layers/omni/components/inbox/` |
| `composables/omnichannel/*.ts` | Pipeline completo do inbox/admin/auditoria/realtime. | `web/layers/omni/composables/` |
| `socket.io-client`, `emoji-mart` | Dependencias funcionais. | Entram com o modulo omni. |

### Indicators

| Grupo | Papel | Destino provavel |
|---|---|---|
| `pages/admin/indicadores/*` | Operacao e configuracoes de indicadores. | A decidir: `analytics`, `crm` ou modulo proprio `indicators`. |
| `components/indicators/*.vue` | Dashboard, charts, governance, templates, provider health, toolbar. | Layer do modulo escolhido. |
| `composables/useIndicators*.ts` | Estado, dados, configuracao e live. | Layer do modulo escolhido. |

### Manager / Core Admin

| Arquivo | Papel | Decisao |
|---|---|---|
| `pages/admin/manage/clientes.vue` | Gestao de clientes/root admin com tabela editavel, lojas, contato e webhook. | `legacy-overlap`: pode substituir `/clientes` no futuro, mas nao agora. |
| `pages/admin/manage/users.vue` | Gestao de usuarios/root admin. | `legacy-overlap`: pode substituir `/usuarios` no futuro, mas nao agora. |
| `pages/admin/manage/modulos.vue` | Gestao de modulos do cliente. | Candidato a `core/account-modules` depois da base multi-tenant. |
| `components/manager/clients/*.vue` | Popovers e toolbar especificos de clientes. | Migrar somente se a pagina de clientes nova for escolhida. |

### Site / Tools / Team

| Grupo | Destino provavel | Decisao |
|---|---|---|
| `pages/admin/site/produtos.vue`, `pages/admin/site/leads.vue` | `web/layers/site/pages/` ou split `site`/`contacts`. | Avaliar quando trouxer site/bio/contacts. |
| `pages/admin/tools/*.vue` | `tools` ou modulos especificos (`qrcodes`, `short-links`, `scripts`). | Nao entra agora. |
| `pages/admin/team/*.vue` | A decidir. | Fora do escopo imediato da Fase 6 original. |

## Inventario de Paginas

| Pagina no `web-reference` | Classificacao | Destino provavel | Decisao |
|---|---|---|---|
| `admin/finance.vue` | `module-page` | `web/layers/finance/pages/finance.vue` | Nao sera o primeiro modulo; importar quando a ordem chegar em Finance. |
| `admin/tasks.vue` | `module-page` | `web/layers/tasks/pages/index.vue` | Importar com visual original quando tasks entrar. |
| `admin/omnichannel/inbox.vue` | `module-page` | `web/layers/omni/pages/inbox.vue` | Importar com modulo omni. |
| `admin/omnichannel/operacao.vue` | `module-page` | `web/layers/omni/pages/operacao.vue` | Importar com modulo omni. |
| `admin/omnichannel/auditoria.vue` | `module-page` | `web/layers/omni/pages/auditoria.vue` | Importar com modulo omni. |
| `admin/omnichannel/docs.vue` | `module-page` | `web/layers/omni/pages/docs.vue` | Importar se docs fizerem parte do produto. |
| `admin/manage/clientes.vue` | `legacy-overlap` | A decidir | Nao substituir `/clientes` atual agora. |
| `admin/manage/users.vue` | `legacy-overlap` | A decidir | Nao substituir `/usuarios` atual agora. |
| `admin/manage/modulos.vue` | `core-admin` | `core` futuro | Bom encaixe para account/modules depois da base RBAC. |
| `admin/themes.vue` | `design-system-page` | `core/design-system` futuro | Portar so apos Nuxt UI/Tailwind estabilizados. |
| `admin/indicadores/index.vue` | `module-page` | `indicators`/`analytics`/`crm` | Decisao de dominio pendente. |
| `admin/indicadores/configuracoes.vue` | `module-page` | `indicators` | Decisao de dominio pendente. |
| `admin/site/produtos.vue` | `module-page` | `site` | Importar quando site/e-commerce entrar. |
| `admin/site/leads.vue` | `module-page` | `site` ou `contacts` | Decidir junto com modelagem de leads/contacts. |
| `admin/tools/qr-code.vue` | `module-page` | `tools`/`qrcodes` | Futuro. |
| `admin/tools/encurtador-link.vue` | `module-page` | `tools`/`short-links` | Futuro. |
| `admin/tools/scripts.vue` | `module-page` | `tools`/`scripts` | Futuro. |
| `admin/team/treinamento.vue` | `module-page` | A decidir | Futuro. |
| `admin/team/candidatos.vue` | `module-page` | A decidir | Futuro. |
| `admin/containers.vue` | `admin-tooling` | A decidir | Monitoramento interno; nao prioritario. |
| `admin/login.vue`, `admin/recuperar-senha.vue`, `admin/profile.vue`, `admin/settings.vue` | `shell/auth` | A decidir | App atual ja tem auth/perfil/settings; nao substituir agora. |

## Decisao por Pagina Atual do Produto

| Pagina atual | Decisao agora | Possivel futuro |
|---|---|---|
| `/clientes` | Manter atual. | Pode ser substituida por `admin/manage/clientes.vue` ou por modulo `contacts`, apos migracao dos modulos. |
| `/usuarios` | Manter atual. | Pode receber update da pagina de users do front de referencia depois do RBAC/account modules estabilizar. |
| `/finance` | Manter placeholder atual por enquanto. | Importar `admin/finance.vue` para `layers/finance` quando Finance for escolhido na ordem. |
| `/tasks` | Placeholder atual pode ser substituido pelo modulo tasks novo. | Importar `admin/tasks.vue` para `layers/tasks`. |
| `/omnichannel` | Placeholder atual pode ser substituido pelo modulo omni novo. | Importar paginas `admin/omnichannel/*`. |
| `/crm`, `/erp` | Seguem a estrategia da Fase 8. | Nao misturar com pagina de clientes do front de referencia ainda. |
| `/operacao`, `/alertas`, `/ranking`, `/relatorios`, `/feedback`, `/configuracoes`, etc. | Manter visual atual. | Revisao visual so depois da migracao completa. |
| `/roadmap` | Pode evoluir localmente. | Nao depende do design system novo. |

## Fases Criadas Depois da Fase 10

A execucao dos modulos foi quebrada em fases proprias no `roadmap.md` e na pagina `/roadmap`:

| Fase | Modulo | Fonte principal no `web-reference` | Observacao |
|---|---|---|---|
| Fase 11 | `theme-studio` | `pages/admin/themes.vue`, `useOmniTheme.ts`, `useThemeStudio.ts`, `components/theme/**` | Concluida em 2026-05-12; `/themes` dev/admin estabiliza tokens e shell antes dos modulos. |
| Fase 12 | `tasks-orchestrator` | `pages/admin/tasks.vue`, `useTasksWorkspace.ts`, `types/tasks.ts` | Nome inicial `Tasks`, mas escopo virou orquestrador notion-like; ver `docs/TASKS_ORCHESTRATOR_PHASE12.md`. |
| Fase 13 | `omni` | `pages/admin/omnichannel/*`, `components/omnichannel/**`, `composables/omnichannel/**` | Maior fase; traz realtime, inbox e dependencias extras. |
| Fase 14 | `finance` | `pages/admin/finance.vue`, `components/admin/finance/*`, `OmniMoneyInput.vue` | Nao comeca primeiro; substitui placeholder `/finance` quando chegar sua vez. |
| Fase 15 | `contacts/admin` | `pages/admin/manage/clientes.vue`, `users.vue`, `modulos.vue` | Nao substitui `/clientes` ou `/usuarios` antes de decisao explicita. |
| Fase 16 | `site` | `pages/admin/site/produtos.vue`, `leads.vue` | Pode integrar leads com `contacts`. |
| Fase 17 | `indicators` | `pages/admin/indicadores/*`, `components/indicators/**` | Destino de dominio ainda precisa ser decidido: proprio, analytics ou CRM. |
| Fase 18 | `tools` | `pages/admin/tools/*`, `useShortLinksManager.ts` | Pode virar modulo unico ou ferramentas menores. |
| Fase 19 | `team` | `pages/admin/team/treinamento.vue`, `candidatos.vue` | Depende de decisao de produto. |
| Fase 20 | `bio` | Nao encontrado como pagina concreta no inventario atual. | Fase de descoberta antes de implementar. |

O teste inicial de `/tasks` mostrou que o Theme Studio nao pode ficar para depois: Nuxt UI e os componentes importados precisam dos tokens e do gerenciador de tema antes da proxima pagina de modulo. Paginas atuais sobrepostas (`clientes`, `usuarios`, etc.) so devem ser revisadas depois dos modulos principais entrarem.

## Encerramento da Fase 10

A Fase 10 termina como fase de inventario, decisao e preparacao. Ela nao porta componentes ativos para o app atual por conta propria.

O trabalho de migrar componentes especificos foi transferido para a Fase 6: cada PR de modulo deve trazer apenas os componentes necessarios daquele modulo, registrar o destino neste documento e promover algo para `Core*` somente quando houver reuso real em mais de um modulo ou necessidade clara do shell/design system.

## Riscos e Guardrails

- **Risco: Nuxt UI/Tailwind afetar o app atual.** Mitigacao: entrar junto com modulo novo e validar visual das telas atuais antes de qualquer refactor.
- **Risco: componentes candidatos virarem Core cedo demais.** Mitigacao: manter no layer do modulo ate aparecer segundo uso real.
- **Risco: tokens globais conflitarem.** Mitigacao: nao renomear tokens atuais; usar tokens novos nas paginas novas e criar pontes pontuais.
- **Risco: BFF/server do `web-reference` duplicar backend Go.** Mitigacao: portar UI/composables com cuidado e trocar chamadas para APIs Go quando o modulo backend existir.
- **Risco: paginas de manager substituirem CRUDs atuais antes da hora.** Mitigacao: marcar como `legacy-overlap` e manter fora da primeira leva.

## Proximos Passos na Fase 6

1. Retomar a Fase 12 (`tasks-orchestrator`) usando o Theme Studio ja portado e o plano em `docs/TASKS_ORCHESTRATOR_PHASE12.md`.
2. Portar componentes compartilhados minimos junto do modulo, sem promover para `Core*` ate validar reuso.
3. Atualizar este documento e a pagina `/roadmap` em cada PR de modulo.
