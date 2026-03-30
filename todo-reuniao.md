# Análise de requisitos — diagnóstico vs. codebase atual
> Checagem rigorosa feita em 23/03/2026. Legenda: ✅ implementado · ⚠️ parcial/ressalva · ❌ falta
> Bug crítico (múltiplos produtos no histórico) — CORRIGIDO em 23/03/2026

---

## Prioridade 1 — Operação

- ✅ Incluir motivo da visita no fluxo principal de atendimento
  - Onde: Página /operacao → botão "Fechar atendimento" → Passo 2 "Cliente" → campo "Motivo da visita"
  - Campo `visitReasons` (array) gravado no histórico

- ✅ Permitir múltipla escolha nos motivos da visita
  - Onde: Mesmo lugar acima — o picker aceita múltiplas seleções
  - Array de IDs enviado e gravado no histórico

- ✅ Manter campo de motivo com opção aberta (preenchimento livre)
  - Onde: Passo 2 → após selecionar um motivo aparece campo "Detalhe opcional"
  - Ativo apenas se configuração "Mostrar detalhes do motivo" estiver ativada em /configuracoes → aba Modal

- ✅ Criar registro de venda não fechada / não convertida
  - Onde: Passo 1 "Atendimento" → opção "Nao compra" no campo "Como terminou"
  - Outcome `nao-compra` gravado no histórico e contabilizado em relatórios e conversão

- ✅ Permitir múltiplos produtos por atendimento (UI + banco)
  - Onde: Passo 1 → campos "Produto visto pelo cliente" e "Produto comprado/reservado"
  - BUG CORRIGIDO em 23/03/2026: arrays `productsSeen[]` e `productsClosed[]` agora são gravados no `serviceHistory`

- ✅ Vínculo entre motivo da visita e fechamento (compra/reserva/não compra)
  - Onde: automático — motivos são filtrados por `outcomes` ao abrir o passo 2
  - Configurável em /configuracoes → aba Motivos (cada motivo pode ter outcomes vinculados)

- ✅ Indicador de qualidade de preenchimento
  - Onde (leitura): /relatorios → tabela "Qualidade" mostra % completo/excelente/incompleto por consultor
  - Onde (leitura): /dados → seção de análise de produtos/motivos/origens
  - Onde (tempo real): Modal passo 2 → barra de dots acima dos botões de ação
  - 5 dots (Nome, Telefone, Produto, Motivo, Origem) + 1 dot menor (Observações)
  - Cores: cinza=vazio · amarelo=incompleto · verde=completo · roxo no dot de notas quando excelente
  - Label: "X/5 campos · Incompleto / Completo / Excelente"

- ✅ Validações / campos obrigatórios configuráveis
  - Onde: /configuracoes → aba Modal → checkboxes "Exigir produto", "Exigir motivo da visita", "Exigir nome e telefone", "Exigir origem"

- ✅ Atendimento fora da vez (queue-jump)
  - Onde: /operacao → coluna "Em atendimento" → botão de iniciar atendimento mesmo não sendo o primeiro da fila
  - Se iniciado fora da vez: campo obrigatório "Motivo do atendimento fora da vez" aparece no passo 2

---

## Prioridade 2 — Configurações

- ✅ Cadastro de motivos da visita
  - Onde: /configuracoes → aba Motivos → botão "Adicionar motivo"
  - Operações: adicionar, editar (clique no nome), remover

- ✅ Cadastro de produtos (catálogo)
  - Onde: /configuracoes → aba Produtos → botão "Adicionar produto"
  - Campos: Nome, Categoria, Preço base, Código
  - Esses produtos aparecem no picker do modal de fechamento

- ✅ Cadastro de origens do cliente
  - Onde: /configuracoes → aba Origens → botão "Adicionar origem"
  - Exemplos: Instagram, Tráfego pago, Google, WhatsApp, Indicação etc.

- ✅ Parâmetros por tipo de loja
  - Onde: /multiloja → formulário de criação/edição de loja → campo "Template padrão" (select)
  - Campo `defaultTemplateId` salvo por loja e propagado via `createStore` / `updateStore`

- ✅ Metas e médias esperadas
  - Onde: /configuracoes → aba Consultores → formulário de consultor
  - Campos: Meta R$, Comissão, Conv. alvo %, Ticket alvo R$, P.A. alvo
  - Campos `conversionGoal`, `avgTicketGoal`, `paGoal` salvos por consultor via `createConsultantProfile` / `updateConsultantProfile`

