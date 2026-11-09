# Accounting — CT-03

## Registro obrigatório de pendências — CT-15

Antes de encerrar qualquer etapa do módulo, atualizar
[CT-15](../../../../docs/accounting/prompts/CT-15.md) com pendências discutidas e não
resolvidas, incluindo MEI/não MEI, emissão/RAG, testes de código/usuário/layout/
funcionalidade e otimizações. Preservar IDs, origem, estado e evidências; não apagar
itens adiados nem confundir teste automático com aprovação do usuário. Ao resolver,
registrar o resultado no item e no owner/handoff correspondente. Manter o backlog
exportado sincronizado; isso não autoriza escrever na aplicação Tasks nem publicar.

## Prioridade vigente — decisão do usuário em 2026-11-04

Ler [escopo inicial](../../../../docs/accounting/ESCOPO_INICIAL.md) antes de
retomar os prompts. Foco central refinado no CT-10: emissão de notas de serviços pelo chat quando
solicitada, reaproveitando emitente/clientes/serviços/valores recorrentes salvos e
perguntando só as lacunas. Cadastro empresarial de clientes/recorrências e envio
real ainda exigem implementação/qualificação; identificação não determina serviço
ou tributação. Também contempla comprovantes externos (Uber, pagamentos, compras),
despesas, orientação sobre lucros/tributação legal e assistência a declarações. **Produtos, catálogo, ERP/estoque e CT-09/NF-e de
mercadorias estão adiados**, fora do escopo inicial. Não gastar implementação
nessa frente por seguir automaticamente a numeração dos prompts.

Comprovante recebido não gera automaticamente nota própria; identificar a operação
e o objetivo, preservar o original e exigir revisão dos dados extraídos. Não
confundir ingestão de conhecimento CT-12 com comprovantes privados. Esses fluxos,
cálculos e declarações ainda precisam de contratos/aceite; não anunciá-los prontos.
Qualificar M1 no CT-10 sem esperar M3, registrando os gaps do escopo atualizado.
CT-11 deve considerar as obrigações efetivamente escolhidas e suportadas.
Continuam válidos isolamento, fontes vigentes, contexto confirmado e pendências
CT-08. MEI e não MEI, início em Sergipe, sem inferir regime ou município.

Domínio backend de emitentes, grants por emitente, diagnóstico, documento fiscal,
revisão e motor de execução. Schema `accounting`, migration
`0310_accounting_foundation.sql`, rotas `/v1/accounting/*`.
Contrato documental v1: `docs/accounting/CONTRATOS.md` e `CONTRATOS_API.md`.

## Camadas e acesso

`http.go` converte Principal em Scope e faz decode estrito/limites; Service aplica
política e regras; `store*.go` contém SQL parametrizado e escopado. Transações
são coordenadas pelo domínio, com operações de repositório tipadas. Não escrever
SQL de Finance/Calendar/Omnichannel aqui. Conta/ator nunca vêm do body.

Host Policy é injetada em `app/accounting.go`: membership ativa explícita,
Accounting habilitado e RBAC account atual. Mesmo admin/agência precisa de vínculo
ativo no emitente. Grants se somam ao teto RBAC, nunca o substituem. API não concede
emissão a `finance.sheets.view`; revisão tem permissão própria. Concessão de grant
exige permissão correspondente do concedente; usuário-alvo deve ser membro ativo.

Criação vincula o criador ao emitente. Perfil tem versões imutáveis; o cadastro
continua declarado, sem inferir enquadramento nem confirmação documental. Answers
aceita unknown/declared com origem; confirmed exige EvidenceVerifier do owner de
conhecimento. Sem verifier retorna evidence_verification_unavailable; implementação
real é CT-06, nenhum verifier fake é ligado em runtime.
Diagnóstico não promove resposta silenciosamente ao perfil. Preparação usa perfil e
campos explícitos e informa pendências. Domínio não depende de IA.

## Rotas

- GET/POST issuers; GET/PATCH/DELETE issuers/{issuer} (delete lógico/versionado).
- PUT issuers/{issuer}/members/{userId} (grants/status/expectedVersion).
- GET/PUT issuers/{issuer}/diagnosis; GET com diagnosisId seleciona, sem ele lista.
- POST issuers/{issuer}/documents/prepare; GET lista/detalhe; PATCH/DELETE documento.
- POST documents/{id}/validate; GET review; POST reviews; DELETE reviews/{reviewId}.
- POST documents/{id}/authorizations e execute; POST authorizations/{id}/revoke.
- GET executions/{id}, GET history paginado; POST recover-artifacts.

