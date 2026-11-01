# Frontend

Leia antes de criar ou alterar componente, página, workspace ou estilo.

## Stack e tipos
- Composition API + `<script setup lang="ts">` em todo componente. Sem Options API.
- Props tipadas (`defineProps<{}>()`). Sem `any`. Preferir tipos explícitos em props e composables.
- Composables (`use*`) para lógica reutilizável; componentes só orquestram UI.
- Imports com alias de raiz (`~/`), nunca `../../../`.
- Código TS deve passar no type-check (`vue-tsc --noEmit` ou equivalente). Pega bug em build time, não em prod.
- Sem `v-html` exceto conteúdo sanitizado. Nunca interpolar input do usuário direto no DOM.

## Componentes reutilizáveis acima de tudo
Sempre que houver repetição de markup ou lógica visual, extrair em componente próprio na layer adequada. Workspaces não podem virar arquivos gigantes — quebrar em cards/seções/listas. Evita duplicação e drift visual; facilita aplicar mudança em um lugar só. Componentes atômicos: um componente faz uma coisa (componente de lista não faz fetch de detalhe).

## Classes semânticas (BEM-like)
Usar nomes semânticos no estilo `.nome-componente__elemento--modificador`. Não usar utility classes inline ou IDs para estilização. Permite ler rápido o escopo de cada estilo e evita colisão global.

## Design system — usar as variáveis de cor, nunca hex hardcoded
Se o projeto tem design system (tokens de cor/borda/sombra/raio), todo componente novo usa as variáveis existentes — **nunca** hex/rgb cravado e **nunca** inventar nome de variável.

- Antes de usar um token, conferir o nome real no arquivo de tokens. Se a cor que você precisa não existe como token, perguntar/adicionar token — não cravar hex.
- **Por quê:** hex hardcoded = componente que não acompanha tema (dark mode) nem rebranding. Um caso real: o CSS usava `var(--color-primary, #16a34a)` — esse nome `--color-*` não existia no design system, então caía sempre no fallback hex e o componente ignorava o tema (ficava claro fora do dark mode do resto do painel). Token inexistente cai no fallback silenciosamente.

## Layout compacto — horizontal, minimiza scroll vertical
Em editores/workspaces, preferir layout horizontal e denso a uma-propriedade-por-linha. Agrupar campos relacionados de um item na MESMA linha via grid responsivo (`repeat(auto-fit, minmax(...))`, que colapsa para coluna em tela estreita sozinho). Listas de itens repetidos ficam em grid de cards lado a lado, não empilhados. Barras de ação em uma linha: botões cujo ícone já transmite o significado ficam só com ícone (nome no hover via `title`/tooltip); label de texto só para a ação primária. Padding/altura enxutos.

## Blocos de edição colapsáveis (accordion) — REGRA OBRIGATÓRIA de página/aba admin
Toda página ou aba de configuração admin organiza seu conteúdo em blocos colapsáveis por **categoria** — nunca uma pilha vertical única que gera rolagem longa. O usuário abre só o bloco que vai mexer; vai direto no que quer.

- **Todos fechados por padrão.** Ao abrir a aba/página, nenhum collapse está expandido — só a lista de títulos (cada um com um resumo: contagem de itens, valor atual, status). O usuário escolhe o que abrir.
- **Uma categoria por collapse.** Agrupar campos/listas relacionados sob um único cabeçalho clicável. Categorias distintas = collapses distintos. Nada de despejar tudo aberto de uma vez.
- **Cada bloco guarda seu estado aberto/fechado** (idealmente persistido por aba, não resetando a cada navegação).
- **Cabeçalho sempre visível e informativo:** título + resumo + chevron. O resumo evita ter que abrir só para saber o que tem dentro.
- **Por quê:** com tudo aberto, o editor vira scroll infinito e o usuário perde tempo rolando atrás de um campo. Página admin densa e navegável > página comprida. Vale para editores, workspaces e QUALQUER aba de configuração — sem exceção. Ao criar/alterar uma aba de config, o padrão-base é o accordion fechado, não a lista vertical.

## Página nova precisa rolar (overflow do layout)
Se o layout envolve a página num container `overflow: hidden`, o componente-raiz da página precisa ser o container de rolagem: `flex: 1; min-height: 0; overflow-y: auto`. Sem isso, conteúdo que passa da altura fica cortado, sem scroll. Caso real: a raiz só tinha `padding`, sem `flex/overflow`, e o editor longo ficava cortado.

## Dropdown/popover/menu SEMPRE fecha no clique-fora e no Esc
Todo dropdown/popover/menu/seletor aberto por clique DEVE fechar quando: (1) clica fora dele, (2) seleciona uma opção/aperta um botão de dentro, (3) aperta `Esc`. Implementar com listener `pointerdown` no `document` que fecha se o alvo não está dentro do root (`rootRef.contains(target)`) + listener `keydown` para `Escape`, ambos removidos no unmount. Componentes prontos de UI já fazem isso — a regra vale para dropdown feito à mão (markup próprio + ref `open`). Ao entrar numa página com dropdown, verificar esse comportamento antes de considerar pronto.

## Cabeçalho de página admin via componente compartilhado (respeita toggle global)
Se há um toggle global para mostrar/esconder cabeçalhos de página, o eyebrow/título/descrição do topo de qualquer página admin tem que vir do componente compartilhado que consome esse toggle. Nunca renderizar cabeçalho à mão (markup próprio) numa página, senão o "desativar" não funciona nela e gera drift (umas páginas respeitam, outras não).

