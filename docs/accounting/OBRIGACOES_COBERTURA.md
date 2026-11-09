# CT-11 — estrutura de obrigações, sem regras reais ativadas

Data: 2026-11-05. Estado: **parcial**. O usuário escolheu estrutura para MEI e
empresas não MEI, com regras a definir depois. Autorizou avançar no desenvolvimento
CT-11; isso não significa aceite do CT-10 nem liberação do piloto.

## Cobertura entregue

| Frente selecionada explicitamente | Código de organização | Disponível agora | Não implementado/qualificado |
|---|---|---|---|
| MEI, DAS | mei.das | Registro de pendência por competência, leitura/histórico | Enquadramento, regra/valor/vencimento reais, guia |
| MEI, declaração anual | mei.dasn_simei | Registro de pendência anual | Preparação/transmissão/recibo oficial |
| Empresa não MEI, impostos | company.tax_review | Organização genérica, sem inferir Simples/tributos | Escolha de regime/obrigações, apuração real |
| Empresa não MEI, declarações | company.declaration_review | Organização genérica de pendências | Obrigações específicas e entrega |

Os códigos são frentes de organização; selecionar uma frente não comprova
obrigatoriedade, enquadramento ou credenciamento. Nenhuma regra, taxa, calendário
ou vencimento é inserido pela migration. Consulta atual informa valor=null,
vencimento=null, pending_data, guideAvailable=false e bankPaymentAvailable=false.
Não exibir imposto zero quando o cálculo está indisponível.

## Regras e cálculos

accounting.obligation_rules mantém versões por conta/emitente/código, vigência,
regime/jurisdição, parâmetros fechados, fonte e revisão técnica/ator. Não há API
pública nem seed para ativar regra. Antes de admitir uma regra real, criar o fluxo
autoritativo de revisão/gestão e qualificar a composição local
ObligationQualification (atualmente ausente no runtime). Essa porta deve validar
contexto documental confirmado, jurisdição, perfil/competência, fonte/revisão e
calendário aplicáveis. Uma confirmação de operador não substitui essas provas.

O único avaliador estrutural é base_percent_fixed_v1, base × percentual/100 + fixo,
com rate decimal até6casas e half_up_2_v1 na saída. Usa big.Rat/big.Int, sem float,
sem fórmulas/código executável recuperados. Regras sintéticas apenas em _test.go;
**não representa fórmula oficial de DAS, Simples ou outra obrigação**. A memória
inclui base, taxa, fixo, fração exata antes de arredondar, total, versão/ID da regra,
perfil e fonte/revisão. Rate, fixo e arredondamento são obrigatórios, sem defaults.

Calendário é mapa explícito de competência→data revisada por versão; não calcula
feriado, dia útil ou prorrogação sem fonte. Ausência de regra/contexto/calendário,
vigência/regime incompatível ou base ausente bloqueia cálculo. Períodos mensais
AAAA-MM e anuais AAAA são suportados como organização; o Finance só aceita vínculo
quando o período da planilha coincide exatamente (associação anual ainda requer
especificação de como distribuir a despesa; não assume um mês).

## Estados, histórico e Finance

pending_data → calculated → reviewed → payment_reported. Revisão é confirmação
separada com RBAC review/grant review. Reabertura explícita exige motivo/referência,
CAS e mantém eventos/versionamentos anteriores. Salvar não sobrescreve obrigação
revisada ou com pagamento informado. Não há estado de guia disponível, conciliado,
declaração entregue ou pagamento bancário implementado. Referência de pagamento é
informação declarada pelo operador; não é comprovante validado ou liquidação.

Identidade de negócio única: conta + emitente + código + competência; chaves de
comando também são idempotentes. Reprocessar com versão antiga conflita, não cria
outra cobrança. Memória/histórico são consultáveis para revisão profissional, sem
alegar entrega oficial. Perfil/regra/valor/vencimento alterados exigem nova revisão.

Finance fornece Service → Store via porta transacional em pgx.Tx. Usa as MESMAS
finance.sheets/lines, com amount NUMERIC(14,2) recebido como decimal string. Teto
financeiro menor que Accounting gera erro, sem conversão float ou truncamento.
Nenhum cálculo fiscal altera a API float legada das planilhas.

Preparar despesa requer obrigação revisada/atual, confirmação, RBAC Finance manage,
planilha aberta no mesmo período e versão autoritativa. Portas usam a mesma
transação de comando/receipt Accounting: falha posterior reverte Finance também.
finance.obligation_links guarda proveniência/identidade. Replay retorna a mesma
linha; linha apagada/alterada, valor diferente ou planilha fechada geram conflito.
Linha manual com mesmo valor exige associação explícita e valor íntegro, sem
sobrescrever dados. Efetivação manual/effectiveDate não confirma pagamento bancário
nem atualiza o estado da obrigação automaticamente. Links lógicos preservados após
exclusão detectam ausência; não recriar a despesa silenciosamente.

## Interface e Crow

No emitente, bloco fechado “Obrigações e pendências”, gated por permissões novas:
consulta paginada, seleção explícita da frente/período e registro de pendência,
valores ausentes visíveis, memória quando houver. Sem botão de pagamento ou guia.
Contexto conta/ator/emitente/permissões invalida respostas antigas; erros preservam
seleção. Testes de componente/controller não são inspeção visual.

Plus: accounting.obligations.read.v1 consulta lista/memória sem IA. “Obrigações” e
“memória de cálculo” roteiam leitura; vencimentos com emitente usam obrigações,
sem emitente mantêm aviso legado de que Finance não tem vencimento por linha.
accounting.obligation.finance.prepare.v1 só por ação estruturada/confirmada,
regra qualificada e permissões; não expõe emissão/pagamento. Não há parser de
linguagem livre que confirme preparação de despesa silenciosamente.

## Fontes iniciais pesquisadas, sem promoção a regra ativa

- [Portal do Empreendedor — serviços para MEI](https://www.gov.br/empresas-e-negocios/pt-br/empreendedor/servicos-para-mei/): catálogo de serviços para mapear DAS/declaração.
- [Receita — declarar apurações mensais do Simples Nacional](https://www.gov.br/pt-br/servicos/declarar-apuracoes-mensais-do-simples-nacional): referência de PGDAS-D para futura cobertura específica, sem presumir que não MEI seja Simples.
- [Portal Simples — PGDAS-D/DEFIS](https://www8.receita.fazenda.gov.br/SimplesNacional/Servicos/Grupo.aspx?grp=5+): referência inicial de serviços, não calendário qualificado do emitente.

Pesquisa de 2026-11-05 não equivale a revisão técnica/jurídica de uma regra. Não
foram fixados valores, alíquotas ou prazos tributários a partir desses resultados.

## Pendências de liberação

- Aceite manual M1 do CT-10 ainda não realizado; não habilitar apuração real antes.
- Confirmar primeiras obrigações/regimes/jurisdição e competências do piloto.
- Gestão/revisão autoritativa de regras e confirmação documental de contexto.
- Calendarização, feriados/prorrogações, fontes, testes oficiais e memórias revisadas.
- Qualificar vínculo Finance por cenário, UI de confirmação/associação/revisão e
  tratamento explícito de ajustes; não recalcular passado silenciosamente.
- Guia, declaração, recibo e conciliação exigem integrações futuras próprias.
