# Revisão de Código — Fase 5.2: Sugestões, Dúvidas e Melhorias

## Sumário

Implementação completa de um canal de feedback onde usuários autenticados podem enviar sugestões, dúvidas e relatos de problemas. Administradores acessam uma tela dedicada para visualizar, filtrar e gerenciar essas mensagens com status e notas internas.

**Escopo:** Autossuficiente, sem dependências de ERP, WebSocket ou integrações externas.

## Arquivos Criados

### Backend

#### Migration e Modelo
- `back/internal/platform/database/migrations/0027_user_feedback.sql`
  - Tabela `user_feedback` com campos de identity, escopo (tenant/store/user), dados de feedback e auditoria
  - Índices para buscas por tenant, store, status, kind e criação

#### Módulo `feedback`
- `back/internal/modules/feedback/model.go`
  - Structs: `Feedback`, `FeedbackView`, `CreateInput`, `UpdateInput`, `ListInput`
  - Constantes: tipos (`suggestion`, `question`, `problem`) e status (`open`, `in_progress`, `resolved`, `closed`)
  - Interface `Repository` para persistência

- `back/internal/modules/feedback/errors.go`
  - `ErrNotFound` para feedback não encontrado
  - `ErrForbidden` para acesso negado

- `back/internal/modules/feedback/service.go`
  - `NewService(repository)` com três operações:
    - `Create(ctx, principal, input)` — qualquer usuário autenticado
    - `List(ctx, principal, input)` — filtra por kind/status, requer owner/manager/platform_admin
    - `Update(ctx, principal, id, input)` — atualiza status/notas, requer owner/manager/platform_admin
  - Helper `canManageFeedback()` para autorização

- `back/internal/modules/feedback/http.go`
  - `RegisterRoutes()` com três endpoints:
    - `POST /v1/feedback` — cria feedback
    - `GET /v1/feedback?kind=&status=` — lista com filtros
    - `PATCH /v1/feedback/{id}` — atualiza status e notas
  - Helper `writeServiceError()` para normalizar respostas de erro

- `back/internal/modules/feedback/store_postgres.go`
  - `PostgresRepository` implementando `Repository` interface
  - Métodos: `Create()`, `GetByID()`, `List()`, `Update()` usando pgx/v5
  - Queries parametrizadas para segurança contra SQL injection

- `back/internal/modules/feedback/AGENT.md`
  - Documentação de escopo, contrato, regras de acesso e integração

#### Integração
- `back/internal/platform/app/app.go` (modificado)
  - Import do módulo feedback
  - Inicialização: `feedbackRepository := feedback.NewPostgresRepository(pool)` e `feedbackService := feedback.NewService(feedbackRepository)`
  - Registro de rotas: `feedback.RegisterRoutes(mux, feedbackService, authMiddleware)`
  - Adição de "feedback" à lista de módulos do healthz

- `back/database/ERD.md` (modificado)
  - Adição da tabela `USER_FEEDBACK` ao diagrama
  - Relações: `TENANTS ||--o{ USER_FEEDBACK : receives`, `STORES ||--o{ USER_FEEDBACK : scopes`, `USERS ||--o{ USER_FEEDBACK : submits`

### Frontend

#### Store
- `web/app/stores/feedback.ts`
  - `useFeedbackStore()` com estado e actions:
    - `submitFeedback(input)` — POST /v1/feedback
    - `fetchFeedbacks(filters?)` — GET /v1/feedback com filtros opcionais
    - `updateFeedback(id, input)` — PATCH /v1/feedback/{id}
  - Refs para `items`, `loading`, `error`
  - Padrão: retorna `{ ok: true, data }` ou `{ ok: false, message }`

#### Componentes
- `web/app/components/feedback/FeedbackFormModal.vue`
  - Modal v-model controlado pelo layout
  - Campos: kind (AppSelectField), subject (input), body (textarea)
  - Tipos: sugestão, dúvida, problema
  - Submit valida campos obrigatórios e chama `feedbackStore.submitFeedback()`
  - CSS: overlay, dialog, form styling com espaçamento e hover states

- `web/app/components/feedback/FeedbackWorkspace.vue`
  - Workspace admin com `AppEntityGrid` para listagem
  - Filtros: por kind e status (atualizáveis em tempo real)
  - Busca por subject, user_name ou body
  - Colunas: tipo, assunto, usuário, status, data
  - Badges coloridas para kind e status
  - `AppDetailDialog` para detalhes: exibe body, permite editar status e admin_note
  - Ações: salvar (PATCH) com feedback visual

#### Página
- `web/app/pages/feedback.vue`
  - Página fina com `definePageMeta({ layout: "dashboard", workspaceId: "feedback" })`
  - Renderiza `FeedbackWorkspace`

#### Configurações
- `web/app/layouts/dashboard.vue` (modificado)
  - Adição do import e ref para `FeedbackFormModal`
  - Botão flutuante (FAB) no canto inferior direito (background azul, emoji 💬)
  - Abre modal ao clique
  - CSS com transições: hover scale, active shrink, sombra

