# AGENTS

## Escopo

Estas instrucoes valem para `web/app/components/campaigns`.

## Responsabilidade

Esta pasta concentra a workspace `campanhas`.

## Regras atuais

- [CampaignWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/campaigns/CampaignWorkspace.vue) continua sendo o ponto unico da tela.
- quando o escopo global do header estiver em `Todas as lojas`, a tela deve consolidar o historico das lojas acessiveis sem trocar automaticamente para uma loja especifica.
- o filtro por loja dentro da tela e local ao workspace; ele nao deve sobrescrever o seletor global do header.
- a comparacao integrada deve destacar:
  - volume de aplicacoes
  - bonus acumulado
  - tracao por loja
- o CRUD de campanhas continua o mesmo no escopo da loja ativa; o modo integrado serve para leitura comparativa do historico.

## Fonte de dados

- configuracao atual de campanhas pelo runtime da loja ativa
- historico integrado derivado de `GET /v1/operations/snapshot?storeId=...` nas lojas acessiveis da sessao
