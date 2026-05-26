# Web-reference -> web: pendencias reais do front

Atualizado em: 2026-05-25

Objetivo: listar o que ainda falta trazer do `web-reference/` para o front atual em `web/`, separando paginas ja migradas, paginas com sobreposicao parcial e paginas que hoje ainda sao apenas demo ou nao existem.

## Leitura rapida

- Heuristica mais forte: quando a rota atual usa `DemoWorkspacePage`, ela nao foi portada de verdade.
- Isso acontece hoje em `web/app/pages/finance.vue`, `web/app/pages/omnichannel.vue`, `web/app/pages/tracking.vue`, `web/app/pages/monitoramento.vue`, `web/app/pages/site/[area].vue`, `web/app/pages/team/[area].vue`, `web/app/pages/tools/[tool].vue` e nas areas ainda nao tratadas de `web/app/pages/manage/[area].vue`.
- `manage/users` deixou de ser demo puro e agora reaproveita a tela antiga de usuarios em modo admin, com cliente no grid; `/usuarios` segue na fila em modo interno, sem gestao de cliente e com resumo de modulos na tabela.
- Os itens escondidos em `web/layers/queue/nav.config.ts` tambem batem com modulos ainda pendentes: `omnichannel`, `tracking`, `finance`, `qr-code`, `encurtador de link`, `scripts` e a secao `team-site`.

## Ordem priorizada agora

- Base ja pronta: Theme Studio ja foi migrado e fica como base visual para as proximas entradas.
- Primeiro lote simples no roadmap: Profile, Team, Site e a frente nova de Users/Clientes/Admin.
- Regra de rollout: `/usuarios` e `/clientes` atuais continuam dentro de fila e nao devem ser substituidos agora; `/usuarios` virou a frente interna da fila, enquanto `/manage/users` e `/manage/clientes` seguem em paralelo para o recorte administrativo.
- Reuso esperado: users reaproveita a base atual sempre que possivel; clientes segue o plano tenant e so traz do `web-reference` o que ainda faltar.
- Modulos mais pesados ficam para depois do lote simples: Omni, Finance, Indicators e Tools.

## Status por bloco

