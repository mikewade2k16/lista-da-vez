# Back

Bootstrap inicial do backend em Go.

## Responsabilidade do `back/`

- autenticacao/autorizacao real
- comandos de fila e atendimento
- persistencia em PostgreSQL
- websocket por loja
- consolidacoes e relatorios server-side

## Responsabilidade do `web/`

- renderizacao da interface Nuxt
- formularios e navegacao
- feedback de UX
- exportacao local enquanto a API ainda nao assumir isso

## Responsabilidade do `core/`

- regras e estruturas de dominio que ainda apoiam o frontend
- calculos e selectors do painel
- configuracoes/mock para o estado atual do MVP

## Proximo passo

Implementar a primeira API de leitura/escrita da fila e substituir o `localStorage` do frontend.