Todos os caminhos após o primeiro prefixo estão sob `/v1/accounting/issuers/{issuer}`.
Escritas exigem Idempotency-Key ASCII 8–200 e expectedVersion; leitura de recurso
alheio/inexistente retorna404; CAS/idempotência/snapshot conflitantes409. Lista usa
page/limit (20 padrão/100 teto). Bodies64KiB, decode estrito; moeda string exata
NUMERIC(20,2), sem float64. Não existe cálculo fiscal oficial nesta fundação.

## Revisão e execução

content_version muda com edição dos dados, enquanto version acompanha também
transições. Hash servidor vincula conteúdo, origem, versões do perfil/diagnóstico e
conector/ambiente; editar revoga revisão/autorização, inclusive se repetir valores.
Validação sem conector é estrutural/local; não comprova cobertura tributária.
Preparação, revisão e login pessoal nunca autorizam emissão. Conectores reais
indisponíveis retornam coverage_unavailable. Nenhum fake é registrado em produção;
o fake existe exclusivamente nos arquivos de teste e responde simulated=true.

Autorização de envio explícita, validade15min, invalidada por mudança/revogação.
Execute cria comando+evento+outbox+receipt atomicamente via jobs.EnqueueTx.
Worker revalida permissão/grants/versões/capability antes de consumir autorização.
Rede fica fora da transação SQL; Capability/Validate de conectores são validação
local, sem chamadas remotas. Submit/Lookup/FetchArtifacts são as portas remotas.

Lease90s e chamadas remotas limitadas a60s, fence monotônico, persistência de
resultado condicionada à posse. Recuperação processing expirada/unknown sempre
consulta; ausência remota não permite novo Submit. Até10 claims de reconciliação;
ao esgotar, permanece unknown visível, sem reenvio. Sweep limitado20 a cada30s
recupera crash sem depender do estado do job. Erros não guardam resposta bruta.

Artefatos originais vão à porta privada Storage, hash/metadados no banco. Falha
mantém authorized+artifactsPending, recuperação não reenvia nota. Cancelamento e
substituição fiscais não estão implementados; excluir rascunho não cancela nota.

## Fonte pessoal e compatibilidade

Facade owner Finance lê somente perfil/rascunho/template do próprio ator, com
versões e consentimento explícitos. Projeção selecionada inicial: descrição e
serviceCode/CNAE/NBS; sem senha, sessão, OCR bruto ou alíquota float importada.
Origem persiste ator/seleção/versões. Outros operadores veem somente snapshot
empresarial compartilhado; verificação de obsolescência usa a porta do owner.
Reimportar é PATCH explícito com nova source e invalida revisão. API Finance e
config/chat anteriores permanecem separados e funcionam com Accounting desligado.

## Testes

`ACCOUNTING_TEST_DATABASE_URL` deve apontar a banco descartável `accounting_test*`.
Fixtures sintéticas; migração reaplicada duas vezes; testes de isolamento, FK/CAS,
concorrência, rollback, restart, revogação, timeout, fencing, artefatos e HTTP.
`FINANCE_TEST_DATABASE_URL=.../finance_test*` habilita regressões/projeção pessoal;
Jobs usa TEST_DATABASE_URL e tabela efêmera, em banco descartável separado.
Não habilitar conector real nem módulo na conta do usuário para rodar os testes.

Bootstrap issue: `enableIssue` na criação é opcional/false; true exige RBAC issue.
Sem qualquer emissor ativo, PUT members permite bootstrap com confirmed:true,
RBAC issue e manage no emitente. Com emissor ativo, exige issue do concedente.
Não confundir essa confirmação de grant com revisão/autorização de documento.

## Porta Plus — CT-05

Module.Service expõe o serviço existente para composição; autorização de emitente,
leitura/diagnóstico/preparação/revisão permanece neste owner. Plus não cria
autorização fiscal nem execução. Contrato consumido em platform/app/finance_plus.go.

## Conhecimento fiscal — CT-06

knowledge_service/http fornecem adapter para serviço neutro; policy valida
accounting.knowledge.view/manage e grant knowledge_view/knowledge_manage do emitente.
Módulo inicializa a capacidade sem exigir Omnichannel ou papel de atendimento.
Rotas sob issuers/{issuer}/knowledge: GET/POST bases; PATCH bases/{base};
GET bases/{base}/documents (page/limit); POST imports; PUT documents/{id}/review;
POST documents/{id}/publish e deactivate; POST search; GET evidence/{document}/{chunk}.
JSON estrito até3MiB (arquivo base64 até2MiB), CAS expectedVersion. Publicação requer
confirmed:true após revisão. IDs alheios404, versões conflitantes409. Não cria
autorização fiscal. Body não concede contextConfirmed; resultados continuam
informativos enquanto não há contexto confirmado do emitente.

