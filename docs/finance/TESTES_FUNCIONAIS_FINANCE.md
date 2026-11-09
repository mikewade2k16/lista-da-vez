# QA funcional do Finance — 2026-11-01

Escopo: testes observáveis nesta sessão, sem alterar planilhas originais. As
suítes automatizadas existentes e a inspeção de código são evidências separadas
de uso funcional no navegador.

## Evidências executadas

- UI Chrome `localhost:3003/finance`: carregou com sessão administrativa; a
  lista mostrou as duas planilhas originais. Foi criada pela interface a fixture
  `Finance 2026-11` (ID conhecido apenas para limpeza), que apareceu na lista e
  abriu com mês/ano novembro de 2026, uma entrada e uma saída iniciais.
- UI: abriu o painel Configuração financeira; inicialmente observou categorias,
  contas fixas e entradas de clientes com 0 itens. Depois criou e removeu uma
  categoria e uma conta fixa QA, sem alterar configurações originais.
- Postgres descartável `finance_test_luna`, sem porta pública: três testes Go
  passaram (`TestSheetsPersistenceDatesAndScope`, `TestFixedAccountsKeepAllMembers`,
  `TestClosedSheetAndConcurrentUpdate`). Depois, container e fixture de UI foram
  removidos. A fixture foi excluída pelo ID+título+período. Nenhuma planilha
  original foi atualizada.
- A automação AX do Chrome não conseguiu clicar confiavelmente nos botões de
  linha inicialmente; ao reler AX antes de cada ação e usar screenshot com
  coordenadas quando necessário, foi possível concluir mais fluxos na UI.
- Na retomada, 7 testes Vitest existentes de fila/sources/editor passaram. Um
  harness temporário real de `useFinanceSheetSources` testou duas contas fixas
  com membros, recorrência por loja, ajustes/notas, efetivação/grupo, soma e IDs
  mensais (3 passaram). Os harnesses foram removidos.
- A fixture UI foi excluída após testar Cancelar e Confirmar. A categoria e a
  conta fixa QA também foram removidas por seus controles. Conferência agregada
  final: 2 planilhas/6 linhas originais; zero planilhas, linhas, categorias ou
  contas com prefixo QA Luna.
- Na verificação final, foi criada uma nova fixture temporária `Finance 2026-11`;
  nela, alternar para `Finance 2026-07` e voltar funcionou. Entradas de R$120 e
  saídas de R$30 foram removidas pelos controles da linha e voltaram a vazio.
  O seletor de categoria fechou com clique fora. Zoom do Chrome foi aumentado
  oito níveis como aproximação de viewport menor: a página ficou muito ampliada
  e alguns controles do cabeçalho ficaram fora da área visível; isso não equivale
  a medir um viewport estreito por dimensão CSS. O zoom foi restaurado a 100%.
  Fixture excluída ao final; a lista voltou às duas planilhas originais.
- Na continuação, a fixture `Finance 2026-11` recebeu uma nova saída via `+ Linha`
  (R$32), e três categorias QA Entrada, Saída e Ambas foram criadas na configuração.
  A edição da categoria de entrada foi cancelada e o nome original permaneceu.
  Categoria Saída foi selecionada na linha. A limpeza dessa seleção não foi
  concluída: as opções do popover mostraram apenas Saída/Ambas e o X não ficou
  disponível como alvo acessível na tela.
- A conta fixa QA foi criada como Saída, R$100, com observação; editada para
  R$120, observação `nota editada QA` e categoria Ambas. A composição visual
  teve Pessoa A R$50 + Pessoa B R$70 = R$120; editar A para R$45 exibiu R$115,
  remover B deixou R$45/1 item, e restaurar A/B mostrou R$120/2 itens.
- O reapply de configurações foi aberto e Cancelar manteve a planilha sem linha
  fixa gerada; confirmar depois criou a linha `QA Luna Composicao` de R$120, com
  os dois membros. Na saída QA, ajuste -R$5, observação `ajuste negativo QA` e
  data 2026-11-01 produziram total R$27. A nota `nota reload QA` persistiu após
  reload; Limpar deixou a data vazia também após reload, com linha efetivada.
- Foram removidas pela configuração as três categorias e a conta fixa QA; a
  planilha temporária foi excluída. Query agregada final no Postgres: zero
  planilhas, linhas, categorias e contas com prefixo QA Luna; 2 planilhas e 6
  linhas originais.

## Checklist integral

