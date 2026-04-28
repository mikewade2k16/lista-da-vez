# AGENTS

## Escopo

Estas instrucoes valem para todo o frontend dentro de `web/`.

## Stack atual

- Nuxt 4
- Vue 3
- Pinia
- Node 24.11.1 em container
- CSS local em `app/assets/styles/*`
- auth real via API Go + PostgreSQL

## Workflow oficial

O frontend agora sobe por padrao dentro do Docker Compose da raiz.

Pela raiz do repositorio:

```bash
npm run dev
```

No Compose, o `web` roda em modo dev com bind mount e hot reload.
Mudancas de layout, pagina, componente e CSS devem atualizar sem rebuild.

No browser:

- `web` responde em `http://localhost:3003`
- a API publica fica em `http://localhost:8080`

No SSR/container:

- o frontend usa `NUXT_API_INTERNAL_BASE`
- por padrao no Compose: `http://api:8080`

## Objetivo da fase atual

O legado de `core/` ja foi removido do frontend.

Toda implementacao nova em `web/` deve:

- nascer no Nuxt;
- usar stores por dominio em `app/stores`;
- evitar reintroduzir qualquer acoplamento legado no frontend;
- preparar a camada para futura integracao com API.

## Regras de arquitetura

### 1. Stores

- Cada area deve consumir seu store de dominio.
- Nao importar `useDashboardStore()` direto em paginas, components ou composables de tela.
- `web/app/stores/app-runtime.ts` e a ponte de infraestrutura do runtime atual.
- `web/app/stores/auth.ts` e a fonte de verdade da sessao autenticada.
- `web/app/stores/reports.ts` ja consome `GET /v1/reports/overview` e `GET /v1/reports/results` como fonte principal de `/relatorios`.
- `web/app/stores/analytics.ts` ja consome `GET /v1/analytics/ranking`, `GET /v1/analytics/data` e `GET /v1/analytics/intelligence`.
- `web/app/stores/multistore.ts` ja consome o CRUD real de lojas via API.
- `web/app/stores/multistore.ts` ja consome `GET /v1/reports/multistore-overview` como fonte principal do comparativo gerencial.
- `web/app/stores/users.ts` ja consome papeis e usuarios reais via API para a area de acessos.
- `web/app/stores/users.ts` ja trabalha com onboarding por convite e link de aceite.
- `web/app/stores/users.ts` ja expõe o papel `store_terminal` para o acesso fixo da unidade.
- `web/app/stores/access-control.ts` consome a matriz de acessos por perfil e os overrides individuais de usuario via `/v1/access/*`.
- `web/app/stores/feedback.ts` gerencia o fluxo de feedback dos usuarios: envio por qualquer usuario autenticado e gestao (listar, filtrar, atualizar) por administradores via `/v1/feedback`.
- `store_terminal` deve permanecer com workspace enxuta, operacao completa da propria loja e acesso apenas a telas seguras de leitura como `consultor`, `ranking`, `dados`, `inteligencia` e `relatorios`.
- `web/app/components/users/UsersAccessManager.vue` e a referencia atual de grade administrativa sem `<table>`, com filtros locais, colunas configuraveis, detalhes e edicao inline.
- `web/app/components/users/UsersRoleMatrixManager.vue` concentra a edicao do padrao de visibilidade/edicao por papel dentro da workspace de usuarios.
- `web/app/stores/consultants.ts` agora cria consultores ja com conta autenticada vinculada.
- `web/app/composables/useContextRealtime.ts` cuida da sincronizacao administrativa por tenant para atualizar lojas, usuarios e header entre instancias.
- `web/app/composables/useContextRealtime.ts` tambem precisa revalidar `web/app/stores/access-control.ts` quando chegar `context.updated` com `resource=access` ou `resource=user`, senao a workspace de Usuarios e acessos fica stale ate reload.
- `web/app/utils/api-client.ts` concentra o client HTTP criado dentro do contexto do store.
- `web/app/composables/useOperationsRealtime.ts` cuida da assinatura WebSocket da operacao e revalidacao do snapshot em tempo real.
- `web/app/composables/useOperationsRealtime.ts` agora tambem cuida do modo integrado multi-loja de `/operacao` quando a sessao tiver mais de uma loja acessivel.
- mudancas em `settings` devem continuar propagando sem refresh via `context.updated` com `resource=settings` e `resourceId={tenantId}`.
- `web/app/utils/runtime-remote.ts` hidrata consultores, settings e operations remotos para a loja ativa.
- toda chamada a `hydrateRuntimeStoreContext`, `refreshRuntimeStoreSettings` ou `fetchRemoteStoreData` deve repassar o `tenantId` (ex.: `auth.activeTenantId`); sem ele a query `tenantId` some e usuarios `platform_admin` com mais de um tenant acessivel recebem 400 `validation_error` em `/v1/settings`.
- `web/app/stores/dashboard.ts` e apenas uma facade temporaria de compatibilidade.
- O runtime de compatibilidade foi fatiado em `web/app/stores/dashboard/runtime/shared.ts`, `state.ts`, `status.ts` e `actions/*`.
- Se precisar tocar esse runtime, editar o menor modulo responsavel e nao voltar a concentrar regra em `create-dashboard-runtime.ts`.
- Se precisar de novo dominio, criar um store proprio em `app/stores`.

