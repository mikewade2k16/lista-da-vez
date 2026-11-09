# Finance — chat e preparação de NFS-e

Atualizado em 2026-11-02. Objetivo: reutilizar o chat existente com configuração e contexto do Finance, preparar dados fiscais persistentes e emitir somente após aprovação de cada rascunho.

## Continuação — 2026-11-03

Finance agora abre em modo de conversa sem IA para cadastro/teste de acesso, perfil e leitura de comprovantes. Credenciais são capturadas dentro da conversa por campos protegidos, usando o mesmo cadastro do perfil. O chat IA descrito abaixo permanece como modo opcional. [Escopo, limites e roteiro do primeiro passo](ACESSO_CONVERSACIONAL.md). O teste de autenticação via formulário web foi validado com acesso real autorizado pelo usuário após corrigir um falso positivo de CAPTCHA; não abre sessão compartilhada no Chrome nem emite nota.

## Sessão persistente e preparação — continuação

Sessão cifrada do portal no servidor, reutilização verificada e preparação conversacional do comprovante selecionado implementadas. [Uso, contratos e evidências reais](SESSAO_E_PREPARACAO.md). Nenhum envio/emissão de NFS-e foi implementado nesta etapa.

## Disponível nesta etapa

- Finance → botão Crow Assistant: chat textual compartilhado com conversas persistidas. Conversas financeiras são pessoais por conta/usuário, inclusive para administradores de agência.
- Chat → Configurar (ou Finance → Configuração → Chat de IA): ativação, credencial do cofre, modelo, instruções, temperatura e memória próprios. Não altera a configuração do Calendar. Sem chave cadastrada, o chat permanece desativado com aviso.
- Administrador transversal pode cadastrar chaves no mesmo painel; gestor financeiro pode selecionar chaves mascaradas existentes, sem administrar o cofre global.
- Contexto de leitura: planilha selecionada, totais, perfil fiscal do próprio usuário e templates. Até50 linhas por seção,10 ajustes por linha e10 templates; truncamento informado, totais originais preservados. Não inclui senhas/ciphertext nem concede ações de escrita.
- Perfil → Dados fiscais e NFS-e: emitente, município, enquadramento e templates persistentes. Não é necessário repetir os dados em cada conversa. Campos ausentes permanecem explícitos.
- Botão **Usar Emissor Nacional com gov.br** preenche endereço e método de acesso; **Salvar dados fiscais** persiste a escolha. Acesso ao portal → **Abrir portal para entrar** abre a entrada oficial. O usuário conclui login gov.br/MFA no navegador. Nenhuma senha gov.br é armazenada ou enviada à IA.