- ✅ Campos gerenciais editáveis no modal
  - Onde: /configuracoes → aba Modal → toggles de mostrar/ocultar (Email, Profissão, Notas, Detalhes de motivo, Detalhes de origem) e de exigir/tornar opcional

- ✅ Estrutura de previsão x realizado
  - Onde: /consultor → cards de conversão, ticket médio e P.A. mostram "Meta: X" com indicador verde/laranja
  - Onde: /relatorios → seção "Meta mensal dos consultores" mostra barra de progresso R$ + badges de conv., ticket e P.A. vs meta por consultor

- ✅ Normalização / padronização de nomenclaturas
  - Onde: /configuracoes → todas as abas com cadastro de opções e produtos
  - `isDuplicate` em SettingsOptionManager bloqueia motivos/origens duplicados com mensagem de erro inline
  - `isDuplicateName` em SettingsProductManager bloqueia produtos com o mesmo nome

---

## Prioridade 3 — Consultor (dashboard individual)

- ✅ Dashboard de não convertidas
  - Onde: /consultor → card "Não convertidas" em ConsultantMetrics

- ✅ Trocar "dias com venda" por ticket médio
  - Onde: /consultor → card "Ticket médio" substituiu "Dias com venda" em ConsultantMetrics
  - Mostra meta de ticket com indicador verde/laranja se conversionGoal estiver configurado

- ✅ P.A. — Peças por Atendimento
  - Onde: /consultor → card "P.A. (pecas por atendimento)" em ConsultantMetrics
  - Onde: /relatorios → tabela de metas e desempenho (buildRankingRows)
  - Mostra meta de P.A. com indicador verde/laranja se paGoal estiver configurado

- ✅ Taxa de conversão por consultor
  - Onde: /consultor → card "Taxa de conversao"
  - Onde: /relatorios → tabela de metas e desempenho por consultor

- ✅ Ordenação por indicadores no ranking
  - Onde: /ranking → botões de ordenar por Valor, Conversão, Ticket, P.A., Qualidade, Score 360, Fora da vez

- ✅ Ranking 360 do consultor
  - Onde: /ranking → tabelas com colunas: Vendas, Conv., Taxa, Ticket, P.A., Qualidade, Tempo, Fora da vez, Score 360
  - Score 360 = combinação ponderada (conversão 35% + valor 25% + qualidade 20% + P.A. 15% + fora da vez 5%)
  - Ordenação por qualquer indicador incluindo Score 360
  - `buildRankingRows` retorna `qualityScore`, `avgDurationMs`, `queueJumpRate` por consultor

- ✅ Indicador "fora da vez"
  - Onde: /consultor → card "Atendimentos fora da vez"
  - Onde: /ranking → coluna visível na tabela de ranking

- ✅ Alertas automáticos por desempenho
  - Onde: /configuracoes → aba Alertas → 4 limites configuráveis (conversão mínima, fora da vez máximo, P.A. mínimo, ticket mínimo)
  - Onde: /ranking → painel de alertas ativos no topo, listando consultor + indicador + valor vs limiar
  - `buildConsultantAlerts()` em `admin-metrics.ts` calcula alertas do mês atual em tempo real

---

## Prioridade 4 — Relatórios

- ✅ Filtros analíticos
  - Onde: /relatorios → barra de filtros incluindo filtro por campanha (campaignIds)

- ✅ Relatório de produtos mais fechados
  - Onde: /dados e /relatorios — contabiliza todos os produtos dos arrays productsClosed[]

- ✅ Relatório de motivos da visita
  - Onde: /dados e /relatorios

- ✅ Relatório de qualidade de preenchimento
  - Onde: /relatorios → tabela "Qualidade" com % completo, % excelente, % incompleto por consultor

- ✅ Comparativo por loja
  - Onde: /multiloja → tabela comparativa com conv., meta, ticket, P.A., score por loja

- ✅ Tela para reunião gerencial
  - Onde: /multiloja → seção "Painel de reuniao gerencial" com cards de progresso por loja (5 metas com barra de progresso verde/laranja)

---

## Prioridade 5 — Campanhas

