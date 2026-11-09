# Sessão do portal e preparação para revisão

Escopo: manter uma sessão de Emissor Nacional cifrada no banco por conta/usuário, reutilizar e verificar antes do uso, invalidar ao trocar/remover acesso e preparar o rascunho com dados canônicos para revisão. Nenhuma emissão nesta etapa. Cookies nunca chegam ao navegador do Finance, à conversa, ao contexto IA ou a logs. Sessão limitada no tempo, recuperada pelo servidor após restart e renovada por autenticação quando expirada.

Preparação reutiliza perfil, comprovante e templates. Só preenche campos fiscais com informação explícita ou template escolhido/padrão; pagador/recebedor e pagamento não determinam automaticamente tomador/competência. A conversa sem IA reconhece comandos delimitados para preparar e completar os dados faltantes. A revisão existente continua vinculada às versões, sem autorização genérica de emissão.

Validar por testes locais e HTTP do produto; não usar Work/navegador. Teste real autorizado somente para login/reutilização da sessão e GET de página autenticada, sem submeter formulário de nota.

## Uso

1. Recarregar Finance e abrir o chat. Pedir acesso ao portal e usar **Manter sessão**. O resultado informa a validade e se a sessão está salva; a conexão verifica autenticação antes do uso.
2. Anexar/selecionar o comprovante pelo clipe. Salvar eventuais edições. Pedir **Preparar nota**: o documento selecionado é vinculado à conversa, em vez de escolher silenciosamente um arquivo diferente.
3. Responder somente às pendências. Campos aceitos, um por linha: `Tomador: ...`, `CPF/CNPJ do tomador: ...`, `Competência: dd/mm/aaaa`, `Serviço: ...`, `Valor: 57,58`, `Template: nome cadastrado`. Dados do emitente continuam editáveis dentro do Finance pedindo abrir perfil. Tomador/competência não são inferidos do recibo.
4. O editor relê os dados salvos e abre a revisão. Conferir e **Confirmar revisão dos dados**. Alterações invalidam a revisão anterior. Confirmação registra revisão local, não autorização de emissão.

A sessão é do servidor, não é uma aba compartilhada no Chrome. Ela expira em até30min e pode ser renovada com o acesso já salvo. A preparação não preenche nem submete o formulário fiscal remoto nesta etapa; isso continua dependente do adaptador de emissão e da aprovação específica.

## Contratos

Migration0309: finance.portal_sessions e draft_id em finance.access_messages. GET/POST /v1/finance/fiscal/portal-session; GET /v1/finance/documents/{id}; POST /{id}/prepare com expectedUpdatedAt. Todos exigem membership/módulo/permissão pessoal, sem userId no corpo. Mensagens aceitam draftId validado contra conta/usuário e preservado no histórico. Nenhum cookie aparece nas respostas.

A preparação aplica template padrão somente quando todos os campos de serviço estão vazios. Sempre preserva valores já preenchidos. Retorna rascunho, revisão autoritativa com pendências e estado público da sessão. EmissionAvailable continua false. Cookies e credenciais são armazenados separadamente, cifrados com o cofre existente; a sessão é vinculada à revisão do acesso, e não ao cadastro fiscal inteiro.

## Validação

- Go Finance/App passaram com Postgres descartável: sessão cifrada, recuperação entre instâncias, expiração/renovação, remoção ao trocar credenciais e bloqueio de ressuscitar sessão antiga; exclusão de cookie sem restauração acidental; isolamento entre contas; template/edições explícitas/versões/revisão sem emissão; draftId na conversa e rejeição de comprovante alheio.
-32 testes Vitest passaram em5 arquivos, incluindo9 compilações completas de componentes fiscais/chat; ESLint dos arquivos alterados passou. Typecheck global permanece com erros anteriores fora destes arquivos; nenhum diagnóstico Finance nesta rodada. Sem teste visual pelo Work, conforme pedido do usuário.
- Teste real autorizado: primeira conexão autenticada, sessão cifrada no banco; segunda instância carregou o cookie do banco e autenticou por GET, com `reused:true` e **zero novos POSTs de login**. Um único POST de login no teste completo. Não foram enviados dados da nota, não houve emissão e nenhum segredo apareceu em logs/artefatos. Harness e binários temporários removidos; banco descartável removido.

## Chat progressivo — 2026-11-06

FinanceConversation inicia somente com mensagens, sem reabrir formulário antigo
a partir do histórico. Formulários de perfil, acesso e documento continuam disponíveis
por pedido explícito; “criar nota” conduz perguntas sequenciais no chat. Respostas
comuns após uma conversa de acesso não são ocultadas como credenciais. Senhas
explícitas continuam bloqueadas/omitidas e devem usar o formulário protegido.

Estado da pergunta é a action da mensagem assistente no histórico canônico
(ask:<campo>/issuer:<campo>), vinculado ao draftId, conta e usuário. Dados são
gravados nas APIs/services fiscais/documentais existentes, sem tabela ou migração
nova. Pode iniciar rascunho sem comprovante; chave de criação derivada do request
evita duplicar no retry. Pergunta um campo faltante por vez, reaproveita dados
do rascunho e template padrão. Não consulta cadastro recorrente empresarial nem
conclui a emissão real. Preparação conversacional não tenta login no portal.

“Cancelar” interrompe perguntas e preserva dados salvos. “Abrir formulário da nota”
continua oferecendo o editor/revisão completo sob solicitação. Histórico antigo
que já foi salvo com dados omitidos não pode ser reconstruído por esta correção.
