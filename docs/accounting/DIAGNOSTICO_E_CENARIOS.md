# CT-01 — Diagnóstico guiado e cenários iniciais

Versão funcional: 1. Data: 2026-11-03. Estado: especificação documental concluída; diagnóstico novo não implementado. Base para CT-02, sem definir API ou schema concorrente. Referências: [contratos](CONTRATOS.md), [ordem](README.md) e [fontes](FONTES.md).

## Escopo e origem das informações

M1 permite orientar, identificar pendências e preparar dados; emissão depende dos marcos M2/M3. Município, UF, regime e operações do piloto ainda são desconhecidos. Não escolher fornecedor, cobertura territorial ou tributação antecipadamente.

Toda resposta futura deve distinguir: **desconhecido** (sem valor), **declarado** (informado pelo responsável) e **confirmado documentalmente** (evidência revisada, origem e aplicabilidade temporal identificadas). OCR, template, cadastro salvo e declaração não constituem confirmação documental. Divergência mantém as duas origens e gera pendência, sem escolher silenciosamente uma delas.

Para cada campo, CT-02 deve modelar valor, estado de confirmação, origem/referência, ator, versão e período de aplicabilidade quando pertinente. Evidência deve ter acesso autorizado, localização e responsável pela conferência; não copiar documentos privados para corpus público. Alteração relevante invalida diagnóstico/revisão dependente, preservando histórico. Ausência de informação não equivale a isenção, zero ou dispensa.

Conta e ator vêm da autenticação; emitente é seleção validada. `coreTenantId` e CNPJ não concedem acesso. Perfil pessoal existente pertence a conta + usuário; emitente empresarial terá membros e permissões próprios. Importar valores exige escolha explícita, conferência e referência à versão original. Nunca importar senha, cookies ou sessão pessoal. Dois operadores podem compartilhar dados empresariais autorizados, sem compartilhar acesso pessoal.

## Inventário verificado no código local

| Campo/objeto | Fonte existente | Destino e lacuna |
|---|---|---|
| Tipo, nome e documento do emitente | `FiscalProfile.issuerType/legalName/document` em `fiscal_model.go` | Perfil pessoal; reutilização empresarial explícita, vínculo ainda inexistente |
| Regime | `FiscalProfile.taxRegime` | Declaração pessoal; falta comprovação/versionamento por competência |
| Inscrição municipal | `municipalRegistration` | Pessoal; conferir por estabelecimento e jurisdição |
| Município, IBGE e UF | `municipality/municipalityCode/state` | Pessoal; falta evidência e histórico territorial |
| Endereço e contato | `postalCode/street/number/complement/district/email` | Pessoal; conferir endereço empresarial e da operação |
| Acesso | `portalUrl/portalLoginMethod`; cofre e sessão separados | Acesso pessoal protegido; não é credenciamento empresarial nem capacidade de API |
| Serviço e classificação | `FiscalTemplateFields`: descrição, serviceCode, CNAE, NBS, ISSRate, ISSWithheld, notes | Templates pessoais opcionais; não comprovam regra aplicável |
| Tomador | `DocumentFields.customerName/customerDocument` em `document_parse.go` | Rascunho pessoal explícito; endereço/localidade e detalhes exigidos pelo conector não modelados nesse objeto |
| Competência e valor | `competenceDate/amount` no rascunho | Data civil explícita; valor atual float64, adaptar para decimal exato no novo contrato |
| Comprovante | paymentDate, payer/payee e documentos, extractedText | Origem para conferência; não determina tomador, competência ou autorização fiscal |
| Inscrição estadual, produto e destinatário NF-e | Sem modelo fiscal estruturado nesses objetos | Novo domínio empresarial/documento: IE, itens, unidades, quantidade, destinatário, operação e classificações |
| Finance | `model.go`: planilhas, linhas, ajustes, effective/effectiveDate, totais | Leitura financeira existente; linha não possui vencimento individual estruturado |
| Revisão | `document_review.go`: hash, versões do perfil/rascunho, confirmação/revogação | Revisão local pessoal; `emissionAvailable` permanece false |
| Preparação | `document_prepare.go`: template padrão só com serviço vazio; atualização explícita pela conversa | Preparação de serviço existente, sem submissão fiscal; não é diagnóstico empresarial |

