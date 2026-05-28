# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/bi`.

## Objetivo

O modulo BI concentra integracoes server-side com APIs externas de BI usadas
pela aplicacao. A primeira integracao e o proxy controlado da Perola BI.

## Regras

- Nao criar proxy aberto para URLs arbitrarias.
- Manter hosts, bases e endpoints externos em allowlist no backend.
- Nao persistir tokens, CompanyKey, login ou senha neste modulo.
- Retornar status/body/headers relevantes da API externa para facilitar
  diagnostico operacional sem vazar segredo em logs.
- Usar timeout e `http.NewRequestWithContext` para chamadas externas.

## Rotas

- `POST /v1/bi/perola/login` — recebe `companyKey`, `login`, `pass` e devolve
  `PerolaProxyResponse` com o JWT capturado em `token`.
- `POST /v1/bi/perola/find` — proxy paginado para endpoints da allowlist
  (`/item/find`, `/nota/find`, `/notaItem/find`, `/inventario/find`). Aceita
  `token` no corpo; quando ausente, cai no token cacheado pelo backend.
- `GET /v1/bi/perola/overview` — agrega as fontes da Perola BI.
  - Query `cnpjEmpresa` sobrepoe o CNPJ default.
  - Header `X-Perola-Token` sobrepoe o token cacheado (usado pelo front
    quando o usuario gera o token via formulario de conexao).

## Carregamento no front (web/app/components/bi/BiWorkspace.vue)

- O painel NAO dispara nenhuma chamada automatica ao montar.
- O carregamento so ocorre quando o usuario clica em "Atualizar BI" ou
  "Carregar com token". Login manual nao encadeia overview automaticamente.
- O store `web/app/stores/bi.ts` tambem nao faz fallback recursivo para
  carregar o inventario em background — chamadas a `/v1/bi/perola/overview`
  acontecem 1x por clique do usuario, evitando volume excessivo na API
  externa.
