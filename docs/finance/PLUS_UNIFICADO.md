# Crow Assistant Plus — entrada Finance unificada

Data: 2026-11-06. Estado: implementação local e testes técnicos entregues;
aceite visual, voz real e modelo real não executados. Não significa emissão fiscal
liberada ou CT-14 concluído. Nenhuma configuração/credencial foi ativada na conta.

## Comportamento

Uma entrada principal no Finance, identificada como Crow Assistant Plus. Não há
botão “com IA/sem IA”. Preparação fiscal pessoal/perfil/acesso são locais, com
perguntas progressivas e formulários apenas por solicitação. Consultas financeiras
leem a planilha selecionada pelo Service Finance, respeitando conta e permissão;
valores são formatados em reais, respostas limitadas e parcialidade sinalizada.

Perguntas abertas podem usar IA se finance.assistant_configs.enabled autorizar,
com credencial/modelo próprios do Finance. Configuração inválida/ausente, limite,
provider não suportado ou falha retorna atendimento local no mesmo transcript.
Isso não transforma o fallback em compreensão irrestrita. Consultas locais não
passam pelo modelo e não dependem de habilitar ferramentas empresariais.
Não acessar portal/emitir, executar tools ou confirmar mutações pela resposta LLM.

## Dados e contratos

- POST /v1/finance/conversation mantém requestId/text/draftId; financeSheetId e
  month são opcionais, derivados da seleção da mesma conta e revalidados no owner.
- Novas respostas locais/IA persistem juntas em finance.access_messages, por
  account_id/user_id; mesmos IDs/ordem e dedupe do request. Sem tabela nova de chat.
- IA recebe histórico pessoal limitado e mascarado e a pergunta, nunca credencial
  de portal. Usa platform/llm/cofre/resolver existentes, schema fechado answer,
  máximo de saída e sem retry interno. Não consulta tabela privada de outro módulo.
- Configuration/permissão revalidadas após a rede; revogação descarta resposta.
- Limites de entrada/tokens/timeout/dia/teto monetário reutilizam o bloco de limites
  Plus. Contagem diária é da conta inteira, incluindo explicações com fontes.
  Tabela só é reutilizada para o mesmo par credencial/modelo; preço desconhecido
  com teto monetário bloqueia a chamada. Não se coleta/preenche preço por suposição.
- Migration0316 torna issuer_id opcional no ledger finance.plus_llm_runs para
  task_class=conversation pessoal. source_summary empresarial continua exigindo
  emitente pelo serviço. Finalização NULL-safe e unicidade account/actor/request.
- Reserva antes da rede impede nova chamada paga em retry com reserva existente,
  inclusive se a gravação da conversa falhar depois da rede. Nesse caso a resposta
  não gravada não é reconstruída e o sistema informa tentativa anterior. Ledger
  mantém retenção30dias; não prometer deduplicação eterna de reserva já expirada.

## Contextos e compatibilidade

Empresarial mantém seleção explícita de emitente, ferramentas/fontes e permissões
Plus existentes; gerenciamento de cadastro/revisão só abre pelo botão solicitado.
Não se copiam dados pessoais automaticamente para empresa. Linguagem natural
empresarial ampla e reutilização de clientes recorrentes continuam pendentes.

Históricos IA/Plus antigos não são mesclados ou apagados. Acesso de consulta nos
atalhos “Histórico anterior” e “Histórico de consultas anterior”; novos pedidos
vão à entrada principal ou ao contexto empresarial autorizado. Os endpoints antigos
continuam compatíveis; registro de transição em docs/LEGADO.md. Não declarar fusão
completa desses históricos. Nenhuma expansão do adapter Calendar foi feita.

Voz na entrada principal reutiliza useVoiceRecorder/useLiveDictation: gravar,
pausar/retomar, cancelar, transcrever e revisar antes de enviar. Ditado ao vivo é
opcional e usa o serviço de voz do navegador; áudio usa o endpoint compartilhado
existente. Permissão/transcrição indisponível retorna erro explícito. Não houve
captura real, envio de áudio ou teste do navegador nesta tarefa.

## Validação executada

PASSOU:
- 37testes Vitest: conversa6, controller Plus7, componentes Accounting14, fiscais10.
- Go -race: TestUnifiedConversationChoosesToolsAIAndFallback em PostgreSQL16
  descartável, cobrindo sem IA, IA fake, transcript/replay, resumo sem LLM, falha
  local, preparação sem LLM, ledger pessoal, limite diário e revogação em voo.
- Regressões Go de perguntas/perfil/rascunho, acesso/segredo/isolamento e preparação.
- TestAllMigrationsApply em banco vazio com SkipDataSeeds=true, incluindo0316.
- Regressões Calendar de surface imutável, capabilities e transcrição/revogação.
- Compilação Vue/Prettier, ESLint implementação sem erro; diff --check.
- Vue-tsc global executado:410diagnósticos da dívida conhecida; nenhum nos arquivos alterados.
- Perguntas informativas sobre nota/comprovante/perfil não abrem editores nem mudam acesso.
- Contrato HTTP: planilha de outro escopo404, permissão negada403, sem converter em erro500.