Fonte persistente existente: schema Finance, documentado em [AGENT Finance](../../back/internal/modules/finance/AGENT.md). Documentos de perfil/planejamento têm trechos históricos anteriores à preparação atual; o código atual e a seção de sessão/preparação do AGENT determinam este inventário. Nenhum teste histórico foi reexecutado nesta tarefa.

## Perguntas progressivas

Perguntar uma coisa por vez, oferecer “não sei” e “continuar depois”. Mostrar resumo conferível e motivo da pendência. Retomar do último campo pendente após reler a versão salva e revalidar acesso. Persistência do diagnóstico será implementada em CT-03; CT-04/05 implementam a condução. Campos confirmados não são perguntados novamente sem mudança, conflito ou expiração. Troca de conta encerra o contexto; troca de emitente exige seleção explícita e nova avaliação.

A obrigatoriedade abaixo é funcional: dados condicionais só se tornam obrigatórios quando o cenário/conector os exigir. CT-02 deve representar essas condições; não transformar a lista em formulário universal.

| ID e pergunta | Por que / quando | Formato e validação previstos | Origem indicada / quando pular / sem resposta |
|---|---|---|---|
| Q1 “O que você quer fazer agora?” | Sempre: consultar Finance, pendências, preparar serviço, preparar produto ou buscar regra | Escolha explícita; operação mista abre dois ramos | Usuário; pular só com intenção explícita já fornecida; desconhecido permite ajuda geral |
| Q2 “Para quem é esta operação?” | Antes de usar dados fiscais | Seleção pessoal/emitente autorizado ou solicitação de cadastro; backend valida conta, vínculo e permissão | Lista autorizada; pular seleção já validada; sem vínculo bloqueia leitura privada e escrita |
| Q3 “Qual é o nome e CPF/CNPJ do emitente?” | Cadastro ou identidade incompleta | Nome não vazio; documento conforme formato oficial aplicável, normalização sem perder caracteres válidos e validação adequada ao tipo; formato não comprova identidade | Cadastro/documento oficial por formulário protegido; pular dado vigente confirmado; ausência bloqueia documento fiscal |
| Q4 “Em qual município e UF fica o estabelecimento?” | Cenário fiscal | Nome, UF e código IBGE coerentes por catálogo oficial; endereço com CEP/logradouro/número/bairro/complemento quando exigido | Cadastro oficial municipal/estadual e comprovante autorizado; não inferir por IP; pular confirmação vigente; ausência bloqueia regra jurisdicional e emissão |
| Q5 “Qual a data/competência desta operação?” | Preparar nota ou regra temporal | Data civil válida, entrada dd/mm/aaaa, contrato a definir em CT-02; distinguir competência, emissão, saída e pagamento | Responsável/contrato/documento operacional; pular só se explícita e confirmada; ausência bloqueia cálculo e documento final |
| Q6 “Você sabe o regime válido nessa competência?” | Após identidade e período | Opção declarada ou desconhecido; confirmar evidência e intervalo, sem dedução pelo CNPJ/CNAE | Responsável/contador e consulta oficial pertinente; pular evidência aplicável; desconhecido permite orientação, bloqueia cálculo e emissão |
| Q7 “Quais inscrições e habilitações esse estabelecimento possui?” | Conforme serviço/produto e jurisdição | IM/IE como texto, situação e evidência; isenção/dispensa exige fundamento, não string vazia | Cadastro municipal/SEFAZ; pular apenas exigência não aplicável revisada; ausência pendencia cobertura |
| Q8 “Qual serviço foi prestado e onde?” | Ramo serviço | Descrição, local de prestação, códigos aplicáveis e retenção explícitos; validar domínio e vigência em fonte revisada; percentuais decimais sem defaults | Contrato, cadastro e regra local; template somente selecionado/conferido; pular campos já confirmados; falta impede cálculo/preparação completa |
| Q9 “Quem é o tomador?” | Ramo serviço | Nome, documento, endereço/localidade e identificação exterior quando exigidos pelo cenário; distinguir pagador de tomador | Responsável/contrato; pode confirmar reaproveitamento de parte do recibo, nunca automático; falta bloqueia revisão completa |
| Q10 “Qual produto, quantidade e operação?” | Ramo produto | Itens com descrição, unidade, quantidade positiva decimal, valor unitário, descontos/acréscimos; natureza/finalidade, origem/destino, NCM/CFOP/CST/CSOSN e demais campos somente se aplicáveis e fundamentados | Cadastro de produto e documento operacional conferidos; não escolher códigos por similaridade textual; pular itens confirmados; falta bloqueia cálculo/preparação completa |
| Q11 “Quem recebe e para onde vai?” | Ramo produto | Destinatário, documento, endereço/UF, situação de inscrição, entrega/retirada e transporte/pagamento se exigidos; coerência com operação | Pedido/contrato/documentação; pular dados confirmados; falta impede qualificação do cenário e revisão |
| Q12 “Qual é o valor que você confirma?” | Ambos os documentos | Decimal exato como string no novo contrato; positivo, moeda/precisão explícitas, total coerente com itens e ajustes; não tratar ausência como zero | Contrato/pedido/comprovante conferido; pular valor explicitamente confirmado; divergência bloqueia revisão/cálculo |
| Q13 “Como você acessa o emissor?” | Só após identificar possível integração | Método suportado + status público da configuração, ambiente e credenciamento; URL permitida pelo conector, nunca destino arbitrário fornecido ao modelo | Formulário protegido/cofre; gov.br/MFA no fluxo próprio; pular configuração válida e autorizada; falta não bloqueia orientação, bloqueia execução remota |
| Q14 “Você confirma estes dados para revisão?” | Preparação completa | Resumo com origens/pendências e versões; confirmação explícita do responsável | Usuário autorizado; não pular usando aprovação antiga; sem confirmação mantém rascunho. Revisão não autoriza envio |

