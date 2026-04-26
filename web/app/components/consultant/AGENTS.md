# AGENTS

## Escopo

Estas instrucoes valem para `web/app/components/consultant`.

## Responsabilidade

Esta pasta concentra a experiencia da workspace `consultor`.

## Regras atuais

- [ConsultantWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantWorkspace.vue) decide entre a leitura da loja ativa e a visao integrada de `Todas as lojas`.
- [ConsultantIntegratedWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantIntegratedWorkspace.vue) e a referencia para comparativo multi-loja.
- a visao integrada deve continuar usando o escopo global salvo no header; navegar para outra rota e voltar nao pode derrubar `Todas as lojas`.
- a visao integrada deve oferecer filtros locais por loja, nome, status e situacao de meta antes de inventar outro painel paralelo.
- o comparativo multi-loja deve priorizar:
  - consolidado do roster acessivel
  - comparativo por loja
  - comparativo completo por consultor
- a pagina individual da loja continua sendo o modo padrao quando o escopo global estiver em `Loja ativa`.

## Fonte de dados

- roster por loja via `GET /v1/consultants?storeId=...`
- status vivo consolidado via `GET /v1/operations/overview`
- metricas comparativas integradas derivadas do historico das lojas acessiveis sem depender da workspace `ranking`
