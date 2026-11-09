# Finance — funcionalidades e roteiro de testes

Guia reutilizável do módulo `/finance`. Resultados de uma execução ficam em documento separado; este roteiro não comprova aprovação.

## Preparação e proteção dos dados

- Subir o ambiente local conforme `docs/SETUP_MAC.md`. Nunca executar fixtures na VPS/produção.
- Usar prefixo único `QA Finance <data/hora>` em todas as fixtures; anotar IDs antes de alterar. Não modificar planilhas, usuários ou configurações existentes.
- Preparar conta QA com módulo Finance, usuários leitor/gestor e administrador; para recorrências, contas e lojas QA com billing persistido pela API canônica. Não alterar senhas/roles de usuários existentes.
- Registrar navegador, viewport em pixels CSS, branch/revisão, permissões e dados iniciais. Registrar resultado UI, API e banco separadamente.
- Em cada gravação aguardar Salvo, recarregar e confirmar resposta autoritativa. Nenhum PASS por inspeção estática ou fixture apenas em memória.

## Funcionalidades e execução

| # | Funcionalidade | Onde e como testar / resultado esperado |
|---|---|---|
| 1 | Criar planilha | Barra lateral → +. Criar planilha QA e confirmar mês, seleção e persistência após recarga. |
| 2 | Editar título/mês e recarregar | Cabeçalho → título e mês. Alterar ambos, aguardar Salvo, recarregar e conferir. |
| 3 | Alternar planilhas | Barra lateral → outra planilha → voltar. Conferir título, mês e linhas de cada uma. |
| 4 | Recolher sidebar | Botão de recolher/expandir barra lateral. Conferir acesso às planilhas e conteúdo. |
| 5 | Adicionar entrada descrição/valor | Entradas → + Linha. Informar descrição e R$100; recarregar e conferir. |
| 6 | Adicionar saída | Saídas → + Linha. Informar descrição e R$30; conferir que a linha anterior permanece. |
| 7 | Categoria da linha: selecionar/criar/limpar | Categoria da linha → escolher, criar e limpar seleção; recarregar após cada ação. Contas fixas usam categoria da configuração. |
| 8 | Efetivar/desefetivar e totais | Interruptor Efetivado → ligar/desligar entrada e saída; conferir os totais efetivados. |
| 9 | Data, Hoje, Limpar, reload | Data de efetivação → selecionar data, Hoje e Limpar; recarregar e conferir cada estado. |
| 10 | Ajustes positivos/negativos com observação/data | Ajustes → adicionar +R$10 e -R$5 com observação e data; conferir total e persistência. |
| 11 | Detalhes: original/ajustes/total/notas | Expandir linha → conferir original, ajustes, total, observações e composição quando houver. |
| 12 | Histórico editar sinal/valor/obs e remover, inclusive último ajuste | Histórico de ajustes → editar sinal, valor, nota e data; excluir um entre vários e depois o último. Esperado: lista vazia, ajuste zero e total igual ao original, inclusive após reload. |
| 13 | Observações da linha | Detalhes → observação da linha. Editar, aguardar Salvo e recarregar. |
| 14 | Remover entradas/saídas | X de uma entrada e uma saída QA → conferir remoção/limpeza e totais, inclusive após reload. |
| 15 | Cálculo 100+10-30=80 esperado/efetivado | Entrada R$100 + ajuste R$10, saída R$30, ambas efetivadas → saldo esperado e efetivado R$80. |
| 16 | Categorias criar/editar/cancelar/excluir e tipos | Configuração → Categorias. Criar Entrada, Saída e Ambas; editar/salvar, editar/cancelar, conferir filtro nas linhas e excluir apenas categorias QA. |
| 17 | Contas fixas CRUD, tipos/categoria/valor/notas | Configuração → Contas fixas. Criar e editar nome, tipo, categoria, valor e nota; salvar/recarregar; excluir apenas conta QA. |
| 18 | Composição: adicionar/editar/remover/soma 2+ contas | Contas fixas → composição. Em duas contas, adicionar dois membros, editar e remover; conferir somas e persistência sem misturar membros. |
| 19 | Mensalidades/vencimentos/ajustes/obs | Configuração → Entradas de clientes. Usar contas QA persistidas com mensalidade, vencimento, ajuste e nota. Confirmar valores após reload e na API/banco. |
| 20 | Por loja: efetivação individual/grupo/datas | Cliente por loja com duas lojas QA faturáveis → efetivar uma loja, grupo inteiro, desefetivar e alterar datas; conferir somas e persistência por loja. |
| 21 | Planilha mês seguinte inicializa fixos/recorrências e IDs | Com configurações QA salvas, criar mês seguinte. Conferir fixos e recorrências em ambos os meses, sem colisão de IDs nem alterações retroativas. |
| 22 | Alterar config preserva histórico até reaplicar | Alterar configuração QA. Reabrir mês anterior: manter snapshot. Aplicar configurações atuais → Cancelar preserva; Confirmar atualiza só a planilha selecionada. |
| 23 | Aberta/conferência/fechada bloqueia edição/exclusão; reabre preservando | Status → Conferência → Fechada. Conferir edição e exclusão bloqueadas na tela e API. Reabrir: preservar linhas e valores. |
| 24 | Excluir planilha com cancelamento | Lixeira da planilha QA → Cancelar mantém; Confirmar remove somente a selecionada. |
| 25 | Autosave ao trocar planilha/conta/página/voltar | Editar e imediatamente trocar planilha, conta e rota. Voltar/recarregar: dados ficam no contexto original. Testar também com request lenta. |
| 26 | Estados pendente/salvando/salvo/erro e retry/rede falha | DevTools → Network → Offline. Editar: mostrar erro/pendência, sem falso Salvo. Navegação não perde rascunho; voltar Online e tentar novamente: persistir e mostrar Salvo. |
| 27 | Popovers Escape/clique fora e janela estreita | Popover → Esc e clique fora. DevTools viewport 390×844, 768×1024 e desktop: alcançar campos, ações e configuração, sem conteúdo cortado. Zoom não substitui viewport. |
| 28 | Perfis leitor/gestor e permissões por operação | Usuários QA leitor e gestor, sem admin: leitor lê sem mutações; gestor edita conforme permissões. Conta sem módulo não abre rota nem API. |
| 29 | Forjar X-Account-Id e IDs alheios | Com sessão QA restrita, forjar X-Account-Id e IDs de planilha/linha alheios: acesso negado e recursos alheios retornam 404, sem vazamento. |
| 30 | API editar/efetivar/excluir fechada conflito | Planilha QA fechada → PUT, PATCH de linha e DELETE autenticados: 409; conteúdo permanece intacto. |
| 31 | Salvar versão antiga rejeitado | Duas versões da mesma planilha: salvar a nova e depois a antiga com expectedUpdatedAt antigo → 409, sem sobrescrever a nova. |