### 2. SSR e Pinia

- Estado exposto por store precisa ser serializavel.
- Nao retornar objetos com funcoes aninhadas dentro de propriedades de estado.
- Ao extrair refs de stores, usar `storeToRefs()`.
- Evitar ler store Pinia como se computed/ref desembrulhado ainda tivesse `.value` fora do contexto correto.

### 3. Organizacao de codigo

- `app/pages`: paginas finas, sem regra pesada.
- `app/components` e `app/features`: interface e comportamento de tela.
- `app/composables`: composicao de comportamento compartilhado da UI.
- `app/stores`: estado e actions por dominio.
- `app/domain/utils`: funcoes puras de negocio reutilizaveis no frontend.
- `app/domain/data`: defaults, templates e seeds locais enquanto a API nao entra.
- `app/utils`: helpers de app e infraestrutura de frontend.

### 4. Legado

- Nao criar novos imports de `@core/*`.
- Nao recriar alias legado no `nuxt.config.ts`.
- Se surgir codigo utilitario compartilhavel, ele deve nascer em `web/app/domain/*` ou `web/app/utils/*`.

### 5. Integracao futura com API

Ao criar ou ajustar actions:

- pensar no formato futuro de request/response;
- manter a action do store como fachada unica para a UI;
- evitar espalhar persistencia diretamente dentro de componentes;
- evitar persistencia client-side para fonte de verdade.
- `operations`, `consultants` e `settings` ja usam a API Go + PostgreSQL como fonte principal.
- `reports` ja usa a API Go como fonte principal para a tela `/relatorios`.
- `analytics` ja usa a API Go como fonte principal para `/ranking`, `/dados` e `/inteligencia`.
- `multiloja` ja usa a API Go para CRUD de lojas e gestao de usuarios/acessos.
- `usuarios` agora e workspace propria e deve concentrar o CRUD administrativo de acesso.
- `perfil` e pagina de autoatendimento do usuario autenticado, separada da area administrativa.
- `perfil` tambem e o lugar obrigatorio para sair de senha temporaria no primeiro acesso.
- `/operacao` agora diferencia:
  - leitura da loja ativa por snapshot
  - leitura integrada de todas as lojas acessiveis por `overview`
