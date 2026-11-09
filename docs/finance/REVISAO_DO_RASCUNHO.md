# Revisão dos dados preparados para NFS-e

Etapa de 2026-11-02. Confirma dados do rascunho e do emitente antes da integração com o portal. Não emite nota nem concede autorização de emissão.

## Fluxo

1. Finance → chat → Enviar comprovante e preparar nota.
2. Preencher/corrigir tomador, competência e serviço; salvar. O emitente continua vindo do perfil fiscal pessoal.
3. Abrir **Revisar dados para a nota**. Conferir emitente, documento, regime informado, município/endereço, tomador, valor, competência, descrição, códigos e ISS/retenção informados. Campos ausentes são explícitos; não deduzir tributação.
4. Completar as pendências do rascunho/perfil e recarregar a revisão. A prontidão aqui é uma validação básica de preenchimento, não uma certificação de elegibilidade fiscal nem a validação completa do portal.
5. **Confirmar revisão dos dados** abre a confirmação canônica da aplicação. O backend vincula o registro à versão exata do rascunho e do perfil.
6. Modificar e salvar qualquer um desses dados exige nova revisão. Trocar de conta/usuário/documento durante o diálogo cancela a aplicação de uma confirmação antiga.
7. **Cancelar confirmação** invalida o registro ativo, preservando o histórico. Repetir confirmação da mesma versão é idempotente.

## Contrato

Migration0307 `finance.document_reviews`, com `account_id`, `user_id`, `draft_id`, versões, hash, snapshot sem credenciais, data de aprovação e revogação. Índice único parcial evita confirmações ativas duplicadas. SQL e serviços filtram por conta e usuário, inclusive administradores.

- GET `/v1/finance/documents/{id}/review` → resumo autoritativo e pendências.
- POST `/v1/finance/documents/{id}/review/approval` → `reviewHash`, `expectedUpdatedAt`, `expectedProfileUpdatedAt`, `confirmed:true`.
- DELETE no mesmo caminho → revoga a confirmação dessa versão.

Membership + módulo Finance + `finance.sheets.view` para autoatendimento pessoal. Recursos alheios404; versão/hash desatualizados409; dados incompletos ou confirmação ausente400. A confirmação usa transação com o mesmo lock de perfil e lock de linha do rascunho para impedir corrida com alterações. Snapshot vem de leitura do servidor, nunca do corpo fornecido pelo navegador.

`emissionAvailable:false` em todas as respostas desta etapa. Uma futura emissão deverá reler versões e obter autorização específica; uma revisão antiga jamais serve de permissão genérica para executar no portal.

## Validação

- Go Finance/App passaram com PostgreSQL descartável `finance_test_review`: migration reaplicável; dados incompletos; idempotência; revogação/auditoria; edição do perfil ou rascunho invalidando confirmação; conflito também no repositório; isolamento entre contas/usuários.
-8testes Vitest de documentos/revisão passaram, incluindo cancelamento, conta alterada com diálogo aberto e falha de gravação; ESLint dos arquivos novos sem erros/avisos.
- UI Chrome local: bloco de revisão abriu para o PDF salvo, exibiu valor, emitente ausente e todas as pendências; confirmação ficou desabilitada. Não foram inventados dados fiscais para habilitar a confirmação no perfil real. Confirmação bem-sucedida/revogação foram verificadas em banco descartável, não com dados reais na UI.
- Nenhuma emissão real, alteração de senha, commit, push ou deploy.

## Próxima dependência

A aba do Emissor Nacional permanece na tela de login. Foi solicitado ao usuário autenticar via gov.br e avisar quando estiver na tela de emissão, sem enviar senha. O perfil fiscal real ainda está incompleto. Sem essa sessão não se validam os campos privados do emissor.

Ainda falta implementar o conector/executor de navegador da aplicação e o adaptador autenticado do portal, além da aprovação específica de emissão e registro do resultado. A ferramenta de navegador do agente de desenvolvimento não é o executor do produto. Esta revisão não resolve essa integração por si só.

Typecheck global executado: permanece com erros anteriores fora do Finance; nenhum diagnóstico nos arquivos Finance desta etapa. API local saudável;2rascunhos e2planilhas preservados,0confirmações reais (perfil incompleto). Banco descartável removido.

## Continuação sem login no portal

Usuário optou por continuar a preparação e autenticar no portal depois. Nenhum acesso ao portal foi realizado nesta continuação.

- Rascunho agora permite editar código do serviço, CNAE, NBS, alíquota ISS e retenção diretamente em bloco colapsável, usando os mesmos campos/API persistidos. `Não informado` continua null; zero e não são valores distintos, nunca defaults inventados.
- **Baixar dados do rascunho (JSON)** refaz GET da revisão antes de gerar uma ficha local. Desabilitado com edição pendente. Mudança de conta/usuário/documento invalida resposta atrasada antes do download.
- Formato `omni.finance.preparation.v1`, propósito `review-only`, `emissionAuthorized:false`. Inclui versões/hash, pendências e projeção explícita de emitente/tomador/serviço. Não inclui credenciais, texto bruto OCR, dados bancários do comprovante nem URL de portal. Arquivo não é NFS-e, XML fiscal ou autorização de execução.
- UI Chrome: edição temporária do código marcou rascunho como pendente; restaurar vazio voltou a limpo sem gravação. Download JSON concluído e arquivo reaberto para conferir formato, valor12, estado incompleto, pendências e ISS/retenção null. Exportação de teste mantida em Downloads para o usuário.
-11testes Vitest passaram (documentos, revisão e exportação), ESLint sem erros/avisos; typecheck global ainda tem dívida externa, sem novos diagnósticos Finance/report-export. Sem alteração de backend nesta continuação, sem commit/push/deploy.

Novos componentes/helpers: `FinanceServiceFields.vue`, `document-preparation.ts`. Reutiliza o helper `downloadText` de `app/utils/report-export.ts`, agora exportado e tipado, sem alterar os consumidores existentes.
