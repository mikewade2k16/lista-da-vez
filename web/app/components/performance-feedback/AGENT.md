# AGENT — Feedback de desempenho

## Escopo

Modal de feedback aberto pelos cards de `/consultor`. O canal de chamados de suporte
continua em `components/feedback` e `/suporte`; os dois dominios nao compartilham estado
nem API. `/feedback-desempenho` existe apenas como redirect de compatibilidade.

## Contratos

- `PerformanceFeedbackModal.vue` recebe `storeId + consultantId` do card selecionado e
  orquestra periodo, editores, snapshot e historico.
- `PerformanceFeedbackMetrics.vue` apenas exibe o snapshot devolvido pelo backend.
- A gestao pode adicionar, renomear e remover ate 20 blocos ordenados com titulo e
  `OmniEditor` em densidade compacta; os topicos usam accordion exclusivo, mantendo apenas
  um editor expandido por vez, e um bloco recem-adicionado abre automaticamente. Os dois
  topicos iniciais sao somente sugestoes para um ciclo novo.
- O consultor vinculado edita somente a devolutiva depois do compartilhamento.
- `PerformanceFeedbackHistory.vue` compara snapshots ja persistidos.
- `usePerformanceFeedback.ts` concentra fetch, draft e mutacoes com `expectedVersion`.
- Cada abertura reidrata o alvo recebido do card e limpa o contexto anterior antes da
  leitura; nunca reaproveita draft de outro consultor ou loja.
- No workspace integrado, a troca de mes ou semana no modal atualiza o periodo autoritativo
  da pagina, recarrega os cards e reidrata os indicadores do modal a partir do mesmo card.
  O endpoint de feedback continua responsavel apenas pelo registro e historico da conversa.
- O botao unico `Salvar feedback` grava o ciclo como compartilhado e envia para persistencia
  exatamente o snapshot exibido no card; rascunhos nao aparecem no historico.
- `PerformanceFeedbackSettingsModal.vue` usa obrigatoriamente o template-core
  `OmniEntityDrawer`; nao cria `UModal`/`USlideover` proprio. O acionador visivel
  `Configurar feedback` fica na barra de filtros da visao integrada (e no cabecalho
  da visao de loja unica), fora do modal de um ciclo, e aparece somente para quem
  possui permissao de edicao.
- O drawer de configuracao usa `preferenceKey="performance-feedback-settings"` para
  restaurar o ultimo modo (`lado`, `centro` ou `tela cheia`) e a largura lateral
  escolhidos neste navegador.
- A configuracao persiste por tenant a cadencia e os topicos padrao. Cadencia mensal
  oculta semanas; semanal libera mes + semanas 1..4.

Semanas seguem o padrao do projeto: mes inteiro (`0`) ou 1–7, 8–14, 15–21 e 22–fim.