FALHOU: lint Finance mantém6ocorrências preexistentes (portal_probe/session e dois
arquivos de testes), nenhuma nova na implementação. ESLint shell possui aviso de
max-lines preexistente. Ocorrência nova unconvert corrigida e lint reexecutado.
NÃO EXECUTADO: provedor pago, preço/fatura reais, browser/layout, áudio/ditado real,
homologação/emissão, merge de históricos, publicação ou aprovação do cliente.

## Menor roteiro manual

1. Finance → abrir Crow Assistant Plus: sem seletor IA/local nem ficha inicial.
2. Na mesma conversa, “criar nota” → responder campos um a um; recarregar/retomar.
3. Selecionar planilha e pedir “resumo”; conferir totais contra a própria planilha.
4. Com configuração IA autorizada, perguntar “olá”; desativar/indisponibilizar IA
   em ambiente de teste e conferir fallback/consultas locais no mesmo histórico.
5. Testar áudio e ditado, cancelar e trocar conta durante transcrição.
6. Abrir os históricos anteriores; conferir leitura, privacidade e retorno à conversa.
7. Empresarial: selecionar emitente, fontes/pendências e abrir cadastro somente
   quando solicitado. Validar permissão/revogação e comportamento mobile/teclado.

Pendências/aceites em docs/accounting/prompts/CT-15.md; nada marcado aprovado por
inferência. Atualização local e reinício são a última operação, sem inspeção após
reinício, conforme preferência do usuário.

## Paridade visual e Whisper local — 2026-11-06

Referência aprovada pelo usuário: layout Crow original (imagem1). Não criar outro
composer/estilo para Plus. CalendarChatComposer extrai o DOM aprovado de áudio/input
e é usado tanto pelo CalendarChatPanel quanto pelo adapter FinanceConversationVoice.
Reutiliza calendar/chat.css, modos Whisper/Ao vivo, waveform, timer, pausa, descarte,
confirmação e botão enviar com ícone. FinanceConversation usa CalendarChatMessage
e stream/empty state canônicos. Formulários só sob solicitação, sem ficha inicial.

A transcrição Finance canônica resolve provider=local, sem consultar ativação/modelo
de conversa ou chave paga. Usa o webhook existente para o Whisper compartilhado.
WithFinanceTranscriptionAuthorization injeta a checagem viva de membership, módulo
Finance e finance.sheets.view via composição; revalidada após transcrever. Nenhuma
permissão concedida pela UI ou mudança em Calendar/Meta/global para transcrição paga.
Serviço local Whisper ativo/healthy e rota de transcrição configurada foram conferidos,
sem expor URL/env/credencial. Teste de áudio sintético cobre providerlocal/chavevazia
e revogação antes/depois; microfone do usuário e comparação visual não executados.
Não trocar modelo/baixar pesos ou ativar configuração de conta para corrigir áudio.

## Seletores independentes por conta — 2026-11-06

O seletor da página Finance troca a conta da página. O seletor do chat troca
somente o contexto do chat: página em A e chat em B é um cenário suportado.
Ambos usam contas autorizadas de `/v2/me/accounts`, sem agregar todas as contas
nem usar `coreTenantId` como fronteira de segurança. Só aparece seletor para
quem tem mais de uma conta; Finance não contratado permanece indisponível.

`FinanceAssistantHost` conserva uma seleção própria por usuário na sessão,
carrega `/v2/me/context` para a conta escolhida e injeta o contexto apenas na
árvore do chat. Conversa, perfil, documentos, ferramentas empresariais e áudio
enviam explicitamente o `X-Account-Id` capturado. Não chama `switchAccount`, não
altera cookie/conta global nem a planilha da página. Ao trocar, a árvore antiga é
desmontada; respostas tardias são descartadas. A API continua validando conta,
usuário, módulo e permissões; selecionar na UI não concede acesso.

Uma planilha da página só entra no pedido se pertencer à conta do chat. Quando
página e chat estão em contas diferentes, não herdar planilha/valores da página;
a escolha independente de planilha/período pelo chat ainda precisa ser ampliada
(P15-26). Perfis/conversas pessoais continuam por conta + usuário; cadastro de
emitentes não substitui a fronteira de conta e seus grants explícitos.

A janela do Plus e a original usam a mesma área-base e medidas compartilhadas
na sessão por conta. Apenas Calendar persiste seu próprio layout após hidratar;
Finance não faz PUT da configuração Calendar.

QA automatizado cobre contexto privado B sem mutar página A, áudio B com página
A/C e descarte após troca do chat, respostas de contexto fora de ordem, geometria
compartilhada e autorização viva de admin/owner/custom/revogação. Layout no
navegador, microfone real e aceite do usuário continuam pendentes.

Evidência deste ajuste em `docs/accounting/evaluations/PLUS-CLIENTES-2026-11-06.json`:
última suíte frontend42/42; testes PostgreSQL de autorização e conversa passaram;
Vue-tsc global mantém410diagnósticos, nenhum na implementação alterada. Lint Go
Auth/app aponta4ocorrências anteriores em avatar_storage/password_reset_delivery;
nenhuma nos arquivos alterados. Build local da API passou. Sem aceite visual.