| # | Fluxo | Estado | Evidência / reprodução |
|---|---|---|---|
| 1 | Criar planilha | **Passou** | Criada `Finance 2026-11` pela UI e exibida na lista; removida ao final. |
| 2 | Editar título/mês e recarregar | **Passou (UI)** | Título mudou para `QA Luna Finance 2026-11`; período mudou de 2026-11 para 2026-12. Ambos persistiram após reload. |
| 3 | Alternar planilhas | **Passou (UI)** | Da fixture `Finance 2026-11`, abriu `Finance 2026-07` e retornou à fixture; seleção e conteúdo mudaram. |
| 4 | Recolher sidebar | **Passou (UI)** | Botão alternou para `Expand sidebar`; estado recolhido permaneceu após reload. |
| 5 | Adicionar entrada descrição/valor | **Passou (UI)** | Entrada `QA Luna receita` recebeu R$100 e foi salva automaticamente. |
| 6 | Adicionar saída | **Passou (UI)** | Clicou `+ Linha` e criou linha de saída `QA Luna sada nova` por R$32. Depois ajuste -R$5 resultou em R$27. |
| 7 | Categoria da linha: selecionar/criar/limpar | **Passou parcialmente (UI)** | Categoria QA Saída foi selecionada na linha; opções filtraram Entrada. A limpeza foi tentada pelo popover e por clique na área do badge, mas nenhuma opção limpa ficou acessível/visível. Configuração QA removida ao final. |
| 8 | Efetivar/desefetivar e totais | **Passou (UI)** | Entrada + ajuste e saída efetivados: saldo R$80; desefetivar saída zerou efetivado da seção. |
| 9 | Data, Hoje, Limpar, reload | **Passou (UI)** | Data de efetivação 2026-11-01 foi limpa em linha efetivada; após reload, popover continuou sem data. `Hoje` também foi acionado na etapa anterior. |
| 10 | Ajustes positivos/negativos com observação/data | **Passou (UI)** | Além do +R$10 anterior, ajuste -R$5 com observação `ajuste negativo QA` e data 2026-11-01 foi salvo. Original R$32, total R$27. |
| 11 | Detalhes: original/ajustes/total/notas | **Passou (UI)** | Expansão exibiu original R$100, ajuste +R$10, total R$110; observação de linha `QA nota da linha` persistiu em tela. |
| 12 | Histórico editar sinal/valor/obs e remover, inclusive último ajuste | **Falhou (reprodução UI + harness real)** | Após editar ajuste para +R$15 e observação `ajuste QA editado`, clicar remover recriou imediatamente `Ajuste legado` por R$15 e manteve total R$115. Harness temporário `web/layers/finance/qa-last-adjustment.test.ts`, executado no Vitest host, falhou no assert `expect(row.adjustments).toHaveLength(0)` (recebeu 1). Arquivo temporário removido após o teste. Sinal negativo ainda não exercitado. |
| 13 | Observações da linha | **Passou (UI)** | `nota reload QA` foi salva e exibida nos detalhes após reload. |
| 14 | Remover entradas/saídas | **Passou (UI)** | Na fixture, preencheu entrada `QA Luna remover receita` R$120 e saída `QA Luna remover despesa` R$30; o controle vermelho de cada linha limpou os dados e totais voltaram a R$0. |
| 15 | Cálculo 100+10-30=80 esperado/efetivado | **Passou (UI)** | Com entrada base R$100 + ajuste R$10 e saída R$30, os esperados e efetivados mostraram saldo R$80. |
| 16 | Categorias criar/editar/cancelar/excluir e tipos | **Passou (UI)** | Criou categorias QA dos tipos Entrada, Saída e Ambas; editou Entrada e cancelou, preservando seu nome. Categorias QA foram removidas ao final. |
| 17 | Contas fixas CRUD, tipos/categoria/valor/notas | **Passou parcialmente (UI)** | Conta QA Saída R$100/nota criada; editada para R$120, categoria Ambas e nota `nota editada QA`. Excluída ao final. Edição do tipo da conta não testada. |
| 18 | Composição: adicionar/editar/remover/soma 2+ contas | **Passou (UI + DB + composable)** | UI: Pessoa A R$50 + Pessoa B R$70 = R$120; alterar A para R$45 mostrou R$115; remover B deixou R$45/1 item; restaurar B e A mostrou R$120/2 itens. O teste SQL e o harness anterior também passaram. |
| 19 | Mensalidades/vencimentos/ajustes/obs | **Passou parcialmente (composable sintético)** | Source real construiu linhas recorrentes de entrada com vencimento, ajuste +5 e nota; painel UI real informou 0 itens. Não há fixture billing persistida. |
| 20 | Por loja: efetivação individual/grupo/datas | **Passou parcialmente (composable sintético)** | Duas lojas geraram total 105, grupo efetivado e data 2026-11-10 agregada. UI individual/grupo não testada. |
| 21 | Planilha mês seguinte inicializa fixos/recorrências e IDs | **Passou parcialmente (composable/testes)** | Sources e teste existente verificaram linhas recorrentes e IDs diferentes entre novembro/dezembro e por loja; não havia configuração inicial no cenário UI. |
| 22 | Alterar config preserva histórico até reaplicar | **Passou (UI + composable/teste)** | Na fixture, Cancelar no diálogo de reapply manteve linhas sem mudança; confirmar gerou a linha fixa QA de R$120 com dois membros. Harness e teste de sources verificaram snapshot preservado até sync explícito. |
| 23 | Aberta/conferência/fechada bloqueia edição/exclusão; reabre preservando | **Passou (UI + serviço/DB)** | UI foi para Conferência e Fechada; fechada desabilitou campos/linhas e avisou. Reaberta com Aberta preservou dados. Teste SQL também passou. |
| 24 | Excluir planilha com cancelamento | **Passou (UI)** | Alerta abriu; Cancelar manteve fixture. Confirmar excluiu apenas `QA Luna Finance 2026-11`. |
| 25 | Autosave ao trocar planilha/conta/página/voltar | **Passou parcialmente (fila automatizada)** | Teste de fila validou contexto conta/planilha, coalescência e edições durante request lenta. Troca visual de conta/página com draft pendente não foi testada. |
| 26 | Estados pendente/salvando/salvo/erro e retry/rede falha | **Passou parcialmente (fila automatizada)** | Teste validou pendente, falha, retenção, retry e salvo; não observou estado salvando nem falha de rede na UI. |
| 27 | Popovers Escape/clique fora e janela estreita | **Passou parcialmente (UI)** | Escape e clique fora fecharam o seletor de categoria. Zoom Chrome a 8 níveis ampliou demais a tela e deixou partes do cabeçalho fora da área visível; viewport CSS estreito não foi dimensionado diretamente, então o comportamento responsivo permanece parcial. |
| 28 | Perfis leitor/gestor e permissões por operação | **Passou (HTTP automatizado existente)** | `TestFinanceRouteAuthorization` passou nesta sessão; cobre leitura, escrita negada a leitor, membership forjada, owner, admin e permissões vazias. Perfis reais na UI não disponíveis. |
| 29 | Forjar X-Account-Id e IDs alheios | **Passou (serviço + HTTP)** | Membership rejeita conta forjada no teste HTTP; serviço retorna not found para planilha de outra conta no teste SQL. |
| 30 | API editar/efetivar/excluir fechada conflito | **Passou (serviço/DB)** | Teste SQL verificou bloqueio de update, efetivação e delete em planilha fechada. Sem chamada HTTP real local. |
| 31 | Salvar versão antiga rejeitado | **Passou (serviço/DB)** | Teste SQL verificou `ErrSheetConflict` após primeiro update com versão antiga. |

## Limites e próximos passos

O socket Docker e a ausência de Go no host foram contornados via imagem Go
existente e Postgres isolado, sem rebuild do stack. Resultado das 31 linhas:
22 passaram integralmente no escopo descrito, 8 passaram parcialmente e 1
falhou; nenhuma linha ficou sem tentativa. Permanecem parciais perfis reais na
UI, configurações de billing persistidas, edição/remoção visual de membros de
composição por múltiplas contas, limpeza de categoria na linha, edição do tipo
da conta fixa, troca visual de conta durante autosave, falha de rede no browser
e medição com viewport CSS estreito. O item 12 é bug
confirmado em UI e harness e requer correção em tarefa própria. Nenhum código
funcional foi corrigido nesta etapa.

## Continuação

Contagem histórica corrigida por conferência da tabela: 22 aprovados, 8 parciais e 1 falha. Correções e evidências posteriores estão em [Validação complementar](VALIDACAO_COMPLEMENTAR.md); o roteiro reutilizável está em [Funcionalidades e testes](ROTEIRO_TESTES.md). Esta tabela histórica não foi reclassificada retroativamente.