## Esconder página não-pronta
Quando um módulo/página não está pronto, escondê-lo do menu via flag (`hidden`) na config de navegação. Para beta, usar flag de beta (renderiza badge). Evita usuário navegar para página quebrada.

## Criar página nova — checklist ANTES (falhas silenciosas)
Os primeiros itens falham **sem erro de build nem type-check** — só aparecem testando a rota no browser (a tela "abre outra página" ou redireciona):

1. **Rota-pai engole a filha.** Se já existe o arquivo `pages/<x>.vue`, qualquer `pages/<x>/<y>.vue` vira rota-filha dele e só renderiza se o pai tiver `<NuxtPage/>`. Antes de criar a filha, conferir que não existe o arquivo pai — se existir, usar outro prefixo.
2. **Gating de página de módulo = PADRÃO OBRIGATÓRIO de workspace + módulo (não é "esconder do menu").** Toda página de um módulo contratável por conta (`core.account_modules`) é gateada em DUAS dimensões, e o PADRÃO do sistema (tasks, crm, meta-ads, automation, finance) tem **as duas**. Esconder do menu NÃO gateia — a URL direta abre a página. Nenhuma delas quebra o build se faltar; o buraco só aparece digitando a URL. Ao criar/gatear página de módulo, replicar o molde do `tasks` nos **6 pontos**:

   **Dimensão A — GATE DE PAPEL (workspace):** "este PAPEL pode ver esta tela?" Papel sem o workspace → volta pra `auth.homePath` (a home do usuário, ex.: `/operacao`). **É este o comportamento esperado** — voltar pra página onde estava, não pra `/perfil`.
   - **A1.** `WORKSPACES` em `web/app/utils/workspaces.ts` — `{ id, label, icon, path }`. Sem isso o id não é válido e `getWorkspacePath` cai no default `/operacao`.
   - **A2.** `WORKSPACE_ACCESS_DEFINITIONS` em `web/app/domain/utils/permissions.ts` — `{ id, label, description, viewPermission, editPermission }`. Módulo sem permissão fina de papel usa `viewPermission: ''` (igual tasks/finance): segue `ROLE_WORKSPACES` sem migration.
   - **A3.** `ROLE_WORKSPACES` (mesmo arquivo) — listar o id nos papéis que veem por padrão (tipicamente `platform_admin` + `owner`; NÃO em `store_terminal`, senão o terminal vê). Papel custom ganha via A4.
   - **A4.** `MODULE_WORKSPACE_PERMISSION_PREFIXES` (mesmo arquivo) — `id: 'prefixo.'` liga o workspace às permissões reais do módulo no banco (`core.permissions`), para papel CUSTOM criado no painel ver a tela ao receber `<modulo>.view`/`.manage`. Conferir que o módulo registra essas permissões no back (`<modulo>/module.go`).
   - **A5.** `definePageMeta({ workspaceId: '<id>' })` na página (e subpáginas). Isso ativa o gate em `auth.global.ts`. **NUNCA** usar `workspaceId: ''` para escapar do gate ("preview aberto") nem inventar um id que o back não concede (esconde de TODOS).

   **Dimensão B — GATE DE MÓDULO (conta):** "esta CONTA contratou o módulo?" Conta sem o módulo → `MODULE_GATED_FALLBACK_PATH` (`/perfil`, rota nunca-gated e sem workspace, evita loop). Roda ANTES do gate de papel.
   - **B1.** `{ prefix, moduleId }` em `MODULE_PATH_GUARDS` no `web/app/middleware/module-enabled.global.ts`. **É a que barra a URL direta por conta.**
   - **B2.** `{ Prefix, ModuleID }` em `moduleGatingRules()` (`back/internal/platform/app/app.go`) → `RequireModuleByPath` → 403 `module_disabled`. Isenta `platform_admin` (por isso o gate de rota do front é o que segura o admin em view-as).
   - **B3.** `moduleId` no item do `nav.config.ts` — some do menu por módulo. **Cosmético; não protege a rota.**

   Os mapas de cada dimensão são **espelhos** entre si (`WORKSPACES`/`ACCESS_DEFINITIONS`/`ROLE_WORKSPACES` de um lado; `MODULE_PATH_GUARDS`/`moduleGatingRules`/nav `moduleId` do outro) — criar/mover/renomear rota exige atualizar TODOS. Drift gera "menu esconde mas a URL abre" ou "vai pro lugar errado ao bloquear". Bug real: calendário nasceu como preview com `workspaceId: ''` e sem entrada no `MODULE_PATH_GUARDS` → abria pela URL e, quando barrado, ia pra `/perfil` em vez de `/operacao` (2026-07-13). Ver `registro-de-falhas.md`.
4. **Estático vence dinâmico.** Com `[param].vue` na pasta, a rota estática nova vence no path exato — mas confirmar que é a sua página que renderiza.

Depois dos checks, abrir a rota no browser pelo papel-alvo e confirmar que renderiza a página certa (não a do pai, nem um redirect).

## UX
- Mínimo de cliques: ação que cabe em 1 clique não deve exigir 2.
- Feedback visual imediato: botão desabilita + spinner no clique, não espera o servidor. Skeleton/loading em < 100ms; sem tela branca.
- Erros dizem o que fazer, não só o que deu errado ("E-mail inválido" > "Erro 422").
- Estados vazios com mensagem orientativa — nunca tabela em branco sem contexto.
- Toasts para feedback de ação, não `alert()` do browser.
- Mobile-first: o painel precisa ser usável em tela pequena.
- Requests cancelados se o usuário navega antes de completar (`AbortController`).