| Bloco do `web-reference` | Paginas de referencia | Equivalente atual em `web/` | Status | O que ainda falta trazer |
| --- | --- | --- | --- | --- |
| Finance | `app/pages/admin/finance.vue` | `/finance` -> `web/app/pages/finance.vue` | nao migrado | Pagina real, autosave, configuracao, historico, recorrencias, cards financeiros, realtime e estilos proprios do modulo. |
| Omnichannel | `app/pages/admin/omnichannel/index.vue`, `inbox.vue`, `operacao.vue`, `auditoria.vue`, `docs.vue` | `/omnichannel` -> `web/app/pages/omnichannel.vue` | nao migrado | Rotas reais, inbox/chat/composer, auditoria, operacao, docs, realtime e dependencias do modulo. |
| Site | `app/pages/admin/site/produtos.vue`, `leads.vue` | `/site/[area]` -> `web/app/pages/site/[area].vue` | nao migrado | Produtos, leads, filtros, tabela editavel, upload de imagem, managers e estilos do modulo. |
| Tools | `app/pages/admin/tools/scripts.vue`, `qr-code.vue`, `encurtador-link.vue` | `/tools/[tool]` -> `web/app/pages/tools/[tool].vue` | nao migrado | Paginas reais, modais, tabelas, short-links, QR dinâmico, scripts e seus managers. |
| Team | `app/pages/admin/team/candidatos.vue`, `treinamento.vue` | `/team/[area]` -> `web/app/pages/team/[area].vue` | nao migrado | Paginas reais de candidatos/treinamento; o menu atual fala em `equipe` e `escalas`, mas segue em demo. |
| Manage extra | `app/pages/admin/manage/componentes.vue`, `qa.vue`, `modulos.vue`, `auditoria.vue` | `/manage/[area]` -> `web/app/pages/manage/[area].vue` | parcial | `manage/users` agora reaproveita a tela antiga de usuarios em modo admin; `manage/clientes` reaproveita workspace real; componentes, QA, modulos e auditoria seguem como casca/demo. |
| Indicadores / metricas | `app/pages/admin/indicadores/index.vue`, `configuracoes.vue` e `modules/fila-atendimento/runtime/app/pages/admin/fila-atendimento/metricas.vue` | workspaces espalhados + `/monitoramento` demo | parcial | O produto atual tem `consultor`, `ranking`, `dados`, `inteligencia` e `relatorios`, mas nao trouxe as paginas especificas de indicadores/metricas 1:1. |
| Settings admin | `app/pages/admin/settings.vue` | sem equivalente direto | nao migrado | Config de sessao admin, inventario de sessoes ativas, revoke/logout por tenant/plataforma. |
| Containers | `app/pages/admin/containers.vue` | sem equivalente direto | nao migrado | Pagina administrativa auxiliar ainda ausente. |
| Profile | `app/pages/admin/profile.vue` | `/perfil` -> `web/app/pages/perfil.vue` | parcial | O fluxo atual cobre avatar, perfil e senha, mas nao e a mesma pagina/estilo/fluxo admin-core da referencia. |
| Manage users | `app/pages/admin/manage/users.vue` | `/usuarios` + `/manage/users` -> `web/app/pages/usuarios.vue` e `web/app/pages/manage/[area].vue` | parcial | `/usuarios` agora opera em modo fila, listando apenas usuarios com acesso a operacao e exibindo modulos na grade; `/manage/users` reaproveita a tela antiga em modo admin com cliente visivel/editavel no grid. |
| Manage clientes | `app/pages/admin/manage/clientes.vue` | `/clientes` + `/manage/clientes` -> `web/app/pages/clientes.vue` e `web/app/pages/manage/[area].vue` | parcial | O front atual usa `TenantsWorkspace` e agora tambem exposto em `/manage/clientes`; a referencia ainda tem tabela editavel e popovers proprios que nao vieram 1:1. |
| Theme Studio | `app/pages/admin/themes.vue` | `/themes` -> `web/layers/core/pages/themes.vue` | migrado | Base visual e funcional do Theme Studio ja esta portada. |
| Tasks | `app/pages/admin/tasks.vue` | `/tasks` -> `web/layers/tasks/pages/tasks.vue` | migrado | Layer, pagina, composables e estilos ja foram portados/evoluidos. |
| Fila de atendimento core | `modules/fila-atendimento/runtime/app/pages/admin/fila-atendimento/operacao.vue`, `consultor.vue`, `dados.vue`, `inteligencia.vue`, `ranking.vue`, `relatorios.vue`, `campanhas.vue`, `configuracoes.vue`, `multiloja.vue`, `perfil.vue` | workspaces atuais de fila | migrado/evoluido | O produto atual ja absorveu esse nucleo com implementacao propria. |
| Fila diagnostico | `modules/fila-atendimento/runtime/app/pages/admin/fila-atendimento/diagnostico.vue` | sem equivalente direto | nao priorizado | Rota de smoke test/host do modulo, nao uma pagina principal de produto. |

## Componentes, composables e estilos que ainda faltam por modulo

### 1. Finance

- Pagina: `web-reference/app/pages/admin/finance.vue`
- Componentes: `web-reference/app/components/admin/finance/FinanceLineCard.vue`, `FinanceRecurringGroupCard.vue`
- Inputs e utilitarios: `web-reference/app/components/omni/inputs/OmniMoneyInput.vue`, `web-reference/app/utils/finance-ids.ts`
- Estado/composables: `web-reference/app/composables/useFinancesManager.ts`, `useFinancesConfigManager.ts`
- Tipos: `web-reference/app/types/finances.ts`
- Sinal claro de falta: `web/app/pages/finance.vue` ainda renderiza `DemoWorkspacePage`.

### 2. Omnichannel

- Paginas: `web-reference/app/pages/admin/omnichannel/*.vue`
- Modulo principal: `web-reference/app/components/omnichannel/OmnichannelInboxModule.vue`
- Subcomponentes: `web-reference/app/components/omnichannel/inbox/*`
- Composables: `web-reference/app/composables/omnichannel/useOmnichannelInbox.ts`, `useOmnichannelAdmin.ts` e varios auxiliares de inbox/realtime/auditoria
- Estilos: o bundle da referencia aponta CSS proprio grande para o inbox (`OmnichannelInboxModule*.css`), inexistente no `web/` atual
- Sinal claro de falta: `web/app/pages/omnichannel.vue` ainda e demo e o item segue `hidden: true` em `web/layers/queue/nav.config.ts`.

### 3. Site