Documentos de identidade/cadastro e credenciais não são solicitados no chat: direcionar upload/formulário protegido com escopo e minimização. A conversa pode registrar que há uma pendência, sem reproduzir dados sensíveis desnecessários. Senha, certificado e cookies nunca entram em mensagem, evidência textual ou contexto IA.

## Dois cenários parametrizados

### S1 — Primeira NFS-e de serviço

Parâmetros abertos: emitente/vínculo, estabelecimento/município/UF/IBGE, competência, regime e vigência, inscrição/habilitação, descrição e local do serviço, classificações aplicáveis, tomador, valor e condições tributárias fundamentadas. Q1–Q9 e Q12–Q14 conduzem o cenário. Todos começam desconhecidos; não há alíquota nem código padrão.

Integração será escolhida após confirmar jurisdição, regime, operação, emissor disponível ao estabelecimento, leiaute vigente e método/ambiente de autenticação. Preferência pessoal pelo Emissor Nacional não prova cobertura empresarial. Sem esses dados: orientar e listar evidências faltantes, sem cálculo oficial ou envio. M1 pode preparar dados e revisão local; CT-08 deve declarar caso coberto, homologar adapter e habilitar emissão via motor CT-03.

### S2 — Primeira NF-e de produto

Parâmetros abertos: emitente/vínculo, estabelecimento/UF/município, regime/competência, IE/habilitação, natureza/finalidade e abrangência da operação, destinatário/localidades, itens, unidades/quantidades/preços, origem/classificações fundamentadas, transporte e condições de pagamento quando exigidos. Q1–Q7 e Q10–Q14 conduzem o cenário. NCM/CFOP/CST/CSOSN não recebem defaults; nenhuma regra fiscal é deduzida do comprovante.

A UF, operação e regime confirmados determinarão documentos técnicos, credenciamento, ambiente, certificado/método suportado e primeiro recorte do CT-09. Operação exterior, devolução, substituição ou outro caso sem cobertura declarada fica pendente, sem improvisar cálculo. M1 pode coletar e orientar sobre pendências; preparação estruturada de NF-e e emissão dependem de CT-03/09 e qualificação CT-10. Operação mista mantém documentos/ramos separados e ligação operacional explícita, sem somar automaticamente bases fiscais.

## Primeiras consultas úteis e roteiro M1

