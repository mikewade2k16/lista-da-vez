# Cadastro no módulo Tasks

## Estado

Página proposta e aceita pelo usuário: **Contábil + Crow Plus**. Os 15 prompts estão preparados (CT-15 acrescentado em 2026-11-05). Este documento não comprova cadastro. A API local respondeu ao health check; conta/quadro e sessão autenticados precisam ser confirmados antes da escrita. Não há conector Tasks nesta sessão do agente.

## Fonte do conteúdo

`prompts/CT-01.md` a `CT-15.md` são canônicos. `backlog.tasks.json` contém os prompts completos e o payload derivado de cada task; não existe endpoint bulk/import confirmado. Se um prompt mudar, regenerar sua cópia no manifesto antes de publicar. Não guardar token/sessão/conta pessoal em arquivo versionado.

## Procedimento seguro de publicação

1. Usar a sessão autorizada pelo usuário. Resolver a conta ativa pelo contexto autenticado; se houver ambiguidade de conta, pedir seleção. Não gerar token administrativo, extrair sessão de navegador sem autorização, alterar senha ou escrever diretamente no banco.
2. Ler boards pela API canônica e localizar nome exato. Se mais de um corresponder, não escolher arbitrariamente. Se não existir, criar a página aprovada na conta selecionada, preservando as páginas existentes. Sem compartilhar com clientes externos, sem atribuir responsáveis ou enviar menções.
3. API existente: GET/POST `/v1/tasks/boards`; detalhe GET `/v1/task-boards/{boardId}`. POST de board usa contrato CreateBoardInput atual; não mandar o manifesto inteiro como body.
4. Listar tasks do board paginando até o fim. Deduplicar pelos IDs CT-01...CT-15 no título e conferir o conteúdo; havendo tarefa já preenchida diferente, reportar em vez de sobrescrever. Não depender só de um GET da primeira página.
5. Criar uma task por `POST /v1/tasks/boards/{boardId}/tasks`, usando somente `createTaskPayload` do item. O contexto de conta é autenticado via Authorization + X-Account-Id validado; não enviar accountId no body. Se usar UI, preencher título, descrição integral, prioridade e checklist equivalentes. Não criar relação nativa de dependência sem verificar que o produto a suporta; dependências já constam da descrição.
6. Após cada POST, guardar ID retornado em recibo local ignorado/temporário e reler por GET. Timeout de criação não justifica repetir cegamente: listar pelo código e conferir antes de tentar novamente. Endpoint CreateTask não tem idempotency key confirmada.
7. Validar 15 títulos únicos, descrição integral não truncada, prioridades e página/conta corretas. Ler tudo de volta da API/UI. Não marcar tarefas concluídas: os prompts foram preparados, a implementação ainda não ocorreu.
8. Registrar neste documento somente resultado agregado e limitações; recibos com IDs da conta e dados de sessão não vão ao Git. Entregar ao usuário o link real da página se observado pela UI.

## Regra de acesso ao navegador

A skill `.agents/skills/principios-engenharia/SKILL.md` exige autorização explícita antes de Work/CUA/navegador, inclusive leitura. Se esse for o meio disponível para usar a sessão, explicar o escopo (apenas Tasks local) e consumo de créditos, e obter autorização. A aprovação do nome da página não equivale a essa autorização técnica.