- ✅ Separação campanha interna ("corrida") vs. campanha comercial/marketing
  - Onde: /campanhas → campo "Tipo" (Interna / Comercial) em cada campanha
  - Botões de filtro: Todas / Internas / Comerciais no topo da listagem
  - `campaignType` salvo via `normalizeCampaign` e persistido

- ✅ Status de campanha
  - Onde: /campanhas → badge colorido por campanha derivado automaticamente
  - `deriveCampaignStatus()` em `campaigns.ts`: Aguardando (azul) · Em andamento (verde) · Encerrada (cinza) · Desativada (vermelho)
  - Calculado a partir de `startsAt`, `endsAt` e `isActive`

- ✅ Metrificação de campanha
  - Onde: /campanhas → seção "Dentro da campanha vs Fora (mesmo período)" por campanha
  - `buildCampaignPerformance()` em `campaigns.ts`: compara atendimentos que bateram vs não bateram no mesmo período
  - Mostra: total de atendimentos, conversão %, ticket médio para cada grupo

- ⚠️ Chave/código de identificação de campanha
  - Vínculo automático por critérios está implementado e funcionando
  - Código manual: decisão de negócio pendente — avaliar se o vínculo automático é suficiente

- ✅ Vínculo da campanha ao dado de venda/atendimento
  - Onde: automático ao fechar atendimento — `campaignMatches[]` e `campaignBonusTotal` gravados no histórico
  - Onde (visualização): /relatorios → métrica "Bônus campanhas" no resumo geral

---

## Features existentes no sistema que não estavam no todo original

- ✅ Página /inteligencia — Score operacional (0-100) com diagnósticos automáticos e recomendações
  - Cards: Críticos, Atenção, Saudáveis
  - Contexto rápido: tempo de espera, fora da vez, ticket médio, conversão geral

- ✅ Página /dados — Inteligência de tempo e análise de produtos
  - Timing por categoria: fechou muito rápido, demorou sem vender, rápido sem vender
  - Dados históricos acumulados vs. dados em tempo real (live)
  - Tabela horária de vendas (distribuição por hora do dia)

- ✅ Página /multiloja — Gestão multi-loja com métricas consolidadas
  - Criar, editar, clonar loja
  - Tabela com todas as lojas: consultores ativos, fila, serviços, conversão, ticket médio, espera, taxa fora da vez
  - Totalizadores consolidados de todas as lojas

- ✅ Exportação CSV e PDF em /relatorios
  - Onde: barra de filtros → botões "Exportar CSV" e "Exportar PDF"

- ✅ Simulador de vendas adicionais
  - Onde: /consultor → seção "Simulador" — calcula impacto de X vendas adicionais na meta e comissão

- ✅ Cadastro de consultores com meta e comissão
  - Onde: /configuracoes → aba Consultores → editar consultor → Meta mensal + Taxa de comissão

- ✅ Aba /ranking com dois rankings: mensal e diário
  - Onde: /ranking → tabela do mês (acumulado) + tabela de hoje (dia atual)

---

## Sequência recomendada de execução

### Bloco A — Já corrigido
1. ✅ BUG CORRIGIDO — `productsSeen[]` e `productsClosed[]` agora gravados no histórico

### Bloco B — Completar Operação (quase pronta)
2. ✅ Feedback de qualidade em tempo real no modal (passo 2)
3. ✅ Validação de duplicidade em cadastros de motivos/produtos/origens

### Bloco C — Evoluir Dashboard Consultor
4. ✅ Card "Não convertidas" explícito
5. ✅ Substituir "Dias com venda" por Ticket médio no /consultor
6. ✅ Calcular e exibir P.A. (agora possível com o bug corrigido)
7. ✅ Ordenação múltipla no ranking (/ranking)

### Bloco D — Relatórios e Gerencial
8. ✅ Filtro por campanha nos relatórios
9. ✅ Comparativo inter-lojas em /multiloja ou nova tela
10. ✅ Tela de reunião gerencial

### Bloco E — Evoluir Campanhas
11. ✅ Tipo de campanha (interna vs. comercial)
12. ✅ Status derivado automático (aguardando / ativa / encerrada)
13. ✅ Análise de performance da campanha (antes vs. durante)

### Bloco F — Longo prazo
14. ✅ Alertas automáticos por desempenho
15. ✅ Ranking 360
16. ✅ Templates vinculados por tipo de loja (defaultTemplateId por loja no painel multiloja)