| Pedido | Resposta possível hoje / limite | Próxima entrega |
|---|---|---|
| “O que paguei nesta planilha?” | Consultar saídas efetivadas e effectiveDate da planilha autorizada, com ajustes/totais; não afirmar liquidação bancária independente | CT-05 tool determinística; contexto IA atual é limitado/truncado |
| “O que vence esta semana?” | Informar que linhas não têm vencimento individual estruturado; não substituir por effectiveDate ou período | Modelar fonte de vencimento antes de prometer contas a pagar; CT-11 |
| “Quais documentos faltam?” | Revisão existente lista pendências do rascunho/perfil pessoal selecionado, não de toda empresa | CT-03/04 diagnóstico empresarial e lista de pendências por operação |
| “Prepare minha nota” | Serviço pessoal: seleção explícita de rascunho, template e dados faltantes; sem emissão | Novo emitente/fluxo CT-03/04/05; produto depende do recorte CT-09 |
| “Qual regra vale para esta operação?” | Base fiscal curada e resposta com aplicabilidade ainda não implementadas | Pedir emitente, jurisdição e competência; CT-06/07 retorna evidência ou insuficiência |
| “Explique esta evidência” | Sem motor de evidências fiscal compartilhado hoje | CT-07 mostra fonte, trecho/localização, versão, vigência e motivo de aplicação; conflito gera pendência |

Roteiro M1: validar conta/permissão → escolher intenção e emitente → perguntar somente lacunas → salvar/retomar diagnóstico versionado → consultar Finance sem depender de IA → preparar serviço com fontes explícitas → revisar e mostrar pendências → orientar com evidências CT-06/07. Se provedor IA falhar, leitura financeira/diagnóstico determinístico continuam utilizáveis. Aceite M1 por CT-10; este roteiro não declara essas integrações entregues.

## Matriz de capacidades

| Capacidade | Disponível hoje | Falta implementar | Confirmação de dado | Configuração | Homologação |
|---|---|---|---|---|---|
| Orientação financeira | Planilhas/API e contexto textual limitado | Tools Plus determinísticas | Planilha/período | Permissões/módulo; IA opcional | QA M1, sem homologação fiscal |
| Diagnóstico empresarial | Não | CT-03/04/05 | Identidade, vínculo, jurisdição/regime | Acesso ao emitente | QA M1 |
| Orientação fiscal fundamentada | Não | CT-06/07 | Operação/competência e aplicabilidade | Corpus revisado/permissões | QA de evidências M1 |
| Simulação fiscal | Não há motor oficial | CT-03 e regras cobertas | Entradas e parâmetros revisados | Regra versionada | CT-10; fake identificado como simulação |
| Preparação | Serviço pessoal com template/rascunho | Adaptação empresarial; produto | Tomador, serviço, valor, competência / itens | Template opcional; acesso remoto não deve ser requisito conceitual do diagnóstico | QA M1 e cenários de produto separados |
| Revisão | Local pessoal com hash/versões | Revisão empresarial do motor | Snapshot completo e responsável | Permissão própria | QA M1; não equivale a aceite remoto |
| Autorização de envio | Não | CT-03, separada de revisão | Consentimento sobre snapshot vigente | Permissão de emissão | CT-10 M2/M3 |
| Emissão | Não; emissionAvailable false | CT-03/08/09 | Dados completos/cobertura confirmada | Conector, credenciamento e segredos protegidos | Fake local, homologação externa e produção separados |
| Pagamento | Registro de efetivação financeira | Execução de pagamento/obrigações CT-11 | Obrigação, favorecido, valor e autorização | Integração futura | Qualificação própria; efetivar linha não transfere dinheiro |

## Fontes mínimas e limite da verificação

Consulta pública via ferramenta web em 2026-11-03, sem navegador do usuário nem credenciais. Não foi validada nenhuma alíquota, enquadramento ou obrigação concreta. A confirmação do piloto exige selecionar e revisar o documento aplicável; link de portal não comprova vigência.