- a troca entre `Loja ativa` e `Todas as lojas` deve continuar explicita na propria tela de operacao, nao escondida apenas no header.
- o seletor global do header deve refletir o escopo global salvo na sessao sem redirecionar o usuario para `/operacao`.
- a opcao `Todas as lojas` no header deve aparecer sempre que a sessao tiver mais de uma loja acessivel, mesmo fora das rotas que suportam leitura integrada.
- quando `Todas as lojas` estiver ativo e a rota nao suportar leitura integrada, o header deve permanecer em `Todas as lojas` e a pagina continua exibindo o recorte padrao da loja ativa.
- hoje o seletor global tambem deve filtrar corretamente `/operacao`, `/relatorios`, `/ranking`, `/dados`, `/inteligencia`, `/consultor` e `/campanhas`.
- `/consultor` em `Todas as lojas` deve consolidar o roster acessivel, comparativos por loja e filtros locais por loja, nome, status e meta.
- `/campanhas` em `Todas as lojas` deve consolidar o historico das lojas acessiveis e oferecer filtro local por loja sem perder o escopo global salvo no header.
- para tirar alguem da fila por tarefa ou reuniao, usar `operationsStore.assignTask(...)`; nao reaproveitar pausa generica sem distinguir o tipo.
- contas `consultant` devem nascer pela gestao de consultores, nao pela tela administrativa de usuarios.
- a tela `usuarios` nao deve editar, convidar nem inativar contas `consultant`; ali so cabe listar e resetar senha quando necessario.
- o runtime local deve ficar restrito a compatibilidade de tela, estado efemero e drafts de UI.
- `ranking`, `dados` e `inteligencia` nao devem voltar a recalcular historico bruto no browser quando houver endpoint server-side cobrindo o caso.

No auth:

- usar `POST /v1/auth/login` e `GET /v1/me/context` como fluxo principal de sessao;
- usar `GET /v1/auth/invitations/{token}` e `POST /v1/auth/invitations/accept` para primeiro acesso por convite;
- usar `POST /v1/auth/password-reset/request` e `POST /v1/auth/password-reset/confirm` para o fluxo publico de esqueci minha senha;
- deixar `auth` como bootstrap da hidratacao remota por loja para `consultants` e `settings`;
- deixar `GET /v1/auth/me` como endpoint auxiliar de identidade quando precisar;
- guardar o token no cookie de app;
- derivar workspaces e loja ativa a partir do principal autenticado;
- usar `NUXT_PUBLIC_API_BASE` para chamadas do browser;
- usar `NUXT_API_INTERNAL_BASE` para SSR/container.
- o link de onboarding entra por `web/app/pages/auth/convite/[token].vue` e deve continuar usando o mesmo layout `auth`.
- a recuperacao publica de senha entra por `web/app/pages/auth/esqueceu-senha.vue` e deve continuar no mesmo layout `auth`.
- o header autenticado deve continuar oferecendo acesso rapido a `/perfil`.
- para acesso `store_terminal`, a tela `/operacao` deve manter comandos operacionais liberados apenas na propria loja.
- quando `auth.user.mustChangePassword` vier verdadeiro, o frontend deve redirecionar para `/perfil` e bloquear o restante ate a troca de senha.

## Regras de implementacao

- Preferir composicao por dominio, nao um store gigante.
- Manter classes CSS semanticas.
- Reaproveitar componentes existentes antes de criar variacoes paralelas.
- `AppSelectField.vue` e o seletor simples padrao do app e deve seguir a mesma linguagem visual do `product-pick`.
- `SettingsOptionManager.vue` e o ponto unico para catalogos ordenaveis simples da area de configuracoes, incluindo `pausas`.
- Para listagens administrativas reutilizaveis, preferir `AppEntityGrid.vue` em vez de tabelas HTML novas.
- Quando a listagem precisar de status booleano ou detalhes laterais/modal, compor com `AppToggleSwitch.vue` e `AppDetailDialog.vue` antes de criar widgets paralelos.
- Quando alterar comportamento de tela importante, atualizar a documentacao correspondente.

## Validacao minima

Quando houver mudanca de codigo em `web/`:

- rodar `npm run build` em `web/` sempre que viavel;
- conferir se nao voltou nenhum import de `@core/*`;
- conferir se paginas/workspaces nao passaram a depender direto do bridge monolitico.

## Referencias internas

- `../AGENT.md`
- `../docs/NUXT_4_STORE_ARCHITECTURE.md`
- `PANEL_EMBED_CONTRACT.md`
- `app/components/AGENTS.md`
- `app/features/operation/components/AGENTS.md`
- `app/pages/operacao/operations.md`

## Comandos uteis

```bash
npm run dev
npm run dev:build
npm run dev:logs
npm run dev:down
npm run build
npm run preview
```
