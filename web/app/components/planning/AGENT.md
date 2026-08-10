# AGENT — Planejamento

## Escopo

Estas instruções valem para `web/app/components/planning` e para a rota
`/planejamento`.

## Estado atual

- As áreas possuem páginas explícitas e URLs canônicas: `/planejamento/metas` monta `section="goals"`, `/planejamento/funcionamento` monta `section="operation"` e `/planejamento/escalas` monta `section="schedule"`; `/planejamento` apenas redireciona para Metas. Não condensar essas páginas novamente em `[section].vue`: a reutilização da mesma instância de rota deixou URL/sidebar e conteúdo em seções diferentes durante a navegação. O redirect da raiz vive obrigatoriamente em `pages/planejamento/index.vue`; não recriar `pages/planejamento.vue` ao lado da pasta de rotas.

- Metas reutilizam `MultiStoreGoalsSection` e a API canônica de metas.
- Funcionamento, modelos, política e parâmetros da equipe persistem em
  `queue.planning_store_configs`; escalas persistem por semana em
  `queue.planning_schedules` através de `/v1/operations/planning`.
- Fixtures tipadas vivem em `web/app/domain/planning/fixtures.ts`.
- Geração, validação e rateio são autoritativos no módulo Go de planejamento. O
  frontend mantém em `scheduler.ts` apenas helpers para edição manual e apresentação.
- As regras e o estado reativo vivem em `planning.ts`; a fronteira HTTP
  fica em `planning-persistence.ts`. PostgreSQL é a fonte autoritativa, sem `localStorage`.
- O workspace possui navegação lateral interna, iniciando por `Metas`, seguida de
  `Funcionamento` e `Escala`. Cada área mantém seus próprios filtros; não recriar
  uma barra global compartilhada entre essas responsabilidades.
- O scroll vertical pertence ao container raiz `.planning-workspace`. O shell das
  abas não pode encolher dentro do flex, pois seu `overflow: hidden` cortaria o
  conteúdo de Metas, Funcionamento e Escala antes de chegar ao scroll raiz.
- A referência de metas espelha o contrato atual do Multi-loja: mês + período
  `month|p1|p2|p3|p4|p5`, com S5 exibida apenas quando o mes tiver ao menos 29 dias. O rateio individual usa a
  meta do período selecionado, nunca um campo mensal paralelo na configuração da loja.
- Lojas, consultores e metas são reidratados das stores reais (`multistore` e
  `operation-goals`); configuração e escala são reidratadas da API de planejamento.
- A rota usa temporariamente `workspaceId = planejamento` e o gate de produto
  `queue`. Quando os módulos `workforce` e `goals` existirem, substituir esse
  gate temporário pelos módulos independentes.

## Invariantes

- Geração e edição manual salvam a semana no banco; não reintroduzir rascunho local.
- A publicação persiste `published` e fica bloqueada quando houver restrições obrigatórias.
- Semana publicada desabilita toda mutação; reabertura exige confirmação e fica
  registrada no histórico. Conflito de versão reidrata a versão mais recente.
- Eventos realtime `planning` invalidam o snapshot da loja e atualizam escala e histórico.
- O workspace canônico é `PlanningWorkspace.vue` e o store é `stores/planning.ts`; nomes
  internos de protótipo não fazem mais parte do módulo persistido.
- Carregamento, erro e vazio devem ser estados explícitos e acionáveis. Salvamento mostra
  `Salvando…`, horário da última gravação ou erro com nova tentativa, sem falha silenciosa.
- Na escala, a barra de loja/mês/semana/status permanece fixa no scroll; o quadro aceita
  navegação horizontal por teclado e controles visíveis, destaca domingos, feriados e dias
  fechados e mostra o saldo semanal de cada funcionário.
