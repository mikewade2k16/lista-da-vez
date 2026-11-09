# Finance — correções e validação complementar de 2026-11-01

Complementa [QA histórico](TESTES_FUNCIONAIS_FINANCE.md). Roteiro permanente: [Funcionalidades e testes](ROTEIRO_TESTES.md). Ambiente exclusivamente local, branch `fix/finance`, sem commit/push/deploy.

## Correções implementadas

1. **Último ajuste:** após normalizar dados antigos e excluir o item solicitado, o editor zera o acumulador anterior antes de recalcular a lista. Assim, excluir o último item não o recria como legado. A leitura de dados antigos continua compatível. Quatro regressões permanentes cobrem último positivo/negativo, múltiplos itens e legado em `useFinanceLineEditor.test.ts`.
2. **Limpeza de categoria:** linha manual tem botão explícito `Limpar categoria`, separado do seletor. Limpa para string vazia e usa o autosave existente. Categoria vinculada a conta fixa continua sendo editada na configuração.
3. **Tela estreita:** ações do cabeçalho quebram linha; aplicação de configuração permite texto em mais de uma linha. Seletor compacto de planilha e botão de criação aparecem abaixo de 1024px, onde a sidebar desktop é escondida. Corrigido seletor CSS `:deep` em folha global.

## Pendências revalidadas

| Item histórico | Resultado e evidência desta execução |
|---|---|
| 7 — Limpar categoria | **Passou, UI + PostgreSQL.** Criada categoria pelo seletor, selecionada e limpa pelo novo botão. Banco confirmou `category=''`; reload manteve campo vazio. |
| 12 — Último ajuste | **Corrigido, UI + PostgreSQL + regressão.** Base100, ajuste+15: excluir levou total115→100, histórico1→0. Banco confirmou `amount=100`, `adjustment_amount=0`. Reload manteve100. Testes também cobrem -5, preservação de itens restantes e legado. |
| 17 — Tipo de conta fixa | **Passou, UI.** Conta QA de saída25 foi editada para entrada25; Salvar alterou o resumo para `entrada` e o autosave concluiu. Os demais campos/CRUD foram exercitados no QA histórico. |
| 19 — Mensalidades persistidas | **Passou, UI + banco real local.** Duas contas QA persistidas: mensalidade170/vencimento31 e cliente por loja/vencimento10. Configuração mostrou170, permitiu ajuste+5 e observação; reaplicar na planilha gerou175. Fixture foi criada por SQL no ambiente local; leitura/escrita Finance seguiram API real, sem mocks de front. |
| 20 — Operações por loja | **Passou, UI + PostgreSQL.** Lojas60 e40, total100. Efetivar só A→saldo60; grupo→saldo100/ambas efetivadas. Data individual A→10/11, depois data do grupo→12/11 para ambas; banco confirmou as duas datas12/11. Desefetivação foi coberta pelos testes de fila/serviço e QA anterior; não foi repetida nesta UI. |
| 21 — Outros meses/IDs | **Passou, UI + PostgreSQL.** Planilhas novembro/dezembro/janeiro coexistiram: totais375/275/300, respectivamente. Janeiro criado após configurar conta fixa25 recebeu automaticamente fixo e recorrências. As diferenças são esperadas: novembro inclui entrada manual100 e dezembro foi criado antes do fixo. Banco confirmou16 linhas e16 IDs distintos, sem colisão. |
| 25 — Navegação/autosave | **Passou nos cenários UI executados + fila automatizada.** Edição seguida de troca de planilha preservou contexto; período/título de janeiro persistiram após ir ao Calendário e voltar. Editar título, trocar Crow Visuals→AM Malls→Crow Visuals preservou o título na conta original. Não houve escrita em AM Malls. Concorrência sob request lenta e captura de contexto são cobertas pela fila automatizada. |
| 26 — Falha de rede/retry | **Passou, UI + PostgreSQL.** DevTools Offline, descrição alterada: pendente→`Falha ao salvar — tente novamente`, sem falso Salvo. Após No throttling e retry, exibiu Salvo; banco confirmou `QA offline persistencia`. Não se forçou fechamento do navegador. |
| 27 — Viewport estreito | **Corrigido e verificado em emulação Chrome iPhone16,393×852 CSS.** Antes: ações cortadas e sem seletor local de planilhas. Depois de reiniciar servidor HMR: seletor/criação e configuração/exclusão visíveis, cartões empilhados e valores acessíveis. Popovers Esc/clique-fora foram cobertos no QA histórico. Não equivale a teste em aparelho físico. |
| 28 — Perfis reais | **Passou na API local com login normal.** Criados usuários QA novos, memberships e roles reais derivados de `finance.viewer`/`finance.manager`. Ambos leram sheets/config; leitor recebeu403 ao criar, gestor criou/excluiu com200. Ambos receberam403 ao forjar conta sem vínculo. Não houve login desses usuários no navegador; o teste de UI permaneceu com admin. |

## Checks e limpeza

- 11 testes Vitest Finance aprovados; ESLint da layer sem erros/avisos; seis templates Vue compilados e `git diff --check` sem erros.
- Nenhuma alteração de backend funcional nesta continuação. Testes Go e build global da rodada anterior continuam documentados no checklist; não são apresentados como reexecutados aqui.
- Consulta final: **2 planilhas e6 linhas originais**, zero contas/usuários/configuração QA desta rodada. Lojas QA removidas por cascade da conta descartável.
- Rede restaurada para No throttling; emulação e DevTools fechados, zoom mantido em100%. Usuários originais, senhas, volumes e produção preservados.

## Alcance da conclusão

O bug confirmado foi corrigido e as lacunas receberam evidências adicionais na camada indicada. Isso não significa garantia absoluta de100% nem que todos os31 fluxos foram repetidos integralmente nesta rodada. Perfis reais foram validados por HTTP, não pela UI; dispositivo físico e encerramento forçado do navegador não foram ensaiados. O roteiro mantém esses cenários explícitos para futuras execuções.

Ajuste posterior de layout: seletor de status limitado a9rem, sem expansão para100%; ações permanecem na mesma linha no desktop e permitem quebra apenas abaixo de1024px. Conferido no navegador local após correção.
