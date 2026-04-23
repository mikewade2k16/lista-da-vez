# Guia de Treinamento por Papel

## Objetivo do sistema

O Omni existe para organizar a lista da vez da loja, registrar o atendimento de forma consistente e transformar a rotina comercial em dado confiavel.

Na pratica, ele ajuda a equipe a:

- evitar conflito na ordem de atendimento
- registrar compra, reserva e perda com padrao
- entender o que o cliente buscava e como ele chegou ate a loja
- dar visao em tempo real para gerente e proprietario
- criar historico para relatorios, auditoria e melhoria operacional

## Regras gerais

- cada consultor deve usar sua propria conta
- o computador fixo da loja deve usar apenas a conta `store_terminal`
- a senha temporaria inicial deve ser trocada no primeiro acesso
- quem nao tiver escopo da loja nao deve operar a fila daquela unidade
- o fechamento do atendimento deve ser feito com cuidado, porque ele alimenta ranking, relatorios e inteligencia

## Consultor

### Para que serve

O consultor usa o sistema para entrar na fila, atender clientes, pausar quando necessario e encerrar o atendimento com os dados corretos.

### O que pode fazer

- entrar na fila
- sair para pausa e voltar
- iniciar atendimento
- atender fora da vez quando a operacao permitir
- encerrar atendimento com desfecho
- preencher produtos vistos, produtos fechados, motivo da visita, origem e perda
- acompanhar seu proprio painel e metas
- editar o proprio perfil e trocar senha

### O que nao deve fazer

- alterar configuracoes da loja
- cadastrar usuarios
- mexer em lojas de outras unidades

### Por que isso e importante

- manter a lista justa entre os consultores
- evitar perda de informacao no fechamento
- gerar historico confiavel de atendimento
- alimentar relatorios e ranking com dados reais

### Fluxo recomendado

1. entrar na fila ao iniciar o turno
2. iniciar o atendimento quando chegar sua vez
3. encerrar no modal preenchendo o que de fato aconteceu
4. conferir se voltou para a fila corretamente
5. usar `/perfil` para trocar senha temporaria e manter seus dados atualizados

## Gerente

### Para que serve

O gerente acompanha a operacao da propria loja, corrige desvios do dia e apoia a equipe sem perder visibilidade da fila.

### O que pode fazer

- acompanhar a lista da vez da sua loja
- acompanhar quem esta em fila, pausa e atendimento
- tirar consultor da fila para tarefa ou reuniao quando a operacao pedir
- acessar relatorios e ultimos atendimentos da sua loja
- acompanhar configuracoes e operacao da unidade conforme o escopo liberado
- usar o proprio perfil e trocar senha

### O que nao deve fazer

- operar lojas fora do seu escopo
- usar conta de consultor para gerencia
- compartilhar credenciais pessoais com a equipe

### Por que isso e importante

- dar suporte rapido sem quebrar a disciplina da fila
- auditar fechamentos e entender perdas
- agir sobre gargalos de atendimento no mesmo dia

## Proprietario

### Para que serve

O proprietario acompanha o desempenho do grupo, gerencia lojas e acessos e valida se a operacao esta rodando do jeito esperado.

### O que pode fazer

- alternar entre lojas do proprio tenant
- abrir a operacao em modo `Todas as lojas`
- filtrar uma loja especifica dentro da visao integrada quando quiser focar
- acompanhar operacao, relatorios e ultimos atendimentos
- enxergar qual consultor pertence a qual loja diretamente nos cards da operacao integrada
- tirar consultor da fila para tarefa ou reuniao quando estiver atuando na operacao
- criar, editar, arquivar, restaurar e remover lojas quando permitido
- administrar usuarios e acessos
- gerenciar consultores pela aba de consultores quando o acesso estiver vinculado ao roster
- gerar convite de onboarding
- definir senha inicial temporaria
- resetar senha temporaria quando necessario
- acompanhar configuracoes e padroes operacionais

### Por que isso e importante

- controlar rollout de novas lojas
- liberar acesso certo para a pessoa certa
- reduzir risco operacional por uso indevido de conta
- manter padrao entre lojas do grupo

## Admin Dev

### Para que serve

O admin dev e o papel interno da plataforma. Ele existe para suporte, homologacao, diagnostico e evolucao controlada do produto.

### O que pode fazer

- acessar tudo que os demais papeis acessam
- cruzar tenants quando necessario
- abrir a operacao integrada multi-loja para auditoria operacional
- validar modulos novos antes de liberar para clientes
- usar areas internas de teste e diagnostico
- apoiar incidentes e auditoria tecnica

### Cuidados

- nao usar esse acesso na rotina normal da loja
- nao operar a fila real sem necessidade clara
- registrar mudancas estruturais e comportamento novo nos AGENTs e docs

## Acesso da loja

### Para que serve

O `store_terminal` e a conta fixa do computador da loja. Ela existe para operar a fila da propria unidade a partir do computador compartilhado da equipe, mantendo a execucao do salao sempre disponivel.

### O que pode fazer

- ver a operacao da propria loja em tempo real
- acompanhar fila, pausas e atendimentos
- colocar consultor na fila
- tirar consultor para tarefa ou reuniao
- pausar e retomar consultor
- iniciar e encerrar atendimento
- consultar `consultor`, `ranking`, `dados`, `inteligencia` e `relatorios` da propria loja

### O que nao deve fazer

- operar lojas fora da propria unidade
- usar a conta fixa para configuracoes administrativas
- acessar areas administrativas como `multiloja`, `usuarios`, `configuracoes` e campanhas
- compartilhar esse acesso fora do computador da loja

### Por que isso e importante

- garante continuidade da operacao mesmo em dispositivo compartilhado
- reduz improviso com login pessoal em maquina de salao
- preserva o escopo da loja sem abrir administracao desnecessaria

## Boas praticas de treinamento

- treinar cada papel usando a propria conta
- obrigar troca de senha no primeiro acesso assistido
- mostrar o motivo de cada campo do fechamento, nao so “onde clicar”
- reforcar que o modal de encerramento alimenta relatorio, auditoria e melhoria de processo
- revisar com a equipe a diferenca entre conta pessoal e conta fixa da loja

## Checklist rapido de onboarding

- usuario recebeu o papel correto
- usuario recebeu a loja correta
- senha inicial foi trocada
- foto, nome e email estao corretos em `/perfil`
- consultor conseguiu entrar na fila e encerrar um atendimento de teste
- gerente/proprietario conseguiram abrir relatorios e ultimos atendimentos
