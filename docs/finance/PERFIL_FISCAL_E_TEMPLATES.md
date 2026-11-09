# Finance — perfil fiscal e templates

Entrega iniciada em 2026-11-02. Escopo aprovado: cadastro persistente no perfil do usuário, acesso protegido ao portal e templates reutilizáveis; integração por API, chat e automação do navegador ficam para entregas seguintes. Não há emissão nesta etapa.

## Implementação

1. Perfil fiscal próprio por `(account_id,user_id)`, acessível apenas pelo usuário autenticado. Cadastro autoritativo na API/banco, editado em `/perfil`; não duplicar em configuração Finance.
2. Acesso ao portal armazenado em tabela separada, cifrado com o cofre AES-GCM já usado na plataforma. API de leitura retorna apenas existência, nunca senha/ciphertext. Salvar o perfil não altera credenciais. Sem login automático nem teste de conexão nesta entrega.
3. Templates pessoais no mesmo escopo, com nome, descrição do serviço, classificações informadas pelo usuário, retenção/ISS quando explicitamente configurados e padrão opcional. CRUD por ID, conferência de versão e isolamento no banco. Não preencher tributação automaticamente.
4. Prévia local aos dados retornados pela API permite conferir o que o template preencheria. Perfil e template são fontes separadas; não copiar documento/endereço/login para o template.
5. Interface com blocos colapsáveis, estados carregando/salvando/erro e proteção contra respostas de outra conta. Finance oferece acesso ao cadastro do perfil.
6. Validar contratos, isolamento entre usuários/contas, criptografia, concorrência, persistência e interface; atualizar documentação.

## Limites e uso posterior

- Dados faltantes permanecem vazios e são apontados; não inventar enquadramento, alíquota ou código fiscal.
- Esta entrega não envia dados ao portal, não guarda sessão gov.br e não emite/cancela/substitui notas.
- A futura automação usará o cadastro salvo, pedirá os dados ausentes e exigirá aprovação do rascunho antes da emissão. Mudanças no rascunho invalidam aprovação. CAPTCHA/MFA exigirão participação do usuário.
- Cadastrar dados fiscais não verifica enquadramento tributário; campos serão informados pelo responsável e reutilizados explicitamente.

## Entrega e validação local

Implementado em `/perfil#fiscal`, com atalho na configuração do Finance. API Go, migration0304 e store Pinia próprios. Os dados são pessoais por usuário e conta ativa. Trocar de conta não reutiliza o cadastro de outra conta.

### O que foi verificado

- Backend Finance + bootstrap da aplicação: testes Go aprovados.
- PostgreSQL descartável `finance_test_fiscal`: migration reaplicável, perfil vazio/incompleto, normalização, revisão desatualizada, isolamento entre usuários/contas, templates padrão exclusivos, edição/exclusão, ausência de alíquota/retenção mantida como null.
- Cofre: ciphertext com prefixov1, decriptação apenas interna ao teste, leitura pública sem login/senha/ciphertext; salvar outros dados preserva acesso; mudar portal invalida acesso anterior; remoção testada.
- HTTP: usuário/conta derivados da sessão, conta forjada rejeitada, userId e senha injetados no JSON de perfil rejeitados, múltiplos objetos JSON rejeitados, inexistência de GET de credenciais.
- Frontend:15 testes Finance aprovados (4 novos de contexto, resposta atrasada e falha de gravação); ESLint sem erros/avisos; typecheck global ainda tem dívida fora desta entrega, sem diagnóstico nos arquivos fiscais/perfil alterados.
- GolangCI-Lint no módulo Finance:0 issues. `git diff --check` limpo. Build completo web não executado, conforme orientação do repositório; API reconstruída localmente.
- UI Chrome: blocos inicialmente fechados; preenchimento e gravação de perfil MEI fictício; cadastro protegido de acesso fictício; criação de template padrão; prévia com emitente do perfil e ISS/retenção não informados; recarga manteve cadastro, acesso salvo e template; campos de login/senha permaneceram vazios após recarga. Nenhum portal foi acessado.
- A janela Chrome ficou inacessível à automação (`cgWindowNotFound`) antes de concluir a remoção pela interface. Editar/excluir/remover acesso e conflitos passaram no backend/DB; esse reteste visual continua pendente. Isso é limite da validação, não aprovação de fluxo não executado.
- Limpeza conferida no banco local:0 perfis,0 credenciais e0 templates fiscais (como antes do QA);2 planilhas e6 linhas originais preservadas. Credencial sintética removida por cascade do perfil QA. Nenhuma senha ou identidade existente foi alterada.
- Sem branch nova, commit, push ou deploy. Alterações permanecem na branch existente `fix/finance`.

### Roteiro para repetir

1. Usar ambiente local ou banco descartável, prefixo QA e usuários de teste. Anotar contagens e IDs antes; nunca alterar dados fiscais existentes.
2. Abrir Perfil → Dados fiscais e NFS-e → Dados do emitente. Preencher dados fictícios, selecionar MEI/empresa/pessoa física, salvar e recarregar. Conferir normalização, campos faltantes e ausência de defaults fiscais inventados.
3. Acesso ao portal: antes de salvar URL deve estar bloqueado. Depois, salvar acesso fictício; recarregar: mostrar apenas status e campos vazios. Editar endereço normal do perfil deve preservar acesso; mudar URL deve invalidar acesso. Remover acesso com confirmação.
4. Templates: criar dois, alternar padrão, editar/cancelar/excluir. Selecionar cada template na prévia e conferir emitente vindo do perfil. Valores não informados permanecem ausentes, inclusive alíquota/retido.
5. Abrir duas abas, salvar uma e tentar sobrescrever/excluir pela versão antiga na outra:409, sem perda da versão nova. Recarregar após confirmação de descarte.
6. Com outro usuário na mesma conta e o mesmo usuário em outra conta, verificar perfil/templates vazios ou próprios; IDs alheios retornam404 e forjar account sem membership é bloqueado.
7. Interromper rede ao salvar, conferir erro e rascunho preservado. Trocar conta durante resposta lenta: a resposta anterior não pode hidratar a nova conta. Navegar com edição não salva deve pedir descarte.
8. Remover somente fixtures anotadas; conferir contagens originais. Não registrar credenciais, payloads de acesso ou ciphertext nos relatórios.

Comandos: `npm test --prefix web -- layers/finance`; `go test ./internal/modules/finance/...` dentro de `back`. Para os testes PostgreSQL, fornecer `FINANCE_TEST_DATABASE_URL` exclusivamente de banco descartável com nome `finance_test*`; sem isso os testes de integração são pulados.

## Emissor Nacional com gov.br e chat (continuação)

O portal inicial é o Emissor Nacional. O perfil tem `portalLoginMethod`; selecionar gov.br oculta o formulário de senha e o backend rejeita seu armazenamento. O link abre a entrada oficial no navegador; não realiza login nem emissão automaticamente. Alterar método de acesso ou URL invalida credenciais de portal anteriores. A preferência solicitada Emissor Nacional/gov.br foi persistida no perfil ativo, sem preencher dados fiscais ausentes.

Integração textual, roteiro e limites atuais em [Chat e automação](CHAT_E_AUTOMACAO.md). O chat ainda não recebe extrato por imagem nem opera o portal. A preparação sem IA hoje cobre cadastro e prévia de templates; rascunhos completos e executor compartilhado entre os dois modos continuam pendentes.
