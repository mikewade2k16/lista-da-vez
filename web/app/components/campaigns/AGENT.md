# AGENTS

## Escopo

Estas instrucoes valem para `web/app/components/campaigns`.

## Hub unificado de campanhas

- A rota canonica e `/campanhas`, com `workspaceId: comunicados`, para manter a
  leitura dos comunicados disponivel aos mesmos papeis da operacao.
- `CampaignHubWorkspace.vue` concentra tres secoes compactas:
  `comunicados`, `campanhas` comerciais e `corridinhas`/premiacoes.
- As secoes comerciais e internas so aparecem para quem possui acesso ao
  workspace `campanhas`; unificar a pagina nunca amplia permissao.
- `/comunicados` permanece como redirect para
  `/campanhas?secao=comunicados`. O item principal do nav usa o nome
  `Campanhas` e nao deve existir uma segunda entrada duplicada em `Site`.
- Comunicados continuam no CRUD autoritativo
  `/v1/operations/communications`. Campanhas e corridinhas continuam usando o
  contrato atual de `state.campaigns`; uma mudanca visual nao deve misturar nem
  copiar dados entre as duas fontes.
- `campaignType: comercial` alimenta a aba Campanhas comerciais e
  `campaignType: interna` alimenta Corridinhas. Todos os campos existentes da
  regra devem ser preservados no `CampaignEditorDrawer`.
- Listagens usam cards compactos. Criar e editar acontece no
  `OmniEntityDrawer`; nao reintroduzir formularios extensos inline.
- A premiacao suportada hoje e a soma de `bonusFixed` com `bonusRate` sobre a
  venda. Nao exibir faixas ou premios arbitrarios sem ampliar primeiro o
  contrato autoritativo.

## Responsabilidade

Esta pasta concentra a workspace `campanhas`.

## Regras atuais

- [CampaignHubWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/campaigns/CampaignHubWorkspace.vue) e o ponto unico da tela e delega cada secao ao componente de dominio.
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