- Paginas: `web-reference/app/pages/admin/site/produtos.vue`, `leads.vue`
- Composables: `web-reference/app/composables/useProductsManager.ts`, `useLeadsManager.ts`
- Tipos: `web-reference/app/types/products.ts`, `types/leads.ts`
- Funcionalidades: filtros, tabela editavel, upload de imagem, publicacao no site e listagem de leads
- Sinal claro de falta: `web/app/pages/site/[area].vue` usa apenas demo generico.

### 4. Tools

- Paginas: `web-reference/app/pages/admin/tools/scripts.vue`, `qr-code.vue`, `encurtador-link.vue`
- Composables: `useScriptsManager.ts`, `useQrcodesManager.ts`, `useShortLinksManager.ts`
- Tipos: `types/scripts.ts`, `types/qrcodes.ts`, `types/short-links.ts`
- Backend/bridge de referencia ligado ao modulo: `web-reference/server/routes/s/[slug].ts`
- Sinal claro de falta: `web/app/pages/tools/[tool].vue` usa demo e os itens do menu seguem ocultos.

### 5. Team

- Paginas: `web-reference/app/pages/admin/team/candidatos.vue`, `treinamento.vue`
- Composables: `web-reference/app/composables/useCandidatesManager.ts`, `useTrainingManager.ts`
- Tipos: `web-reference/app/types/candidates.ts`, `types/training.ts`
- Sinal claro de falta: o `web/` nao tem managers nem paginas reais para candidatos/treinamento.

### 6. Manage extra / admin support

- Paginas: `web-reference/app/pages/admin/manage/componentes.vue`, `qa.vue`, `modulos.vue`, `auditoria.vue`, `settings.vue`, `containers.vue`
- Composables/tipos principais: `useQaManager.ts`, `types/qa.ts`, `types/admin-session.ts`, `useAdminSession.ts`
- Sinal claro de falta: `web/app/pages/manage/[area].vue` e uma casca demo unica para tudo isso.

## O que ja veio de verdade

- Theme Studio: `web/layers/core/pages/themes.vue` ja replica a base de `web-reference/app/pages/admin/themes.vue`.
- Tasks: `web/layers/tasks/pages/tasks.vue` virou um layer proprio e ja evoluiu alem da referencia original.
- Fila core: operacao, consultor, dados, inteligencia, ranking, relatorios, campanhas, configuracoes e multiloja ja estao no produto atual com componentes proprios.

## Sobreposicoes parciais que precisam decisao, nao copia cega

- `admin/manage/users.vue` x `/usuarios`: hoje a base antiga atende dois contextos diferentes, com `/usuarios` em modo fila e `/manage/users` em modo admin.
- `admin/manage/clientes.vue` x `/clientes`: hoje existe CRUD de tenants/clientes, mas nao com o mesmo escopo da referencia.
- `admin/profile.vue` x `/perfil`: existe pagina funcional, mas nao com a mesma base visual/admin-core.
- `admin/indicadores/*` x workspaces atuais: parte do dominio foi absorvida por workspaces especificos, mas as paginas da referencia nao foram trazidas 1:1.

## Priorizacao sugerida

### Critico

1. Finance
2. Omnichannel

Motivo: ambos ainda sao placeholder/demo no front atual, estao no topo do valor de negocio e ja possuem superficie pronta na referencia para reaproveitar estilo e UX.

### Alto

1. Site (`produtos`, `leads`)
2. Tools (`scripts`, `qr-code`, `encurtador-link`)
3. Manage extra (`qa`, `modulos`, `auditoria`, `componentes`)
4. Indicadores / metricas

Motivo: ja ha paginas prontas no `web-reference`, mas no `web` atual ainda sao demos ou nem existem; trazem bastante funcionalidade administrativa de uma vez.

### Medio

1. Team (`candidatos`, `treinamento`)
2. Settings admin / containers
3. Paridade visual 1:1 de `clientes`, `usuarios` e `perfil`

Motivo: sao importantes, mas ou dependem de decisao de produto/escopo, ou ja possuem alguma cobertura funcional no front atual.

## Sequencia pratica recomendada

1. Portar `finance` como novo layer, substituindo o placeholder atual de `/finance`.
2. Portar `omnichannel` como layer proprio com `inbox` primeiro e depois `auditoria/operacao`.
3. Em seguida trazer `site` e `tools`, porque o ganho de paginas reais por esforco e alto.
4. Deixar `manage extra`, `team` e revisoes 1:1 das sobreposicoes para a ultima leva.