- `web/app/domain/utils/permissions.ts` (modificado)
  - Adição em `WORKSPACE_ACCESS_DEFINITIONS` da workspace "feedback" com view/edit permissions
  - Adição em `ROLE_WORKSPACES`:
    - `platform_admin`: inclui "feedback"
    - `owner`: inclui "feedback"
    - `manager`: inclui "feedback"

- `web/nuxt.config.ts` (modificado)
  - Adição de `"/feedback": { ssr: false }` em `routeRules`

#### Documentação
- `web/app/components/AGENTS.md` (modificado)
  - Adição de seção `feedback` descrevendo `FeedbackFormModal.vue` e `FeedbackWorkspace.vue`

- `web/AGENTS.md` (modificado)
  - Adição em regras de stores mencionando `useFeedbackStore` e seu fluxo

## Decisões de Design

### Backend
1. **Modelo simples**: sem related data, apenas IDs de tenant/store/user; dados do usuário (nome) capturados no CREATE
2. **Permissões por role**: owner, manager, platform_admin podem listar e atualizar; consultor pode apenas criar
3. **Sem notificações automáticas**: podem ser adicionadas em futures via event listeners no módulo
4. **Sem chat/resposta direta**: admin_note é interna; design deixa espaço para Chat futuro via nova feature

### Frontend
1. **Modal flutuante**: FAB (Floating Action Button) sempre disponível no dashboard para baixa fricção de envio
2. **Filtros locais + remotos**: filtragem no Create (kind/status) refaz a query; busca é local (para UX fluida)
3. **Badges coloridas**: diferenciam visualmente tipos e status para rápida identificação
4. **AppDetailDialog reutilizado**: consistência com outras areas administrativas

## Checklist de Revisão

### Migration
- [x] Nomes de colunas seguem convenção snake_case
- [x] Constraints CHECK para enums (kind, status)
- [x] Foreign keys para tenants, stores, users
- [x] Índices para queries frequentes (tenant, status, kind, created_at)
- [x] Timestamps com timestamptz e default now()

### Backend — Código
- [x] Model com types corretos (uuid, string, time.Time)
- [x] ToView() para serialização JSON
- [x] Service valida permissões antes de listar/atualizar
- [x] HTTP handlers decodificam request e retornam erro se inválido
- [x] Erros normalizados via writeServiceError()
- [x] Queries parametrizadas em store_postgres.go
- [x] Repository interface bem definida

### Frontend — Código
- [x] Store segue padrão Pinia: defineStore + refs + actions
- [x] Actions retornam `{ ok, data? }` ou `{ ok: false, message }`
- [x] Componentes usam v-model para modais
- [x] Validação de campos obrigatórios no modal
- [x] Workspace usa AppEntityGrid (sem tabela HTML)
- [x] Filtros atualizáveis sem refresh completo

### Frontend — UI/UX
- [x] Modal tem fechar (botão X) e cancelar (botão)
- [x] Toast feedback após envio/atualização (ui.success/error)
- [x] Badges com cores distintas (sugestão azul, dúvida rosa, problema vermelho)
- [x] FAB posicionado sem cobrir conteúdo (bottom-right, z-index apropriado)
- [x] Hover states nos botões (scale, cor)
- [x] Formatação de datas consistente

### Permissões
- [x] Usuário comum vê FAB e modal funciona
- [x] Admin (owner/manager/platform_admin) acessa `/feedback`
- [x] Usuário sem permissão redireciona (middleware de auth)
- [x] ROLE_WORKSPACES inclui "feedback" para os papéis certos

### Documentação
- [x] AGENT.md do módulo feedback criado
- [x] AGENTS.md componentes atualizado
- [x] AGENTS.md web atualizado
- [x] ERD.md com nova tabela e relações

## Como Testar

### Setup
```bash
npm run dev  # na raiz
# Backend em localhost:8080, web em localhost:3003
```

### Fluxo de Envio (Usuário)
1. Login com qualquer usuário (consultor, manager, etc)
2. Botão 💬 flutuante no canto inferior direito
3. Clique → abre modal
4. Preencher: tipo, assunto, mensagem
5. Enviar → toast de sucesso
6. Verificar DB: `select * from user_feedback order by created_at desc limit 1;`

### Fluxo Admin (Owner/Manager/Platform Admin)
1. Login com owner ou platform_admin
2. Navbar → Feedback (ou `/feedback` direto)
3. Grade lista todos os feedbacks da sessão
4. Filtrar por Tipo (sugestão/dúvida/problema) e Status (aberto/análise/resolvido/fechado)
5. Buscar por assunto, usuário ou conteúdo
6. Clicar "Detalhes" → abre modal com body, permite editar status e nota
7. Salvar → toast de sucesso, grade atualiza
8. Verificar DB: `select * from user_feedback where id='...' \gx`