Portal inicial informado pelo usuário: [Emissor Nacional](https://www.nfse.gov.br/EmissorNacional/Login?ReturnUrl=%2femissornacional). A preferência pelo Emissor Nacional/gov.br foi salva no perfil ativo nesta sessão; demais dados fiscais não foram inventados. Alterar portal/método invalida credenciais de portal antigas.

## O que ainda impede chamar a automação de concluída

Não existe executor de navegador fiscal no produto. Abrir o link não conecta o chat à sessão do portal. A ferramenta de navegador disponível ao agente de desenvolvimento não é uma integração instalada na aplicação.

Faltam implementar e validar:

1. Rascunhos pessoais persistidos e edição manual implementados; revisão básica com confirmação vinculada às versões do rascunho/perfil implementada. Falta validar os campos específicos do emissor e obter aprovação específica de emissão.
2. PDF, JPEG/PNG, TXT, CSV e OFX com leitura local e preenchimento de rascunho implementados; extração visual por modelo ainda não implementada. Veja [Comprovantes e rascunhos](COMPROVANTES_E_RASCUNHOS.md).
3. Executor de navegador e adaptador Emissor Nacional na sessão autenticada escolhida pelo usuário, com pausas para CAPTCHA/MFA e falhas recuperáveis. Mapear o formulário após login; não adivinhar seletores de telas privadas.
4. Aprovação vinculada à versão exata do rascunho, invalidada após alterações; proteção contra duplicidade/repetição e registro de resultado/protocolo/documento. Nunca emitir silenciosamente.
5. Fluxo completo com e sem IA: ambos usam o mesmo rascunho, validações e aprovação. Sem IA usa dados estruturados/templates; com IA ajuda a extrair e esclarecer os campos.

Nenhuma nota real foi emitida. Não foi validado login gov.br autenticado nem preenchimento do formulário fiscal. Para continuar a validação real, o usuário precisa preencher o perfil e autenticar no portal; a resposta de IA real também depende de cadastrar uma chave válida. Não solicitar senha gov.br pelo chat.

## Testes reproduzíveis do chat

1. Acessar Finance autenticado. Abrir o botão do assistente; conferir identificação Finance e botão Configurar levando a `/finance?config=ia`.
2. Sem configuração/chave, conferir aviso de desativado e envio bloqueado. Não considerar esse estado uma conversa real validada.
3. Como administrador, cadastrar chave no cofre e selecionar credencial/modelo no Finance. Ativar e salvar; recarregar e conferir persistência. Conferir que Calendar mantém seus próprios valores.
4. Selecionar uma planilha de teste e perguntar pelo saldo/período. Conferir resposta contra o banco e interface, sem alterar a planilha. Trocar planilha e repetir.
5. Criar template/perfil QA e perguntar por campos faltantes. Conferir dados do próprio usuário. Em outra conta/usuário, conferir que perfil e histórico Finance não aparecem. Solicitações forjadas devem retornar404/negação conforme contrato.
6. Navegar de outro módulo com conversa já aberta: conferir aviso de contexto da conversa e usar Nova conversa para Finance. Histórico de outra surface não deve virar contexto fiscal implicitamente.
7. Perfil: selecionar Emissor Nacional/gov.br, salvar e recarregar. Abrir Acesso ao portal: link oficial presente, formulário de senha ausente. Backend também deve rejeitar armazenamento de senha para esse método.
8. Limpar somente fixtures QA conhecidas; preservar perfil real, chaves e planilhas do usuário.

### Evidências desta sessão

- UI Chrome local: login, abertura do Crow Assistant Finance, aviso de desativado, configuração independente, escolha/gravação do Emissor Nacional/gov.br e link oficial sem campos de senha.
-39 testes Vitest aprovados em8 arquivos: Finance, API de chat, rotas e permissões.
- Go Finance/Calendar/Automation/Auth/App aprovados; Finance com PostgreSQL descartável `finance_test_chat`, incluindo conflitos, isolamento, perfil gov.br e configuração independente. Testes de preflight Finance não exigem contexto Calendar. Há teste de limites do contexto sem mutar linhas/totais.
- ESLint dos novos componentes/stores: sem erros. Typecheck global possui dívida anterior fora do Finance; não foi executado build web completo, conforme regras do repositório.
- GolangCI-Lint dos cinco pacotes:7 apontamentos existentes em Auth (avatar, multipart, TLS/SMTP), nenhum nos arquivos novos. Não equivale a lint global limpo.
- Sem credencial IA local: resposta real do provedor não testada. Sem sessão fiscal autenticada/executor: automação de emissão não testada. Não classificar estes fluxos como100%.

## Continuação — envio de comprovante

O chat Finance agora oferece **Enviar comprovante e preparar nota**. A imagem é lida por OCR local e gera rascunho persistente/editável, mesmo sem chave de IA. Detalhes, limites e roteiro em [Comprovantes e rascunhos](COMPROVANTES_E_RASCUNHOS.md). A IA textual recebe os dados lidos para ajudar a preparar a nota; isso não equivale a emissão no portal.

## Revisão e confirmação de dados

Implementada revisão básica persistida, vinculada a versões do perfil e do rascunho. [Contrato e roteiro](REVISAO_DO_RASCUNHO.md). Não é autorização de emissão. Emissor ainda sem sessão autenticada, conector de navegador e adaptador pendentes.