## NFS-e e download privado — CT-08

connectors/nfse implementa adapter local por portas. Não foi registrado em runtime;
sem pacote oficial/Swagger/certificado/preparação assinada/cobertura qualificados,
emissão permanece indisponível. Ver docs/accounting/NFSE_COBERTURA.md. Não alterou
motor/worker/autorização/receipts nem adapter NF-e. Fixtures exclusivamente sintéticas.

Extensão mínima coordenada do owner: ArtifactReader opcional + DownloadArtifact,
rota GET issuers/{issuer}/executions/{id}/artifacts/{artifact}. Reutiliza tabela
artifacts e Storage; exige accounting.documents.view e grant view, SQL por conta/
emitente/execution/artefato, tamanho<=25MiB/hash e revalidação após I/O. Nunca manter
transação enquanto baixa storage. Resposta attachment/private no-store/nosniff;
sem objeto/key/URL de storage no payload. Leitor ausente503, recurso alheio404,
original corrompido502 sem bytes expostos. XML/PDF originais não são alterados.

App injeta leitor Storage source=accounting, sem novo cofre nem blob no banco.
Recibo técnico do adapter local não é protocolo fiscal; DANFSe não implementado.
Homologação real/produção exigem etapas próprias; estado parcial em handoffs/CT-08.md.

## Pendências para retomar o CT-08 — 2026-11-04

**CT-08 permanece parcial. Não habilitar emissão real nem registrar o connector
no runtime apenas porque os testes locais passaram.** Adapter, reconciliação e
download foram testados com HTTP/PostgreSQL locais e fixtures sintéticas.

- [ ] Definir o primeiro estabelecimento/município do piloto em Sergipe e confirmar
  regime, vigência, credenciamento e integração aplicável. O produto deve contemplar
  MEI e empresas não MEI em outras localidades; não fixar UF/regime nem inferir
  Simples Nacional por estar acima do limite do MEI.
- [ ] Obter e revisar o pacote XSD e Swagger oficiais completos. A coleta do ZIP
  retornou403 e o Swagger não ficou acessível. Conferir versões, hashes, campos,
  códigos, formato JSON e identificação DPS; não contornar a proteção. O teste XSD
  atual usa schema sintético, sem comprovar conformidade oficial.
- [ ] Integrar configuração autoritativa por conta/emitente e o producer canônico
  de DPS assinada/manifesto imutável, vinculado às versões do documento/perfil/config
  e ao original privado antes da revisão/autorização. Não inventar série/número pelo
  UUID/pagamento nem criar outra coleção de documentos/cofre.
- [ ] Integrar certificado pelo cofre empresarial, com acesso restrito, expiração,
  cadeia/revogação e representação verificadas. Implementar e qualificar assinatura
  e verificação XMLDSig reais. Login/cookies pessoais Finance não substituem isso.
- [ ] Disponibilizar o executor de validação offline e os schemas revisados na API;
  repetir fixtures contra XSD e assinaturas oficiais. Não presumir que xmllint
  disponível no Mac também exista na imagem da API.
- [ ] Realizar homologação externa com cenário, credencial e autorização específicos;
  registrar resultado/artefatos sanitizados e qualificar M2 no CT-10. Só depois
  avaliar registro do connector e aprovação separada de produção.
- [ ] Qualificar DANFSe/PDF e eventual protocolo fiscal oficial quando suportados.
  Hoje há XML e recibo técnico no fluxo simulado; recibo técnico não é protocolo
  fiscal. Outros formatos de identificação/representação exigem cobertura própria.

Referências para retomada: [handoff CT-08](../../../../docs/accounting/handoffs/CT-08.md),
[cobertura e decisão técnica](../../../../docs/accounting/NFSE_COBERTURA.md),
[manifesto da coleta](../../../../docs/accounting/nfse/fontes.json) e
[AGENT do adapter](connectors/nfse/AGENT.md). Nenhuma dessas pendências autoriza
emissão real, contratação de fornecedor, alteração de credencial ou deploy.

## Qualificação CT-10 — 2026-11-04

