# Queue Communications

Escopo: CRUD e leitura dos comunicados exibidos na Operacao.

## Autoridade

- PostgreSQL: `queue.communications` e `queue.communication_stores`.
- `account_id` sempre vem do Principal autenticado; nunca do corpo ou query.
- Lojas selecionadas usam FK composta `(account_id, store_id)`.
- Exclusao e logica por `archived_at`.

## Contrato

- Rotas: `/v1/operations/communications`.
- Leitura exige `workspace.operacao.view`.
- Escrita exige `queue.communications.manage`.
- `publishedOnly=true&storeId=<uuid>` e a consulta usada pelo painel operacional.
- `targetsAllStores=true` nao cria linhas em `queue.communication_stores`.

## Limites

- titulo: 160 caracteres;
- resumo curto: 300 caracteres;
- conteudo: 20.000 caracteres;
- ordem: -10.000 a 10.000.