- A escala reutiliza `AppCalendarPeriodRail`, `AppCalendarSurface` e `AppDayItemsPanel`; nao copiar o calendario ou seus estilos. O trilho exibe `S1` a `S4` e inclui `S5` nos meses com ao menos 29 dias, adaptando os rótulos aos períodos persistidos.
- Clicar no cabeçalho de um dia abre o painel compartilhado; mutações continuam passando pelos emits autoritativos do planejamento.
- A visualização principal da escala é `PlanningScheduleCalendar.vue`: grade mensal baseada na mesma matriz de datas do calendário, turnos como chips e edição do dia no `OmniEntityDrawer`. Não reintroduzir o quadro semanal de cards como visualização principal.
- O trilho `M`/`S1`-`S4`/`S5` controla também a visualização: `M` mostra a matriz mensal e qualquer semana mostra somente seus sete dias. Trocar o período no cabeçalho também abre a semana correspondente.
- Na Escala, loja, mês, `Mês anterior` e `Mês atual` vivem dentro da superfície do calendário. Não duplicar o seletor horizontal de semanas no cabeçalho; o trilho lateral é a fonte única dessa navegação visual.
- Loja, mês e ações por ícone formam uma única linha no cabeçalho da superfície. Não exibir prévias do período anterior ou posterior; a troca direta acontece pelo input de mês e pelo trilho semanal. Durante a reidratação de um contexto que já possui equipe, a superfície permanece montada e bloqueada, sem trocar todo o painel por skeleton.
- Na visão semanal, todos os funcionários aparecem agrupados nas faixas de turno configuradas da loja. O drop em uma faixa usa a mutação atômica `placeShift`, alterando dia e modelo antes de uma única persistência.
- Durante o arrasto de um turno, a área externa de descarte fica visível e remove pela mesma mutação persistida do botão de lixeira de cada card; soltar arbitrariamente fora do calendário nunca exclui.
- Os cards do quadro semanal sao densos: nome e horario/carga semanal ocupam duas
  linhas; seletor de turno e setas de movimento dividem a mesma linha. Nao reintroduzir
  a terceira linha Mover, sombras fortes ou espacamento vertical de card administrativo.
- Alertas apontam para o funcionário/dia correspondente e o rateio individual usa a grade
  administrativa com busca, ordenação, horas, participação e meta persistida.
- O acesso usa exclusivamente `workspace.planejamento.view` e `workspace.planejamento.edit`; permissões de Multi-loja não concedem acesso implícito ao Planejamento. RBAC de acesso ao painel não deve ser usado como cargo ou regra trabalhista.
- Cada loja mantém perfis de funcionamento independentes para `shopping` e
  `street`. Trocar o tipo exibe o perfil correspondente sem copiar ou sobrescrever
  os horários editados do outro tipo durante a sessão. A troca persiste o
  `store_type` pelo CRUD real de lojas antes de atualizar o planejamento; assim reload,
  geração automática e inclusão manual usam a mesma classificação autoritativa.
- Horário de loja, política, disponibilidade e peso de meta são editáveis e salvos
  na configuração autoritativa da loja.
- Cobertura mínima (abertura/pico/fechamento), feriados, ausências por data e
  revezamento de domingos/feriados são configurados no drawer e validados no Go.
- Reaproveitamento oferece copiar semana anterior, replicar todas as semanas do mês,
  limpar a semana e reaplicar o modelo automático; semanas publicadas nunca são sobrescritas.
- Alterar tipo ou horário de funcionamento reconstrói os turnos já montados da
  semana ativa; dia fechado remove os turnos do dia e limites menores recortam o turno.
- Alterar o limite diário do funcionário ou da política ativa também reconstrói os
  turnos existentes, preservando abertura/fechamento e descontando o intervalo.
- Mutações locais de configuração e escala suprimem somente o próprio eco realtime
  por uma janela curta; eventos de outro gestor continuam reidratando o banco. Assim,
  trocar o modelo de um card não substitui o quadro inteiro por um estado de loading.
- Alertas exibem uma orientação acionável. Ao clicar, cobertura leva ao seletor do
  primeiro/último turno relevante do dia; excesso e folga levam ao último turno do
  funcionário; falta semanal leva ao card do funcionário para nova alocação.
- A escala possui seleção explícita de Semana 1–4. A meta semanal autoritativa da
  loja é rateada no backend entre os funcionários pelas horas escaladas ponderadas,
  sendo persistida novamente após gerar, mover, trocar, remover ou reconstruir turnos.
  Cada gravação também recalcula a meta mensal individual (`week=0`) a partir das semanas
  geradas; o evento `operationgoal:generated` atualiza a aba Metas sem reload manual.