## Regressões automatizadas

No diretório `web`:

```sh
npm test -- layers/finance
npx eslint layers/finance --max-warnings=0
```

No diretório `back`, com Go disponível:

```sh
go test ./internal/modules/finance/...
```

Os testes PostgreSQL exigem `FINANCE_TEST_DATABASE_URL` apontando exclusivamente para banco descartável `finance_test*`, com migrations/fixtures de teste. Sem essa variável, os testes de integração são pulados: não registrar como aprovados. Nunca usar URL do banco da aplicação.

## Registro e limpeza

Para cada item registrar: passou/parcial/falhou, passos, valores antes/depois, evidência de reload, resposta HTTP quando relevante e limite concreto. Parcial não equivale a aprovado. Para bugs, guardar teste de regressão permanente quando houver lógica afetada.

Excluir somente IDs QA anotados, incluindo planilhas, configurações, lojas, contas, usuários e sessões criados. Conferir contagens iniciais/finais e restaurar rede, viewport e zoom. Nunca restaurar configuração inteira por cima de alterações de outra sessão. Não registrar tokens/senhas nos relatórios.

Execução histórica: [QA 2026-11-01](TESTES_FUNCIONAIS_FINANCE.md). Plano e contratos: [Plano do módulo](PLANO_MODULO_FINANCE.md).