| Recorte | Caminho oficial e resultado desta consulta | Próximo passo condicionado |
|---|---|---|
| Serviço | [Documentação NFS-e](https://www.gov.br/nfse/pt-br/biblioteca/documentacao-tecnica/documentacao-atual): índice acessível com separação produção/testes/APIs | CT-06/08 selecionar manual, leiaute, versão e ambiente conforme cobertura confirmada; prefeitura/finanças oficial do município informado valida requisitos locais |
| Produto | [Portal NF-e](https://www.nfe.fazenda.gov.br/portal/): redirecionou para página sem texto legível pela ferramenta | CT-06/09 obter MOC/notas técnicas/esquemas pelo portal e SEFAZ oficial da UF informada; coleta pendente, sem substituir por resumo de terceiros |
| Regime declarado Simples/Simei | [Portal Simples Nacional](https://www8.receita.fazenda.gov.br/SimplesNacional/): índice acessível com Consulta Optantes, manuais e legislação | Responsável confirmar situação e período por evidência oficial, sem inferência automática; curadoria de ato aplicável em CT-06 |
| Outros regimes/jurisdições | Caminhos de Receita, Planalto, SEFAZ/CONFAZ e prefeitura em [FONTES](FONTES.md), sem coleta nesta tarefa | Após respostas Q4–Q7, identificar órgão e ato aplicável, publicação/vigência, original/hash e revisão; não escolher UF/município pelo fuso do usuário |

## Revisão de mesa — casos documentais

Resultados abaixo são análise do fluxo especificado, sem execução de software. Todos usam entradas abstratas, sem dados reais.

| Caso | Entrada | Próxima pergunta | Pendência | Operações bloqueadas / resultado esperado |
|---|---|---|---|---|
| T1 Tudo desconhecido | Nenhum dado fiscal | Q1: objetivo | Identidade, vínculo e todos os parâmetros | Orientação genérica possível; cálculo, revisão completa, envio e pagamento bloqueados |
| T2 Perfil pessoal existente | Perfil pessoal salvo, sem empresa vinculada | Q2: usar perfil pessoal ou selecionar emitente autorizado? | Reutilização explícita e confirmação de origem | Sem migração/associação automática; acesso empresarial e emissão bloqueados |
| T3 Dois operadores | Empresa autorizada para A/B; A tem acesso pessoal salvo | Q1 para B, usando somente empresa permitida | Permissão específica de revisão/envio e configuração empresarial | B não lê credencial/sessão de A; sem herança de emissão pela permissão Finance |
| T4 Serviço | Emitente autorizado, serviço declarado, município desconhecido | Q4: município/UF | Jurisdição, regime e período aplicável | Preparação parcial; sem tributação/default nem envio; seguir S1 |
| T5 Produto | Emitente e UF confirmados; itens sem classificação | Q5/Q6 se pendentes; senão Q10: produto/operação | Classificações revisadas, destinatário e cobertura | Não usar template de serviço; sem cálculo/envio; seguir S2 |
| T6 Mista | Mesmo pedido com produto e serviço | Q1: identificar cada operação e respectivo documento | Separação dos ramos e valores confirmados | Sem converter tudo em NFS-e ou NF-e; sem dupla contagem automática |
| T7 Mudança temporal | Município/regime mudaram; operação anterior à mudança | Q5: competência e data de efeito de cada mudança | Evidência histórica aplicável | Não aplicar cadastro mais recente retroativamente; revisão/cálculo/envio pendentes até resolver |
| T8 Documento insuficiente/conflitante | Recibo com CPF mascarado, pagador distinto, dois valores | Q9: tomador explícito; Q12 em seguida | Documento completo protegido, valor e competência confirmados | Não reconstruir CPF nem escolher pagador/valor/data por inferência; revisão completa/envio bloqueados |
| T9 Retomada concorrente | Diagnóstico editado por outro operador | Mostrar versão atual e solicitar reconciliação | Conflito de versão | Sem sobrescrita; snapshot anterior não autoriza operação |
| T10 Fonte indisponível | Regime declarado, sem evidência acessível | “Você possui documento oficial verificável no formulário protegido?” | Origem, vigência e revisão | Orientação com insuficiência explícita; sem regra inventada ou imposto zero |

## Entrega para CT-02

Modelar separadamente perfil pessoal, emitente/membros, diagnóstico versionado e suas evidências, operação/competência e snapshots de documento/revisão/autorização. Decidir referência/adaptação dos recursos Finance sem duplicá-los. Formalizar formato de datas por operação, decimais, estados de pendência/confirmação, permissões e invalidação por alterações/revogação. Não implementar enums fiscais extensivos antes do recorte confirmado.

Critério de conclusão CT-01: campos com origem/validação e ausência definidos nas perguntas; ambos os cenários parametrizados; matriz sem confundir preparação/revisão/emissão/pagamento; dez casos de mesa e roteiro M1. Dados reais do emitente continuam desconhecidos por desenho e não bloqueiam a modelagem documental CT-02.