### Validações
- [ ] Usuário sem permissão não consegue acessar `/feedback` (redireciona para home)
- [ ] Usuário manager pode criar feedback e acessar admin (testes dos dois papéis)
- [ ] Filtros combinados (kind=problem AND status=open) funcionam
- [ ] Busca filtra corretamente
- [ ] Modal fecha com ESC
- [ ] Toast desaparece sozinho após timeout
- [ ] Atualização sem mudança não faz POST desnecessário

## Próximos Passos Possíveis

1. **Notificações**: PublishContextEvent no service para avisar admins de novo feedback
2. **Chat**: Expandir admin_note para mensagens bidirecionais (requer nova feature)
3. **Exportação**: Adicionar CSV/PDF dos feedbacks (action em toolbar)
4. **Categorias**: Subcategorias customizáveis por tenant em lugar de kinds fixos
5. **Inteligência**: Agregar feedback por tema/sentimento usando analytics
6. **Métricas**: Dashboard de feedback trends (volume, tempo médio de resolução)

## Notas para Próximas Mudanças

- Qualquer mudança em permissões deve atualizar tanto backend (roles.go) quanto frontend (ROLE_WORKSPACES)
- Se adicionar novos campos ao feedback, atualizar migration, model, store e componentes
- O modal é reutilizável se criarem outras features de envio (reportar bugs, solicitar features, etc)
- Admin_note é campo livre; se virar "respostas formais", considerar tipo de resposta (template) separado

---

## Ajustes Pós-Implementação (27/04/2026)

### Problemas encontrados e corrigidos durante testes

#### 1. Dependência externa não declarada — `github.com/google/uuid`
- **Problema**: O módulo usava `uuid.Parse` e `uuid.New()` do pacote `github.com/google/uuid`, que não estava no `go.mod` do projeto.
- **Erro**: Build do Docker falhou com `no required module provides package github.com/google/uuid`.
- **Correção**: Reescreveu todos os arquivos do módulo (`model.go`, `service.go`, `store_postgres.go`, `http.go`) para usar `string` como tipo de ID, alinhado ao padrão dos demais módulos do projeto (ex: `consultants`). O banco gera o UUID via `gen_random_uuid()` e retorna via `RETURNING id::text`.

#### 2. Bug no `argCount` do `store_postgres.go`
- **Problema**: O código usava `string(rune(argCount))` para gerar `$2`, `$3`, etc. — `string(rune(2))` retorna o caractere Unicode de código 2, não a string `"2"`.
- **Correção**: Substituído por `fmt.Sprintf("$%d", argCount)`.

#### 3. Permissões de feedback ausentes no banco de dados
- **Problema**: As permissões `workspace.feedback.view` e `workspace.feedback.edit` não estavam na tabela `access_role_permissions`. O sistema carrega permissões do banco (não do código Go), então o middleware bloqueava o acesso mesmo com permissões corretas no código.
- **Correção**: Criado `0028_feedback_permissions.sql` para inserir as permissões e grants para `manager`, `owner` e `platform_admin`.

#### 4. Workspace "feedback" ausente em `utils/workspaces.ts`
- **Problema**: A navbar filtra workspaces pelo array `WORKSPACES` em `utils/workspaces.ts`. O item "feedback" não havia sido adicionado, então nunca aparecia na nav mesmo com permissão.
- **Correção**: Adicionado `{ id: "feedback", label: "Feedback", icon: "chat_bubble", path: "/feedback" }`.

#### 5. GET /v1/feedback retornava 500 para `platform_admin`
- **Problema**: A query de listagem filtrava por `WHERE tenant_id = $1::uuid`. O `platform_admin` pode não ter `TenantID`, fazendo o cast `::uuid` de string vazia falhar.
- **Correção**: Alterado para `WHERE 1=1` com filtro condicional por `tenant_id` apenas quando não vazio.

#### 6. POST /v1/feedback retornava 500 para `platform_admin`
- **Problema**: O `store_id` na tabela era `NOT NULL`, mas `platform_admin` não tem loja vinculada.
- **Correção**: Criado `0029_user_feedback_nullable_store.sql` para tornar `store_id` e `tenant_id` nullable. Adicionado helper `nullableUUID()` no store para passar `nil` quando o campo for vazio.

#### 7. Design da modal fora do padrão do design system
- **Problema**: A modal estava com fundo branco, completamente diferente do design system dark do app.
- **Correção**: Redesenhada seguindo o padrão do `AppDetailDialog` — fundo escuro com gradiente, overlay com blur, inputs e botões no padrão dark. Adicionada animação de entrada (slideUp + fadeIn) e scroll lock no body.

#### 8. Toast muito grande e não adaptativo
- **Problema**: O toast tinha `min-width: 320px` e `max-width: 420px` com font-size grande, ocupando muito espaço.
- **Correção**: Reduzido para `min-width: 280px` / `max-width: 360px`, font-size menor (0.8rem/0.75rem), `word-break: break-word` e media query mobile mais ajustada.

---

**Data de Criação**: 26/04/2026  
**Última Atualização**: 27/04/2026  
**Status**: Funcional — GET lista feedbacks, POST cria, página admin acessível  
**Próximo Review**: Após testes completos de envio e gestão de status