Aceite integrado parcial em [ACEITE_PILOTO](../../../../docs/accounting/ACEITE_PILOTO.md).
Testes locais passaram; nenhuma emissão real ou liberação de produção. Dois casos
novos em pilot_acceptance_test.go comprovam perguntas só sobre campos ausentes e
retomada persistida sem autorização de envio; não implementam chat livre ou cadastro
de clientes recorrentes. Próximas lacunas: reutilização empresarial de clientes/
serviços/valores, preenchimento conversacional e conclusão CT-08. O container web
em uso ainda não possui a nova interface; API local não serve as rotas Accounting.
Atualizar o ambiente pelo fluxo local antes de alegar que a interface está visível.
RAG atual é busca/resumo limitado, sem promessa de cobertura tributária universal.

### Aceite do usuário — 2026-11-05

Usuário ainda não testou nem aprovou as funcionalidades. CT-10 permanece em
qualificação; testes automáticos não substituem aceite. A retomada prepara o
ambiente local e roteiro de teste em docs/accounting/ACEITE_PILOTO.md, sem prometer
emissão real ou cadastro recorrente completo. Não prosseguir presumindo M1 aceito.

### Complementação técnica CT-10 — 2026-11-05

Migrations completas/Calendar proposals/PDF com Poppler/XSD sintético com xmllint
e cofre secretbox passaram;89testes frontend passaram. Ver complemento do aceite.
Isso fecha verificações técnicas antes puladas, sem qualificar schema/assinatura
fiscais reais. CT-10 continua parcial: aceite manual do usuário não realizado e
emissão real pendente CT-08. Não interpretar a conclusão dos testes como aprovação.

## Obrigações — CT-11 (2026-11-05)

Estrutura MEI/não MEI, regras reais a definir por escolha do usuário. M1 ainda sem
aceite; continuidade técnica autorizada não libera piloto. Migration0314 adiciona
obligation_rules/obligations/obligation_events e link canônico no owner Finance.
Rotas obligations + history/review/reopen/prepare-finance/report-payment, CAS e
idempotência; RBAC obligations.view/manage/review e grants correspondentes.

Nil ObligationQualification no runtime bloqueia cálculo real; não ativar com um
callback que apenas confia no body/banco. Exige contexto confirmado e regras/fontes/
calendário revisados. Não há gestão/ativação de regra pública, taxa ou calendário
seedado. Avaliador decimal fechado e regra sintética só nos testes. Valor/data
inexistentes=null, nunca0por falta de configuração. Ver docs/accounting/
OBRIGACOES_COBERTURA.md e handoffs/CT-11.md.

Vínculo Finance por porta transacional; código Accounting não lê tabelas Finance.
Preparar exige obrigação revisada e confirmação; host valida Finance manage.
Pagamento informado não é conciliação; não inferir liquidação de effective=true.
Histórico/reabertura explícita preservam snapshots. Sem guia/declaração transmitida,
pagamento real ou mudança no motor fiscal. Tools Plus read/preparar específicas.

## Host de ingestão Knowledge — CT-12

Reutiliza o worker/outbox existentes: knowledge.ingest.v1 e IngestionTick no ciclo
já gerenciado pelo handle. Novas rotas knowledge/ingestion/catalog, subscriptions
(GET/PUT) e runs(GET paginado), mesmos grants/RBAC knowledge.view/manage. Body não
aceita URL/conta/ator; origem só catálogo do servidor, opt-in por emitente/base.
AuthorizeKnowledge normaliza erro de domínio para Knowledge.Error, preservando
code/status e permitindo distinguir revogação de indisponibilidade temporária.
Nenhuma fonte/conta foi habilitada. Não registrar ingestão como publicação/regra
confirmada; detalhes no AGENT Knowledge e docs/accounting/INGESTAO_E_BUSCA.md.

## Ciclo de regras de obrigação — CT-15 pacote 7 (2026-11-06)

Migration0321: versões de `accounting.obligation_rules` imutáveis (trigger) e
`obligation_rule_events` append-only (activate/retire, ator, motivo). Estado derivado do
último evento: draft/active/retired. `obligation_rules.go`: POST/GET
`/v1/accounting/issuers/{issuer}/obligation-rules`, POST `/{rule}/activate|retire`
(accounting.obligations.manage, grant manage, Idempotency-Key). Criação valida código,
vigência, fonte https, referência de revisão e parâmetros do avaliador; ativação exige
ator diferente do revisor técnico (quatro olhos) e nenhuma versão ativa sobreposta.
Cálculo usa somente versão ativa, preferindo a que cobre a competência. Nenhuma regra
real foi cadastrada; ObligationQualification continua ausente no runtime.
