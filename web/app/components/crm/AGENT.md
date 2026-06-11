# AGENT - CRM Components

## Escopo

Componentes da workspace CRM em `web/app/components/crm/`.

## Contratos atuais

- `CrmWorkspace.vue` orquestra filtros, resumo, lojas e consultores usando `useCrmStore()` e `useCrmConsultantMetrics()`.
- `CrmSummarySection.vue` recebe dados prontos por props. Nao faz fetch.
- `CrmStoresSection.vue` exibe venda ERP por loja comercial e metas operacionais.
- `CrmConsultantsSection.vue` cruza consultor ERP com fila via vinculo resolvido, codigo ou nome, sempre respeitando loja comercial quando ela existe.
- A metrica `Uso da lista` e cobertura por consultor: consultor com pedido ERP no periodo conta como coberto quando `atendimentos da fila >= pedidos ERP`. Nao usar `atendimentos / pedidos ERP` como KPI principal porque pode passar de 100%.
- A porcentagem exibida por consultor em `Cobertura lista` e `min(atendimentos / pedidos ERP, 100%)`.
- Melhor/pior loja usam media de cobertura por consultor da loja. Se nenhuma loja atingir a faixa `Normal` configurada, o card deve ser diagnostico (`Todas ruins`/`abaixo do normal`), nao premio.
- Melhor/pior consultor usam cobertura capped por consultor e respeitam `crmListUsageMinOrdersForHighlight` para nao destacar amostra pequena.
- Faixas de uso da lista e politica de recebimento por meta vem de `runtime.state.settings` e sao normalizadas por `web/app/domain/utils/crm-performance-policy.ts`.
- `Recebimento` na grade de consultores usa o `% meta` da loja do consultor e aplica a faixa de `crmGoalPayoutPolicy.consultant` sobre o vendido do consultor.

## Regras locais

- Componentes continuam sem fetch direto; regra reutilizavel fica em composables ou `web/app/domain/utils`.
- Ao mudar coluna de `AppEntityGrid`, atualizar `storage-key` para evitar configuracao antiga de colunas quebrando a tabela.
- Nao usar fallback global de fila para linha ERP que ja tenha `storeSlug`, para nao cruzar atendimento de outra loja em vendedor multi-loja.
- Ao adicionar metrica comercial nova, manter regra pura em `web/app/domain/utils` e cobrir com Vitest antes de plugar no componente.