- Os filtros da escala seguem a toolbar compacta já consolidada em `Metas por loja`:
  `AppSelectField compact`, `AppMonthInput` e `AppGoalPeriodFilter` (semanas comerciais dinâmicas). Esses componentes
  em `components/ui` sao o padrão canônico também para outras páginas; não recriar
  campos grandes, paddings, chips ou cápsulas locais para esse contexto.
- Cada loja mantém três modelos de turno independentes por tipo de localização,
  assim como o funcionamento. O perfil de shopping fecha às 22h por padrão e o
  de rua preserva seus próprios horários. `id` é estável (`opening`, `middle`,
  `closing`) e o nome é editável; início e fim são derivados do expediente
  predominante cadastrado em Funcionamento, com duração-base de até 9h brutas para
  comportar 8h trabalhadas e o intervalo configurado.
  Geração automática e opções da grade sempre consomem o perfil do tipo ativo. O editor apresenta
  abas para `Loja de rua` e `Shopping`, permitindo configurar os dois perfis sem
  alterar o tipo ativo da loja.
- Modelo com fim anterior ou igual ao início é inválido, aparece como erro e é
  ignorado pelo gerador. Em expediente menor, o turno é limitado ao horário da loja.
- `Gerar escala automática` executa o gerador e salva a escala; essa ação vive no
  cabeçalho do quadro semanal. O quadro permite
  adicionar, excluir, mover e trocar cards de funcionários entre os sete dias; todo
  movimento recalcula validações e metas.
- O gerador ajusta a duração do modelo ao menor limite diário entre funcionário e
  política, preservando o início da abertura e o fim do fechamento. Havendo ao menos
  duas pessoas no dia, reserva modelos distintos para cobrir abertura e fechamento.
- Configurações transversais (modelos de turno, política ativa e equipe/disponibilidade)
  abrem pelo botão `Configurações` em `PlanningSettingsDrawer`, sempre usando o
  template canônico `OmniEntityDrawer` e permanecendo acessíveis em qualquer seção.
- O drawer também persiste a preferência de exibir ou ocultar o cabeçalho institucional da página.
  As ações da escala ficam como ícones com tooltip junto aos filtros do calendário. Horas,
  participação e meta calculada pertencem à tabela de consultores da aba Metas, não ao rodapé da escala.
- O cabecalho do calendario mantem os atalhos textuais `Mes anterior` e `Este mes`
  imediatamente apos o seletor de mes. Eles apontam para o calendario real atual,
  enquanto o input continua permitindo qualquer competencia.
- Identidade de funcionario usa `core.users.nick` exposto por `/v1/consultants`, com nome
  completo somente como fallback. Os tamanhos da pagina partem das variaveis
  `--planning-user-font-size`, `--planning-user-font-weight`,
  `--planning-user-meta-font-size`, `--planning-calendar-day-font-size` e
  `--planning-calendar-lane-font-size` em `planning-workspace.css`.
- O workspace nao consulta metas, consultores ou escala antes de conta ativa, lojas reais e
  hidratacao client-side estarem resolvidas. Lista autoritativa vazia ou troca de conta limpa
  imediatamente lojas, equipe, turnos e configuração anteriores. As fixtures do store servem
  apenas a testes e nunca podem virar `storeId` de request HTTP nem permanecer na UI autenticada.
- O drag-and-drop dos cards usa `dragstart` em captura no card, desabilita o arraste nativo
  dos botoes internos e trata `dragover`/`drop` explicitamente em cada faixa. Preserve o teste
  de regressao que move um funcionario entre abertura e fechamento no mesmo dia sem duplica-lo.
- A interface e o contrato de metas pertencem ao `MultiStoreGoalsSection`: filtros,
  cards de situação, Meta total, Ticket médio, Conversão e P.A. devem evoluir uma
  única vez ali. Salvar a meta mensal distribui automaticamente a meta financeira
  em todas as semanas comerciais do mês (quatro ou cinco) e replica os indicadores. Editar uma semana nunca altera a
  meta mensal; após semanas realizadas, o painel pode sugerir e aplicar o saldo da
  meta mensal somente às semanas futuras.
