# Acesso ao Emissor Nacional pelo Finance

Primeiro passo: cadastro canônico de portal/método/credenciais, edição no perfil e dentro do Finance, conversa determinística persistente sem IA e teste explícito de autenticação. Não implementar emissão ou dados da nota antes de validar esse acesso.

Credenciais são capturadas em formulário protegido dentro da conversa e gravadas pela API fiscal existente, cifradas. Não pertencem a mensagens, contexto IA, logs ou storage do navegador. gov.br continua sendo entrada assistida; senha do Emissor Nacional e senha gov.br não são intercambiáveis.

O teste por usuário/senha acessa o formulário web oficial e verifica evidência de autenticação; sem API de emissão e sem navegador do Work. Não faz emissão, não contorna CAPTCHA/MFA e não transforma um simples redirect em sucesso. A sessão temporária do teste não é uma sessão compartilhada com o Chrome. Login real só poderá ser aprovado após o usuário cadastrar o acesso e testar.

Conversa sem IA usa intenções delimitadas para acesso e instruções explícitas. Não simula compreensão irrestrita de qualquer pergunta financeira; o chat de IA existente permanece acessível como modo opcional.

## Como testar o primeiro passo

1. Abrir Finance → Crow Assistant. O modo padrão funciona sem chave de IA, com histórico pessoal persistido.
2. Digitar **Quero usar CPF/CNPJ e senha do Emissor Nacional**. O assistente salva endereço/método no cadastro canônico e mostra o acesso dentro da conversa.
3. Preencher CPF/CNPJ e senha nos campos protegidos → **Salvar acesso**. Nenhum login acontece neste momento. Não digitar senha em mensagem comum.
4. Ir ao perfil (ou solicitar **abrir perfil** na conversa) e conferir que o método/acesso salvo é o mesmo. A senha não pode ser lida de volta. Se abrir outra edição já antiga, recarregar para evitar conflito de versão.
5. Voltar ao acesso na conversa e clicar **Testar acesso**. Conferir resultado e horário. Só `authenticated` aprova o primeiro passo. Resultado manual_required/unavailable/rejected não autoriza prosseguir como se o acesso estivesse validado.
6. Alterar acesso no perfil: o Finance deve usar essa versão após recarregar. Teste antigo fica inválido; o backend rejeita versão antiga. Outra conta/usuário não vê conversa, credenciais ou resultado.
7. Com acesso salvo, pedir novamente acesso: não solicitar credenciais de novo. Substituição é opcional. Repetir uma requisição de mensagem não duplica histórico. Teste repetido em10s reaproveita o resultado da mesma versão.

O primeiro passo NÃO deixa uma sessão aberta no navegador do usuário: ele valida o login web no servidor com sessão temporária. Compartilhar uma sessão de navegador e automatizar formulários de emissão continuam como implementação posterior, condicionada a passar neste teste.

O modo IA anterior fica disponível pelo botão Usar chat de IA. O modo sem IA atual atende intenções de acesso, perfil e comprovante; perguntas financeiras abertas ainda precisam do modo IA ou de capacidades determinísticas futuras.

Na implementação inicial não houve teste real de credenciais nem emissão. O diagnóstico real posterior está registrado abaixo. Testes locais usam portal HTTP simulado e Postgres descartável, sem Work/navegador.

## Evidências locais

Go Finance e App passaram, inclusive Postgres descartável `finance_test_access`. Cobertos: portal simulado com cookie/CSRF/formulário, autenticação/recusa/desafio/resposta inconclusiva; escolha do método persistida no cadastro canônico; senha omitida do histórico; idempotência de mensagens; leitura do cofre cifrado; intervalo de teste e invalidação por versão; isolamento de conversa/resultado por conta e usuário. Nenhuma senha real usada.

Vitest: conversa sem IA, perfil, documentos; proteção contra senha explícita na mensagem, retry com requestId, troca de conta e descarte de resposta atrasada. Typecheck global tem dívida preexistente fora destes arquivos; validação visual não executada por orientação do usuário.

Correção de compilação (2026-11-03): handlers de seleção de método e estado de credenciais foram extraídos do template para funções tipadas no script. A combinação de cast TypeScript e múltiplas instruções inline não compilava no Vite apesar dos testes de composables aprovados. `fiscal-components.test.ts` agora compila scripts e templates dos componentes fiscais e do chat para detectar essa classe de falha, sem Work/navegador.

## Diagnóstico do login real — 2026-11-03

Tentativa autorizada pelo usuário, usando o mesmo FiscalService/cofre/cadastro acionados pelo painel. GET de login 200 com token CSRF e formulário; POST 302; página de destino 200 sem formulário de senha e com link de saída. O teste antigo marcava manual_required pela mera presença da palavra captcha no código dessa página autenticada. Isso era falso positivo do nosso classificador, não bloqueio de CAPTCHA nem rejeição da credencial.

Correção: reconhecer evidência de autenticação e distinguir conteúdo executável de desafio apresentado. Regressões cobrem página autenticada e login recusado com biblioteca CAPTCHA, além de desafio explícito. Nenhuma nota emitida, segredo impresso ou senha/cookie persistido em diagnóstico. A sessão do teste continua temporária, sem sessão compartilhada no Chrome.

Após a correção, uma segunda tentativa autorizada confirmou o mesmo fluxo GET200 → POST302 → página autenticada200 e gravou `authenticated` no resultado canônico do painel. Primeiro passo de autenticação validado com acesso real. Harness/binaries de diagnóstico removidos; nenhum original, cookie ou credencial entrou no repositório.
