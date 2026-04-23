# Arquitetura de stores no Nuxt 4

## Objetivo

Esta fase organiza o frontend para que o estado da aplicacao passe a morar no Nuxt 4, com stores Pinia por dominio e sem dependencia direta da antiga pasta `core/`.

O objetivo pratico e:

- quebrar o store monolitico em dominos menores;
- manter o estado SSR-safe;
- preparar e sustentar a troca do runtime operacional em memoria por API;
- concentrar regras de tela no frontend Nuxt, em vez de espalhar imports de `@core`.

## Base adotada

Seguimos o padrao recomendado pela documentacao oficial do Nuxt 4 e do modulo do Pinia:

- estado compartilhado precisa ser serializavel no SSR;
- stores devem ficar em `app/stores`;
- `storeToRefs()` deve ser usado quando precisarmos extrair refs de um store;
- utilitarios sem estado ficam em `app/utils`;
- regras de composicao de tela ficam em `app/composables`.

Referencias oficiais:

- Nuxt 4 State Management: https://nuxt.com/docs/4.x/getting-started/state-management
- Nuxt 4 `app/utils`: https://nuxt.com/docs/4.x/directory-structure/app/utils
- Pinia + Nuxt: https://pinia.vuejs.org/ssr/nuxt.html

## Estrutura atual

### `web/app/stores/app-runtime.ts`

Bridge de infraestrutura do runtime atual.

Responsabilidades:

- inicializar o runtime atual;
- hidratar estado inicial;
- sincronizar o runtime de compatibilidade em memoria;
- expor `state`, `ensure()`, `hydrate()` e `run()` para os stores de dominio.

Importante:

- ele e a unica ponte entre os stores Pinia e o runtime de compatibilidade;
- paginas e componentes nao devem consumir esse store diretamente;
- ele existe para permitir migracao gradual sem quebrar o app.

### `web/app/stores/dashboard.ts`

Facade de compatibilidade.

Responsabilidades:

- manter o contrato antigo enquanto a migracao acontece;
- delegar para `web/app/stores/app-runtime.ts`;
- evitar quebrar qualquer uso legado residual.

Importante:

- ele nao deve ser o ponto de entrada padrao para codigo novo;
- stores de dominio devem falar com `app-runtime` diretamente.

### `web/app/stores/dashboard/runtime/create-dashboard-runtime.ts`

Orquestrador fino do runtime de compatibilidade.

Responsabilidades:

- montar o runtime local a partir dos modulos internos;
- manter o contrato atual sem crescer de novo como arquivo monolitico;
- centralizar apenas `state`, `subscribe()`, `hydrate()` e o wiring das actions.

Os modulos internos agora vivem em:

- `web/app/stores/dashboard/runtime/shared.ts`
- `web/app/stores/dashboard/runtime/status.ts`
- `web/app/stores/dashboard/runtime/state.ts`
- `web/app/stores/dashboard/runtime/actions/*`

## Stores por dominio

Nesta fase, as telas passam a consumir stores menores:

- `workspace.ts`
  - shell global, perfil ativo, loja ativa e workspaces permitidos.
- `operations.ts`
  - fila, atendimento ativo, modal de encerramento e acoes operacionais.
- `settings.ts`
  - configuracoes, catalogos, opcoes de modal e cadastros auxiliares.
- `campaigns.ts`
  - campanhas comerciais e manutencao desse dominio.
- `consultants.ts`
  - foco na tela do consultor e simulacoes relacionadas.
- `analytics.ts`
  - leitura compartilhada para `dados`, `inteligencia` e `ranking`.
- `reports.ts`
  - filtros e acoes da tela de relatorios.
- `multistore.ts`
  - visao consolidada e gestao de lojas.

## Dominio temporariamente espelhado no frontend

Os utilitarios e seeds que ainda sustentam o MVP agora vivem em:

- `web/app/domain/data/*`
- `web/app/domain/utils/*`

Isso substitui o consumo historico de `core/data/*` e `core/utils/*` pela UI do Nuxt.

## Regra desta arquitetura

### Paginas

Paginas devem:

- importar um store de dominio;
- extrair somente refs necessarios com `storeToRefs()`;
- repassar estado para workspaces/componentes;
- evitar logica de negocio pesada.

### Workspaces e componentes

Workspaces devem:

- consumir actions do store de dominio correspondente;
- usar `props.state` para leitura serializavel;
- evitar importar o bridge `dashboard.ts` diretamente.

### Composables

Composables devem:

- encapsular regras de shell/navegacao;
- trabalhar com refs vindos de `storeToRefs()`;
- evitar guardar estado mutavel global fora de `setup()`.

## O que ja saiu do legado

- o alias `@core` foi removido do `nuxt.config.ts`;
- a UI do Nuxt nao depende mais de imports `@core/*`;
- as paginas principais ja consomem stores de dominio;
- os workspaces de `operacao`, `configuracoes`, `campanhas`, `relatorios` e `multiloja` ja foram desacoplados do consumo direto do store monolitico;
- a serializacao SSR foi corrigida para nao expor funcoes no payload.

## O que ainda e legado

- `web/app/stores/app-runtime.ts` ainda usa um runtime compativel com o store antigo;
- `web/app/stores/dashboard.ts` ainda existe como facade de compatibilidade;
- `web/app/stores/dashboard/runtime/actions/*` ainda existem por compatibilidade, mas `operations` ja envia comandos reais para a API;
- a pasta `core/` ja foi removida do caminho ativo do frontend e do repositorio;
- o runtime local ainda sustenta drafts e estado efemero de UI, mas a fila operacional principal ja vem do backend.

## Proxima etapa recomendada

### Fase 2

- manter o runtime interno fatiado por responsabilidade, sem voltar a crescer arquivos monoliticos;
- migrar os modulos de `runtime/actions/*` para stores/API reais quando cada dominio entrar no backend;
- reduzir `dashboard.ts` ate virar apenas alias fino de compatibilidade ou ser removido.

### Fase 3

- concluida para `operations`, `consultants` e `settings`;
- usar `$fetch`/`useAsyncData` onde fizer sentido para leitura;
- manter actions Pinia como fachada unica consumida pelas telas.

### Fase 4

- reduzir o runtime de compatibilidade ao minimo estritamente visual;
- introduzir websocket e resync em tempo real;
- remover o bridge monolitico restante quando os dominios auxiliares tambem sairem do legado local.

## Regra para novas implementacoes

Qualquer funcionalidade nova deve seguir este fluxo:

1. criar ou ampliar um store de dominio em `web/app/stores`;
2. colocar funcoes puras reaproveitaveis em `web/app/domain/utils` ou `web/app/utils`;
3. manter a pagina fina;
4. evitar reintroduzir imports ou aliases de legado no frontend;
5. pensar a action do store ja no formato futuro da API.