## Cadastro fiscal e templates

O cadastro pessoal do emitente, acesso ao portal e templates é exercitado em `/perfil#fiscal`. Preparação, passos de regressão e evidências: [PERFIL_FISCAL_E_TEMPLATES.md](PERFIL_FISCAL_E_TEMPLATES.md). A emissão ainda não foi implementada; não confundir prévia de template com nota emitida.

## Comprovantes e preparação de NFS-e

Roteiro novo: [Comprovantes e rascunhos](COMPROVANTES_E_RASCUNHOS.md). Cobrir envio JPEG/PNG, preenchimento por OCR, revisão, template, save/reload, duplicidade, contexto de conta/usuário e preparação de pergunta no chat. Nenhuma emissão real faz parte do teste.

## Entrada Plus unificada — 2026-11-06

Roteiro e limites atuais: [PLUS_UNIFICADO.md](PLUS_UNIFICADO.md). No Finance,
a conversa principal não oferece escolha IA/local: orquestrador decide; validar
perguntas progressivas, consulta de planilha, IA autorizada/fallback, histórico
único de novos turnos, voz e navegação de históricos anteriores. Contextos
empresariais e históricos antigos não foram fundidos; aceite visual permanece aberto.

## Crow Plus: cliente do chat independente da página (2026-11-06)

Preparar usuário com duas contas autorizadas com Finance habilitado e dados
identificáveis diferentes. Manter a página na conta A e abrir o Plus.

1. Escolher B no seletor do chat. A página, suas planilhas e o seletor global
   devem continuar em A; o histórico/perfil/documentos do chat pertencem a B.
2. Enviar texto e gravar áudio para B. Trocar a página para C/A durante o pedido:
   não muda o cliente do chat. Trocar o chat durante a transcrição descarta o
   resultado anterior; não inserir texto/valor/arquivo da conta anterior.
3. Voltar o chat para A e depois B: conferir os históricos separados por usuário
   e conta; uma planilha selecionada em A nunca é enviada ao pedir dados de B.
4. Conferir que usuário com só uma conta não vê seletor, conta sem Finance fica
   indisponível e ID forjado/usuário sem acesso é rejeitado no backend.
5. Comparar tamanho/posição do Crow original e do Plus na mesma área do painel;
   conferir redimensionamento, seletor no topo, mensagens, mobile e teclado.

Estado: roteiro preparado, teste visual/manual e aceite ainda não executados.
A seleção independente de planilha/período pelo chat permanece em P15-26.

## Busca com fontes sem IA paga — pacotes 1/2

Manter IA desligada, sem cadastrar chave ou emitente. No chat principal Finance:

1. Perguntar “qual o limite de recebimento de MEI anual?”: trechos do acervo público,
   com título, fonte e versão; ressalvas de data e atualização permanecem visíveis.
2. “O que é o Simples Nacional?” e “Como declarar DASN-SIMEI?”: fontes pertinentes;
   nenhuma solicitação de senha/emitente nem chamada de modelo paga.
3. “Buscar fonte entropiazz”: ausência de evidência clara, sem inventar resposta.
4. Recarregar: manter fontes publicadas. Retirada de fonte em ambiente de teste
   oculta snapshot anterior. Trocar cliente: histórico de outra conta não aparece.
5. Repetir criação de rascunho, áudio e consulta financeira: funcionalidades anteriores
   permanecem disponíveis. Material público não autoriza emissão ou cálculo.

Preparado; teste manual e aprovação ainda pendentes. Cinco sínteses iniciais não
representam corpus fiscal amplo, legislação integral ou confirmação de vigência.
