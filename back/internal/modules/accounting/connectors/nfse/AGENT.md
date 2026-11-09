# Adapter NFS-e nacional — CT-08

Estado: parcial/prototype local; **não registrado no produto**. Ver
`docs/accounting/NFSE_COBERTURA.md` e `handoffs/CT-08.md`. XSD/Swagger oficiais não
obtidos integralmente; fixture é sintética e não demonstra conformidade fiscal.

Implementa accounting.Connector. Não criar worker, receipt, autorização fiscal,
sequenciador DPS ou cofre concorrente. Motor CT-03 continua fonte de estado/fila.
Config/Manifest/Credential são portas do servidor; atualmente não há producer/config
canônicos registrados para este adapter. Sem portas/validação/cobertura, capability
false. Não preencher ausência com mock em runtime.

Config deve vir do banco por conta/emitente, com versão, município/regime/cobertura,
ref de cofre e contrato wire revisado. Manifest precisa ser imutável/durável antes
revisão/autorização: documento, content/profile/config versions, DPSID, hash/objeto
privado e FieldsDigest. Reader de Manifest/Config/Credential deve ser local (nenhum
HTTP dentro de Capability/Validate, pois o motor pode estar em transação).

XML assinado original é lido somente fora da transação, conferido por hash, schema,
assinatura, escopo e projeção do documento. Assinatura/ICP/representação exigem
implementação qualificada do owner; VerifySignature é porta obrigatória, não um
verificador fake em runtime. Certificado não é serializado/logado; provider do
cofre deve verificar cadeia/revogação/representação e limitar acesso por emitente.
Sessão/login pessoal Finance não concede acesso a esta API.

HomologationAuthorized permite o teste externo explicitamente autorizado no
ambiente de homologação; produção exige Homologated e ProductionApproved separados.
Nenhum desses campos cria autorização de documento. New não aceita fake_local;
a chave privada simulated é ativada apenas pelos helpers _test.go.

Transporte: bases HTTPS fixas, mTLS, TLS>=1.2, sem redirect/retry; POST uma vez.
Erro/timeout/HTTP200 vazio/404 lookup são unknown. Só XML/chave/identidade coerentes,
validadores e documento revisado permitem authorized. Rejeição de negócio é
separada; não persistir corpo/descrição bruta de erro. Consulta não reenvia POST.

XML/gzip até1MiB descomprimido; resposta2MiB; profundidade/elementos limitados, sem
DTD/entities externas. Dinheiro decimal exato. Não inferir tomador, local, serviço,
competência, regime ou série/número da DPS pelo pagamento/UUID. Campo faltante
retorna pendência. Identificadores/formato wire locais precisam de conferência
com Swagger/XSD oficiais antes de disponibilizar no produto.

SchemaValidator usa pacote offline revisado/hash, importações contidas e xmllint
--nonet em stdin, catálogo externo desligado e timeout5s. Dependência do executor
precisa existir na API antes de habilitar. Nenhum arquivo XSD oficial foi versionado
sem coleta/verificação. Não contornar403 nem baixar schemas durante Submit.

Teste integrado: NFSE_TEST_DATABASE_URL em banco descartável accounting_test*.
Se compartilhar banco com ACCOUNTING_TEST_DATABASE_URL, executar go test -p 1;
fixtures reaplicam DDL e não podem concorrer entre pacotes. Local fake HTTP +
PostgreSQL comprovam motor/replay/restart/timeout/artefatos; não são homologação.
XSD sintético via TestOfflineSchemaValidatorWithSyntheticSchema, com xmllint
local. Não usar credenciais/dados/nota real, navegador, build completo ou deploy.

## CT-15 pacote 7 — XSD oficial e produtor de DPS (2026-11-06)

XSD oficial 1.01 coletado por download direto (HTTP200) e versionado sem alteração em
docs/accounting/nfse/xsd/1.01 com manifesto de hashes (pacote e arquivos). Swagger segue
403; não houve contorno. `schema.go` aceita somente o DOCTYPE do xmldsig W3C oficial,
fixado por hash; qualquer outra diretiva continua proibida (teste de XXE).
`dps.go` BuildDPS/DPSPending: DPS 1.01 sem assinatura, determinística, a partir de
códigos explícitos (tpAmb, opSimpNac, regEspTrib, tribISSQN, tpRetISSQN, cTribNac,
municípios IBGE, totTrib); ausência/formato inválido → pendência, nunca default.
Identificador DPS vem do sequenciador canônico (não implementado); composição não
confirmada em fonte oficial. Texto fora de Latin-1 vira pendência (padrão TSString).
Achado: TSSerieDPS usa `^...$`, literais em XSD; xmllint rejeita qualquer série no
pacote original. Teste prova que é a única reprovação e valida o restante numa cópia
temporária sem as âncoras. Confirmar comportamento da SEFIN na homologação.
xmllint não existe na imagem da API nem na de ferramentas: validação offline em runtime
exige incluir libxml2-utils (decisão de build). Produtor não está registrado no runtime.
Assinatura XMLDSig não implementada: requer biblioteca de canonicalização qualificada
(decisão de dependência) e cofre de certificado; não usar C14N caseiro.
