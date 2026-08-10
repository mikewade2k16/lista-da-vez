# AGENT — Performance Feedback

## Escopo

Este modulo cuida das reunioes de feedback de desempenho entre gestao e consultores.
Nao confundir com `queue/feedback`, que permanece sendo o canal publico de Suporte.

## Contratos

- `GET /v1/performance-feedback/context` devolve loja acessivel, consultores do escopo,
  periodo, KPIs, avaliacao existente e historico comparavel.
- `PUT /v1/performance-feedback/manager` cria/atualiza a avaliacao com uma lista ordenada
  de blocos livres (`id + title + contentHtml`) e persiste o mesmo snapshot de
  indicadores exibido no card no momento do salvamento definitivo.
- `PUT /v1/performance-feedback/settings` grava por tenant a cadencia mensal/semanal e
  os topicos padrao usados em novos ciclos.
- `PUT /v1/performance-feedback/{id}/consultant` permite apenas ao usuario vinculado ao
  consultor registrar sua devolutiva depois do compartilhamento pela gestao.

## Regras

- Gestor de loja so acessa lojas validadas pelo Principal; o repository repete
  `tenant_id + store_id + consultant_id` em todas as queries.
- Consultor ve apenas a propria linha, resolvida por `queue.consultants.user_id`.
- Semanas: 1–7, 8–14, 15–21 e 22–fim do mes; `week=0` representa o mes inteiro.
- Rascunhos legados continuam editaveis, mas nao entram no historico; ao serem salvos
  definitivamente, recebem o snapshot atual do card. Edicoes usam `version` para impedir
  sobrescrita silenciosa concorrente.
- `metrics_snapshot` guarda somente indicadores e metas; `feedback_sections` guarda no
  maximo 20 blocos de texto rico com titulo. A nota da transcricao e opcional enquanto o
  contrato de analise evolui.
- Cadencia mensal normaliza `week=0`; cadencia semanal permite `week=0..4`.
