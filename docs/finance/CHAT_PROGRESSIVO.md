# Correção de UX conversacional — 2026-11-06

Estado: código/testes locais entregues; aceite visual do usuário pendente.

- Histórico carrega sem reabrir acesso/perfil/documento de uma mensagem antiga.
- Mensagens comuns após acesso deixam de ser substituídas pelo aviso de credencial.
  O conteúdo já omitido no histórico antigo não é recuperável.
- “Criar nota” inicia rascunho pessoal, sem exigir comprovante e sem abrir formulário.
- Perguntas sequenciais: cliente, CPF/CNPJ, valor, competência, serviço e cadastro
  do emitente faltante. Resposta simples é gravada no campo perguntado; template
  padrão e valores já salvos são reaproveitados. Não infere regime ou papéis fiscais.
- Estado da pergunta persiste na mesma mensagem assistente, conta/usuário/draftId.
  Reinicializar o Service ou repetir request não cria outro rascunho/turno.
- “Cancelar” interrompe perguntas sem apagar dados. “Abrir formulário da nota”,
  “abrir perfil” e pedidos de acesso/anexo mantêm os editores existentes.
- Preparação via conversa não acessa o portal nem emite. Senhas explícitas continuam
  bloqueadas/omitidas; somente o formulário protegido recebe credenciais.
- Mensagens não encolhem para acomodar editor; stream fica dentro da janela e
  composer preserva espaço. Ao abrir editor, a última resposta permanece no início
  da área visível em vez de rolar diretamente para o fim do formulário.

PASSOU: testes Vitest de conversa/compilação fiscal e Accounting; backend -race com
PostgreSQL16 descartável finance_test_chat: perguntas, perfil/rascunho persistidos,
retomada, replay, cancelamento, mensagens comuns após acesso e proteção de senha;
regressões de preparação, acesso e isolamento. Build local API concluído.
ESLint da implementação alterada passou. Lint Go mantém 6 ocorrências preexistentes,
nenhuma nova nos arquivos alterados. Erro unused novo foi corrigido antes da entrega.
NÃO EXECUTADO: navegador, layout real, aceite usuário, portal/modelo/emissão real.
Sem migrations, alteração de senha/seed, conta habilitada, commit/push/deploy.

Teste manual: reabrir chat (histórico sem ficha); enviar “criar nota”; responder
somente o nome do cliente e conferir que aparece no histórico; informar os próximos
campos separadamente; recarregar e continuar; pedir formulário explicitamente.
Verificar desktop/mobile e após trocar conta. Nova jornada empresarial recorrente
continua em CT-15; esta correção cobre o fluxo pessoal mostrado no feedback.

## Pacote 3 — variações, retomada e pedidos compostos (2026-11-06)

Teste manual sugerido (sem IA configurada): “faz uma nota pra mim” → chat pergunta o
cliente; enviar “quanto eu gastei esse mês?” → responde a consulta e lembra a pergunta
do cliente, sem gravá-la como nome; “mostra meu saldo e depois faz uma nota” → pede
para escolher o primeiro; responder o cliente; “deixa pra lá” → mantém dados; “vamos
continuar” → repergunta o CPF/CNPJ. Também: “saldo de outubro” sem mês no chat pede o
período; “como emitir nota fiscal?” continua sendo busca de fontes. Aceite pendente.
